package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/geo"
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
	inputTipper port.InputTipper         // 地址联想（可选注入，P1；未注入→空列表降级）
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

func monitoringResultToView(r entity.MonitoringResult) gin.H {
	return gin.H{
		"id":               r.ID,
		"tenant_id":        r.TenantID,
		"brand_id":         r.BrandID,
		"keyword_id":       r.KeywordID,
		"engine_name":      r.EngineName,
		"sample_count":     r.SampleCount,
		"mention_count":    r.MentionCount,
		"mention_rate":     r.MentionRate,
		"avg_position":     r.AvgPosition,
		"sentiment":        r.Sentiment,
		"competitors":      r.Competitors,
		"competitor_rates": r.CompetitorRates, // 竞品提及率（对比坐标系）
		// 竞品沉淀候选（AI 回答中自然出现的其他品牌）——「从监测结果推荐」数据源
		"candidate_competitors": r.CandidateCompetitors,
		"confidence":       r.Confidence,
		"probed_at":        r.ProbedAt,
		"raw_sample":       r.RawSample,
	}
}

func monitoringResultsToView(rs []entity.MonitoringResult) []gin.H {
	out := make([]gin.H, 0, len(rs))
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
		"status":         c.Status,
		"index_status":   c.IndexStatus,
		"created_at":     c.CreatedAt,
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
		WebsiteURL string   `json:"website_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	brand, err := h.brandUC.Create(c.Request.Context(), middleware.CurrentTenantID(c), geo.BrandInput{
		Name: req.Name, Positioning: req.Positioning, CoreSelling: req.CoreSelling,
		Competitors: req.Competitors, BizType: req.BizType, WebsiteURL: req.WebsiteURL,
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
		WebsiteURL string   `json:"website_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	brand, err := h.brandUC.Update(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"), geo.BrandInput{
		Name: req.Name, Positioning: req.Positioning, CoreSelling: req.CoreSelling,
		Competitors: req.Competitors, BizType: req.BizType, WebsiteURL: req.WebsiteURL,
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

func (h *GEOHandler) HandleLatestMonitor(c *gin.Context) {
	keywordID := c.Param("keywordId")
	results, err := h.monitorUC.GetLatest(c.Request.Context(), middleware.CurrentTenantID(c), keywordID)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, monitoringResultsToView(results))
}

// HandleLatestMonitorByBrand GET /api/v1/geo/brands/:id/monitor-results
// 取某品牌下所有关键词的最新监测结果（关键词一览页用）。
func (h *GEOHandler) HandleLatestMonitorByBrand(c *gin.Context) {
	brandID := c.Param("id")
	results, err := h.monitorUC.GetLatestByBrand(c.Request.Context(), middleware.CurrentTenantID(c), brandID)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, monitoringResultsToView(results))
}

// HandleAllMonitorResults GET /api/v1/geo/monitor-results
// 取租户下所有关键词的最新监测结果（不依赖品牌筛选，关键词一览页默认用这个）。
func (h *GEOHandler) HandleAllMonitorResults(c *gin.Context) {
	results, err := h.monitorUC.GetLatestByTenant(c.Request.Context(), middleware.CurrentTenantID(c))
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
func (h *GEOHandler) HandleSetContentStatus(c *gin.Context) {
	contentID := c.Param("contentId")
	var req struct {
		Status string `json:"status" binding:"required"` // draft / published
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	oc, err := h.contentUC.SetStatus(c.Request.Context(), middleware.CurrentTenantID(c), contentID, req.Status)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, optimizedContentToView(oc))
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
		Keywords      []string `json:"keywords" binding:"required"`
		BrandInfo     string   `json:"brand_info"`
		LLMConfigName string   `json:"llm_config_name"`
		TargetEngine  string   `json:"target_engine"`
		UseDiagnose   bool     `json:"use_diagnose"` // 诊断→优化闭环（P5-03）
		Format        string   `json:"format"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	oc, err := h.contentUC.Generate(c.Request.Context(), geo.GenerateInput{
		TenantID:      middleware.CurrentTenantID(c),
		BrandID:       brandID,
		Keywords:      req.Keywords,
		BrandInfo:     req.BrandInfo,
		LLMConfigName: req.LLMConfigName,
		TargetEngine:  req.TargetEngine,
		UseDiagnose:   req.UseDiagnose,
		Format:        req.Format,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, optimizedContentToView(oc))
}

// HandleGenerateContentStream POST /api/v1/geo/brands/:id/contents/generate-stream
// 流式生成内容（SSE）：用户实时看到文章逐字输出，不用干等。
// 只推送正文 content delta，不推思考过程。
func (h *GEOHandler) HandleGenerateContentStream(c *gin.Context) {
	brandID := c.Param("id")
	var req struct {
		Keywords      []string `json:"keywords" binding:"required"`
		BrandInfo     string   `json:"brand_info"`
		LLMConfigName string   `json:"llm_config_name"`
		TargetEngine  string   `json:"target_engine"`
		Format        string   `json:"format"`
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
		TenantID:      middleware.CurrentTenantID(c),
		BrandID:       brandID,
		Keywords:      req.Keywords,
		BrandInfo:     req.BrandInfo,
		LLMConfigName: req.LLMConfigName,
		TargetEngine:  req.TargetEngine,
		Format:        req.Format,
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

// ---- 关键词生成 ----

// HandleGenerateKeywords POST /api/v1/geo/brands/:brandId/keywords/generate
// AI 根据品牌定位自动生成候选关键词，供商户勾选添加。
func (h *GEOHandler) HandleGenerateKeywords(c *gin.Context) {
	brandID := c.Param("id")
	tenantID := middleware.CurrentTenantID(c)
	keywords, err := h.brandUC.GenerateKeywords(c.Request.Context(), tenantID, brandID)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"keywords": keywords})
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
	keywords, err := h.distillUC.Distill(c.Request.Context(), req.Source, port.KeywordSourceInput{
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
	success(c, gin.H{"keywords": keywords})
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
