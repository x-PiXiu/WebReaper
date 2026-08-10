package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ============ 内容优化用例 ============

// ContentUseCase 编排内容优化：LLM 改写 + GEO 评分。
//
// 评分分工（免费快筛 + 深度评审 分层思想）：
//   - ruleScorer（免费规则评分）：优化前后对比用，零成本、可测。
//   - scorer（默认 LLM 深评）：最终落库的 Score 用，烧 token 但维度更深。
//     两套都是 port.GEOScorer，经 SetRuleScorer 注入，未注入时降级为同 scorer。
//
// 收录通知（发布副作用）：
//   - urlSubmitter（IndexNow 等）：内容发布为 published 时自动通知搜索引擎收录。
//   - 经 SetURLSubmitter 注入；未注入（未配 Key）时静默跳过，发布流程不受影响。
type ContentUseCase struct {
	aiGen         port.AIGenerator // 复用现有 AI 生成器（调 LLM）
	scorer        port.GEOScorer   // GEO 评分（默认 LLM 深评，落库用）
	ruleScorer    port.GEOScorer   // 免费规则评分（前后对比用，可注入）
	contentRepo   port.OptimizedContentRepository
	urlSubmitter  port.URLSubmitter // 收录通知（可选）
	publicBaseURL string            // 公开站根地址（拼收录 URL 用）
	logger        port.Logger       // 日志（标题兜底等告警用）
}

func NewContentUseCase(ai port.AIGenerator, sc port.GEOScorer, cr port.OptimizedContentRepository) *ContentUseCase {
	return &ContentUseCase{aiGen: ai, scorer: sc, contentRepo: cr, ruleScorer: sc, logger: port.NopLogger{}}
}

// SetLogger 注入日志（可选；标题兜底等告警输出）。
func (uc *ContentUseCase) SetLogger(l port.Logger) {
	if l != nil {
		uc.logger = l
	}
}

// SetRuleScorer 注入免费规则评分器（前后对比用）。
// 未注入时 ruleScorer 降级为 scorer（与 LLM 深评同源，行为不变）。
func (uc *ContentUseCase) SetRuleScorer(s port.GEOScorer) {
	if s != nil {
		uc.ruleScorer = s
	}
}

// SetURLSubmitter 注入收录通知器（内容发布为 published 时自动通知搜索引擎）。
// 未注入（未配 INDEXNOW_KEY）时静默跳过。
func (uc *ContentUseCase) SetURLSubmitter(s port.URLSubmitter) {
	uc.urlSubmitter = s
}

// SetPublicBaseURL 注入公开站根地址（拼收录通知的 URL）。
func (uc *ContentUseCase) SetPublicBaseURL(baseURL string) {
	uc.publicBaseURL = baseURL
}

// ---- 生成器调用（结构化输出增强）----

// generatedArticle 结构化输出契约：标题 + 正文（引擎强制 JSON，标题零解析成本）。
type generatedArticle struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// callGenerator 调用 AI 生成器，优先使用结构化输出（OptionsAwareGenerator 可选接口）。
//
// 策略（开闭原则 + 渐进增强）：
//   - onDelta != nil（流式路径）：保持纯文本——json 模式下流式增量是 JSON 片段，
//     会污染前端 SSE 渲染；流式走提示词约束 + StripThinkTags。
//   - onDelta == nil（非流式）：尝试 json 结构化输出（引擎强制 {title, content}）：
//     解析成功 → 标题/正文零解析成本；失败 → 降级为原始输出
//     （提示词硬约束路径，ExtractTitle 兜底）——支持方受益、不支持方行为不变。
func (uc *ContentUseCase) callGenerator(ctx context.Context, convID, llmConfigName string, messages []port.ChatMessage, onDelta func(string)) (string, error) {
	if onDelta != nil {
		return uc.aiGen.ChatStream(ctx, convID, llmConfigName, messages, onDelta)
	}

	gen, ok := uc.aiGen.(port.OptionsAwareGenerator)
	if !ok {
		return uc.aiGen.ChatStream(ctx, convID, llmConfigName, messages, nil)
	}

	out, err := gen.ChatStreamWithOptions(ctx, port.ChatStreamInput{
		ConversationID: convID,
		LLMConfigName:  llmConfigName,
		Messages:       messages,
		Options: port.ChatOptions{
			ResponseFormat:    "json",
			SchemaExample:     &generatedArticle{},
			SchemaDescription: "文章标题与正文（title=标题，content=正文）",
			DisableThinking:   true, // 请求层关闭思考（适配器按厂商映射，不支持则提示词兜底）
		},
	})
	if err != nil {
		return "", err
	}
	// 解析结构化 JSON；失败降级为原始输出（提示词硬约束仍保证格式，ExtractTitle 兜底）
	var art generatedArticle
	if jsonBlock := extractJSONFromOutput(out); json.Unmarshal([]byte(jsonBlock), &art) == nil && art.Content != "" {
		title := strings.TrimSpace(art.Title)
		if title != "" {
			return "# " + title + "\n\n" + art.Content, nil
		}
		return art.Content, nil
	}
	return out, nil
}

// extractJSONFromOutput 从生成输出中提取 JSON 块（引擎强制 JSON 时一般直接可用；
// 个别模型带 ```json 包裹时去包裹）。返回空串表示未找到。
func extractJSONFromOutput(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			return strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	return s
}

// OptimizeInput 内容优化的输入。
type OptimizeInput struct {
	TenantID      string
	BrandID       string
	KeywordID     string   // 单关键词模式的 keywordID（兼容旧接口）
	OriginalText  string   // 原始素材（Optimize 模式用；Generate 模式可空）
	Keyword       string   // 单关键词（兼容旧接口）
	Keywords      []string // 多关键词组合模式（灵活模式：多个词合成一篇）
	LLMConfigName string   // 用哪个 LLM 优化（空则 default）
	TargetEngine  string   // 目标 AI 引擎偏好（chatgpt/perplexity/kimi/doubao；空=通用）
}

// OptimizeResult 是 Optimize 的返回结果：优化产物 + 前后对比反馈。
//
// 设计动机（量化闭环）：
//
//	优化是否有效不能靠感觉——ScoreBefore/ScoreAfter 的差值 + Recommendations
//	让"优化-评分-反馈"形成可验证的闭环（借鉴 geo-optimizer 的 Compare + 建议生成，
//	但用免费规则评分做对比，不额外烧 token）。
type OptimizeResult struct {
	Content         entity.OptimizedContent // 优化产物（含 Score=优化后分数）
	ScoreBefore     entity.GEOScore         // 优化前的规则评分（免费快筛）
	Recommendations []string                // 基于优化前低分维度的改进建议
}

// Optimize 执行内容优化：规则评分原文 → LLM 改写 → 评分 → 存版本 → 返回前后对比。
// 支持单关键词（Keyword）或多关键词组合（Keywords）两种模式。
func (uc *ContentUseCase) Optimize(ctx context.Context, in OptimizeInput) (OptimizeResult, error) {
	if in.OriginalText == "" {
		return OptimizeResult{}, fmt.Errorf("原始内容不能为空")
	}

	// 统一关键词描述：多关键词组合时拼接
	keywordDesc := in.Keyword
	if len(in.Keywords) > 0 {
		keywordDesc = strings.Join(in.Keywords, "、")
	}

	// 优化前评分（规则快筛，免费；失败不阻断——对比是增强项）
	scoreKeyword := in.Keyword
	if len(in.Keywords) > 0 {
		scoreKeyword = in.Keywords[0]
	}
	scoreBefore, sErr := uc.ruleScorer.Score(ctx, in.OriginalText, scoreKeyword)
	if sErr != nil {
		scoreBefore = entity.GEOScore{}
	}

	// 标题：优先取原文的 markdown 标题；没有则用关键词兜底
	title := pkg.ExtractTitle(in.OriginalText)
	if len(title) < 4 {
		title = keywordDesc
	}

	systemPrompt := `你是一个 GEO（生成式内容优化）专家。
目标：把给定内容优化得更可能被 AI 搜索引擎引用。
优化方向：
1. 增强权威性：补充具体数据、案例、资质（不可编造）
2. 增强具体性：用数字、细节、可验证信息替代模糊表述
3. 结构化：使用标题层级、列表、FAQ 格式
4. 自然融入关键词：避免堆砌
5. 保持真实性：绝不编造虚假信息

` + entity.BuildEnginePrefInstruction(in.TargetEngine) + `

硬性要求：
- 严禁输出任何思考过程、推理过程或 <think> 内容——只输出最终文章
- 第一行必须输出标题，以 # 开头（如 # 北京装修公司哪家好？10 年老牌真实对比）
- 只输出优化后的内容，不要解释`

	userPrompt := fmt.Sprintf("目标关键词：%s\n\n原始内容：\n%s", keywordDesc, in.OriginalText)

	messages := []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	convID := fmt.Sprintf("content-opt-%d", time.Now().UnixNano())
	// 用量计量上下文（经济系统基础）：租户 + 场景
	ctx = port.WithUsageContext(ctx, in.TenantID, "content-opt")
	optimized, err := uc.callGenerator(ctx, convID, in.LLMConfigName, messages, nil)
	if err != nil {
		return OptimizeResult{}, fmt.Errorf("优化失败: %w", err)
	}
	optimized = strings.TrimSpace(optimized)
	// 过滤模型推理过程的 think 标签（MiniMax 等推理模型输出 think 块，
	// 必须过滤后再存库/展示——否则会泄漏到公开站、llms.txt、发布内容）
	optimized = pkg.StripThinkTags(optimized)
	optimized = strings.TrimSpace(optimized)

	// GEO 评分（用第一个关键词或单关键词评分）
	score, sErr := uc.scorer.Score(ctx, optimized, scoreKeyword)
	if sErr != nil {
		score = entity.GEOScore{}
	}

	// 查当前最大版本号，递增
	maxVer, _ := uc.contentRepo.FindMaxVersion(ctx, in.TenantID, in.BrandID, in.KeywordID)

	now := time.Now()
	oc := entity.OptimizedContent{
		ID:            fmt.Sprintf("oc-%d", now.UnixNano()),
		TenantID:      in.TenantID,
		BrandID:       in.BrandID,
		KeywordID:     in.KeywordID,
		Title:         title,
		OriginalText:  in.OriginalText,
		OptimizedText: optimized,
		Version:       maxVer + 1,
		Score:         score,
		Status:        "draft",
		CreatedAt:     now,
	}
	if err := uc.contentRepo.Save(ctx, oc); err != nil {
		return OptimizeResult{}, fmt.Errorf("save content: %w", err)
	}
	return OptimizeResult{
		Content:         oc,
		ScoreBefore:     scoreBefore,
		Recommendations: generateRecommendations(scoreBefore),
	}, nil
}

// generateRecommendations 基于规则评分生成改进建议（纯函数，可单测）。
// 借鉴 geo-optimizer：任一维度低于 60 分就给出对应建议；全达标则提示保持。
func generateRecommendations(s entity.GEOScore) []string {
	var recs []string
	if s.Authority < 60 {
		recs = append(recs, "权威性偏低：补充具体数据、官方来源、资质证书或案例（不可编造）")
	}
	if s.Specificity < 60 {
		recs = append(recs, "具体性偏低：用数字、型号、日期等可验证细节替代模糊表述")
	}
	if s.Structure < 60 {
		recs = append(recs, "结构化不足：增加标题层级、列表和分段，方便 AI 抓取语义块")
	}
	if s.Uniqueness < 60 {
		recs = append(recs, "独特性不足：补充与竞品的差异化对比或独家观点")
	}
	if s.Recency < 60 {
		recs = append(recs, "时效性不足：补充最新年份的数据或近期动态")
	}
	if len(recs) == 0 {
		recs = append(recs, "各维度表现良好，可保持当前内容质量")
	}
	return recs
}

// List 列出品牌的优化内容。
func (uc *ContentUseCase) List(ctx context.Context, tenantID, brandID string) ([]entity.OptimizedContent, error) {
	return uc.contentRepo.ListByBrand(ctx, tenantID, brandID)
}

// ListAll 全平台内容列表（admin 旁路——仅管理后台全局管理端点调用；
// 可按状态过滤。商户上下文一律走 List，租户隔离）。
func (uc *ContentUseCase) ListAll(ctx context.Context, status string, limit int) ([]entity.OptimizedContent, error) {
	return uc.contentRepo.ListAll(ctx, status, limit)
}

// AdminSetStatus 全平台内容状态流转（admin 旁路——管理后台上下架控制，
// 不做租户校验，由 admin 路由守卫保护。复用发布副作用：published 触发收录通知）。
func (uc *ContentUseCase) AdminSetStatus(ctx context.Context, contentID, status string) (entity.OptimizedContent, error) {
	switch status {
	case "draft", "published":
	default:
		return entity.OptimizedContent{}, fmt.Errorf("不支持的状态: %q（仅支持 draft/published）", status)
	}
	oc, err := uc.contentRepo.FindPublishedByID(ctx, contentID)
	if err != nil {
		// FindPublishedByID 只查 published——draft 内容需走通用查询
		oc, err = uc.findAnyByID(ctx, contentID)
		if err != nil {
			return entity.OptimizedContent{}, err
		}
	}
	if oc.Status == status {
		return oc, nil // 幂等
	}
	oc.Status = status
	if err := uc.contentRepo.Save(ctx, oc); err != nil {
		return entity.OptimizedContent{}, fmt.Errorf("save status: %w", err)
	}
	// 发布副作用：通知搜索引擎收录（与商户端 SetStatus 同口径）
	if status == "published" && uc.urlSubmitter != nil && uc.publicBaseURL != "" {
		publicURL := strings.TrimRight(uc.publicBaseURL, "/") + "/public/articles/" + oc.ID
		_ = uc.urlSubmitter.SubmitURLs(ctx, []string{publicURL})
	}
	return oc, nil
}

// findAnyByID 无条件按 ID 查内容（admin 旁路辅助：FindPublishedByID 之外的全状态查询）。
func (uc *ContentUseCase) findAnyByID(ctx context.Context, contentID string) (entity.OptimizedContent, error) {
	return uc.contentRepo.FindByID(ctx, "", contentID)
}

// AdminDelete 全平台内容删除（admin 旁路——管理后台绝对控制，不做租户校验）。
func (uc *ContentUseCase) AdminDelete(ctx context.Context, contentID string) error {
	return uc.contentRepo.Delete(ctx, "", contentID)
}

// Delete 删除优化内容（内容工作台/管理后台用）。
// 先 FindByID 做租户校验（只允许删自己租户的内容），再物理删除。
func (uc *ContentUseCase) Delete(ctx context.Context, tenantID, contentID string) error {
	if _, err := uc.contentRepo.FindByID(ctx, tenantID, contentID); err != nil {
		return err
	}
	return uc.contentRepo.Delete(ctx, tenantID, contentID)
}

// SetStatus 内容状态流转（draft → published 发布到公开站 / published → draft 下线）。
//
// 状态语义：
//   - draft：草稿，仅商户端可见，公网不可访问
//   - published：已发布，公开站点（/public/articles/:id）对 AI 引擎/搜索引擎可见
//   - approved：保留给未来审核流（当前不可直接设置）
//
// 通过 FindByID 做租户校验——只能流转自己租户的内容。
func (uc *ContentUseCase) SetStatus(ctx context.Context, tenantID, contentID, status string) (entity.OptimizedContent, error) {
	switch status {
	case "draft", "published":
	default:
		return entity.OptimizedContent{}, fmt.Errorf("不支持的状态: %q（仅支持 draft/published）", status)
	}

	oc, err := uc.contentRepo.FindByID(ctx, tenantID, contentID)
	if err != nil {
		return entity.OptimizedContent{}, err
	}
	if oc.Status == status {
		return oc, nil // 幂等：已是目标状态直接返回
	}

	oc.Status = status
	if err := uc.contentRepo.Save(ctx, oc); err != nil {
		return entity.OptimizedContent{}, fmt.Errorf("save status: %w", err)
	}

	// 发布副作用：通知搜索引擎收录（IndexNow，尽力而为——失败不影响发布）
	if status == "published" && uc.urlSubmitter != nil && uc.publicBaseURL != "" {
		publicURL := strings.TrimRight(uc.publicBaseURL, "/") + "/public/articles/" + oc.ID
		_ = uc.urlSubmitter.SubmitURLs(ctx, []string{publicURL})
	}
	return oc, nil
}

// GenerateInput 从零生成内容的输入（不需要原始素材，AI 根据品牌+关键词原创）。
type GenerateInput struct {
	TenantID      string
	BrandID       string
	Keywords      []string // 一个或多个关键词（组合模式）
	BrandInfo     string   // 品牌定位/卖点摘要（供 LLM 参考，让内容贴合品牌）
	LLMConfigName string
	TargetEngine  string // 目标 AI 引擎偏好（chatgpt/perplexity/kimi/doubao；空=通用）
}

// Generate 从零生成内容：根据品牌信息 + 关键词，AI 原创一篇 GEO 优化文章。
// 支持单关键词（一个词一篇文章）或多关键词组合（多个词合成一篇深度文）。
// 这是"关键词→内容"闭环的核心——关键词确定后，直接生成可发布的内容。
func (uc *ContentUseCase) Generate(ctx context.Context, in GenerateInput) (entity.OptimizedContent, error) {
	return uc.GenerateStream(ctx, in, nil) // 非流式 = 流式但不回调
}

// GenerateStream 流式生成内容。
// onDelta 回调实时推送正文增量（供 SSE 流式输出用）；传 nil 则等全部生成完返回。
// 只推正文 content，不推思考过程（ChatStream 的 onDelta 本身只推 content delta）。
func (uc *ContentUseCase) GenerateStream(ctx context.Context, in GenerateInput, onDelta func(delta string)) (entity.OptimizedContent, error) {
	if len(in.Keywords) == 0 {
		return entity.OptimizedContent{}, fmt.Errorf("关键词不能为空")
	}
	keywordDesc := strings.Join(in.Keywords, "、")
	isMulti := len(in.Keywords) > 1

	systemPrompt := `你是一个 GEO（生成式引擎优化）内容创作专家。
目标：根据品牌信息和关键词，创作一篇容易被 AI 搜索引擎引用的高质量文章。
要求：
1. 围绕关键词展开，自然融入（不堆砌）
2. 结构化：标题层级清晰、有列表/小标题
3. 有权威性：包含具体观点、方法论、可操作建议
4. 真实可信：不编造数据，基于品牌真实信息创作
5. 字数 800-1500 字

` + entity.BuildEnginePrefInstruction(in.TargetEngine) + `

硬性要求：
- 严禁输出任何思考过程、推理过程或 <think> 内容——只输出最终文章
- 第一行必须输出标题，以 # 开头（如 # 北京装修公司哪家好？10 年老牌真实对比）
- 只输出文章正文（含标题），不要解释`

	modeHint := "围绕这个关键词创作一篇文章"
	if isMulti {
		modeHint = fmt.Sprintf("把这 %d 个关键词有机融合到一篇文章中（各有侧重、逻辑连贯）", len(in.Keywords))
	}
	userPrompt := fmt.Sprintf(`品牌信息：%s
目标关键词：%s

请%s。`, in.BrandInfo, keywordDesc, modeHint)

	messages := []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	convID := fmt.Sprintf("content-gen-%d", time.Now().UnixNano())
	// 用量计量上下文（经济系统基础）：租户 + 场景
	ctx = port.WithUsageContext(ctx, in.TenantID, "content-gen")
	content, err := uc.callGenerator(ctx, convID, in.LLMConfigName, messages, onDelta)
	if err != nil {
		return entity.OptimizedContent{}, fmt.Errorf("生成内容失败: %w", err)
	}
	// 双重保障：即使流式过滤有遗漏，最终结果再过滤一次
	content = pkg.StripThinkTags(content)
	content = strings.TrimSpace(content)

	// 提取标题：AI 生成的文章通常以 markdown 标题开头，取首个标题行作发布标题。
	// 标题非空校验（P0）：LLM 未按格式输出标题时用关键词兜底，保证发布字段完整。
	title := pkg.ExtractTitle(content)
	if len(title) < 4 {
		title = keywordDesc // 兜底：用关键词作标题
		uc.logger.Warn("生成内容缺少标题，已用关键词兜底",
			port.String("brand", in.BrandID), port.String("keyword", keywordDesc))
	}

	// GEO 评分
	score, sErr := uc.scorer.Score(ctx, content, in.Keywords[0])
	if sErr != nil {
		score = entity.GEOScore{}
	}

	// 查当前最大版本号（生成模式不绑定 keywordID，按 brandID 查）
	maxVer, _ := uc.contentRepo.FindMaxVersion(ctx, in.TenantID, in.BrandID, "")

	now := time.Now()
	oc := entity.OptimizedContent{
		ID:            fmt.Sprintf("oc-%d", now.UnixNano()),
		TenantID:      in.TenantID,
		BrandID:       in.BrandID,
		KeywordID:     "", // 生成模式可能对应多关键词，不绑定单个 keywordID
		Title:         title,
		OriginalText:  "[AI原创生成]",
		OptimizedText: content,
		Version:       maxVer + 1,
		Score:         score,
		Status:        "draft",
		CreatedAt:     now,
	}
	if err := uc.contentRepo.Save(ctx, oc); err != nil {
		return entity.OptimizedContent{}, fmt.Errorf("save content: %w", err)
	}
	return oc, nil
}
