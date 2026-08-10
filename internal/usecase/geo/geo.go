// Package geo 实现 GEO（生成式引擎优化）核心用例。
//
// 用例编排（应用级业务规则），只依赖 port 接口和 domain 实体，不依赖 LLM/GORM 等框架。
//
// 核心用例：
//   - BrandUseCase：品牌资产 CRUD（商户管理自己的品牌）
//   - MonitorUseCase：AI 引擎监测（闭环起点，采样问 LLM 解析提及）
//   - RankUseCase：排行榜（聚合监测结果，品牌 vs 竞品）
//   - ContentUseCase：内容优化（LLM 改写 + GEO 评分）
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

// ============ 品牌管理用例 ============

// BrandUseCase 品牌资产 CRUD + 关键词生成。
type BrandUseCase struct {
	brandRepo   port.BrandRepository
	keywordRepo port.KeywordRepository
	aiGen       port.AIGenerator  // 可选：关键词生成用
	webSearch   port.WebSearcher  // 可选：全网搜索（RAG 增强关键词发现）
}

// WebSearcher 全网搜索抽象（用例层声明，适配器实现）。
// 让关键词生成能结合"全网在搜什么"，而非只凭品牌信息拍脑袋。
type WebSearcher interface {
	// SearchByBrand 根据品牌信息搜索全网相关内容摘要，返回供 LLM 参考的上下文。
	SearchByBrand(ctx context.Context, brandName, positioning string, competitors []string) (string, error)
}

func NewBrandUseCase(br port.BrandRepository, kr port.KeywordRepository) *BrandUseCase {
	return &BrandUseCase{brandRepo: br, keywordRepo: kr}
}

// SetAIGenerator 注入 AI 生成器（供关键词生成用；未注入则 GenerateKeywords 报错降级）。
func (uc *BrandUseCase) SetAIGenerator(ai port.AIGenerator) {
	uc.aiGen = ai
}

// SetWebSearcher 注入全网搜索器（RAG 增强关键词发现；未注入则纯 LLM 推断）。
func (uc *BrandUseCase) SetWebSearcher(ws port.WebSearcher) {
	uc.webSearch = ws
}

// BrandInput 创建/更新品牌的输入。
type BrandInput struct {
	Name        string
	Positioning string
	CoreSelling []string
	Competitors []string
}

// Create 创建品牌。
func (uc *BrandUseCase) Create(ctx context.Context, tenantID string, in BrandInput) (entity.Brand, error) {
	if tenantID == "" {
		return entity.Brand{}, fmt.Errorf("tenant_id 不能为空")
	}
	now := time.Now()
	b := entity.Brand{
		ID:          fmt.Sprintf("brand-%d", now.UnixNano()),
		TenantID:    tenantID,
		Name:        in.Name,
		Positioning: in.Positioning,
		CoreSelling: in.CoreSelling,
		Competitors: in.Competitors,
		CreatedAt:   now,
	}
	if !b.IsValid() {
		return entity.Brand{}, fmt.Errorf("品牌无效：name 不能为空")
	}
	if err := uc.brandRepo.Save(ctx, b); err != nil {
		return entity.Brand{}, fmt.Errorf("save brand: %w", err)
	}
	return b, nil
}

// List 列出租户的全部品牌。
func (uc *BrandUseCase) List(ctx context.Context, tenantID string) ([]entity.Brand, error) {
	return uc.brandRepo.ListByTenant(ctx, tenantID)
}

// ListAll 全平台品牌列表（admin 旁路——仅管理后台全局管理端点调用）。
// 商户上下文一律走 List（租户隔离）；此处显式无租户过滤，由 admin 路由守卫保护。
func (uc *BrandUseCase) ListAll(ctx context.Context) ([]entity.Brand, error) {
	return uc.brandRepo.ListAll(ctx)
}

// AdminDelete 全平台品牌删除（admin 旁路——管理后台绝对控制，不做租户校验）。
// 级联清理其下关键词。
func (uc *BrandUseCase) AdminDelete(ctx context.Context, brandID string) error {
	kws, err := uc.keywordRepo.ListByBrand(ctx, "", brandID)
	if err == nil {
		for _, kw := range kws {
			_ = uc.keywordRepo.Delete(ctx, "", kw.ID)
		}
	}
	return uc.brandRepo.Delete(ctx, "", brandID)
}

// Delete 删除品牌（同时清理其下关键词）。
func (uc *BrandUseCase) Delete(ctx context.Context, tenantID, brandID string) error {
	// 先删关键词（简化：逐个删；生产可批量）
	kws, err := uc.keywordRepo.ListByBrand(ctx, tenantID, brandID)
	if err == nil {
		for _, kw := range kws {
			_ = uc.keywordRepo.Delete(ctx, tenantID, kw.ID)
		}
	}
	return uc.brandRepo.Delete(ctx, tenantID, brandID)
}

// AddKeyword 给品牌添加关键词。
func (uc *BrandUseCase) AddKeyword(ctx context.Context, tenantID, brandID, term, intent string) (entity.Keyword, error) {
	now := time.Now()
	kw := entity.Keyword{
		ID:        fmt.Sprintf("kw-%d", now.UnixNano()),
		TenantID:  tenantID,
		BrandID:   brandID,
		Term:      term,
		Intent:    intent,
		CreatedAt: now,
	}
	if !kw.IsValid() {
		return entity.Keyword{}, fmt.Errorf("关键词无效")
	}
	if err := uc.keywordRepo.Save(ctx, kw); err != nil {
		return entity.Keyword{}, fmt.Errorf("save keyword: %w", err)
	}
	return kw, nil
}

// ListKeywords 列出品牌的关键词。
func (uc *BrandUseCase) ListKeywords(ctx context.Context, tenantID, brandID string) ([]entity.Keyword, error) {
	return uc.keywordRepo.ListByBrand(ctx, tenantID, brandID)
}

// ListAllKeywords 列出租户所有关键词（跨品牌，关键词管理页用）。
func (uc *BrandUseCase) ListAllKeywords(ctx context.Context, tenantID string) ([]entity.Keyword, error) {
	return uc.keywordRepo.ListByTenant(ctx, tenantID)
}

// DeleteKeyword 删除关键词。
func (uc *BrandUseCase) DeleteKeyword(ctx context.Context, tenantID, keywordID string) error {
	return uc.keywordRepo.Delete(ctx, tenantID, keywordID)
}

// GenerateKeywords 根据品牌定位/卖点/竞品 + 全网相关内容，生成最合适的候选关键词。
//
// 设计演进：
//   - 初版：纯 LLM 凭品牌信息推断（闭门造车，可能脱离真实搜索需求）
//   - 现在：RAG 增强——先爬全网看大家在搜/写什么，再结合品牌信息生成
//     这样产出的关键词更贴合真实用户搜索习惯，监测/优化才有意义
//
// 降级：webSearch 未注入或搜索失败时，退化为纯 LLM 推断（仍可用，只是不够准）。
func (uc *BrandUseCase) GenerateKeywords(ctx context.Context, tenantID, brandID string) ([]string, error) {
	if uc.aiGen == nil {
		return nil, fmt.Errorf("AI 生成器未配置")
	}
	brand, err := uc.brandRepo.FindByID(ctx, tenantID, brandID)
	if err != nil {
		return nil, fmt.Errorf("品牌不存在: %w", err)
	}

	sellingPoints := "无"
	if len(brand.CoreSelling) > 0 {
		sellingPoints = strings.Join(brand.CoreSelling, "、")
	}
	competitors := "无"
	if len(brand.Competitors) > 0 {
		competitors = strings.Join(brand.Competitors, "、")
	}

	// RAG 增强：爬全网看相关内容（可选，失败降级为纯 LLM 推断）
	webContext := ""
	if uc.webSearch != nil {
		if wc, e := uc.webSearch.SearchByBrand(ctx, brand.Name, brand.Positioning, brand.Competitors); e == nil && wc != "" {
			webContext = wc
		}
	}

	systemPrompt := "你是 GEO（生成式引擎优化）关键词专家。结合品牌信息和全网相关内容，生成用户在 AI 搜索引擎里最可能搜索的关键词。"
	userPrompt := fmt.Sprintf(`品牌名：%s
品牌定位：%s
核心卖点：%s
竞品：%s
`, brand.Name, brand.Positioning, sellingPoints, competitors)

	if webContext != "" {
		userPrompt += fmt.Sprintf(`
全网相关内容摘要（真实用户/作者在关注的话题）：
%s

请结合上述全网内容，生成 20 个最合适的候选关键词（贴合真实搜索需求，包含品牌词、行业热词、长尾问题词）。
`, truncateStr(webContext, 2000))
	} else {
		userPrompt += "\n请生成 20 个用户可能搜索的相关关键词（包含品牌词、行业词、长尾问题词）。"
	}
	userPrompt += "\n每行一个，不要编号，不要解释。"

	messages := []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	convID := fmt.Sprintf("kw-gen-%d", time.Now().UnixNano())
	resp, err := uc.aiGen.ChatStream(ctx, convID, "", messages, nil)
	if err != nil {
		return nil, fmt.Errorf("生成关键词失败: %w", err)
	}
	// 过滤 <think> 块后再解析关键词
	resp = pkg.StripThinkTags(resp)
	return parseKeywordLines(resp), nil
}

// parseKeywordLines 从 LLM 响应解析关键词列表（去编号/markdown/说明性文字）。
func parseKeywordLines(resp string) []string {
	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "```") {
		lines := strings.Split(resp, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			resp = strings.Join(lines, "\n")
		}
	}
	var keywords []string
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "0123456789.、)-* ")
		line = strings.Trim(line, "\"'`")
		if line == "" || len([]rune(line)) < 2 {
			continue
		}
		if (strings.Contains(line, "。") || strings.Contains(line, "？")) && len([]rune(line)) > 20 {
			continue
		}
		keywords = append(keywords, line)
	}
	return keywords
}

// truncateStr 截断字符串到指定字符数。
func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
