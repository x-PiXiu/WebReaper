package handler

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/geo"
	"webreaper/internal/usecase/hotvideo"
	"webreaper/internal/usecase/port"
)

// GEOHandler 是 GEO 业务（品牌/监测/排行榜/内容/诊断）的 HTTP 适配器。
//
// 多租户：所有请求从 JWT 取 tenant_id（merchant 只能看自己的，admin 看全局）。
//
// 整洁架构要点（接口适配器层职责）：
//   - 所有响应通过 brandToView/keywordToView 等 DTO 转换函数，把领域实体（PascalCase）
//     翻译成 API 契约（snake_case）。领域实体不依赖 JSON tag，API 契约稳定可控。
type GEOHandler struct {
	brandUC    *geo.BrandUseCase
	monitorUC  *geo.MonitorUseCase
	rankUC     *geo.RankUseCase
	contentUC  *geo.ContentUseCase
	diagnoseUC *geo.DiagnoseUseCase
	distillUC  *geo.KeywordDistillUseCase
	storeUC    *geo.StoreLocationUseCase // 门店档案（可选注入）
	nearbyUC   *geo.NearbyUseCase        // 附近同行双榜（可选注入）
	airProbeUC *geo.AIRankProbeUseCase   // AI 榜单探查（可选注入，v2：AI 榜数据源）
	adviceUC   *geo.AdviceUseCase        // 行动建议（可选注入，P5-05）
	citationUC *geo.CitationUseCase      // 内容引用统计（可选注入，P5-02）
	healthUC   *geo.HealthUseCase        // 健康报告聚合（可选注入，v3 归位：单一事实源）
	industryUC *geo.IndustryUseCase      // 行业全景聚合（可选注入，v3 P2：admin 看板）
	inputTipper port.InputTipper         // 地址联想（可选注入，P1；未注入→空列表降级）
	hotVideoUC  *hotvideo.HotVideoUseCase // 热门同款视频发现（可选注入，人设档案 tab）
}

func NewGEOHandler(br *geo.BrandUseCase, mo *geo.MonitorUseCase, ra *geo.RankUseCase, co *geo.ContentUseCase, du *geo.DiagnoseUseCase) *GEOHandler {
	return &GEOHandler{brandUC: br, monitorUC: mo, rankUC: ra, contentUC: co, diagnoseUC: du}
}

// SetDistillUC 注入关键词蒸馏用例（可选；未注入则蒸馏端点不注册）。
func (h *GEOHandler) SetDistillUC(uc *geo.KeywordDistillUseCase) {
	h.distillUC = uc
}

// SetStoreUC 注入门店档案用例（可选；未注入则门店端点不注册）。
func (h *GEOHandler) SetStoreUC(uc *geo.StoreLocationUseCase) {
	h.storeUC = uc
}

// SetNearbyUC 注入附近同行用例（可选；未注入则双榜端点不注册）。
func (h *GEOHandler) SetNearbyUC(uc *geo.NearbyUseCase) {
	h.nearbyUC = uc
}

// SetAIRankProbeUC 注入 AI 榜单探查用例（可选；未注入则探查端点不注册）。
func (h *GEOHandler) SetAIRankProbeUC(uc *geo.AIRankProbeUseCase) {
	h.airProbeUC = uc
}

// SetAdviceUC 注入行动建议用例（可选；未注入则建议端点不注册）。
func (h *GEOHandler) SetAdviceUC(uc *geo.AdviceUseCase) {
	h.adviceUC = uc
}

// SetCitationUC 注入内容引用统计用例（可选；未注入则引用端点不注册）。
func (h *GEOHandler) SetCitationUC(uc *geo.CitationUseCase) {
	h.citationUC = uc
}

// SetHealthUC 注入健康报告聚合用例（可选；未注入则健康报告端点不注册）。
func (h *GEOHandler) SetHealthUC(uc *geo.HealthUseCase) {
	h.healthUC = uc
}

// SetHotVideoUC 注入热门同款视频用例（可选；未注入则热门同款端点不注册）。
func (h *GEOHandler) SetHotVideoUC(uc *hotvideo.HotVideoUseCase) {
	h.hotVideoUC = uc
}

// HandleListHotVideos GET /merchant/brands/:id/hot-videos —— 品牌同赛道热门视频
// 支持两种模式：
//   - ?force=true：实时搜索+LLM 筛选（24h 缓存；结果自动落库积累）
//   - 默认/带筛选参数：从 DB 列出（支持 ?platform=&q=&sort_by=&limit=&offset= 搜索/排序/分页）
func (h *GEOHandler) HandleListHotVideos(c *gin.Context) {
	if h.hotVideoUC == nil {
		fail(c, fmt.Errorf("热门同款视频功能未启用"))
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	brandID := c.Param("id")
	force := c.Query("force") == "true"
	platform := c.Query("platform")
	keyword := c.Query("q")
	sortBy := c.Query("sort_by")

	// 带筛选参数或非强制 → 优先从 DB 读（定时采集积累的历史数据）
	if !force && (platform != "" || keyword != "" || sortBy != "" || c.Query("limit") != "") {
		limit := 20
		offset := 0
		if v := c.Query("limit"); v != "" {
			fmt.Sscanf(v, "%d", &limit)
		}
		if v := c.Query("offset"); v != "" {
			fmt.Sscanf(v, "%d", &offset)
		}
		videos, total, err := h.hotVideoUC.ListFromDB(c.Request.Context(), brandID, hotvideo.ListOptions{
			Platform: platform, Keyword: keyword, SortBy: sortBy,
			Limit: limit, Offset: offset,
		})
		if err != nil {
			fail(c, err); return
		}
		if videos == nil {
			videos = []entity.HotVideo{}
		}
		success(c, gin.H{"videos": videos, "total": total})
		return
	}

	// 默认/force → 实时搜索（结果自动落库）
	videos, err := h.hotVideoUC.ListHotVideos(c.Request.Context(), tenantID, brandID, force)
	if err != nil {
		fail(c, err); return
	}
	if videos == nil {
		videos = []entity.HotVideo{}
	}
	success(c, gin.H{"videos": videos})
}

// SetIndustryUC 注入行业全景聚合用例（可选；未注入则行业看板端点不注册）。
func (h *GEOHandler) SetIndustryUC(uc *geo.IndustryUseCase) {
	h.industryUC = uc
}

// SetInputTipper 注入地址联想服务（可选；未注入则联想端点返回空列表，表单纯手输）。
func (h *GEOHandler) SetInputTipper(t port.InputTipper) {
	if t != nil {
		h.inputTipper = t
	}
}

// ---- DTO 转换（实体 → API 响应，PascalCase → snake_case）----

func brandToView(b entity.Brand) gin.H {
	return gin.H{
		"id":           b.ID,
		"tenant_id":    b.TenantID,
		"name":         b.Name,
		"positioning":  b.Positioning,
		"core_selling": b.CoreSelling,
		"competitors":  b.Competitors,
		"biz_type":     b.BizType,
		"industry":     b.Industry,
		"website_url":  b.WebsiteURL,
		"created_at":   b.CreatedAt,
	}
}

func brandsToView(bs []entity.Brand) []gin.H {
	out := make([]gin.H, 0, len(bs))
	for _, b := range bs {
		out = append(out, brandToView(b))
	}
	return out
}

func keywordToView(k entity.Keyword) gin.H {
	return gin.H{
		"id":         k.ID,
		"tenant_id":  k.TenantID,
		"brand_id":   k.BrandID,
		"term":       k.Term,
		"intent":     k.Intent,
		"created_at": k.CreatedAt,
	}
}

func keywordsToView(ks []entity.Keyword) []gin.H {
	out := make([]gin.H, 0, len(ks))
	for _, k := range ks {
		out = append(out, keywordToView(k))
	}
	return out
}

// monitoringResultView 监测结果 API 视图（typed struct）。
// 整洁架构要点：视图曾用 gin.H（map 无编译期字段检查）——实体新增字段时序列化
// 静默丢失（信源断流缺陷根因）。改为 struct 后由编译期 + 契约守护测试
//（geo_handler_view_test.go 反射断言"实体公开字段 ⊆ 视图 json 字段"）双重保障。
type monitoringResultView struct {
	ID                   string             `json:"id"`
	TenantID             string             `json:"tenant_id"`
	BrandID              string             `json:"brand_id"`
	KeywordID            string             `json:"keyword_id"`
	EngineName           string             `json:"engine_name"`
	SampleCount          int                `json:"sample_count"`
	MentionCount         int                `json:"mention_count"`
	MentionRate          float64            `json:"mention_rate"`
	AvgPosition          int                `json:"avg_position"`
	Sentiment            string             `json:"sentiment"`
	Competitors          []string           `json:"competitors"`
	CompetitorRates      map[string]float64 `json:"competitor_rates"`      // 竞品提及率（对比坐标系）
	CompetitorSentiments map[string]string  `json:"competitor_sentiments"` // 竞品情感（对标视图语义维度）
	// 竞品沉淀候选（AI 回答中自然出现的其他品牌）——「从监测结果推荐」数据源
	CandidateCompetitors []string  `json:"candidate_competitors"`
	Confidence           float64   `json:"confidence"`
	ProbedAt             time.Time `json:"probed_at"`
	RawSample            string    `json:"raw_sample"`
	Sources              []string  `json:"sources"`           // 引用来源（链接/平台名，归因 P5-01）
	SelfSourceCount      int       `json:"self_source_count"` // 自营公开站被引用次数（>0 = 内容真的被 AI 引用）
	FirstPickCount       int       `json:"first_pick_count"`  // 被提及且位次=1 的采样数（首选率分子；迁移 045）
	SemanticDegraded     bool      `json:"semantic_degraded"` // 采样中出现过解析降级（情感/位次可能失真）
	// MentionLabel 提及率可读等级（服务端领域规则下发——此前前端被迫重写同一映射，口径漂移）
	MentionLabel string `json:"mention_label"`
}

func monitoringResultToView(r entity.MonitoringResult) monitoringResultView {
	return monitoringResultView{
		ID:                   r.ID,
		TenantID:             r.TenantID,
		BrandID:              r.BrandID,
		KeywordID:            r.KeywordID,
		EngineName:           r.EngineName,
		SampleCount:          r.SampleCount,
		MentionCount:         r.MentionCount,
		MentionRate:          r.MentionRate,
		AvgPosition:          r.AvgPosition,
		Sentiment:            r.Sentiment,
		Competitors:          r.Competitors,
		CompetitorRates:      r.CompetitorRates,
		CompetitorSentiments: r.CompetitorSentiments,
		CandidateCompetitors: r.CandidateCompetitors,
		Confidence:           r.Confidence,
		ProbedAt:             r.ProbedAt,
		RawSample:            r.RawSample,
		Sources:              r.Sources,
		SelfSourceCount:      r.SelfSourceCount,
		FirstPickCount:       r.FirstPickCount,
		SemanticDegraded:     r.SemanticDegraded,
		MentionLabel:         r.MentionRateLabel(),
	}
}

func monitoringResultsToView(rs []entity.MonitoringResult) []monitoringResultView {
	out := make([]monitoringResultView, 0, len(rs))
	for _, r := range rs {
		out = append(out, monitoringResultToView(r))
	}
	return out
}

func geoScoreToView(s entity.GEOScore) gin.H {
	return gin.H{
		"total":       s.Total,
		"authority":   s.Authority,
		"specificity": s.Specificity,
		"structure":   s.Structure,
		"uniqueness":  s.Uniqueness,
		"recency":     s.Recency,
		// 等级（A/B/C/D）服务端领域规则下发——此前 Level() 只活在代码里，前端无从展示
		"level": s.Level(),
	}
}

func optimizedContentToView(c entity.OptimizedContent) gin.H {
	return gin.H{
		"id":             c.ID,
		"tenant_id":      c.TenantID,
		"brand_id":       c.BrandID,
		"keyword_id":     c.KeywordID,
		"title":          c.Title,
		"original_text":  c.OriginalText,
		"optimized_text": c.OptimizedText,
		"version":        c.Version,
		"score":          geoScoreToView(c.Score),
		// 评分来源标记（P1）：落库分数 = LLM 深度评审；前端据此区分"深度评审"与
		// 优化前的"规则快筛"（score_before），两套量纲不再混排无标记
		"score_type":   "llm",
		"status":       c.Status,
		"index_status": c.IndexStatus,
		"created_at":   c.CreatedAt,
	}
}

func optimizedContentsToView(cs []entity.OptimizedContent) []gin.H {
	out := make([]gin.H, 0, len(cs))
	for _, c := range cs {
		out = append(out, optimizedContentToView(c))
	}
	return out
}

// ---- 品牌 CRUD ----

func (h *GEOHandler) HandleListBrands(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	brands, err := h.brandUC.List(c.Request.Context(), tenantID)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, brandsToView(brands))
}

func (h *GEOHandler) HandleCreateBrand(c *gin.Context) {
	var req struct {
		Name        string   `json:"name" binding:"required"`
		Positioning string   `json:"positioning"`
		CoreSelling []string `json:"core_selling"`
		Competitors []string `json:"competitors"`
		BizType     string   `json:"biz_type"`
		WebsiteURL  string   `json:"website_url"`
		Industry    string   `json:"industry"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	brand, err := h.brandUC.Create(c.Request.Context(), middleware.CurrentTenantID(c), geo.BrandInput{
		Name: req.Name, Positioning: req.Positioning, CoreSelling: req.CoreSelling,
		Competitors: req.Competitors, BizType: req.BizType, WebsiteURL: req.WebsiteURL,
		Industry: req.Industry,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, brandToView(brand))
}

func (h *GEOHandler) HandleDeleteBrand(c *gin.Context) {
	id := c.Param("id")
	if err := h.brandUC.Delete(c.Request.Context(), middleware.CurrentTenantID(c), id); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": id})
}

// HandleUpdateBrand PUT /api/v1/geo/brands/:id —— 修改品牌信息（名称/定位/卖点/竞品/业务类型）。
func (h *GEOHandler) HandleUpdateBrand(c *gin.Context) {
	var req struct {
		Name        string   `json:"name"`
		Positioning string   `json:"positioning"`
		CoreSelling []string `json:"core_selling"`
		Competitors []string `json:"competitors"`
		BizType     string   `json:"biz_type"`
		WebsiteURL  string   `json:"website_url"`
		Industry    string   `json:"industry"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	brand, err := h.brandUC.Update(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"), geo.BrandInput{
		Name: req.Name, Positioning: req.Positioning, CoreSelling: req.CoreSelling,
		Competitors: req.Competitors, BizType: req.BizType, WebsiteURL: req.WebsiteURL,
		Industry: req.Industry,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, brandToView(brand))
}

// ---- 关键词 ----

func (h *GEOHandler) HandleListKeywords(c *gin.Context) {
	brandID := c.Param("id")
	tenantID := middleware.CurrentTenantID(c)
	kws, err := h.brandUC.ListKeywords(c.Request.Context(), tenantID, brandID)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, keywordsToView(kws))
}

func (h *GEOHandler) HandleAddKeyword(c *gin.Context) {
	brandID := c.Param("id") // 路由参数统一为 :id（Gin 同层只允许一个参数名）
	var req struct {
		Term   string `json:"term" binding:"required"`
		Intent string `json:"intent"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	kw, err := h.brandUC.AddKeyword(c.Request.Context(), middleware.CurrentTenantID(c), brandID, req.Term, req.Intent)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, keywordToView(kw))
}

// ---- 监测 ----

func (h *GEOHandler) HandleMonitor(c *gin.Context) {
	var req struct {
		BrandID    string `json:"brand_id" binding:"required"`
		EngineName string `json:"engine_name"`
		SampleSize int    `json:"sample_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	results, err := h.monitorUC.Monitor(c.Request.Context(), geo.MonitorInput{
		TenantID:   middleware.CurrentTenantID(c),
		BrandID:    req.BrandID,
		EngineName: req.EngineName,
		SampleSize: req.SampleSize,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, monitoringResultsToView(results))
}

// HandleAllMonitorResults GET /api/v1/geo/monitor-results
// 取租户下所有关键词的最新监测结果（不依赖品牌筛选，关键词一览页默认用这个）。
// R2：经 HealthUseCase 缓存读侧（60s TTL + 写后主动失效——见 MonitorUseCase.invalidateAfterWrite）。
func (h *GEOHandler) HandleAllMonitorResults(c *gin.Context) {
	results, err := h.healthUC.CachedLatestByTenant(c.Request.Context(), middleware.CurrentTenantID(c))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, monitoringResultsToView(results))
}

// HandleMonitorKeyword POST /api/v1/geo/monitor-keyword
// 单关键词即时监测（关键词一览页"刷新"按钮用，比品牌级批量更快）。
func (h *GEOHandler) HandleMonitorKeyword(c *gin.Context) {
	var req struct {
		KeywordID  string `json:"keyword_id" binding:"required"`
		EngineName string `json:"engine_name"`
		SampleSize int    `json:"sample_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	result, err := h.monitorUC.MonitorKeyword(c.Request.Context(), geo.MonitorKeywordInput{
		TenantID:   middleware.CurrentTenantID(c),
		KeywordID:  req.KeywordID,
		EngineName: req.EngineName,
		SampleSize: req.SampleSize,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, monitoringResultToView(result))
}

// HandleMonitorMultiEngine POST /api/v1/geo/monitor-multi
// 多引擎批量监测（一次对同一关键词用多个引擎监测，产生多条不同引擎的结果）。
func (h *GEOHandler) HandleMonitorMultiEngine(c *gin.Context) {
	var req struct {
		KeywordID   string   `json:"keyword_id" binding:"required"`
		EngineNames []string `json:"engine_names" binding:"required"`
		SampleSize  int      `json:"sample_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	results, err := h.monitorUC.MonitorMultiEngine(c.Request.Context(),
		middleware.CurrentTenantID(c), req.KeywordID, req.EngineNames, req.SampleSize)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, monitoringResultsToView(results))
}

func (h *GEOHandler) HandleBrandOverview(c *gin.Context) {
	brandID := c.Param("id") // 路由参数统一为 :id（Gin 同层只允许一个参数名）
	brandName := c.Query("name")
	overview, err := h.rankUC.Overview(c.Request.Context(), middleware.CurrentTenantID(c), brandID, brandName)
	if err != nil {
		fail(c, err)
		return
	}
	// BrandOverview 里的 Trend 是 []MonitoringResult，也要转 View
	success(c, gin.H{
		"brand_id":         overview.BrandID,
		"brand_name":       overview.BrandName,
		"avg_mention_rate": overview.AvgMentionRate,
		"keyword_count":    overview.KeywordCount,
		"last_probed_at":   overview.LastProbedAt,
		"trend":            monitoringResultsToView(overview.Trend),
	})
}

// HandleGetAIRank GET /api/v1/geo/brands/:id/ai-rank —— AI 榜最近一次缓存（F4 品牌卡徽章；
// 只读缓存不重跑——强制刷新走 POST ai-rank-probe）。返回全部条目（含未提及），
// 品牌自身位次由前端按门店名匹配计算（榜上门店来自该品牌的门店档案）。
func (h *GEOHandler) HandleGetAIRank(c *gin.Context) {
	if h.airProbeUC == nil {
		fail(c, fmt.Errorf("AI 榜用例未初始化"))
		return
	}
	res, err := h.airProbeUC.Latest(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"))
	if err != nil {
		success(c, gin.H{"available": false})
		return
	}
	type item struct {
		Name      string  `json:"name"`
		Rate      float64 `json:"rate"`
		Mentioned bool    `json:"mentioned"`
		AvgPos    int     `json:"avg_pos"`
	}
	items := make([]item, 0, len(res.Results))
	for _, it := range res.Results {
		items = append(items, item{Name: it.Name, Rate: it.Rate, Mentioned: it.Mentioned, AvgPos: it.AvgPos})
	}
	success(c, gin.H{
		"available": true,
		"probed_at": res.ProbedAt,
		"expire_at": res.ExpireAt,
		"items":     items,
	})
}

// ---- 健康报告（v3 归位：后端聚合单一事实源）----

type healthThreatView struct {
	Name      string  `json:"name"`
	AvgRate   float64 `json:"avg_rate"`   // 0-100
	Sentiment string  `json:"sentiment"`  // positive/negative/""（中性）
}

type healthBrandView struct {
	BrandID        string  `json:"brand_id"`
	BrandName      string  `json:"brand_name"`
	Total          float64 `json:"total"`            // 与总分同口径（三处展示位统一）
	AvgMentionRate float64 `json:"avg_mention_rate"` // 0-1
}

type healthReportView struct {
	Total      float64 `json:"total"`
	Indicators struct {
		MentionCoverage float64 `json:"mention_coverage"`
		SentimentScore  float64 `json:"sentiment_score"`
		FirstPickRate   float64 `json:"first_pick_rate"`
		ContentAsset    float64 `json:"content_asset"`
		SourceIntegrity float64 `json:"source_integrity"`
	} `json:"indicators"`
	PrevTotal   *float64 `json:"prev_total"` // 上一期总分（无历史为 null）
	HasPrev     bool     `json:"has_prev"`
	Competitor  struct {
		SelfAvg float64            `json:"self_avg"` // 0-1
		CompAvg float64            `json:"comp_avg"` // 0-1
		GapPct  float64            `json:"gap_pct"`  // 百分点（+领先/-落后）
		Size    int                `json:"size"`
		Threats []healthThreatView `json:"threats"` // 按提及率降序
	} `json:"competitor"`
	Brands []healthBrandView `json:"brands"`
}

// HandleHealthReport GET /api/v1/geo/health-report
// 租户健康报告（总分+五指数+环比+竞品对标+品牌级分值）——驾驶舱/品牌徽章的单一数据源。
func (h *GEOHandler) HandleHealthReport(c *gin.Context) {
	if h.healthUC == nil {
		fail(c, fmt.Errorf("健康报告用例未初始化"))
		return
	}
	report, err := h.healthUC.Report(c.Request.Context(), middleware.CurrentTenantID(c))
	if err != nil {
		fail(c, err)
		return
	}
	var view healthReportView
	view.Total = report.Total
	view.Indicators.MentionCoverage = report.Indicators.MentionCoverage
	view.Indicators.SentimentScore = report.Indicators.SentimentScore
	view.Indicators.FirstPickRate = report.Indicators.FirstPickRate
	view.Indicators.ContentAsset = report.Indicators.ContentAsset
	view.Indicators.SourceIntegrity = report.Indicators.SourceIntegrity
	view.PrevTotal = report.PrevTotal
	view.HasPrev = report.PrevTotal != nil
	view.Competitor.SelfAvg = report.Competitor.SelfAvg
	view.Competitor.CompAvg = report.Competitor.CompAvg
	view.Competitor.GapPct = report.Competitor.GapPct
	view.Competitor.Size = report.Competitor.Size
	view.Competitor.Threats = make([]healthThreatView, 0, len(report.Competitor.Threats))
	for _, t := range report.Competitor.Threats {
		view.Competitor.Threats = append(view.Competitor.Threats, healthThreatView{Name: t.Name, AvgRate: t.AvgRate, Sentiment: t.Sentiment})
	}
	view.Brands = make([]healthBrandView, 0, len(report.Brands))
	for _, b := range report.Brands {
		view.Brands = append(view.Brands, healthBrandView{BrandID: b.BrandID, BrandName: b.BrandName, Total: b.Total, AvgMentionRate: b.AvgMentionRate})
	}
	success(c, view)
}

// HandleAdminIndustryOverview GET /api/v1/admin/geo/industry-overview
// 行业全景看板（跨商户聚合）：行业能见度榜 + 品牌美誉度榜 + 信源域名榜。
func (h *GEOHandler) HandleAdminIndustryOverview(c *gin.Context) {
	if h.industryUC == nil {
		fail(c, fmt.Errorf("行业全景用例未初始化"))
		return
	}
	ov, err := h.industryUC.Overview(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	industries := make([]gin.H, 0, len(ov.Industries))
	for _, i := range ov.Industries {
		industries = append(industries, gin.H{"industry": i.Industry, "avg_rate": i.AvgRate, "brand_count": i.BrandCount})
	}
	reputation := make([]gin.H, 0, len(ov.Reputation))
	for _, r := range ov.Reputation {
		reputation = append(reputation, gin.H{"brand_name": r.BrandName, "industry": r.Industry, "positive_rate": r.PositiveRate, "sample_count": r.SampleCount})
	}
	sources := make([]gin.H, 0, len(ov.TopSources))
	for _, s := range ov.TopSources {
		sources = append(sources, gin.H{"domain": s.Domain, "count": s.Count})
	}
	success(c, gin.H{"industries": industries, "reputation": reputation, "top_sources": sources})
}

// ---- 内容优化 ----

func (h *GEOHandler) HandleOptimizeContent(c *gin.Context) {
	var req struct {
		BrandID       string `json:"brand_id" binding:"required"`
		KeywordID     string `json:"keyword_id"`
		OriginalText  string `json:"original_text" binding:"required"`
		Keyword       string `json:"keyword" binding:"required"`
		LLMConfigName string `json:"llm_config_name"`
		TargetEngine  string `json:"target_engine"`
		Format        string `json:"format"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	oc, err := h.contentUC.Optimize(c.Request.Context(), geo.OptimizeInput{
		TenantID:      middleware.CurrentTenantID(c),
		BrandID:       req.BrandID,
		KeywordID:     req.KeywordID,
		OriginalText:  req.OriginalText,
		Keyword:       req.Keyword,
		LLMConfigName: req.LLMConfigName,
		TargetEngine:  req.TargetEngine,
		Format:        req.Format,
	})
	if err != nil {
		fail(c, err)
		return
	}
	// 优化结果视图：原字段向后兼容 + 新增前后对比反馈
	view := optimizedContentToView(oc.Content)
	view["score_before"] = geoScoreToView(oc.ScoreBefore)
	// 优化前分数为免费规则快筛（非 LLM 深评）——来源标记随数据下发，量纲不混排
	view["score_before_type"] = "rule"
	view["recommendations"] = oc.Recommendations
	success(c, view)
}

func (h *GEOHandler) HandleListContents(c *gin.Context) {
	brandID := c.Param("id") // 路由参数统一为 :id（Gin 同层只允许一个参数名）
	ocs, err := h.contentUC.List(c.Request.Context(), middleware.CurrentTenantID(c), brandID)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, optimizedContentsToView(ocs))
}

// HandleSetContentStatus POST /api/v1/geo/brands/:id/contents/:contentId/status
// 内容状态流转：draft ↔ published（published 后公开站点可访问，AI 引擎可爬取）。
// 响应在内容视图上附加 index_submitted / publish_warnings——发布是否已提交收录、
// 低分未提交等警告随结果下发（修复"发布成功但永不收录"的用户黑洞）。
func (h *GEOHandler) HandleSetContentStatus(c *gin.Context) {
	contentID := c.Param("contentId")
	var req struct {
		Status string `json:"status" binding:"required"` // draft / published
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	oc, outcome, err := h.contentUC.SetStatus(c.Request.Context(), middleware.CurrentTenantID(c), contentID, req.Status)
	if err != nil {
		fail(c, err)
		return
	}
	view := optimizedContentToView(oc)
	view["index_submitted"] = outcome.IndexSubmitted
	view["publish_warnings"] = outcome.Warnings
	success(c, view)
}

// HandleDeleteContent DELETE /api/v1/geo/brands/:id/contents/:contentId
// 删除优化内容（租户校验；删除后公开页立即 404）。
func (h *GEOHandler) HandleDeleteContent(c *gin.Context) {
	contentID := c.Param("contentId")
	if err := h.contentUC.Delete(c.Request.Context(), middleware.CurrentTenantID(c), contentID); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": true})
}

// HandleResubmitIndex POST /api/v1/geo/brands/:id/contents/:contentId/resubmit-index
// 商户端自助补提交收录（IndexNow）——重新通知搜索引擎抓取已发布内容。
func (h *GEOHandler) HandleResubmitIndex(c *gin.Context) {
	contentID := c.Param("contentId")
	if err := h.contentUC.ResubmitIndex(c.Request.Context(), middleware.CurrentTenantID(c), contentID); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"submitted": true})
}

// HandleAdminListBrands GET /api/v1/admin/brands —— 全平台品牌列表（admin 旁路）。
// 注意：不走 CurrentTenantID（商户上下文租户隔离）；由 adminGroup 角色守卫保护，
// 仅管理后台全局管理端点调用——杜绝"空租户看全局"的越权路径。
func (h *GEOHandler) HandleAdminListBrands(c *gin.Context) {
	brands, err := h.brandUC.ListAll(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(brands))
	for _, b := range brands {
		views = append(views, brandToView(b))
	}
	success(c, views)
}

// HandleAdminListContents GET /api/v1/admin/contents —— 全平台内容列表（admin 旁路）。
// 支持 ?status=draft|published 过滤；不限定租户，由 adminGroup 角色守卫保护。
func (h *GEOHandler) HandleAdminListContents(c *gin.Context) {
	items, err := h.contentUC.ListAll(c.Request.Context(), c.Query("status"), 0)
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(items))
	for _, oc := range items {
		views = append(views, optimizedContentToView(oc))
	}
	success(c, views)
}

// HandleAdminDeleteBrand DELETE /api/v1/admin/brands/:id —— 全平台品牌删除（admin 旁路）。
func (h *GEOHandler) HandleAdminDeleteBrand(c *gin.Context) {
	if err := h.brandUC.AdminDelete(c.Request.Context(), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": true})
}

// HandleAdminSetContentStatus POST /api/v1/admin/contents/:id/status —— 全平台内容上下架（admin 旁路）。
func (h *GEOHandler) HandleAdminSetContentStatus(c *gin.Context) {
	var req struct {
		Status string `json:"status" binding:"required"` // draft / published
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	oc, err := h.contentUC.AdminSetStatus(c.Request.Context(), c.Param("id"), req.Status)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, optimizedContentToView(oc))
}

// HandleAdminDeleteContent DELETE /api/v1/admin/contents/:id —— 全平台内容删除（admin 旁路）。
func (h *GEOHandler) HandleAdminDeleteContent(c *gin.Context) {
	if err := h.contentUC.AdminDelete(c.Request.Context(), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": true})
}

// HandleGenerateContent POST /api/v1/geo/brands/:id/contents/generate
// 从零生成内容：根据品牌信息 + 关键词（单个或多个组合），AI 原创一篇 GEO 优化文章。
func (h *GEOHandler) HandleGenerateContent(c *gin.Context) {
	brandID := c.Param("id")
	var req struct {
		Keywords      []string `json:"keywords"`           // 可选（获客智能体转型：留空 = AI 从知识库+品牌资料全自动提炼）
		Topic         string   `json:"topic"`               // 用户一句话（可选——"想写什么"）
		BrandInfo     string   `json:"brand_info"`
		LLMConfigName string   `json:"llm_config_name"`
		TargetEngine  string   `json:"target_engine"`
		UseDiagnose   bool     `json:"use_diagnose"` // 诊断→优化闭环（P5-03）
		Format        string   `json:"format"`
		CitationToggles []string `json:"citation_toggles"` // 可引用结构开关（v3 P2，可组合）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	oc, warnings, err := h.contentUC.Generate(c.Request.Context(), geo.GenerateInput{
		TenantID:         middleware.CurrentTenantID(c),
		BrandID:          brandID,
		Keywords:         req.Keywords,
		Topic:            req.Topic,
		BrandInfo:        req.BrandInfo,
		LLMConfigName:    req.LLMConfigName,
		TargetEngine:     req.TargetEngine,
		UseDiagnose:      req.UseDiagnose,
		Format:           req.Format,
		CitationToggles:  req.CitationToggles,
	})
	if err != nil {
		fail(c, err)
		return
	}
	// 重复内容软提示（A4）：同品牌已有相似已发布内容——前端展示但不阻断
	view := optimizedContentToView(oc)
	if len(warnings) > 0 {
		view["duplicate_warnings"] = warnings
	}
	success(c, view)
}

// HandleGenerateContentStream POST /api/v1/geo/brands/:id/contents/generate-stream
// 流式生成内容（SSE）：用户实时看到文章逐字输出，不用干等。
// 只推送正文 content delta，不推思考过程。
func (h *GEOHandler) HandleGenerateContentStream(c *gin.Context) {
	brandID := c.Param("id")
	var req struct {
		Keywords      []string `json:"keywords"`  // 可选（获客智能体转型）
		Topic         string   `json:"topic"`      // 用户一句话（可选）
		BrandInfo     string   `json:"brand_info"`
		LLMConfigName string   `json:"llm_config_name"`
		TargetEngine  string   `json:"target_engine"`
		Format        string   `json:"format"`
		CitationToggles []string `json:"citation_toggles"` // 可引用结构开关（v3 P2，可组合）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	// 设置 SSE 响应头
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	ctx := c.Request.Context()
	flusher, _ := c.Writer.(interface{ Flush() })

	// 流式生成（onDelta 实时推送正文增量）
	oc, err := h.contentUC.GenerateStream(ctx, geo.GenerateInput{
		TenantID:         middleware.CurrentTenantID(c),
		BrandID:          brandID,
		Keywords:         req.Keywords,
		BrandInfo:        req.BrandInfo,
		LLMConfigName:    req.LLMConfigName,
		TargetEngine:     req.TargetEngine,
		Format:           req.Format,
		CitationToggles:  req.CitationToggles,
	}, func(delta string) {
		// 只推正文 content delta（AI SDK text-delta 格式）
		writeSSE(c.Writer, map[string]any{"type": "text-delta", "textDelta": delta})
		if flusher != nil {
			flusher.Flush()
		}
	})
	if err != nil {
		writeSSE(c.Writer, map[string]any{"type": "error", "error": err.Error()})
		if flusher != nil {
			flusher.Flush()
		}
		return
	}
	// 推送最终结果（含 GEO 评分）+ finish
	writeSSE(c.Writer, map[string]any{"type": "result", "data": optimizedContentToView(oc)})
	writeSSE(c.Writer, map[string]any{"type": "finish"})
	if flusher != nil {
		flusher.Flush()
	}
}

// ---- GEO 诊断 ----

func competitorStatToView(cs entity.CompetitorStat) gin.H {
	return gin.H{
		"name":            cs.Name,
		"appearance_rate": cs.AppearanceRate,
		"avg_position":    cs.AvgPosition,
	}
}

func diagnoseReportToView(r entity.DiagnoseReport) gin.H {
	comps := make([]gin.H, 0, len(r.CompetitorStats))
	for _, cs := range r.CompetitorStats {
		comps = append(comps, competitorStatToView(cs))
	}
	return gin.H{
		"brand_id":              r.BrandID,
		"keyword_id":            r.KeywordID,
		"content_coverage":      r.ContentCoverage,
		"brand_appearance_rate": r.BrandAppearanceRate,
		"competitor_stats":      comps,
		"suggestions":           r.Suggestions,
	}
}

// HandleDiagnose POST /api/v1/geo/brands/:brandId/diagnose
// 诊断品牌为什么没被 AI 提及，给出数据驱动的改进建议。
func (h *GEOHandler) HandleDiagnose(c *gin.Context) {
	if h.diagnoseUC == nil {
		fail(c, fmt.Errorf("诊断功能未启用"))
		return
	}
	brandID := c.Param("id") // 路由参数统一为 :id（Gin 同层只允许一个参数名）
	var req struct {
		KeywordID string `json:"keyword_id"`
	}
	_ = c.ShouldBindJSON(&req)
	report, err := h.diagnoseUC.Diagnose(c.Request.Context(), geo.DiagnoseInput{
		TenantID:  middleware.CurrentTenantID(c),
		BrandID:   brandID,
		KeywordID: req.KeywordID,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, diagnoseReportToView(report))
}

// ---- 关键词蒸馏 ----

// HandleDistillKeywords POST /api/v1/geo/keywords/distill
// 关键词蒸馏：按来源（品牌/文本/种子/文件/网络）蒸馏出关键词候选。
func (h *GEOHandler) HandleDistillKeywords(c *gin.Context) {
	if h.distillUC == nil {
		fail(c, fmt.Errorf("关键词蒸馏功能未启用"))
		return
	}
	var req struct {
		Source     string   `json:"source" binding:"required"` // brand/text/seed/file/web
		BrandID    string   `json:"brand_id"`
		Text       string   `json:"text"`
		Seeds      []string `json:"seeds"`
		Topic      string   `json:"topic"`
		LLMConfig  string   `json:"llm_config_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	keywords, intents, err := h.distillUC.DistillWithIntents(c.Request.Context(), req.Source, port.KeywordSourceInput{
		TenantID:  middleware.CurrentTenantID(c),
		BrandID:   req.BrandID,
		Text:      req.Text,
		Seeds:     req.Seeds,
		Topic:     req.Topic,
		LLMConfig: req.LLMConfig,
	})
	if err != nil {
		fail(c, err)
		return
	}
	// F3-2：questions 源附带意图标注（信息型/比较型/推荐型）——前端结果列表展示标签
	success(c, gin.H{"keywords": keywords, "keyword_intents": intents})
}

// HandleListAllKeywords GET /api/v1/geo/keywords
func (h *GEOHandler) HandleListAllKeywords(c *gin.Context) {
	kws, err := h.brandUC.ListAllKeywords(c.Request.Context(), middleware.CurrentTenantID(c))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, keywordsToView(kws))
}

// HandleDeleteKeyword DELETE /api/v1/geo/keywords/:id
func (h *GEOHandler) HandleDeleteKeyword(c *gin.Context) {
	id := c.Param("id")
	if err := h.brandUC.DeleteKeyword(c.Request.Context(), middleware.CurrentTenantID(c), id); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": id})
}

// ---- 门店档案（本地生活 GEO 地基，P0）----

func storeLocationToView(s entity.StoreLocation) gin.H {
	return gin.H{
		"id":          s.ID,
		"tenant_id":   s.TenantID,
		"brand_id":    s.BrandID,
		"name":        s.Name,
		"address":     s.Address,
		"city":        s.City,
		"district":    s.District,
		"adcode":      s.Adcode,
		"lat":         s.Lat,
		"lng":         s.Lng,
		"phone":       s.Phone,
		"hours":       s.Hours,
		"price_level": s.PriceLevel,
		"biz_type":    s.BizType,
		"geo_status":  s.GeoStatus,
		"has_geo":     s.HasGeo(),
		"created_at":  s.CreatedAt,
		"updated_at":  s.UpdatedAt,
	}
}

func storeLocationsToView(ss []entity.StoreLocation) []gin.H {
	out := make([]gin.H, 0, len(ss))
	for _, s := range ss {
		out = append(out, storeLocationToView(s))
	}
	return out
}

// HandleListStoreLocations GET /api/v1/geo/brands/:id/store-locations
func (h *GEOHandler) HandleListStoreLocations(c *gin.Context) {
	if h.storeUC == nil {
		fail(c, fmt.Errorf("门店档案用例未注入"))
		return
	}
	stores, err := h.storeUC.List(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, storeLocationsToView(stores))
}

// HandleCreateStoreLocation POST /api/v1/geo/brands/:id/store-locations
func (h *GEOHandler) HandleCreateStoreLocation(c *gin.Context) {
	if h.storeUC == nil {
		fail(c, fmt.Errorf("门店档案用例未注入"))
		return
	}
	var req struct {
		Name       string `json:"name"`
		Address    string `json:"address" binding:"required"`
		Phone      string `json:"phone"`
		Hours      string `json:"hours"`
		PriceLevel string `json:"price_level"`
		BizType    string `json:"biz_type"`
		WebsiteURL string `json:"website_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	loc, err := h.storeUC.Create(c.Request.Context(), geo.StoreLocationInput{
		TenantID:   middleware.CurrentTenantID(c),
		BrandID:    c.Param("id"),
		Name:       req.Name,
		Address:    req.Address,
		Phone:      req.Phone,
		Hours:      req.Hours,
		PriceLevel: req.PriceLevel,
		BizType:    req.BizType,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, storeLocationToView(loc))
}

// HandleUpdateStoreLocation PUT /api/v1/geo/brands/:id/store-locations/:storeId
func (h *GEOHandler) HandleUpdateStoreLocation(c *gin.Context) {
	if h.storeUC == nil {
		fail(c, fmt.Errorf("门店档案用例未注入"))
		return
	}
	var req struct {
		Name       string `json:"name"`
		Address    string `json:"address"`
		Phone      string `json:"phone"`
		Hours      string `json:"hours"`
		PriceLevel string `json:"price_level"`
		BizType    string `json:"biz_type"`
		WebsiteURL string `json:"website_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	loc, err := h.storeUC.Update(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"), c.Param("storeId"), geo.StoreLocationInput{
		TenantID:   middleware.CurrentTenantID(c),
		BrandID:    c.Param("id"),
		Name:       req.Name,
		Address:    req.Address,
		Phone:      req.Phone,
		Hours:      req.Hours,
		PriceLevel: req.PriceLevel,
		BizType:    req.BizType,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, storeLocationToView(loc))
}

// HandleDeleteStoreLocation DELETE /api/v1/geo/brands/:id/store-locations/:storeId
func (h *GEOHandler) HandleDeleteStoreLocation(c *gin.Context) {
	if h.storeUC == nil {
		fail(c, fmt.Errorf("门店档案用例未注入"))
		return
	}
	if err := h.storeUC.Delete(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"), c.Param("storeId")); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": c.Param("storeId")})
}

// HandleReGeocodeStoreLocation POST /api/v1/geo/brands/:id/store-locations/:storeId/re-geocode
// 重试地理编码（配置地图服务后，为 pending/failed 门店补齐坐标）。
func (h *GEOHandler) HandleReGeocodeStoreLocation(c *gin.Context) {
	if h.storeUC == nil {
		fail(c, fmt.Errorf("门店档案用例未注入"))
		return
	}
	loc, err := h.storeUC.ReGeocode(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("storeId"))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, storeLocationToView(loc))
}

// ---- 附近同行双榜（现实世界 + AI 世界，P2）----

func nearbyRankingToView(v geo.NearbyRanking) gin.H {
	mapRanking := make([]gin.H, 0, len(v.MapRanking))
	for _, e := range v.MapRanking {
		mapRanking = append(mapRanking, gin.H{
			"name": e.Name, "address": e.Address, "distance_m": e.DistanceM,
			"rating": e.Rating, "category": e.Category, "open_status": e.OpenStatus,
			"lat": e.Lat, "lng": e.Lng,
			// 门店卡扩展（v5 show_fields=business,navi）
			"city_name": e.CityName, "ad_name": e.AdName, "cost": e.Cost,
			"business_area": e.BusinessArea, "open_time_today": e.OpenTimeToday,
			"tag": e.Tag, "tel": e.Tel, "entr_location": e.EntrLocation, "photo_url": e.PhotoURL,
			// 驾车耗时（P2 距离测量补全）
			"drive_distance_m": e.DriveDistanceM, "drive_duration_sec": e.DriveDurationSec,
		})
	}
	aiRanking := make([]gin.H, 0, len(v.AIRanking))
	for _, e := range v.AIRanking {
		aiRanking = append(aiRanking, gin.H{
			"name": e.Name, "rate": e.Rate, "sample_cnt": e.SampleCnt,
			"mentioned": e.Mentioned, "mention_cnt": e.MentionCnt,
			"is_own": e.IsOwn,
		})
	}
	return gin.H{
		"store":          storeLocationToView(v.Store),
		"map_ranking":    mapRanking,
		"ai_ranking":     aiRanking,
		"own_rate":       v.OwnRate,
		"map_available":  v.MapAvailable,
		"search_keyword": v.SearchKeyword,
		// AI 榜来源与覆盖（v2：AI 榜单探查——全量补位 + 上榜率）
		"ai_rank_from_probe": v.AIRankFromProbe,
		"ai_rank_probed_at":  v.AIRankProbedAt,
		"ai_rank_total":      v.AIRankTotal,
		"ai_rank_mentioned":  v.AIRankMentioned,
		"ai_rank_sample":     v.AIRankSample,
	}
}

// HandleNearbyCompetitors GET /api/v1/geo/brands/:id/nearby-competitors
// 可选 query 参数：types（POI 分类编码，如 050000 餐饮——按类目扫描竞品）。
func (h *GEOHandler) HandleNearbyCompetitors(c *gin.Context) {
	if h.nearbyUC == nil {
		fail(c, fmt.Errorf("附近同行用例未注入"))
		return
	}
	view, err := h.nearbyUC.GetRanking(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"), c.Query("types"))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, nearbyRankingToView(view))
}

// HandleAIRankProbe POST /api/v1/geo/brands/:id/ai-rank-probe
// 手动触发"AI 榜单探查"（force 重跑并缓存 24h；返回最新 AI 榜视图）。
// 请求体可选：types（POI 分类编码，如 050000 餐饮）。
func (h *GEOHandler) HandleAIRankProbe(c *gin.Context) {
	if h.airProbeUC == nil {
		fail(c, fmt.Errorf("AI 榜单探查用例未注入"))
		return
	}
	var req struct {
		Types string `json:"types"`
	}
	_ = c.ShouldBindJSON(&req)
	result, err := h.airProbeUC.Run(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"), req.Types, true)
	if err != nil {
		fail(c, err)
		return
	}
	// 探查后返回完整双榜视图（含新 AI 榜）——前端刷新一次即完成
	view, vErr := h.nearbyUC.GetRanking(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"), req.Types)
	if vErr != nil {
		success(c, gin.H{"ai_rank_probed": true, "sample_count": result.SampleCount})
		return
	}
	view.AIRankFromProbe = true
	view.AIRankProbedAt = result.ProbedAt.Format("01-02 15:04")
	view.AIRankSample = result.SampleCount
	success(c, nearbyRankingToView(view))
}

// HandleSuggestCompetitors GET /api/v1/geo/brands/:id/competitor-suggestions
// 竞品自动推荐——两种来源（?source=poi|monitoring）：
//   poi（默认）：附近同行 POI 按评分/距离 top N（仅 local 品牌）
//   monitoring：监测结果 CompetitorRates 蒸馏（local/online 都适用——online 核心竞品来源）
func (h *GEOHandler) HandleSuggestCompetitors(c *gin.Context) {
	if h.nearbyUC == nil {
		fail(c, fmt.Errorf("附近同行用例未注入"))
		return
	}
	limit := 5
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 20 {
			limit = n
		}
	}
	source := c.DefaultQuery("source", "poi")
	var suggestions []geo.CompetitorSuggestion
	var err error
	if source == "monitoring" {
		suggestions, err = h.nearbyUC.SuggestCompetitorsFromMonitoring(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"), limit)
	} else {
		suggestions, err = h.nearbyUC.SuggestCompetitors(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"), limit)
	}
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(suggestions))
	for _, s := range suggestions {
		views = append(views, gin.H{
			"name": s.Name, "rating": s.Rating, "distance_m": s.DistanceM,
			"address": s.Address, "category": s.Category,
		})
	}
	success(c, views)
}

// HandleSuggestLocations GET /api/v1/geo/location/suggest?q=&city=&location=
// 地址联想（P1 输入提示）：门店建档表单"边输入边联想"，免手输地址。
// city：citycode/adcode（空=全国）；location："lng,lat" 附近优先（需 city 非空）。
func (h *GEOHandler) HandleSuggestLocations(c *gin.Context) {
	if h.inputTipper == nil {
		success(c, []gin.H{}) // 未注入（未配置地图服务）→ 空列表，表单退化为纯手输
		return
	}
	q := c.Query("q")
	if len([]rune(q)) < 1 {
		success(c, []gin.H{})
		return
	}
	tips, err := h.inputTipper.InputTips(c.Request.Context(), q, c.Query("city"), c.Query("location"))
	if err != nil {
		success(c, []gin.H{}) // 联想失败降级为空（不阻断建档流程）
		return
	}
	view := make([]gin.H, 0, len(tips))
	for _, t := range tips {
		view = append(view, gin.H{
			"name": t.Name, "address": t.Address, "district": t.District,
			"adcode": t.Adcode, "location": t.Location, "poi_id": t.POIID,
		})
	}
	success(c, view)
}

// ---- 行动建议（P5-05：给老板"下一步做什么"）----

// HandleAdvice GET /api/v1/geo/brands/:id/advice
// 基于监测/门店/内容数据规则生成可执行建议（零 LLM 成本，可单测）。
func (h *GEOHandler) HandleAdvice(c *gin.Context) {
	if h.adviceUC == nil {
		fail(c, fmt.Errorf("行动建议用例未注入"))
		return
	}
	advices, err := h.adviceUC.GetAdvice(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	view := make([]gin.H, 0, len(advices))
	for _, a := range advices {
		view = append(view, gin.H{"level": a.Level, "message": a.Message, "page": a.Page})
	}
	success(c, gin.H{"advices": view})
}

// ---- 内容引用统计（P5-02 校准基础设施）----

// HandleContentCitations GET /api/v1/geo/brands/:id/citations
// 统计每篇内容被 AI 回答引用的次数（归因细化到篇——"哪篇内容真正起作用"）。
// 返回 {content_id: 引用次数} 映射。
func (h *GEOHandler) HandleContentCitations(c *gin.Context) {
	if h.citationUC == nil {
		fail(c, fmt.Errorf("内容引用统计用例未注入"))
		return
	}
	counts, err := h.citationUC.GetByBrand(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, counts)
}
