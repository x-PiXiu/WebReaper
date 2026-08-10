// Package process 实现"数据结构化处理"用例。
//
// 审核通过后自动触发：LLM 提取 title/summary/tags → 更新 DataItem → 向量化 → 存向量库。
// 这是数据质量闭环的核心环节。
package process

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ProcessUseCase 数据结构化处理用例。
type ProcessUseCase struct {
	dataItemRepo port.DataItemRepository
	ai           port.AIGenerator
	embedder     port.Embedder
	vectorStore  port.VectorStore
	tracer       port.Tracer
}

// 编译期断言：ProcessUseCase 实现 port.KnowledgeSearcher 和 port.ItemProcessor。
// 让 adapter/crawler 的 KnowledgeSearcher 工具、usecase/dataitem 的审核编排
// 都能依赖 port 接口而非 usecase 具体类型（依赖倒置）。
var _ port.KnowledgeSearcher = (*ProcessUseCase)(nil)
var _ port.ItemProcessor = (*ProcessUseCase)(nil)

func NewProcessUseCase(
	repo port.DataItemRepository,
	ai port.AIGenerator,
	embedder port.Embedder,
	vectorStore port.VectorStore,
) *ProcessUseCase {
	return &ProcessUseCase{
		dataItemRepo: repo,
		ai:           ai,
		embedder:     embedder,
		vectorStore:  vectorStore,
		tracer:       port.NewNopTracer(),
	}
}

// SetTracer 注入链路追踪器（可选，默认 NopTracer 不采集 trace）。
func (uc *ProcessUseCase) SetTracer(t port.Tracer) {
	if t != nil {
		uc.tracer = t
	}
}

// ProcessItem 处理单条数据项：
// 1. LLM 提取结构化字段（title/summary/tags）
// 2. 更新 DataItem
// 3. 向量化并存入向量库
func (uc *ProcessUseCase) ProcessItem(ctx context.Context, itemID string) error {
	ctx, span := uc.tracer.StartSpan(ctx, "process.item")
	defer span.End()
	span.SetAttribute("item_id", itemID)

	// 1. 读取数据项
	item, err := uc.dataItemRepo.FindByID(ctx, itemID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("find item: %w", err)
	}

	// 如果已经有 title 和 summary（之前 LLM 已处理过），跳过结构化
	if item.Title != "" && item.Summary != "" && len(item.Tags) > 0 {
		// 直接到向量化步骤
		return uc.vectorize(ctx, item)
	}

	// 2. LLM 提取结构化字段
	// 先 stripHTML 去标签再截断：避免 HTML 标签浪费 token，且避免截断到半个标签。
	raw := stripHTML(item.Content + item.RawContent)
	prompt := fmt.Sprintf(`请分析以下内容，提取结构化信息。返回 JSON 格式：
{"title":"简洁标题","summary":"一句话摘要","tags":["标签1","标签2"]}

内容：
%s`, truncate(raw, 6000))

	resp, err := uc.ai.ChatStream(ctx, "", "", []port.ChatMessage{
		{Role: "system", Content: "你是数据结构化助手。只返回 JSON，不要其他文字。"},
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		return fmt.Errorf("llm structure: %w", err)
	}

	// 3. 解析 LLM 输出
	var structured struct {
		Title   string   `json:"title"`
		Summary string   `json:"summary"`
		Tags    []string `json:"tags"`
	}
	// 尝试从回复中提取 JSON（LLM 可能包裹 markdown）
	jsonStr := extractJSON(resp)
	if err := json.Unmarshal([]byte(jsonStr), &structured); err != nil {
		// 解析失败不中断，用原始值
		structured.Title = item.Title
		structured.Summary = ""
		structured.Tags = []string{}
	}

	// 4. 更新 DataItem
	if structured.Title != "" {
		item.Title = structured.Title
	}
	if structured.Summary != "" {
		item.Summary = structured.Summary
	}
	if len(structured.Tags) > 0 {
		item.Tags = structured.Tags
	}
	item.UpdatedAt = time.Now()
	if err := uc.dataItemRepo.Save(ctx, item); err != nil {
		return fmt.Errorf("save structured item: %w", err)
	}

	// 5. 向量化并存储
	return uc.vectorize(ctx, item)
}

// vectorize 把 DataItem 内容向量化并存入向量库。
func (uc *ProcessUseCase) vectorize(ctx context.Context, item entity.DataItem) error {
	if !uc.vectorStore.IsAvailable() {
		return nil // 向量库不可用时跳过（降级）
	}

	// 向量化内容（用 title + summary + content 拼接）
	text := item.Title + "\n" + item.Summary + "\n" + truncate(item.Content, 2000)
	vec, err := uc.embedder.Embed(ctx, text)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}

	// 存入向量库
	metadata := map[string]string{
		"title":      item.Title,
		"source_url": item.SourceURL,
	}
	if len(item.Tags) > 0 {
		tagsJSON, _ := json.Marshal(item.Tags)
		metadata["tags"] = string(tagsJSON)
	}

	if err := uc.vectorStore.Store(ctx, item.ID, vec, metadata); err != nil {
		return fmt.Errorf("store vector: %w", err)
	}

	return nil
}

// SearchKnowledge 语义搜索已采集的知识（供 Agent 工具调用）。
func (uc *ProcessUseCase) SearchKnowledge(ctx context.Context, query string, topK int) ([]port.VectorSearchResult, error) {
	ctx, span := uc.tracer.StartSpan(ctx, "process.search_knowledge")
	defer span.End()
	span.SetAttribute("top_k", topK)

	if !uc.vectorStore.IsAvailable() {
		return nil, nil
	}
	if topK <= 0 {
		topK = 5
	}

	vec, err := uc.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	return uc.vectorStore.Search(ctx, vec, topK)
}

// truncate 截断字符串。
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// blockTagRe 匹配块级标签（开/闭）——这些标签边界应替换为空格，
// 避免相邻块级文本粘连（如 <li>Go</li><li>MySQL</li> → "GoMySQL" 而非 "Go MySQL"）。
var blockTagRe = regexp.MustCompile(`(?i)</?(p|div|li|br|tr|h[1-6]|hr|section|article|header|footer|td|th)\b[^>]*>`)

// scriptRe / styleRe 匹配 script/style 块（连内容一起移除，避免把 JS/CSS 当正文）。
// 注：Go regexp 是 RE2 语法，不支持反向引用 \1，故 script/style 分两条规则。
var scriptRe = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
var styleRe = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`)

// tagRe 匹配剩余的任何 HTML 标签（清理后直接移除）。
var tagRe = regexp.MustCompile(`<[^>]*>`)

// stripHTML 去除 HTML 标签，返回纯文本。
//
// 动机：采集到的 description/Content 常含 HTML（如 arbeitnow 的 <p>...</p>），
// 直接喂 LLM 会浪费 token（标签是无意义噪音）。
//
// 设计（按需清洗，保留原始数据）：
//   - 仅在"喂 LLM"前清洗，不修改 DataItem 的 Content/RawContent（原始数据完整保留备查）。
//   - 三步处理：① 移除 script/style 块 ② 块级标签边界转空格 ③ 移除剩余标签 ④ 解码 HTML 实体。
//   - 块级标签边界补空格，避免 "GoMySQL" 粘连影响 LLM 理解。
//   - 无标签符号的纯文本直接返回，跳过正则开销。
//
// 放 usecase 层合法：清洗是"为 AI 加工准备输入"的应用逻辑，非框架细节。
//
// 注：用正则而非 goquery 遍历——goquery 的 Selection.Contents 对文本节点提取
// 在不同 HTML 结构下表现不一致，正则方案更稳定可控，对 LLM 输入完全够用。
func stripHTML(s string) string {
	if !strings.ContainsAny(s, "<>") {
		// 无标签符号，纯文本，跳过解析开销
		return s
	}
	// ① 移除 script/style 块（连内容）
	s = scriptRe.ReplaceAllString(s, " ")
	s = styleRe.ReplaceAllString(s, " ")
	// ② 块级标签边界转空格（关键：避免粘连）
	s = blockTagRe.ReplaceAllString(s, " ")
	// ③ 移除剩余所有标签
	s = tagRe.ReplaceAllString(s, "")
	// ④ 解码 HTML 实体（&amp; → & 等）
	s = html.UnescapeString(s)
	// 压缩多余空白为单个空格
	return strings.Join(strings.Fields(s), " ")
}

// extractJSON 从可能含 markdown 包裹的文本中提取 JSON 对象。
func extractJSON(s string) string {
	// 去除 ```json ... ``` 包裹
	start := indexOf(s, "{")
	if start < 0 {
		return s
	}
	end := lastIndexOf(s, "}")
	if end < 0 || end <= start {
		return s
	}
	return s[start : end+1]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func lastIndexOf(s, sub string) int {
	for i := len(s) - len(sub); i >= 0; i-- {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
