// Package process 实现"数据结构化处理"用例。
//
// 审核通过后自动触发：LLM 提取 title/summary/tags → 更新 DataItem → 向量化 → 存向量库。
// 这是数据质量闭环的核心环节。
package process

import (
	"context"
	"encoding/json"
	"fmt"
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
	}
}

// ProcessItem 处理单条数据项：
// 1. LLM 提取结构化字段（title/summary/tags）
// 2. 更新 DataItem
// 3. 向量化并存入向量库
func (uc *ProcessUseCase) ProcessItem(ctx context.Context, itemID string) error {
	// 1. 读取数据项
	item, err := uc.dataItemRepo.FindByID(ctx, itemID)
	if err != nil {
		return fmt.Errorf("find item: %w", err)
	}

	// 如果已经有 title 和 summary（之前 LLM 已处理过），跳过结构化
	if item.Title != "" && item.Summary != "" && len(item.Tags) > 0 {
		// 直接到向量化步骤
		return uc.vectorize(ctx, item)
	}

	// 2. LLM 提取结构化字段
	prompt := fmt.Sprintf(`请分析以下内容，提取结构化信息。返回 JSON 格式：
{"title":"简洁标题","summary":"一句话摘要","tags":["标签1","标签2"]}

内容：
%s`, truncate(item.Content+item.RawContent, 6000))

	resp, err := uc.ai.ChatStream(ctx, "", []port.ChatMessage{
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
