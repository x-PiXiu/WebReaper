package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	authadapter "webreaper/internal/adapter/auth"
	"webreaper/internal/adapter/agent"
	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/usecase/account"
	"webreaper/internal/usecase/agentconfig"
	"webreaper/internal/usecase/auth"
	"webreaper/internal/usecase/billing"
	"webreaper/internal/usecase/conversation"
	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/geo"
	"webreaper/internal/usecase/inspiration"
	"webreaper/internal/usecase/works"
	"webreaper/internal/usecase/indexing"
	"webreaper/internal/usecase/knowledge"
	"webreaper/internal/usecase/llmconfig"
	"webreaper/internal/usecase/notification"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/providerconfig"
	"webreaper/internal/usecase/stats"
	"webreaper/internal/usecase/structured"
	"webreaper/internal/usecase/systemsettings"
	"webreaper/internal/usecase/videotranscript"
)

// Router 组装所有 HTTP 路由。
//
// 设计要点（整洁架构 / 依赖倒置）：
//   - Router 只依赖 usecase 和 port 接口，不直接持有仓储、不依赖具体 adapter struct。
//   - 这样 handler 层薄化为"DTO 转换 + 调用 usecase"，业务流程编排全部在 usecase 层。
//   - Agent 执行依赖 port.AgentSyncRunner 接口（非具体 TrpcAgentRunner），可替换。
//
// 文件组织（按域拆分，避免单文件巨型路由）：
//   router.go        —— Router 结构 + 依赖注入（SetXxx）+ Engine 骨架（公开/认证/中间件）
//   router_core.go   —— AI 对话/工具/通知/统计/Agent/LLM 配置/会话
//   router_geo.go    —— GEO 业务（品牌/关键词/监测/内容/门店/附近同行）+ 账号发布 + 结构化
//   router_admin.go  —— 管理端（admin 角色域）+ 商户端计费
type Router struct {
	authRegister   *auth.RegisterUseCase
	authLogin      *auth.LoginUseCase
	authChangePw   *auth.ChangePasswordUseCase
	tokenParser    *authadapter.JWTGenerator
	ai             port.AIGenerator
	agentCfgUC     *agentconfig.AgentConfigUseCase
	llmCfgUC       *llmconfig.LLMConfigUseCase
	conversationUC *conversation.ConversationUseCase
	toolRegistry   *port.ToolRegistry  // 全局工具注册表（供 /tools 端点查询）
	statsUC        *stats.StatsUseCase // 仪表盘统计聚合
	// GEO 业务（商户端核心）——通过 SetGEO 延迟注入，可选
	geoBrandUC    *geo.BrandUseCase
	geoMonitorUC  *geo.MonitorUseCase
	geoRankUC     *geo.RankUseCase
	geoContentUC  *geo.ContentUseCase
	geoDiagnoseUC *geo.DiagnoseUseCase
	geoDistillUC  *geo.KeywordDistillUseCase // 关键词蒸馏用例（可选）
	geoStoreUC    *geo.StoreLocationUseCase  // 门店档案用例（本地生活地基，可选）
	geoNearbyUC   *geo.NearbyUseCase         // 附近同行双榜用例（可选）
	geoAirProbeUC *geo.AIRankProbeUseCase    // AI 榜单探查用例（可选，v2：AI 榜数据源）
	geoAdviceUC   *geo.AdviceUseCase         // 行动建议用例（可选，P5-05）
	geoCitationUC *geo.CitationUseCase       // 内容引用统计用例（可选，P5-02）
	geoHealthUC   *geo.HealthUseCase         // 健康报告聚合用例（可选，v3 归位：单一事实源）
	geoIndustryUC *geo.IndustryUseCase       // 行业全景聚合用例（可选，v3 P2：admin 看板）
	metrics       port.MetricsCollector      // 运营指标采集（可选，R3；nil=不采集）
	inputTipper   port.InputTipper           // 地址联想（可选，P1；未注入→空列表降级）
	// 结构化数据用例（JSON-LD/llms.txt）——通过 SetStructured 注入，可选
	structuredUC *structured.StructuredDataUseCase
	// 公开内容站处理器——通过 SetPublic 注入，可选
	publicHandler *PublicHandler
	// 收录管理用例（运行时配置/提交日志/手动补提交）——通过 SetIndexing 注入，可选
	indexingUC *indexing.IndexingUseCase
	// 知识库用例（向量配置/行业采集配置/素材管理）——通过 SetKnowledge 注入，可选
	knowledgeUC *knowledge.KnowledgeUseCase
	// 多平台发布账号域（扫码绑定 + 半自动发布）——通过 SetAccount 延迟注入，可选
	accountUC     *account.AccountUseCase
	publishSemiUC *account.PublishUseCase
	// 用户管理（管理端）——通过 SetAdmin 延迟注入，可选
	userRepo port.UserRepository
	// 平台系统设置（运行时开关）——通过 SetSystemSettings 注入，可选
	settingsUC *systemsettings.SystemSettingsUseCase
	// 站内通知（主动唤醒）——通过 SetNotifications 注入，可选
	notifyUC *notification.NotifyUseCase
	// 统一生成（Vidu 全量接入：视频/图片/音频/数字人）
	generationUC       *generation.GenerationUseCase
	generationProvider port.GenerationProvider
	generationRegistry port.EndpointRegistry // 规格管理（管理后台矩阵）
	generationSpecRepo port.GenerationSpecRepository
	integrationRepo    integrationRepo // 能力路由新表（vendor + capability）
	generationVoices   port.VoiceLibrary      // 官方音色库（TTS/主体音色选择用；可选）
	providerConfigUC   *providerconfig.UseCase // 厂商配置（管理后台）
	mediaStore         port.MediaAssetStore    // 素材托管/转存（可选）
	transcriptUC       *videotranscript.UseCase // 视频文案提取（可选；08 计划 D4）
	composerUC         port.Composer // B-Roll 合成编排（可选；22 计划——未注入则端点不注册）
	mediaDir           string                  // 本地媒体静态目录（可选；非空时 /media 托管）
	apiPrefix          string                  // 路由统一前缀（nginx 分流用，如 /webreaper；空=无前缀）
	rootGroup          *gin.RouterGroup        // Engine() 装配时记录——延迟注册的公开路由用（OAuth 回调等）
	accountFrontendURL string                  // 账号域 OAuth 回调后 302 跳回的前端地址
	worksUC            *works.WorksUseCase     // 作品库聚合（可选；未注入则端点不注册）
	pendingPublish     *agent.PendingPublishStore // 发布计划暂存（主 Agent 硬确认；可选）
	transportRegistry  *port.TransportRegistry    // 通道轴注册表（管理后台切换端点用；可选）
	settingRepo        port.SystemSettingRepository // 系统设置仓储（通道 override 持久化用；可选）
	contentAdapters    port.ContentAdapterRegistry // 内容适配器注册表（向导适配预览；可选）
	draftCache         DraftCache                 // 向导云草稿存储（Redis；可选）
	healthCheck        func() error            // 健康检查函数（DB ping 等；nil=只返回 ok）
	// 提示词模板仓库（admin 管理内容生成/优化提示词）——通过 SetPromptTemplates 注入，可选
	promptTemplateRepo port.PromptTemplateRepository
	// 主体资产仓储（26 号计划——资产读路径；可选）
	subjectAssetRepo port.SubjectAssetRepository
	// 经济系统（套餐/订阅/订单/计费）——通过 SetBilling 注入，可选
	billingUC *billing.BillingUseCase
	// 配额检查门（注入到 ChatHandler 等无独立 usecase 的端点）——通过 SetQuotaGate 注入，可选
	quotaGate port.QuotaStore
	// 模板管理用例（管理后台可动态配置生成模板）——通过 SetTemplate 注入，可选
	templateUC *generation.TemplateUseCase
	// 灵感广场用例（热门视频采集+展示）——通过 SetInspiration 注入，可选
	inspirationUC       *inspiration.UseCase
	inspirationVideoRepo port.InspirationVideoRepository
	brandRepo            port.BrandRepository // 用于灵感数据租户隔离校验
	// 爬虫管理用例（管理后台爬虫配置/账号管理）——通过 SetCrawlerAdmin 注入，可选
	crawlerConfigRepo   port.CrawlerConfigRepository
	crawlerAccountRepo  port.CrawlerAccountRepository
	crawlerTaskLogRepo  port.CrawlerTaskLogRepository
	crawlerVault        port.CookieVault // cookie 加解密（账号健康检测解密/手动添加加密）
	// 品牌发布配置——通过 SetPublishConfig 注入，可选
	publishConfigRepo  port.BrandPublishConfigRepository
	publishBindingRepo port.AccountBrandBindingRepository
	publishUsageRepo   port.PublishUsageRepository
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

// SetKnowledge 注入平台知识库用例（可选；未注入则知识库管理端点不注册）。
func (r *Router) SetKnowledge(uc *knowledge.KnowledgeUseCase) {
	r.knowledgeUC = uc
}

// SetGEO 注入 GEO 业务用例（可选；未注入则 GEO 端点不注册）。
func (r *Router) SetGEO(brand *geo.BrandUseCase, monitor *geo.MonitorUseCase, rank *geo.RankUseCase, content *geo.ContentUseCase, diagnose *geo.DiagnoseUseCase) {
	r.geoBrandUC = brand
	r.geoMonitorUC = monitor
	r.geoRankUC = rank
	r.geoContentUC = content
	r.geoDiagnoseUC = diagnose
}

// SetGEOHealth 注入健康报告聚合用例（可选；未注入则健康报告端点不注册）。
func (r *Router) SetGEOHealth(uc *geo.HealthUseCase) {
	r.geoHealthUC = uc
}

// SetGEOIndustry 注入行业全景聚合用例（可选；未注入则行业看板端点不注册）。
func (r *Router) SetGEOIndustry(uc *geo.IndustryUseCase) {
	r.geoIndustryUC = uc
}

// SetMetrics 注入运营指标采集器（可选，R3；未注入则 /debug/metrics 返回空）。
func (r *Router) SetMetrics(m port.MetricsCollector) {
	r.metrics = m
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

// SetTransportRegistry 注入通道轴注册表（可选；管理后台通道切换端点用）。
func (r *Router) SetTransportRegistry(tr *port.TransportRegistry, sr port.SystemSettingRepository) {
	r.transportRegistry = tr
	r.settingRepo = sr
}

// SetPendingPublishStore 注入发布计划暂存（可选；主 Agent 硬确认卡片端点用）。
func (r *Router) SetPendingPublishStore(ps *agent.PendingPublishStore) {
	r.pendingPublish = ps
}

// SetWorks 注入作品库聚合用例（可选）。
func (r *Router) SetWorks(uc *works.WorksUseCase) {
	r.worksUC = uc
}

// SetAccount 注入多平台发布账号域用例（可选；未注入则账号/发布端点不注册）。
// frontendBaseURL：OAuth 授权回调完成后 302 跳回的前端地址。
func (r *Router) SetAccount(au *account.AccountUseCase, pu *account.PublishUseCase, frontendBaseURL string) {
	r.accountUC = au
	r.publishSemiUC = pu
	r.accountFrontendURL = frontendBaseURL
}

// SetPublishWizard 注入向导配套服务（可选；未注入则适配预览/云草稿端点报"未配置"，
// 前端自动降级本地规则/localStorage）。
func (r *Router) SetPublishWizard(adapters port.ContentAdapterRegistry, dc DraftCache) {
	r.contentAdapters = adapters
	r.draftCache = dc
}

// SetGeneration 注入统一生成用例（可选；Vidu 全量接入——未注入则生成端点不注册）。
func (r *Router) SetGeneration(uc *generation.GenerationUseCase, provider port.GenerationProvider, registry port.EndpointRegistry, specRepo port.GenerationSpecRepository) {
	r.generationUC = uc
	r.generationProvider = provider
	r.generationRegistry = registry
	r.generationSpecRepo = specRepo
}

// SetTemplate 注入模板管理用例（可选；未注入则模板端点不注册）。
func (r *Router) SetTemplate(uc *generation.TemplateUseCase) {
	r.templateUC = uc
}

// SetGenerationVoices 注入官方音色库（可选——未注入则 /generation/voices 不返回数据）。
func (r *Router) SetGenerationVoices(v port.VoiceLibrary) {
	r.generationVoices = v
}

// SetSubjectAssetRepo 注入主体资产仓储（可选——26 号计划读路径；未注入则 /subjects/mine 不注册）。
func (r *Router) SetSubjectAssetRepo(repo port.SubjectAssetRepository) {
	r.subjectAssetRepo = repo
}

// SetComposer 注入 B-Roll 合成编排（可选——未注入则 timeline/compose 端点不注册）。
func (r *Router) SetComposer(c port.Composer) {
	r.composerUC = c
}

// SetTranscript 注入视频文案提取用例（可选——未注入则提取端点不注册）。
func (r *Router) SetTranscript(uc *videotranscript.UseCase) {
	r.transcriptUC = uc
}

// SetIntegrationRepo 注入能力路由仓储（可选——未注入则集成中心 vendor/capability 管理端点不注册）。
func (r *Router) SetIntegrationRepo(repo integrationRepo) {
	r.integrationRepo = repo
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

// SetHealthCheck 注入健康检查函数（healthz 端点调用——检查 DB 连通性等）。
func (r *Router) SetHealthCheck(fn func() error) {
	r.healthCheck = fn
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

// registerInspirationRoutes 注册灵感广场路由（用户端，无需登录）。
func (r *Router) registerInspirationRoutes(api *gin.RouterGroup) {
	if r.inspirationUC == nil {
		return
	}
	ih := NewInspirationHandler(r.inspirationUC, r.inspirationVideoRepo)
	if r.brandRepo != nil {
		ih.SetBrandRepo(r.brandRepo)
	}
	api.GET("/inspirations", ih.HandleList)
	api.GET("/inspirations/:id", ih.HandleGet)
	api.GET("/inspirations/platforms", ih.HandleListPlatforms)
	api.GET("/inspirations/brands", ih.HandleBrandsStats)
}

// registerCrawlerAdminRoutes 注册爬虫管理路由（管理后台，需要 admin 角色）。
func (r *Router) registerCrawlerAdminRoutes(api *gin.RouterGroup) {
	if r.crawlerConfigRepo == nil && r.crawlerAccountRepo == nil && r.inspirationUC == nil {
		return
	}
	adminGroup := api.Group("/admin")
	adminGroup.Use(middleware.RequireRole("admin"))

	cah := NewCrawlerAdminHandler(r.inspirationUC, r.crawlerConfigRepo, r.crawlerAccountRepo, r.crawlerTaskLogRepo, r.crawlerVault)

	// 平台方账号管理
	adminGroup.GET("/crawler-accounts", cah.HandleListAccounts)
	adminGroup.POST("/crawler-accounts", cah.HandleCreateAccount)
	adminGroup.PUT("/crawler-accounts/:id", cah.HandleUpdateAccount)
	adminGroup.DELETE("/crawler-accounts/:id", cah.HandleDeleteAccount)
	adminGroup.POST("/crawler-accounts/:id/health", cah.HandleCheckAccountHealth)

	// 爬虫配置管理
	adminGroup.GET("/crawlers", cah.HandleListConfigs)
	adminGroup.GET("/crawlers/:platform", cah.HandleGetConfig)
	adminGroup.PUT("/crawlers/:platform", cah.HandleUpdateConfig)
	adminGroup.POST("/crawlers/:platform/test", cah.HandleTestConnection)
	adminGroup.POST("/crawlers/:platform/trigger", cah.HandleTriggerCrawl)
	adminGroup.POST("/crawlers/:platform/refresh-metrics", cah.HandleRefreshMetrics)

	// 任务监控
	adminGroup.GET("/crawlers/tasks", cah.HandleListTasks)
	adminGroup.GET("/crawlers/tasks/:id", cah.HandleGetTask)

	// 灵感内容管理（审核/统计）
	if r.inspirationUC != nil && r.inspirationVideoRepo != nil {
		ih := NewInspirationHandler(r.inspirationUC, r.inspirationVideoRepo)
		adminGroup.PUT("/inspirations/:id", ih.HandleUpdateInspiration)
		adminGroup.DELETE("/inspirations/:id", ih.HandleDeleteInspiration)
		adminGroup.POST("/inspirations/batch", ih.HandleBatchInspirations)
		adminGroup.GET("/inspirations/stats", ih.HandleStats)
	}
}

// registerPublishConfigRoutes 注册品牌发布配置路由（商户端，需要登录）。
func (r *Router) registerPublishConfigRoutes(api *gin.RouterGroup) {
	if r.publishConfigRepo == nil && r.publishBindingRepo == nil {
		return
	}
	pch := NewPublishConfigHandler(r.publishConfigRepo, r.publishBindingRepo, r.publishUsageRepo)

	// 品牌发布配置
	api.GET("/merchant/brands/:id/publish-config", pch.HandleGetBrandPublishConfig)
	api.PUT("/merchant/brands/:id/publish-config", pch.HandleUpdateBrandPublishConfig)
	api.DELETE("/merchant/brands/:id/publish-config/:platform", pch.HandleDeleteBrandPublishConfig)

	// 账号品牌绑定
	api.POST("/merchant/brands/:id/publish-config/bindings", pch.HandleBindAccount)
	api.DELETE("/merchant/brands/:id/publish-config/bindings/:accountId", pch.HandleUnbindAccount)

	// 发布统计
	api.GET("/merchant/brands/:id/publish-stats", pch.HandleGetPublishStats)
}

// SetInspiration 注入灵感广场用例（可选；未注入则灵感端点不注册）。
func (r *Router) SetInspiration(uc *inspiration.UseCase, videoRepo port.InspirationVideoRepository, brandRepo port.BrandRepository) {
	r.inspirationUC = uc
	r.inspirationVideoRepo = videoRepo
	r.brandRepo = brandRepo
}

// SetCrawlerAdmin 注入爬虫管理仓储（可选；未注入则爬虫管理端点不注册）。
func (r *Router) SetCrawlerAdmin(configRepo port.CrawlerConfigRepository, accountRepo port.CrawlerAccountRepository, taskLogRepo port.CrawlerTaskLogRepository, vault port.CookieVault) {
	r.crawlerConfigRepo = configRepo
	r.crawlerAccountRepo = accountRepo
	r.crawlerTaskLogRepo = taskLogRepo
	r.crawlerVault = vault
}

// SetPublishConfig 注入品牌发布配置仓储（可选；未注入则发布配置端点不注册）。
func (r *Router) SetPublishConfig(configRepo port.BrandPublishConfigRepository, bindingRepo port.AccountBrandBindingRepository, usageRepo port.PublishUsageRepository) {
	r.publishConfigRepo = configRepo
	r.publishBindingRepo = bindingRepo
	r.publishUsageRepo = usageRepo
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

// SetAuthChangePassword 注入改密用例（可选，F1-5；未注入则改密端点不注册）。
func (r *Router) SetAuthChangePassword(uc *auth.ChangePasswordUseCase) {
	r.authChangePw = uc
}

// SetAI 注入 AI 生成器（对话/工具调用）。
func (r *Router) SetAI(ai port.AIGenerator) {
	r.ai = ai
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

// SetToolRegistry 注入全局工具注册表（供 /tools 端点查询）。
func (r *Router) SetToolRegistry(reg *port.ToolRegistry) {
	r.toolRegistry = reg
}

// Engine 组装全部路由：公开端点（healthz/媒体/认证/公开内容站/支付回调）
// + 受保护业务端点（JWT + IP 限流），后者按域委托给 router_{core,geo,admin}.go。
func (r *Router) Engine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.Use(gin.Recovery(), gin.Logger(), corsMiddleware())

	// 统一前缀（nginx 分流用；空前缀时 root 等同 e，本地开发零影响）
	root := e.Group(r.apiPrefix)
	r.rootGroup = root

	root.GET("/healthz", func(c *gin.Context) {
		if r.healthCheck != nil {
			if err := r.healthCheck(); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 媒体静态托管（素材/转存产物——LocalMediaStore 的数据目录对外只读）
	if r.mediaDir != "" {
		root.Static("/media", r.mediaDir)
	}

	// 认证（公开）——核心依赖，未注入则认证端点不注册
	if r.authRegister != nil && r.authLogin != nil {
		authHandler := NewAuthHandler(r.authRegister, r.authLogin, r.authChangePw)
		authGroup := root.Group("/api/v1/auth")
		{
			authGroup.POST("/register", authHandler.HandleRegister)
			authGroup.POST("/login", authHandler.HandleLogin)
		}
	}

	// 公开内容站（无认证——让 AI 引擎/搜索引擎可爬取已发布内容）
	// 注意：公开站路由挂 e（无 apiPrefix 前缀）——URL 是给外部爬虫/AI 引擎用的
	// 资产，必须干净（https://geo.zhichen.chat/public/articles/xxx），
	// 与 IndexNow key 文件（/:key.txt）同一设计原则；nginx 用 location /public/ 反代。
	if r.publicHandler != nil {
		// 文章列表页（站点"首页/目录"——爬虫与用户从单一入口发现全部文章）
		e.GET("/public", r.publicHandler.GetPublicIndex)
		e.GET("/public/", r.publicHandler.GetPublicIndex)
		e.GET("/public/articles/:id", r.publicHandler.GetArticleHTML)
		e.GET("/public/sitemap.xml", r.publicHandler.GetSitemapXML)
		e.GET("/public/llms.txt", r.publicHandler.GetLLMSTxt)
		e.GET("/public/store-map/:brandId", r.publicHandler.GetStoreMap)
		e.GET("/public/indexnow-key.txt", r.publicHandler.GetIndexNowKeyFile)
		// robots.txt 协议要求域名根目录（爬虫访问规则 + sitemap 指向）
		e.GET("/robots.txt", r.publicHandler.GetRobotsTXT)
		// IndexNow 协议要求密钥文件在域名根目录（不加前缀）
		e.GET("/:key.txt", r.publicHandler.GetIndexNowKeyFile)
	}

	// 支付网关异步回调（公开——支付平台回调无 JWT，靠签名验证安全）
	if r.billingUC != nil {
		root.GET("/api/v1/billing/webhook/:gateway", r.HandlePaymentCallback)
		root.POST("/api/v1/billing/webhook/:gateway", r.HandlePaymentCallback)
	}

	// Vidu 生成回调（公开——服务商回调无 JWT，靠 HMAC 验签+nonce 防重放保护；
	// 修复：此前误注册在 JWT 保护组内，真实回调无 Authorization 头会被 401 拒绝，
	// 回调通道实际不可用、全靠 20s 轮询兜底）
	if r.generationUC != nil && r.generationProvider != nil {
		gh := NewGenerationHandler(r.generationUC)
		root.POST("/api/v1/generation/callback", func(c *gin.Context) {
			gh.HandleCallback(c, r.generationProvider)
		})
	}

	// 业务路由（受 JWT 中间件保护）
	api := root.Group("/api/v1")
	api.Use(middleware.JWTAuth(r.tokenParser))
	// API 限流（令牌桶，IP 维度——防恶意高频调用耗尽 LLM 配额）
	api.Use(middleware.NewRateLimiter(20, 40).Middleware())

	// 改密（JWT 保护——当前登录用户；F1-5 默认弱口令治理闭环）
	if r.authChangePw != nil {
		pwHandler := NewAuthHandler(nil, nil, r.authChangePw)
		api.PUT("/auth/password", pwHandler.HandleChangePassword)
	}

	r.registerCoreRoutes(api)
	// geoHandler 提升到外层：管理后台的全局管理端点（/admin/brands 等）也复用它。
	geoHandler := r.registerGEORoutes(api)
	r.registerStructuredRoutes(api)
	r.registerAccountRoutes(api)
	r.registerGenerationRoutes(api)
	r.registerMediaRoutes(api)
	r.registerAgentRoutes(api)
	r.registerMerchantBillingRoutes(api)
	r.registerInspirationRoutes(api)
	r.registerCrawlerAdminRoutes(api)
	r.registerPublishConfigRoutes(api)
	r.registerAdminRoutes(api, geoHandler)
	return e
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
