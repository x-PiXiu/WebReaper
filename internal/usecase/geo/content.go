package geo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ============ 内容优化用例 ============

// ContentUseCase 编排内容优化：LLM 改写 + GEO 评分。
type ContentUseCase struct {
	aiGen      port.AIGenerator     // 复用现有 AI 生成器（调 LLM）
	scorer     port.GEOScorer       // GEO 评分
	contentRepo port.OptimizedContentRepository
}

func NewContentUseCase(ai port.AIGenerator, sc port.GEOScorer, cr port.OptimizedContentRepository) *ContentUseCase {
	return &ContentUseCase{aiGen: ai, scorer: sc, contentRepo: cr}
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
}

// Optimize 执行内容优化：LLM 改写 → GEO 评分 → 存版本。
// 支持单关键词（Keyword）或多关键词组合（Keywords）两种模式。
func (uc *ContentUseCase) Optimize(ctx context.Context, in OptimizeInput) (entity.OptimizedContent, error) {
	if in.OriginalText == "" {
		return entity.OptimizedContent{}, fmt.Errorf("原始内容不能为空")
	}

	// 统一关键词描述：多关键词组合时拼接
	keywordDesc := in.Keyword
	if len(in.Keywords) > 0 {
		keywordDesc = strings.Join(in.Keywords, "、")
	}

	systemPrompt := `你是一个 GEO（生成式引擎优化）内容优化专家。
目标：把给定内容优化得更可能被 AI 搜索引擎引用。
优化方向：
1. 增强权威性：补充具体数据、案例、资质（不可编造）
2. 增强具体性：用数字、细节、可验证信息替代模糊表述
3. 结构化：使用标题层级、列表、FAQ 格式
4. 自然融入关键词：避免堆砌
5. 保持真实性：绝不编造虚假信息

只输出优化后的内容，不要解释。`

	userPrompt := fmt.Sprintf("目标关键词：%s\n\n原始内容：\n%s", keywordDesc, in.OriginalText)

	messages := []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	convID := fmt.Sprintf("content-opt-%d", time.Now().UnixNano())
	optimized, err := uc.aiGen.ChatStream(ctx, convID, in.LLMConfigName, messages, nil)
	if err != nil {
		return entity.OptimizedContent{}, fmt.Errorf("优化失败: %w", err)
	}
	optimized = strings.TrimSpace(optimized)

	// GEO 评分（用第一个关键词或单关键词评分）
	scoreKeyword := in.Keyword
	if len(in.Keywords) > 0 {
		scoreKeyword = in.Keywords[0]
	}
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
		OriginalText:  in.OriginalText,
		OptimizedText: optimized,
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

// List 列出品牌的优化内容。
func (uc *ContentUseCase) List(ctx context.Context, tenantID, brandID string) ([]entity.OptimizedContent, error) {
	return uc.contentRepo.ListByBrand(ctx, tenantID, brandID)
}

// GenerateInput 从零生成内容的输入（不需要原始素材，AI 根据品牌+关键词原创）。
type GenerateInput struct {
	TenantID      string
	BrandID       string
	Keywords      []string // 一个或多个关键词（组合模式）
	BrandInfo     string   // 品牌定位/卖点摘要（供 LLM 参考，让内容贴合品牌）
	LLMConfigName string
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

只输出文章正文（含标题），不要解释。`

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
	content, err := uc.aiGen.ChatStream(ctx, convID, in.LLMConfigName, messages, onDelta)
	if err != nil {
		return entity.OptimizedContent{}, fmt.Errorf("生成内容失败: %w", err)
	}
	// 双重保障：即使流式过滤有遗漏，最终结果再过滤一次
	content = pkg.StripThinkTags(content)
	content = strings.TrimSpace(content)

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
