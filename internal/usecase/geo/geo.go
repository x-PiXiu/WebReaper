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
	"encoding/json"
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
	aiGen       port.AIGenerator // 可选：关键词生成用
	webSearch   port.WebSearcher // 可选：全网搜索（RAG 增强关键词发现）
	storeRepo   port.StoreLocationRepository // 可选：门店档案（本地意图关键词生成用）
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

// SetStoreRepo 注入门店档案仓储（可选；本地生活 P0 补全）。
// 注入后关键词生成自动附加门店位置（城市/区），产出"望京 川菜馆"类本地意图词——
// 实体餐饮的核心搜索入口。
func (uc *BrandUseCase) SetStoreRepo(r port.StoreLocationRepository) {
	if r != nil {
		uc.storeRepo = r
	}
}

// BrandInput 创建/更新品牌的输入。
type BrandInput struct {
	Name        string
	Positioning string
	CoreSelling []string
	Competitors []string
	BizType     string // local/online（空=local）
	WebsiteURL  string // 官网地址（online 品牌 NAP）
	Industry    string // 行业（F1-4：知识库检索/行业看板的过滤维度——此前链路断裂被静默丢弃）
}

// validateBrandInput 用例层校验（F1-1/F3-3：前端拦截是体验，这里是底线）。
// ① online 品牌官网必填（防绕过前端直调 API）；② 乱码/控制字符拒绝；③ 长度上限。
func validateBrandInput(in BrandInput, effectiveBizType string) error {
	if effectiveBizType == "online" && strings.TrimSpace(in.WebsiteURL) == "" {
		return fmt.Errorf("线上品牌必填官网地址（官网是 AI 引用你的核心信源）")
	}
	if s := strings.TrimSpace(in.WebsiteURL); s != "" && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return fmt.Errorf("官网地址必须以 http:// 或 https:// 开头")
	}
	if len([]rune(strings.TrimSpace(in.Name))) > 50 {
		return fmt.Errorf("品牌名过长（≤50 字）")
	}
	if len([]rune(in.Positioning)) > 200 {
		return fmt.Errorf("品牌定位过长（≤200 字）")
	}
	if len([]rune(in.Industry)) > 20 {
		return fmt.Errorf("行业名过长（≤20 字）")
	}
	for _, field := range []string{in.Name, in.Positioning, in.WebsiteURL, in.Industry} {
		if hasIllegalChars(field) {
			return fmt.Errorf("输入包含非法字符（乱码/控制字符），请检查后重试")
		}
	}
	for _, list := range [][]string{in.CoreSelling, in.Competitors} {
		for _, item := range list {
			if len([]rune(item)) > 30 {
				return fmt.Errorf("卖点/竞品单项过长（≤30 字）：%s", item)
			}
			if hasIllegalChars(item) {
				return fmt.Errorf("输入包含非法字符（乱码/控制字符），请检查后重试")
			}
		}
	}
	return nil
}

// hasIllegalChars 检测乱码替换符与控制字符（F3-3：历史曾因此入库乱码定位字段）。
func hasIllegalChars(s string) bool {
	return strings.ContainsAny(s, "\uFFFD") || strings.ContainsFunc(s, func(r rune) bool {
		return r < 0x09 || (r > 0x0D && r < 0x20)
	})
}

// sanitizeBrandText 净化文本字段（trim；不做截断——长度由校验拦截）。
func sanitizeBrandText(s string) string {
	return strings.TrimSpace(s)
}

// sanitizeBrandSlice 净化列表字段：trim、去空、单项截断到 maxLen、限制条数。
func sanitizeBrandSlice(items []string, maxLen int, maxCount int) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		if r := []rune(it); len(r) > maxLen {
			it = string(r[:maxLen])
		}
		out = append(out, it)
		if len(out) >= maxCount {
			break
		}
	}
	return out
}

// Create 创建品牌。
func (uc *BrandUseCase) Create(ctx context.Context, tenantID string, in BrandInput) (entity.Brand, error) {
	if tenantID == "" {
		return entity.Brand{}, fmt.Errorf("tenant_id 不能为空")
	}
	// 用例层校验（F1-1 双保险：防绕过前端直调 API）+ 输入净化（F3-3：拒绝乱码/控制字符）
	if err := validateBrandInput(in, in.BizType); err != nil {
		return entity.Brand{}, err
	}
	now := time.Now()
	b := entity.Brand{
		ID:          fmt.Sprintf("brand-%d", now.UnixNano()),
		TenantID:    tenantID,
		Name:        sanitizeBrandText(in.Name),
		Positioning: sanitizeBrandText(in.Positioning),
		CoreSelling: sanitizeBrandSlice(in.CoreSelling, 30, 8),
		Competitors: sanitizeBrandSlice(in.Competitors, 30, 16),
		BizType:     in.BizType,
		WebsiteURL:  sanitizeBrandText(in.WebsiteURL),
		Industry:    sanitizeBrandText(in.Industry),
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
// 级联清理其下关键词（R1：单事务原子删除，不再逐个删留孤儿风险）。
func (uc *BrandUseCase) AdminDelete(ctx context.Context, brandID string) error {
	return uc.brandRepo.DeleteCascade(ctx, "", brandID)
}

// Delete 删除品牌（同时清理其下关键词——事务级联，见 DeleteCascade）。
func (uc *BrandUseCase) Delete(ctx context.Context, tenantID, brandID string) error {
	return uc.brandRepo.DeleteCascade(ctx, tenantID, brandID)
}

// Update 修改品牌信息（名称/定位/卖点/竞品/业务类型）。
// PATCH 语义：空字段保留原值（只传 competitors 不会清空 name/positioning）。
// 业务类型变更影响门店/附近同行/监测问法分流（local↔online 切换）。
func (uc *BrandUseCase) Update(ctx context.Context, tenantID, brandID string, in BrandInput) (entity.Brand, error) {
	old, err := uc.brandRepo.FindByID(ctx, tenantID, brandID)
	if err != nil {
		return entity.Brand{}, fmt.Errorf("品牌不存在: %w", err)
	}
	if in.Name != "" {
		old.Name = sanitizeBrandText(in.Name)
	}
	if in.Positioning != "" {
		old.Positioning = sanitizeBrandText(in.Positioning)
	}
	if in.CoreSelling != nil {
		old.CoreSelling = sanitizeBrandSlice(in.CoreSelling, 30, 8)
	}
	if in.Competitors != nil {
		old.Competitors = sanitizeBrandSlice(in.Competitors, 30, 16)
	}
	if in.BizType != "" {
		old.BizType = in.BizType
	}
	if in.WebsiteURL != "" {
		old.WebsiteURL = sanitizeBrandText(in.WebsiteURL)
	}
	if in.Industry != "" {
		old.Industry = sanitizeBrandText(in.Industry)
	}
	// 用例层校验（F1-1：按合并后的有效业务类型校验——切到 online 且无官网时拦截）
	if err := validateBrandInput(BrandInput{
		Name: old.Name, Positioning: old.Positioning, CoreSelling: old.CoreSelling,
		Competitors: old.Competitors, WebsiteURL: old.WebsiteURL, Industry: old.Industry,
	}, old.BizType); err != nil {
		return entity.Brand{}, err
	}
	if !old.IsValid() {
		return entity.Brand{}, fmt.Errorf("品牌无效：name 不能为空")
	}
	if err := uc.brandRepo.Save(ctx, old); err != nil {
		return entity.Brand{}, fmt.Errorf("update brand: %w", err)
	}
	return old, nil
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

	// 本地意图关键词（本地生活 P0 补全）：品牌有门店时注入位置，
	// 要求生成含"城市/区+品类"的本地搜索词（如"望京 川菜馆""朝阳区 聚餐餐厅"）——
	// 实体业态的命脉搜索入口，纯 LLM 凭卖点生成不出来。
	if localCtx := uc.buildLocalKeywordContext(ctx, brandID); localCtx != "" {
		userPrompt += fmt.Sprintf(`
门店位置：%s

请额外生成包含门店位置（城市/区/商圈）的本地搜索词——如"望京 川菜馆"、"朝阳区 适合聚餐的餐厅"这类用户搜附近门店时会用的词。
`, localCtx)
	}

	if webContext != "" {
		userPrompt += fmt.Sprintf(`
全网相关内容摘要（真实用户/作者在关注的话题）：
%s

请结合上述全网内容，生成 20 个最合适的候选关键词（贴合真实搜索需求，包含品牌词、行业热词、长尾问题词）。
`, truncateStr(webContext, 2000))
	} else {
		userPrompt += "\n请生成 20 个用户可能搜索的相关关键词（包含品牌词、行业词、长尾问题词）。"
	}
	userPrompt += "\n请生成 20 个用户可能搜索的相关关键词（包含品牌词、行业词、长尾问题词）。只输出 JSON，不要其他内容。"

	messages := []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	convID := fmt.Sprintf("kw-gen-%d", time.Now().UnixNano())

	// 结构化输出（防随机化）：JSON schema 强制 {keywords:[{term,intent}]}，
	// 数量/格式稳定；生成器不支持或解析失败时降级纯文本路径。
	if gen, ok := uc.aiGen.(port.OptionsAwareGenerator); ok {
		out, err := gen.ChatStreamWithOptions(ctx, port.ChatStreamInput{
			ConversationID: convID,
			LLMConfigName:  "",
			Messages:       messages,
			Options: port.ChatOptions{
				ResponseFormat:    "json",
				SchemaExample:     &port.KeywordList{},
				SchemaDescription: "候选关键词列表（term=关键词，intent=搜索意图）",
				DisableThinking:   true,
			},
		})
		if err == nil {
			var kl port.KeywordList
			if jsonBlock := pkg.ExtractJSONBlock(out); json.Unmarshal([]byte(jsonBlock), &kl) == nil && len(kl.Keywords) > 0 {
				terms := make([]string, 0, len(kl.Keywords))
				for _, k := range kl.Keywords {
					if term := strings.TrimSpace(k.Term); term != "" {
						terms = append(terms, term)
					}
				}
				if len(terms) > 0 {
					return terms, nil
				}
			}
		}
	}

	resp, err := uc.aiGen.ChatStream(ctx, convID, "", messages, nil)
	if err != nil {
		return nil, fmt.Errorf("生成关键词失败: %w", err)
	}
	// 过滤 <think> 块后再解析关键词
	resp = pkg.StripThinkTags(resp)
	return parseKeywordLines(resp), nil
}

// buildLocalKeywordContext 取品牌主门店并格式化为本地关键词上下文（纯文本，零失败风险）。
// 未注入仓储/品牌无门店时返回空串（行为与改造前一致——不强制本地化）。
// 业务分流（P0-2）：online 品牌（线上业务）无地理约束——跳过本地化，监测走品类词。
// 位置优先级：商圈 > 区 > 城市（P1 商圈补全后，"望京"比"朝阳区"更贴近真实搜索意图）。
func (uc *BrandUseCase) buildLocalKeywordContext(ctx context.Context, brandID string) string {
	if uc.storeRepo == nil || brandID == "" {
		return ""
	}
	// online 品牌：无门店/无地理约束，跳过（线上业务监测走品类词，非本地词）
	if b, err := uc.brandRepo.FindByID(ctx, "", brandID); err == nil && !b.IsLocal() {
		return ""
	}
	store, err := uc.storeRepo.FindPrimaryByBrand(ctx, brandID)
	if err != nil {
		return ""
	}
	// 地理编码回填的商圈/区/城市；编码失败（pending）时用地址前半段兜底
	if ctx := localContextFromStore(store); ctx != "" {
		return ctx + "（" + store.Address + "）"
	}
	return store.Address
}

// localContextFromStore 从门店提取本地位置上下文（P1 商圈补全，关键词生成/监测问法共用）：
// 商圈 > 区 > 城市——商圈级（如"望京"）最贴近"附近"搜索意图，无商圈数据逐级回退。
func localContextFromStore(s entity.StoreLocation) string {
	if s.BusinessArea != "" {
		return s.BusinessArea
	}
	if s.District != "" {
		return s.District
	}
	return s.City
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
