package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/structured"
)

// registerGEORoutes GEO 业务路由（商户端核心：品牌/关键词/监测/内容/门店/附近同行）。
// 返回构造好的 geoHandler（nil=未装配）——管理后台的全局管理端点复用它。
func (r *Router) registerGEORoutes(api *gin.RouterGroup) *GEOHandler {
	var geoHandler *GEOHandler
	if r.geoBrandUC == nil {
		return nil
	}
	geoHandler = NewGEOHandler(r.geoBrandUC, r.geoMonitorUC, r.geoRankUC, r.geoContentUC, r.geoDiagnoseUC)
	// 注入关键词蒸馏能力（可选）
	if r.geoDistillUC != nil {
		geoHandler.SetDistillUC(r.geoDistillUC)
	}
	// 注入本地生活能力（门店档案 + 附近同行双榜，可选）
	if r.geoStoreUC != nil {
		geoHandler.SetStoreUC(r.geoStoreUC)
	}
	if r.geoNearbyUC != nil {
		geoHandler.SetNearbyUC(r.geoNearbyUC)
	}
	if r.geoAirProbeUC != nil {
		geoHandler.SetAIRankProbeUC(r.geoAirProbeUC)
	}
	if r.geoAdviceUC != nil {
		geoHandler.SetAdviceUC(r.geoAdviceUC)
	}
	if r.geoCitationUC != nil {
		geoHandler.SetCitationUC(r.geoCitationUC)
	}
	// 健康报告聚合（v3 归位：总分/五指数/竞品对标的单一事实源，替代前端各自合成）
	if r.geoHealthUC != nil {
		geoHandler.SetHealthUC(r.geoHealthUC)
	}
	if r.geoHotVideoUC != nil {
		geoHandler.SetHotVideoUC(r.geoHotVideoUC)
	}
	// 行业全景聚合（v3 P2：注册进 handler，admin 路由组挂端点）
	if r.geoIndustryUC != nil {
		geoHandler.SetIndustryUC(r.geoIndustryUC)
	}
	if r.inputTipper != nil {
		geoHandler.SetInputTipper(r.inputTipper)
	}
	// 品牌 CRUD（Gin 同层 wildcard 参数名必须统一，全部用 :id）
	api.GET("/merchant/brands", geoHandler.HandleListBrands)
	api.POST("/merchant/brands", geoHandler.HandleCreateBrand)
	api.DELETE("/merchant/brands/:id", geoHandler.HandleDeleteBrand)
	api.PUT("/merchant/brands/:id", geoHandler.HandleUpdateBrand) // 修改品牌信息（名称/定位/卖点/竞品/业务类型）
	// 关键词（:id 即 brandId，handler 内用 c.Param("id") 取）
	api.GET("/merchant/brands/:id/keywords", geoHandler.HandleListKeywords)
	api.POST("/merchant/brands/:id/keywords", geoHandler.HandleAddKeyword)
	// 监测
	api.POST("/merchant/monitor", geoHandler.HandleMonitor)
	api.POST("/merchant/monitor-keyword", geoHandler.HandleMonitorKeyword)   // 单关键词即时监测
	api.POST("/merchant/monitor-multi", geoHandler.HandleMonitorMultiEngine) // 多引擎批量监测
	// 商户端自动盯盘开关（租户级：我的品牌是否参与每日自动监测）
	if r.settingsUC != nil {
		api.GET("/merchant/monitor-auto", r.HandleGetTenantAutoMonitor)
		api.PUT("/merchant/monitor-auto", r.HandleSetTenantAutoMonitor)
	}
	api.GET("/merchant/monitor-results", geoHandler.HandleAllMonitorResults) // 租户全部监测结果（关键词一览页用）
	api.GET("/merchant/brands/:id/overview", geoHandler.HandleBrandOverview)
	// 引擎名单（商户端速查/矩阵执行选择——仅 name/provider/model，不含厂商密钥；
	// 完整 LLM 配置视图已迁移至 admin 路由）
	if r.llmCfgUC != nil {
		api.GET("/merchant/engines", r.handleListEngineNames)
	}
	// 健康报告（驾驶舱/品牌徽章单一数据源——总分+五指数+竞品对标+品牌级分值）
	if geoHandler.healthUC != nil {
		api.GET("/merchant/health-report", geoHandler.HandleHealthReport)
	}
	// 内容优化
	api.POST("/merchant/optimize", geoHandler.HandleOptimizeContent)
	api.GET("/merchant/brands/:id/contents", geoHandler.HandleListContents)
	api.POST("/merchant/brands/:id/contents/generate", geoHandler.HandleGenerateContent)
	api.POST("/merchant/brands/:id/contents/generate-stream", geoHandler.HandleGenerateContentStream)   // SSE 流式生成
	api.POST("/merchant/brands/:id/contents/:contentId/status", geoHandler.HandleSetContentStatus)      // 状态流转 draft↔published
	api.DELETE("/merchant/brands/:id/contents/:contentId", geoHandler.HandleDeleteContent)              // 删除内容（管理后台/工作台）
	api.POST("/merchant/brands/:id/contents/:contentId/resubmit-index", geoHandler.HandleResubmitIndex) // 商户端自助补提交收录
	// GEO 诊断
	api.POST("/merchant/brands/:id/diagnose", geoHandler.HandleDiagnose)
	// 关键词蒸馏（按来源：品牌/文本/种子/文件/网络）
	api.POST("/merchant/keywords/distill", geoHandler.HandleDistillKeywords)
	// 关键词管理（跨品牌聚合列表 + 删除）
	api.GET("/merchant/keywords", geoHandler.HandleListAllKeywords)
	api.DELETE("/merchant/keywords/:id", geoHandler.HandleDeleteKeyword)
	// 门店档案（本地生活 GEO 地基；路由参数 :storeId 为门店 ID）
	api.GET("/merchant/brands/:id/store-locations", geoHandler.HandleListStoreLocations)
	api.POST("/merchant/brands/:id/store-locations", geoHandler.HandleCreateStoreLocation)
	api.PUT("/merchant/brands/:id/store-locations/:storeId", geoHandler.HandleUpdateStoreLocation)
	api.DELETE("/merchant/brands/:id/store-locations/:storeId", geoHandler.HandleDeleteStoreLocation)
	api.POST("/merchant/brands/:id/store-locations/:storeId/re-geocode", geoHandler.HandleReGeocodeStoreLocation)
	// 附近同行双榜（现实世界地图榜 + AI 竞品榜）
	api.GET("/merchant/brands/:id/nearby-competitors", geoHandler.HandleNearbyCompetitors)
	api.GET("/merchant/brands/:id/competitor-suggestions", geoHandler.HandleSuggestCompetitors) // 竞品自动推荐（附近同行 top N）
	// AI 榜单探查（v2：AI 榜真实数据源——手动刷新时强制重跑并缓存 24h）
	api.POST("/merchant/brands/:id/ai-rank-probe", geoHandler.HandleAIRankProbe)
	// AI 榜缓存读取（F4 品牌卡徽章——只读不烧配额，无缓存时 available=false）
	if geoHandler.airProbeUC != nil {
		api.GET("/merchant/brands/:id/ai-rank", geoHandler.HandleGetAIRank)
	}
	// 行动建议（P5-05：给老板"下一步做什么"）
	api.GET("/merchant/brands/:id/advice", geoHandler.HandleAdvice)
	// 热门同款视频发现（人设档案 tab：LLM+搜索发现，24h 缓存）
	if geoHandler.hotVideoUC != nil {
		api.GET("/merchant/brands/:id/hot-videos", geoHandler.HandleListHotVideos)
	}
	// 内容引用统计（P5-02：每篇被 AI 引用几次）
	api.GET("/merchant/brands/:id/citations", geoHandler.HandleContentCitations)
	// 地址联想（P1 输入提示：门店建档表单边输入边联想）
	api.GET("/merchant/location/suggest", geoHandler.HandleSuggestLocations)
	return geoHandler
}

// registerStructuredRoutes 结构化数据端点（JSON-LD 生成 / llms.txt 生成）——纯逻辑，无 DB/LLM 依赖。
func (r *Router) registerStructuredRoutes(api *gin.RouterGroup) {
	if r.structuredUC == nil {
		return
	}
	api.POST("/merchant/structured/jsonld", r.handleGenerateJSONLD)
	api.POST("/merchant/structured/llms-txt", r.handleGenerateLLMSTxt)
}

// registerAccountRoutes 多平台发布账号域路由（扫码绑定 + 半自动发布）——通过 SetAccount 延迟注入，可选。
func (r *Router) registerAccountRoutes(api *gin.RouterGroup) {
	if r.accountUC == nil {
		return
	}
	accountHandler := NewAccountHandler(r.accountUC, r.publishSemiUC)
	if r.accountFrontendURL != "" {
		accountHandler.SetFrontendBaseURL(r.accountFrontendURL)
	}
	// 抖音 OAuth 授权回调（公开——浏览器从抖音授权页重定向至此，无 JWT；
	// 安全体在 state HMAC 签名：验签还原租户上下文，伪造/过期 state 一律拒绝）
	if r.rootGroup != nil {
		r.rootGroup.GET("/api/v1/merchant/accounts/douyin/oauth/callback", accountHandler.HandleDouyinOAuthCallback)
	}
	// 账号管理
	api.GET("/merchant/accounts", accountHandler.HandleListAccounts)
	api.POST("/merchant/accounts/qr-login", accountHandler.HandleStartQRLogin)
	api.GET("/merchant/accounts/qr-login/:sessionId", accountHandler.HandlePollQRLogin)
	api.DELETE("/merchant/accounts/qr-login/:sessionId", accountHandler.HandleCancelQRLogin)
	api.DELETE("/merchant/accounts/:id", accountHandler.HandleDeleteAccount)
	// 官方 OAuth 授权绑定（抖音开放平台 API 通道；回调端点在 router.go 公开段注册）
	api.GET("/merchant/accounts/douyin/oauth/url", accountHandler.HandleDouyinOAuthURL)
	// 发布管理
	api.GET("/merchant/publish/channels", accountHandler.HandleListChannels) // 平台能力清单（发布页能力驱动）
	// 品牌知识库（商户上传品牌文档——获客智能体转型：知识库成为内容生成输入源）
	if r.knowledgeUC != nil {
		bkHandler := NewBrandKnowledgeHandler(r.knowledgeUC)
		api.POST("/merchant/brands/:id/knowledge/materials", bkHandler.HandleUploadMaterial)
		api.GET("/merchant/brands/:id/knowledge/materials", bkHandler.HandleListMaterials)
		api.DELETE("/merchant/brands/:id/knowledge/materials/:mid", bkHandler.HandleDeleteMaterial)
	}
	api.POST("/merchant/publish", accountHandler.HandlePublish)
	api.GET("/merchant/publish-jobs", accountHandler.HandleListPublishJobs)
	api.GET("/merchant/works/analytics-summary", accountHandler.HandleAnalyticsSummary) // 作品数据页聚合
	api.POST("/merchant/publish-jobs/:id/published", accountHandler.HandleMarkPublished)
	api.GET("/merchant/publish-jobs/:id/status", accountHandler.HandleGetJobStatus)
	api.POST("/merchant/publish-jobs/:id/re-monitor", accountHandler.HandleReMonitor) // 发布效果复测（收录周期后验证提及率爬升）
}

// registerGenerationRoutes 统一生成任务（Vidu 全量接入：视频/图片/音频/数字人）。
// Vidu 回调入口已迁移至公开路由（router.go 公开段，与支付 webhook 同等待遇）——
// 服务商回调无 JWT，HMAC 验签+nonce 防重放是它的安全边界。
func (r *Router) registerGenerationRoutes(api *gin.RouterGroup) {
	if r.generationUC == nil {
		return
	}
	gh := NewGenerationHandler(r.generationUC)
	api.POST("/generation/tasks", gh.HandleSubmit)
	api.GET("/generation/tasks/:id", gh.HandleGet)
	api.GET("/generation/tasks", gh.HandleList)
	api.GET("/generation/types", gh.HandleTypes)
	api.POST("/generation/tasks/:id/cancel", gh.HandleCancel)
}

// registerMediaRoutes 素材库（上传 + 列表 + 删除——用户图片/音频 → 本地 → URL 供 Vidu 引用）。
func (r *Router) registerMediaRoutes(api *gin.RouterGroup) {
	if r.mediaStore == nil {
		return
	}
	mh := NewMediaHandler(r.mediaStore)
	api.POST("/media/assets", mh.HandleUpload)
	api.GET("/media/assets", mh.HandleList)
	api.DELETE("/media/assets/:id", mh.HandleDelete)
}

// jsonldRequest POST /api/v1/geo/structured/jsonld 的请求体
type jsonldRequest struct {
	Title       string `json:"title" binding:"required"`
	Content     string `json:"content" binding:"required"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	BrandName   string `json:"brand_name"`
	PublishDate string `json:"publish_date"` // 可选：2006-01-02
}

// handleGenerateJSONLD POST /api/v1/geo/structured/jsonld —— 内容 → JSON-LD 结构化数据。
// 类型自动推断（FAQPage/Product/HowTo/Organization/Article）。
func (r *Router) handleGenerateJSONLD(c *gin.Context) {
	if r.structuredUC == nil {
		fail(c, fmt.Errorf("结构化用例未初始化"))
		return
	}
	var req jsonldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	var publishDate time.Time
	if req.PublishDate != "" {
		if t, err := time.Parse("2006-01-02", req.PublishDate); err == nil {
			publishDate = t
		}
	}
	sd, err := r.structuredUC.GenerateJSONLD(c.Request.Context(), structured.StructuredDataInput{
		Title:       req.Title,
		Content:     req.Content,
		URL:         req.URL,
		Author:      req.Author,
		BrandName:   req.BrandName,
		PublishDate: publishDate,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"type": sd.Type, "jsonld": sd.JSONLD})
}

// llmsTxtRequest POST /api/v1/geo/structured/llms-txt 的请求体
type llmsTxtRequest struct {
	SiteTitle   string `json:"site_title" binding:"required"`
	SiteSummary string `json:"site_summary"`
	Entries     []struct {
		URL     string `json:"url" binding:"required"`
		Title   string `json:"title"`
		Summary string `json:"summary"`
	} `json:"entries"`
}

// handleGenerateLLMSTxt POST /api/v1/geo/structured/llms-txt —— 站点内容索引 → llms.txt 全文。
func (r *Router) handleGenerateLLMSTxt(c *gin.Context) {
	if r.structuredUC == nil {
		fail(c, fmt.Errorf("结构化用例未初始化"))
		return
	}
	var req llmsTxtRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	entries := make([]entity.LLMSTxtEntry, 0, len(req.Entries))
	for _, e := range req.Entries {
		entries = append(entries, entity.LLMSTxtEntry{URL: e.URL, Title: e.Title, Summary: e.Summary})
	}
	txt, err := r.structuredUC.GenerateLLMSTxt(c.Request.Context(), req.SiteTitle, req.SiteSummary, entries)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"llms_txt": txt})
}
