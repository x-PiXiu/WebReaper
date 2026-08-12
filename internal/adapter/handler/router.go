package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	authadapter "webreaper/internal/adapter/auth"
	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/agentconfig"
	"webreaper/internal/usecase/account"
	"webreaper/internal/usecase/auth"
	"webreaper/internal/usecase/billing"
	"webreaper/internal/usecase/conversation"
	"webreaper/internal/usecase/crawlconfig"
	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/geo"
	"webreaper/internal/usecase/indexing"
	"webreaper/internal/usecase/llmconfig"
	"webreaper/internal/usecase/notification"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/providerconfig"
	"webreaper/internal/usecase/stats"
	"webreaper/internal/usecase/systemsettings"
	"webreaper/internal/usecase/structured"
	taskuc "webreaper/internal/usecase/task")

// Router 组装所有 HTTP 路由。
//
// 设计要点（整洁架构 / 依赖倒置）：
//   - Router 只依赖 usecase 和 port 接口，不直接持有仓储、不依赖具体 adapter struct。
//   - 这样 handler 层薄化为"DTO 转换 + 调用 usecase"，业务流程编排全部在 usecase 层。
//   - Agent 执行依赖 port.AgentSyncRunner 接口（非具体 TrpcAgentRunner），可替换。
type Router struct {
	authRegister     *auth.RegisterUseCase
	authLogin        *auth.LoginUseCase
	tokenParser      *authadapter.JWTGenerator
	ai               port.AIGenerator
	enqueueUC        *taskuc.EnqueueUseCase
	agentRunner      port.AgentSyncRunner   // 接口，非具体 struct（DIP）
	agentCfgUC       *agentconfig.AgentConfigUseCase
	llmCfgUC         *llmconfig.LLMConfigUseCase
	conversationUC   *conversation.ConversationUseCase
	crawlCfgUC       *crawlconfig.CrawlConfigUseCase
	toolRegistry     *port.ToolRegistry // 全局工具注册表（供 /tools 端点查询）
	statsUC          *stats.StatsUseCase               // 仪表盘统计聚合
	// GEO 业务（商户端核心）——通过 SetGEO 延迟注入，可选
	geoBrandUC   *geo.BrandUseCase
	geoMonitorUC *geo.MonitorUseCase
	geoRankUC    *geo.RankUseCase
	geoContentUC *geo.ContentUseCase
	geoDiagnoseUC *geo.DiagnoseUseCase
	geoDistillUC *geo.KeywordDistillUseCase // 关键词蒸馏用例（可选）
	geoStoreUC   *geo.StoreLocationUseCase  // 门店档案用例（本地生活地基，可选）
	geoNearbyUC  *geo.NearbyUseCase         // 附近同行双榜用例（可选）
	geoAirProbeUC *geo.AIRankProbeUseCase   // AI 榜单探查用例（可选，v2：AI 榜数据源）
	geoAdviceUC  *geo.AdviceUseCase         // 行动建议用例（可选，P5-05）
	geoCitationUC *geo.CitationUseCase      // 内容引用统计用例（可选，P5-02）
	inputTipper   port.InputTipper          // 地址联想（可选，P1；未注入→空列表降级）
	// 结构化数据用例（JSON-LD/llms.txt）——通过 SetStructured 注入，可选
	structuredUC *structured.StructuredDataUseCase
	// 公开内容站处理器——通过 SetPublic 注入，可选
	publicHandler *PublicHandler
	// 收录管理用例（运行时配置/提交日志/手动补提交）——通过 SetIndexing 注入，可选
	indexingUC *indexing.IndexingUseCase
	// 多平台发布账号域（扫码绑定 + 半自动发布）——通过 SetAccount 延迟注入，可选
	accountUC *account.AccountUseCase
	publishSemiUC *account.PublishUseCase
	// 用户管理（管理端）——通过 SetAdmin 延迟注入，可选
	userRepo port.UserRepository
	// 平台系统设置（运行时开关）——通过 SetSystemSettings 注入，可选
	settingsUC *systemsettings.SystemSettingsUseCase
	// 站内通知（主动唤醒）——通过 SetNotifications 注入，可选
	notifyUC *notification.NotifyUseCase
	// 统一生成（Vidu 全量接入：视频/图片/音频/数字人）
	generationUC      *generation.GenerationUseCase
	generationProvider port.GenerationProvider
	generationRegistry port.EndpointRegistry   // 规格管理（管理后台矩阵）
	generationSpecRepo port.GenerationSpecRepository
	providerConfigUC   *providerconfig.UseCase // 厂商配置（管理后台）
	mediaStore         port.MediaAssetStore    // 素材托管/转存（可选）
	mediaDir           string                  // 本地媒体静态目录（可选；非空时 /media 托管）
	apiPrefix          string                  // 路由统一前缀（nginx 分流用，如 /webreaper；空=无前缀）
	// 提示词模板仓库（admin 管理内容生成/优化提示词）——通过 SetPromptTemplates 注入，可选
	promptTemplateRepo port.PromptTemplateRepository
	// 经济系统（套餐/订阅/订单/计费）——通过 SetBilling 注入，可选
	billingUC *billing.BillingUseCase
	// 配额检查门（注入到 ChatHandler 等无独立 usecase 的端点）——通过 SetQuotaGate 注入，可选
	quotaGate port.QuotaStore
}

// SetKeywordDistill 注入关键词蒸馏用例（可选；未注入则蒸馏端点不注册）。
func (r *Router) SetKeywordDistill(uc *geo.KeywordDistillUseCase) {
	r.geoDistillUC = uc
}

// SetGeoLocal 注入门店档案 + 附近同行用例（可选；未注入则对应端点不注册）。
// 与 SetGEO 分离——门店/附近同行是本地生活改造新增的独立用例，晚装配不污染原 GEO 装配。
func (r *Router) SetGeoLocal(storeUC *geo.StoreLocationUseCase, nearbyUC *geo.NearbyUseCase) {
	r.geoStoreUC = storeUC
	r.geoNearbyUC = nearbyUC
}

// SetAIRankProbe 注入 AI 榜单探查用例（可选；未注入则探查端点不注册）。
func (r *Router) SetAIRankProbe(uc *geo.AIRankProbeUseCase) {
	r.geoAirProbeUC = uc
}

// SetAdvice 注入行动建议用例（可选；未注入则建议端点不注册）。
func (r *Router) SetAdvice(uc *geo.AdviceUseCase) {
	r.geoAdviceUC = uc
}

// SetCitation 注入内容引用统计用例（可选；未注入则引用端点不注册）。
func (r *Router) SetCitation(uc *geo.CitationUseCase) {
	r.geoCitationUC = uc
}

// SetInputTipper 注入地址联想服务（可选；未注入则联想端点返回空列表，表单纯手输）。
func (r *Router) SetInputTipper(t port.InputTipper) {
	r.inputTipper = t
}

// SetStructured 注入结构化数据用例（可选；未注入则结构化端点不注册）。
// 纯逻辑零依赖，无 DB/LLM 也能用。
func (r *Router) SetStructured(uc *structured.StructuredDataUseCase) {
	r.structuredUC = uc
}

// SetPublic 注入公开内容站处理器（可选；未注入则公开端点不注册）。
// 公开站需要内容仓储（查已发布内容）+ 结构化用例（JSON-LD/llms.txt 生成）。
func (r *Router) SetPublic(h *PublicHandler) {
	r.publicHandler = h
}

// SetIndexing 注入收录管理用例（可选；未注入则收录管理端点不注册）。
func (r *Router) SetIndexing(uc *indexing.IndexingUseCase) {
	r.indexingUC = uc
}

// SetGEO 注入 GEO 业务用例（可选；未注入则 GEO 端点不注册）。
func (r *Router) SetGEO(brand *geo.BrandUseCase, monitor *geo.MonitorUseCase, rank *geo.RankUseCase, content *geo.ContentUseCase, diagnose *geo.DiagnoseUseCase) {
	r.geoBrandUC = brand
	r.geoMonitorUC = monitor
	r.geoRankUC = rank
	r.geoContentUC = content
	r.geoDiagnoseUC = diagnose
}

// SetAdmin 注入用户管理能力（可选；未注入则管理端用户端点不注册）。
func (r *Router) SetAdmin(userRepo port.UserRepository) {
	r.userRepo = userRepo
}

// SetStats 重新注入平台总览统计用例（晚装配：GEO/发布仓储就绪后再组装完整统计）。
// 未注入时 /stats 返回 null（前端降级隐藏）。
func (r *Router) SetStats(uc *stats.StatsUseCase) {
	r.statsUC = uc
}

// SetSystemSettings 注入平台系统设置用例（可选；未注入则设置端点不注册）。
func (r *Router) SetSystemSettings(uc *systemsettings.SystemSettingsUseCase) {
	r.settingsUC = uc
}

// SetNotifications 注入站内通知用例（可选；未注入则通知端点返回空）。
func (r *Router) SetNotifications(uc *notification.NotifyUseCase) {
	r.notifyUC = uc
}

// SetAccount 注入多平台发布账号域用例（可选；未注入则账号/发布端点不注册）。
func (r *Router) SetAccount(au *account.AccountUseCase, pu *account.PublishUseCase) {
	r.accountUC = au
	r.publishSemiUC = pu
}

// SetGeneration 注入统一生成用例（可选；Vidu 全量接入——未注入则生成端点不注册）。
func (r *Router) SetGeneration(uc *generation.GenerationUseCase, provider port.GenerationProvider, registry port.EndpointRegistry, specRepo port.GenerationSpecRepository) {
	r.generationUC = uc
	r.generationProvider = provider
	r.generationRegistry = registry
	r.generationSpecRepo = specRepo
}

// SetProviderConfig 注入厂商配置用例（可选；管理后台按厂商设置 API Key——未注入则端点不注册）。
func (r *Router) SetProviderConfig(uc *providerconfig.UseCase) {
	r.providerConfigUC = uc
}

// SetMedia 注入媒体资产存储（可选；素材上传/转存——未注入则上传端点不注册）。
func (r *Router) SetMedia(store port.MediaAssetStore, dir string) {
	r.mediaStore = store
	r.mediaDir = dir
}

// SetAPIPrefix 设置路由统一前缀（nginx 分流用，如 /webreaper）。
// 必须在 Engine() 调用前设置；空=无前缀（本地开发默认）。
func (r *Router) SetAPIPrefix(prefix string) {
	r.apiPrefix = strings.TrimSuffix(prefix, "/")
}

// SetPromptTemplates 注入提示词模板仓库（可选；admin 管理内容生成/优化提示词）。
func (r *Router) SetPromptTemplates(repo port.PromptTemplateRepository) {
	r.promptTemplateRepo = repo
}

// SetBilling 注入经济系统用例（可选；未注入则计费端点不注册）。
func (r *Router) SetBilling(uc *billing.BillingUseCase) {
	r.billingUC = uc
}

// SetQuotaGate 注入配额检查门（可选；注入到 ChatHandler 等无独立 usecase 的端点）。
func (r *Router) SetQuotaGate(g port.QuotaStore) {
	r.quotaGate = g
}

// NewRouter 创建路由器（零参数——所有依赖通过 SetXxx 可选注入）。
// 整洁架构"推迟决策"：Router 不绑死依赖，端点按注入的 usecase 条件注册。
func NewRouter() *Router {
	return &Router{}
}

// ---- 核心依赖注入（auth/ai/agent/task 等基础能力）----

// SetAuth 注入认证用例（register/login + JWT 解析）。
func (r *Router) SetAuth(registerUC *auth.RegisterUseCase, loginUC *auth.LoginUseCase, tokenParser *authadapter.JWTGenerator) {
	r.authRegister = registerUC
	r.authLogin = loginUC
	r.tokenParser = tokenParser
}

// SetAI 注入 AI 生成器（对话/工具调用）。
func (r *Router) SetAI(ai port.AIGenerator) {
	r.ai = ai
}

// SetTask 注入异步任务用例（enqueue + agent runner）。
func (r *Router) SetTask(enqueueUC *taskuc.EnqueueUseCase, agentRunner port.AgentSyncRunner) {
	r.enqueueUC = enqueueUC
	r.agentRunner = agentRunner
}

// SetAgentConfig 注入 Agent 配置管理用例。
func (r *Router) SetAgentConfig(uc *agentconfig.AgentConfigUseCase) {
	r.agentCfgUC = uc
}

// SetLLMConfig 注入 LLM 配置管理用例。
func (r *Router) SetLLMConfig(uc *llmconfig.LLMConfigUseCase) {
	r.llmCfgUC = uc
}

// SetConversation 注入对话历史用例。
func (r *Router) SetConversation(uc *conversation.ConversationUseCase) {
	r.conversationUC = uc
}

// SetCrawlConfig 注入采集配置用例（保留：crawler 速率/robots 策略）。
func (r *Router) SetCrawlConfig(uc *crawlconfig.CrawlConfigUseCase) {
	r.crawlCfgUC = uc
}

// SetToolRegistry 注入全局工具注册表（供 /tools 端点查询）。
func (r *Router) SetToolRegistry(reg *port.ToolRegistry) {
	r.toolRegistry = reg
}

func (r *Router) Engine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.Use(gin.Recovery(), gin.Logger(), corsMiddleware())

	// 统一前缀（nginx 分流用；空前缀时 root 等同 e，本地开发零影响）
	root := e.Group(r.apiPrefix)

	root.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// 媒体静态托管（素材/转存产物——LocalMediaStore 的数据目录对外只读）
	if r.mediaDir != "" {
		root.Static("/media", r.mediaDir)
	}

	// 认证（公开）——核心依赖，未注入则认证端点不注册
	if r.authRegister != nil && r.authLogin != nil {
		authHandler := NewAuthHandler(r.authRegister, r.authLogin)
		authGroup := root.Group("/api/v1/auth")
		{
			authGroup.POST("/register", authHandler.HandleRegister)
			authGroup.POST("/login", authHandler.HandleLogin)
		}
	}

	// 采集政策（公开，无需认证，让外部可查询合规承诺）
	if r.crawlCfgUC != nil {
		root.GET("/api/v1/crawl-policy", NewCrawlConfigHandler(r.crawlCfgUC).HandlePolicy)
	}

	// 公开内容站（无认证——让 AI 引擎/搜索引擎可爬取已发布内容）
	if r.publicHandler != nil {
		root.GET("/public/articles/:id", r.publicHandler.GetArticleHTML)
		root.GET("/public/sitemap.xml", r.publicHandler.GetSitemapXML)
		root.GET("/public/llms.txt", r.publicHandler.GetLLMSTxt)
		root.GET("/public/store-map/:brandId", r.publicHandler.GetStoreMap)
		root.GET("/public/indexnow-key.txt", r.publicHandler.GetIndexNowKeyFile)
		// IndexNow 协议要求密钥文件在域名根目录（不加前缀）
		e.GET("/:key.txt", r.publicHandler.GetIndexNowKeyFile)
	}

	// 支付网关异步回调（公开——支付平台回调无 JWT，靠签名验证安全）
	if r.billingUC != nil {
		root.GET("/api/v1/billing/webhook/:gateway", r.HandlePaymentCallback)
		root.POST("/api/v1/billing/webhook/:gateway", r.HandlePaymentCallback)
	}

	// 业务路由（受 JWT 中间件保护）
	api := root.Group("/api/v1")
	api.Use(middleware.JWTAuth(r.tokenParser))
	{
		// AI 对话（SSE 流式）——配额检查在 SSE 头设置前，超限返回 JSON 402
		if r.ai != nil {
			chatHandler := NewChatHandler(r.ai)
			chatHandler.SetQuotaGate(r.quotaGate)
			api.POST("/chat", chatHandler.HandleStream)
		}
		// 工具面板（需 toolRegistry）
		if r.toolRegistry != nil {
			api.GET("/tools", r.handleListTools)
			api.PUT("/tools/:name/toggle", r.handleToggleTool)
		}
		// 站内通知（主动唤醒：提及率变化/自动复测/排期发布）
		if r.notifyUC != nil {
			api.GET("/notifications", r.HandleListNotifications)
			api.GET("/notifications/unread-count", r.HandleNotificationUnread)
			api.POST("/notifications/:id/read", r.HandleMarkNotificationRead)
		}
		// 仪表盘统计聚合
		if r.statsUC != nil {
			api.GET("/stats", r.handleGetStats)
		}
		// Agent 同步执行 + 异步任务投递
		if r.agentRunner != nil {
			api.POST("/agents/run", NewAgentHandler(r.agentRunner).HandleRun)
		}
		if r.enqueueUC != nil {
			taskHandler := NewTaskHandler(r.enqueueUC)
			api.POST("/tasks", taskHandler.HandleEnqueue)
		}
		// Agent 配置管理
		if r.agentCfgUC != nil {
			api.GET("/agents", r.handleListAgentConfigs)
			api.POST("/agents", r.handleCreateAgentConfig)
			api.PUT("/agents/:name", r.handleUpdateAgentConfig)
			api.DELETE("/agents/:name", r.handleDeleteAgentConfig)
		}
		// LLM 配置管理
		if r.llmCfgUC != nil {
			api.GET("/llm-configs", r.handleListLLMConfigs)
			api.POST("/llm-configs", r.handleCreateLLMConfig)
			api.PUT("/llm-configs/:name", r.handleUpdateLLMConfig)
			api.DELETE("/llm-configs/:name", r.handleDeleteLLMConfig)
		}
		// 聊天会话（按用户隔离，跨设备持久化）
		if r.conversationUC != nil {
			convHandler := NewConversationHandler(r.conversationUC)
			api.GET("/conversations", convHandler.HandleList)
			api.POST("/conversations", convHandler.HandleCreate)
			api.GET("/conversations/:id/messages", convHandler.HandleGetMessages)
			api.POST("/conversations/:id/messages", convHandler.HandleSaveMessage)
			api.PUT("/conversations/:id", convHandler.HandleRename)
			api.DELETE("/conversations/:id", convHandler.HandleDelete)
		}
		// 采集配置（运行时可调的速率/robots 开关）
		if r.crawlCfgUC != nil {
			crawlCfgHandler := NewCrawlConfigHandler(r.crawlCfgUC)
			api.GET("/crawl-config", crawlCfgHandler.HandleGet)
			api.PUT("/crawl-config", crawlCfgHandler.HandleUpdate)
		}
		// 知识搜索（向量库配置后启用）
		}

		// GEO 业务路由（商户端核心：品牌/关键词/监测/排行榜/内容）
		// geoHandler 提升到外层：管理后台的全局管理端点（/admin/brands 等）也复用它。
		var geoHandler *GEOHandler
		if r.geoBrandUC != nil {
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
			if r.inputTipper != nil {
				geoHandler.SetInputTipper(r.inputTipper)
			}
			// 品牌 CRUD（Gin 同层 wildcard 参数名必须统一，全部用 :id）
			api.GET("/geo/brands", geoHandler.HandleListBrands)
			api.POST("/geo/brands", geoHandler.HandleCreateBrand)
			api.DELETE("/geo/brands/:id", geoHandler.HandleDeleteBrand)
			api.PUT("/geo/brands/:id", geoHandler.HandleUpdateBrand) // 修改品牌信息（名称/定位/卖点/竞品/业务类型）
			// 关键词（:id 即 brandId，handler 内用 c.Param("id") 取）
			api.GET("/geo/brands/:id/keywords", geoHandler.HandleListKeywords)
			api.POST("/geo/brands/:id/keywords", geoHandler.HandleAddKeyword)
			api.POST("/geo/brands/:id/keywords/generate", geoHandler.HandleGenerateKeywords)
			// 监测
			api.POST("/geo/monitor", geoHandler.HandleMonitor)
			api.POST("/geo/monitor-keyword", geoHandler.HandleMonitorKeyword) // 单关键词即时监测
			api.POST("/geo/monitor-multi", geoHandler.HandleMonitorMultiEngine) // 多引擎批量监测
			// 商户端自动盯盘开关（租户级：我的品牌是否参与每日自动监测）
			if r.settingsUC != nil {
				api.GET("/geo/monitor-auto", r.HandleGetTenantAutoMonitor)
				api.PUT("/geo/monitor-auto", r.HandleSetTenantAutoMonitor)
			}
			api.GET("/geo/monitor/:keywordId", geoHandler.HandleLatestMonitor)
			api.GET("/geo/brands/:id/monitor-results", geoHandler.HandleLatestMonitorByBrand) // 品牌批量结果
			api.GET("/geo/monitor-results", geoHandler.HandleAllMonitorResults) // 租户全部监测结果（关键词一览页用）
			api.GET("/geo/brands/:id/overview", geoHandler.HandleBrandOverview)
			// 内容优化
			api.POST("/geo/optimize", geoHandler.HandleOptimizeContent)
			api.GET("/geo/brands/:id/contents", geoHandler.HandleListContents)
			api.POST("/geo/brands/:id/contents/generate", geoHandler.HandleGenerateContent)
			api.POST("/geo/brands/:id/contents/generate-stream", geoHandler.HandleGenerateContentStream) // SSE 流式生成
			api.POST("/geo/brands/:id/contents/:contentId/status", geoHandler.HandleSetContentStatus) // 状态流转 draft↔published
			api.DELETE("/geo/brands/:id/contents/:contentId", geoHandler.HandleDeleteContent) // 删除内容（管理后台/工作台）
			api.POST("/geo/brands/:id/contents/:contentId/resubmit-index", geoHandler.HandleResubmitIndex) // 商户端自助补提交收录
			// GEO 诊断
			api.POST("/geo/brands/:id/diagnose", geoHandler.HandleDiagnose)
			// 关键词蒸馏（按来源：品牌/文本/种子/文件/网络）
			api.POST("/geo/keywords/distill", geoHandler.HandleDistillKeywords)
			// 关键词管理（跨品牌聚合列表 + 删除）
			api.GET("/geo/keywords", geoHandler.HandleListAllKeywords)
			api.DELETE("/geo/keywords/:id", geoHandler.HandleDeleteKeyword)
			// 门店档案（本地生活 GEO 地基；路由参数 :storeId 为门店 ID）
			api.GET("/geo/brands/:id/store-locations", geoHandler.HandleListStoreLocations)
			api.POST("/geo/brands/:id/store-locations", geoHandler.HandleCreateStoreLocation)
			api.PUT("/geo/brands/:id/store-locations/:storeId", geoHandler.HandleUpdateStoreLocation)
			api.DELETE("/geo/brands/:id/store-locations/:storeId", geoHandler.HandleDeleteStoreLocation)
			api.POST("/geo/brands/:id/store-locations/:storeId/re-geocode", geoHandler.HandleReGeocodeStoreLocation)
			// 附近同行双榜（现实世界地图榜 + AI 竞品榜）
			api.GET("/geo/brands/:id/nearby-competitors", geoHandler.HandleNearbyCompetitors)
			api.GET("/geo/brands/:id/competitor-suggestions", geoHandler.HandleSuggestCompetitors) // 竞品自动推荐（附近同行 top N）
			// AI 榜单探查（v2：AI 榜真实数据源——手动刷新时强制重跑并缓存 24h）
			api.POST("/geo/brands/:id/ai-rank-probe", geoHandler.HandleAIRankProbe)
			// 行动建议（P5-05：给老板"下一步做什么"）
			api.GET("/geo/brands/:id/advice", geoHandler.HandleAdvice)
			// 内容引用统计（P5-02：每篇被 AI 引用几次）
			api.GET("/geo/brands/:id/citations", geoHandler.HandleContentCitations)
			// 地址联想（P1 输入提示：门店建档表单边输入边联想）
			api.GET("/geo/location/suggest", geoHandler.HandleSuggestLocations)
		}

		// 结构化数据端点（JSON-LD 生成 / llms.txt 生成）——纯逻辑，无 DB/LLM 依赖
		if r.structuredUC != nil {
			api.POST("/geo/structured/jsonld", r.handleGenerateJSONLD)
			api.POST("/geo/structured/llms-txt", r.handleGenerateLLMSTxt)
		}

		// 多平台发布账号域路由（扫码绑定 + 半自动发布）——通过 SetAccount 延迟注入，可选
		if r.accountUC != nil {
			accountHandler := NewAccountHandler(r.accountUC, r.publishSemiUC)
			// 账号管理
			api.GET("/geo/accounts", accountHandler.HandleListAccounts)
			api.POST("/geo/accounts/qr-login", accountHandler.HandleStartQRLogin)
			api.GET("/geo/accounts/qr-login/:sessionId", accountHandler.HandlePollQRLogin)
			api.DELETE("/geo/accounts/qr-login/:sessionId", accountHandler.HandleCancelQRLogin)
			api.DELETE("/geo/accounts/:id", accountHandler.HandleDeleteAccount)
			// 发布管理
			api.POST("/geo/publish", accountHandler.HandlePublish)
			api.GET("/geo/publish-jobs", accountHandler.HandleListPublishJobs)
			api.POST("/geo/publish-jobs/:id/published", accountHandler.HandleMarkPublished)
			api.GET("/geo/publish-jobs/:id/status", accountHandler.HandleGetJobStatus)
			api.POST("/geo/publish-jobs/:id/re-monitor", accountHandler.HandleReMonitor) // 发布效果复测（收录周期后验证提及率爬升）
		}

		// 统一生成任务（Vidu 全量接入：视频/图片/音频/数字人）
		if r.generationUC != nil {
			gh := NewGenerationHandler(r.generationUC)
			api.POST("/generation/tasks", gh.HandleSubmit)
			api.GET("/generation/tasks/:id", gh.HandleGet)
			api.GET("/generation/tasks", gh.HandleList)
			api.GET("/generation/types", gh.HandleTypes)
			api.POST("/generation/tasks/:id/cancel", gh.HandleCancel)
			// Vidu 回调入口（无需商户 token——验签保护；provider 闭包注入）
			api.POST("/generation/callback", func(c *gin.Context) {
				gh.HandleCallback(c, r.generationProvider)
			})
		}
		// 素材库（上传 + 列表 + 删除——用户图片/音频 → 本地 → URL 供 Vidu 引用）
		if r.mediaStore != nil {
			mh := NewMediaHandler(r.mediaStore)
			api.POST("/media/assets", mh.HandleUpload)
			api.GET("/media/assets", mh.HandleList)
			api.DELETE("/media/assets/:id", mh.HandleDelete)
		}

			// 管理端路由（仅 admin 角色可访问）
			if r.userRepo != nil {
				adminGroup := api.Group("/admin")
				adminGroup.Use(middleware.RequireRole("admin"))
				{
					userHandler := NewUserHandler(r.authRegister, r.userRepo)
					adminGroup.GET("/users", userHandler.HandleListUsers)
					adminGroup.POST("/users", userHandler.HandleCreateMerchant)
					adminGroup.DELETE("/users/:id", userHandler.HandleDeleteUser)
				// 全平台资源管理（admin 旁路：显式全局查询，不走商户租户上下文）
				if r.geoBrandUC != nil {
					adminGroup.GET("/brands", geoHandler.HandleAdminListBrands)
					adminGroup.GET("/contents", geoHandler.HandleAdminListContents)
					adminGroup.DELETE("/brands/:id", geoHandler.HandleAdminDeleteBrand)
					adminGroup.POST("/contents/:id/status", geoHandler.HandleAdminSetContentStatus)
					adminGroup.DELETE("/contents/:id", geoHandler.HandleAdminDeleteContent)
				}
				// Tavily 搜索 API 配置（需 toolRegistry）
				if r.toolRegistry != nil {
					adminGroup.GET("/tavily-status", r.handleTavilyStatus)
					adminGroup.PUT("/tavily-key", r.handleUpdateTavilyKey)
				}
			// 平台系统设置（运行时开关）
			if r.settingsUC != nil {
				adminGroup.GET("/settings/auto-monitor", r.HandleGetAutoMonitor)
				adminGroup.PUT("/settings/auto-monitor", r.HandleSetAutoMonitor)
			}
			// 提示词模板管理（内容生成/优化系统提示词可管理、可热更新）
			if r.promptTemplateRepo != nil {
				adminGroup.GET("/prompt-templates", r.HandleListPromptTemplates)
				adminGroup.PUT("/prompt-templates/:key", r.HandleUpdatePromptTemplate)
			}
			// 生成规格管理（Vidu 端点×模型矩阵——DB 驱动全局掌控，30s 热生效）
			if r.generationRegistry != nil && r.generationSpecRepo != nil {
				gh := NewGenerationAdminHandler(r.generationRegistry, r.generationSpecRepo)
				adminGroup.GET("/generation/specs", gh.HandleListSpecs)
				adminGroup.PUT("/generation/specs/:subType/:model", gh.HandleSaveSpec)
				adminGroup.DELETE("/generation/specs/:subType/:model", gh.HandleDeleteSpec)
			}
			// 厂商配置管理（按厂商设置 API Key——保存后对已装配厂商热生效）
			if r.providerConfigUC != nil {
				pch := NewProviderConfigHandler(r.providerConfigUC, r.generationProvider)
				adminGroup.GET("/provider-configs", pch.HandleList)
				adminGroup.PUT("/provider-configs/:provider", pch.HandleSave)
			}
			// 收录管理（运行时配置/提交日志/手动补提交）
			if r.indexingUC != nil {
				adminGroup.GET("/indexing/config", r.HandleGetIndexingConfig)
				adminGroup.PUT("/indexing/config", r.HandleUpdateIndexingConfig)
				adminGroup.GET("/indexing/logs", r.HandleListIndexingLogs)
				adminGroup.POST("/indexing/re-submit", r.HandleReSubmitAll)
				adminGroup.POST("/indexing/generate-key", r.HandleGenerateIndexingKey) // 自动生成密钥（IndexNow 所有权证明）
				adminGroup.GET("/indexing/verify-key", r.HandleVerifyIndexingKey)      // 验证 key 文件可访问
			}
			// 经济系统——套餐/订阅/订单管理（admin）
			if r.billingUC != nil {
				adminGroup.GET("/billing/plans", r.HandleAdminListPlans)
				adminGroup.POST("/billing/plans", r.HandleAdminSavePlan)
				adminGroup.DELETE("/billing/plans/:id", r.HandleAdminDeletePlan)
				adminGroup.GET("/billing/subscriptions", r.HandleAdminListSubscriptions)
				adminGroup.PUT("/billing/subscriptions/:tenant", r.HandleAdminAssignPlan) // 手动开通（线下收款）
				adminGroup.GET("/billing/orders", r.HandleAdminListOrders)
				adminGroup.GET("/billing/revenue", r.HandleAdminRevenueReport) // 收入概览
				adminGroup.GET("/billing/cost-analysis", r.HandleAdminCostAnalysis) // 成本分析（X-01：收入 vs 成本双报表）
				adminGroup.GET("/billing/payment-config", r.HandleGetPaymentConfig) // 支付网关配置
				adminGroup.PUT("/billing/payment-config", r.HandleSetPaymentConfig) // 保存支付配置
			}
			// 经济系统——商户端（我的套餐/订阅/订单，多租户隔离）
			if r.billingUC != nil {
				api.GET("/billing/plans", r.HandleListActivePlans)
				api.GET("/billing/my-plan", r.HandleGetMyPlan)
				api.GET("/billing/usage", r.HandleGetMyUsage) // 配额余量（进度条）
				api.GET("/billing/orders", r.HandleListMyOrders)
				api.POST("/billing/orders", r.HandleCreateOrder)              // 下单购买
				api.POST("/billing/orders/:id/confirm", r.HandleConfirmOrder) // 确认支付（mock 自动/真实回调）
			}
		}
	}
	return e
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

// handleListTools GET /api/v1/tools —— 返回所有工具及启用状态（工具面板用）
func (r *Router) handleListTools(c *gin.Context) {
	if r.toolRegistry == nil {
		success(c, []any{})
		return
	}
	statuses := r.toolRegistry.AllWithStatus()
	views := make([]gin.H, 0, len(statuses))
	for _, s := range statuses {
		views = append(views, gin.H{
			"name":        s.Name,
			"description": s.Description,
			"enabled":     s.Enabled,
		})
	}
	success(c, views)
}

// toolToggleRequest PUT /api/v1/tools/:name/toggle
type toolToggleRequest struct {
	Enabled bool `json:"enabled"`
}

// handleToggleTool PUT /api/v1/tools/:name/toggle —— 动态启用/禁用工具
func (r *Router) handleToggleTool(c *gin.Context) {
	if r.toolRegistry == nil {
		fail(c, fmt.Errorf("工具注册表未初始化"))
		return
	}
	name := c.Param("name")
	var req toolToggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	// 校验工具存在
	if _, ok := r.toolRegistry.Lookup(name); !ok {
		fail(c, fmt.Errorf("工具 %q 不存在", name))
		return
	}
	r.toolRegistry.SetEnabled(name, req.Enabled)
	success(c, gin.H{"name": name, "enabled": req.Enabled})
}

// handleGetStats GET /api/v1/stats —— 仪表盘统计聚合（一次返回全量指标）
func (r *Router) handleGetStats(c *gin.Context) {
	if r.statsUC == nil {
		success(c, gin.H{"totals": map[string]int{}, "status_breakdown": map[string]int{}})
		return
	}
	success(c, r.statsUC.Get(c.Request.Context()))
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// handleTavilyStatus GET /api/v1/admin/tavily-status —— 查看 Tavily 搜索配置状态
func (r *Router) handleTavilyStatus(c *gin.Context) {
	enabled := false
	hasKey := false
	if t, ok := r.toolRegistry.Lookup("tavily_search"); ok {
		// 透过 RateLimitCrawler 装饰器拿到内层的 TavilyCrawler
		// RateLimitCrawler 透传了 inner，但 Lookup 返回的是包装后的实例
		// 这里通过 AllWithStatus 查启用状态
		statuses := r.toolRegistry.AllWithStatus()
		for _, s := range statuses {
			if s.Name == "tavily_search" {
				enabled = s.Enabled
			}
		}
		_ = t
		hasKey = true // 能 Lookup 到说明注册了
	}
	success(c, gin.H{
		"registered": hasKey,
		"enabled":    enabled,
	})
}

// handleUpdateTavilyKey PUT /api/v1/admin/tavily-key —— 更新 Tavily API Key
// 注意：由于 TavilyCrawler 被 RateLimitCrawler 包装，这里只更新启用状态。
// Key 本身需要在 .env 里配置（TAVILY_API_KEY），运行时改 Key 需重启。
// 这个端点主要用于启用/禁用工具。
type tavilyKeyRequest struct {
	Enabled bool   `json:"enabled"`
	APIKey  string `json:"api_key"` // 可选：传入新 Key（后续版本支持）
}

func (r *Router) handleUpdateTavilyKey(c *gin.Context) {
	var req tavilyKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	// 更新启用状态
	r.toolRegistry.SetEnabled("tavily_search", req.Enabled)
	success(c, gin.H{
		"name":    "tavily_search",
		"enabled": req.Enabled,
		"note":    "API Key 请在 .env 文件配置 TAVILY_API_KEY，修改后重启生效",
	})
}
