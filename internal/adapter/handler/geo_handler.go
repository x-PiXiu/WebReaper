package handler

import (
	"fmt"

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
}

func NewGEOHandler(br *geo.BrandUseCase, mo *geo.MonitorUseCase, ra *geo.RankUseCase, co *geo.ContentUseCase, du *geo.DiagnoseUseCase) *GEOHandler {
	return &GEOHandler{brandUC: br, monitorUC: mo, rankUC: ra, contentUC: co, diagnoseUC: du}
}

// SetDistillUC 注入关键词蒸馏用例（可选；未注入则蒸馏端点不注册）。
func (h *GEOHandler) SetDistillUC(uc *geo.KeywordDistillUseCase) {
	h.distillUC = uc
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
		"id":            r.ID,
		"tenant_id":     r.TenantID,
		"brand_id":      r.BrandID,
		"keyword_id":    r.KeywordID,
		"engine_name":   r.EngineName,
		"sample_count":  r.SampleCount,
		"mention_count": r.MentionCount,
		"mention_rate":  r.MentionRate,
		"avg_position":  r.AvgPosition,
		"sentiment":     r.Sentiment,
		"competitors":   r.Competitors,
		"confidence":    r.Confidence,
		"probed_at":     r.ProbedAt,
		"raw_sample":    r.RawSample,
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
		"original_text":  c.OriginalText,
		"optimized_text": c.OptimizedText,
		"version":        c.Version,
		"score":          geoScoreToView(c.Score),
		"status":         c.Status,
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
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	brand, err := h.brandUC.Create(c.Request.Context(), middleware.CurrentTenantID(c), geo.BrandInput{
		Name: req.Name, Positioning: req.Positioning, CoreSelling: req.CoreSelling, Competitors: req.Competitors,
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
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, optimizedContentToView(oc))
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

// HandleGenerateContent POST /api/v1/geo/brands/:id/contents/generate
// 从零生成内容：根据品牌信息 + 关键词（单个或多个组合），AI 原创一篇 GEO 优化文章。
func (h *GEOHandler) HandleGenerateContent(c *gin.Context) {
	brandID := c.Param("id")
	var req struct {
		Keywords      []string `json:"keywords" binding:"required"`
		BrandInfo     string   `json:"brand_info"`
		LLMConfigName string   `json:"llm_config_name"`
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
