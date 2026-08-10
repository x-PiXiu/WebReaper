package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	authadapter "webreaper/internal/adapter/auth"
	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/agentconfig"
	"webreaper/internal/usecase/account"
	"webreaper/internal/usecase/auth"
	"webreaper/internal/usecase/conversation"
	"webreaper/internal/usecase/crawlconfig"
	"webreaper/internal/usecase/dataitem"
	"webreaper/internal/usecase/geo"
	"webreaper/internal/usecase/indexing"
	"webreaper/internal/usecase/llmconfig"
	"webreaper/internal/usecase/orchestrate"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/publish"
	"webreaper/internal/usecase/stats"
	"webreaper/internal/usecase/structured"
	taskquery "webreaper/internal/usecase/taskquery"
	taskuc "webreaper/internal/usecase/task"
)

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
	taskQueryUC      *taskquery.TaskQueryUseCase
	dataItemUC       *dataitem.DataItemUseCase
	agentCfgUC       *agentconfig.AgentConfigUseCase
	llmCfgUC         *llmconfig.LLMConfigUseCase
	conversationUC   *conversation.ConversationUseCase
	crawlCfgUC       *crawlconfig.CrawlConfigUseCase
	publishUC        *publish.PublishUseCase
	sysCfgUC         *publish.SystemConfigUseCase
	toolRegistry     *port.ToolRegistry // 全局工具注册表（供 /tools 端点查询）
	knowledgeSearch  port.KnowledgeSearcher // 可为 nil（未配置向量库时降级）
	orchestrateUC    *orchestrate.OrchestratorUseCase // 可为 nil（未配置编排器时该端点 503）
	statsUC          *stats.StatsUseCase               // 仪表盘统计聚合
	// GEO 业务（商户端核心）——通过 SetGEO 延迟注入，可选
	geoBrandUC   *geo.BrandUseCase
	geoMonitorUC *geo.MonitorUseCase
	geoRankUC    *geo.RankUseCase
	geoContentUC *geo.ContentUseCase
	geoDiagnoseUC *geo.DiagnoseUseCase
	geoDistillUC *geo.KeywordDistillUseCase // 关键词蒸馏用例（可选）
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
}

// SetKeywordDistill 注入关键词蒸馏用例（可选；未注入则蒸馏端点不注册）。
func (r *Router) SetKeywordDistill(uc *geo.KeywordDistillUseCase) {
	r.geoDistillUC = uc
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

// SetAccount 注入多平台发布账号域用例（可选；未注入则账号/发布端点不注册）。
func (r *Router) SetAccount(au *account.AccountUseCase, pu *account.PublishUseCase) {
	r.accountUC = au
	r.publishSemiUC = pu
}

func NewRouter(
	registerUC *auth.RegisterUseCase,
	loginUC *auth.LoginUseCase,
	tokenParser *authadapter.JWTGenerator,
	ai port.AIGenerator,
	enqueueUC *taskuc.EnqueueUseCase,
	agentRunner port.AgentSyncRunner,
	taskQueryUC *taskquery.TaskQueryUseCase,
	dataItemUC *dataitem.DataItemUseCase,
	agentCfgUC *agentconfig.AgentConfigUseCase,
	llmCfgUC *llmconfig.LLMConfigUseCase,
	conversationUC *conversation.ConversationUseCase,
	crawlCfgUC *crawlconfig.CrawlConfigUseCase,
	publishUC *publish.PublishUseCase,
	sysCfgUC *publish.SystemConfigUseCase,
	toolRegistry *port.ToolRegistry,
	knowledgeSearch port.KnowledgeSearcher,
	orchestrateUC *orchestrate.OrchestratorUseCase,
	statsUC *stats.StatsUseCase,
) *Router {
	return &Router{
		authRegister: registerUC, authLogin: loginUC, tokenParser: tokenParser,
		ai: ai, enqueueUC: enqueueUC, agentRunner: agentRunner,
		taskQueryUC: taskQueryUC, dataItemUC: dataItemUC,
		agentCfgUC: agentCfgUC, llmCfgUC: llmCfgUC,
		conversationUC: conversationUC, crawlCfgUC: crawlCfgUC,
		publishUC: publishUC, sysCfgUC: sysCfgUC,
		toolRegistry: toolRegistry, knowledgeSearch: knowledgeSearch,
		orchestrateUC: orchestrateUC,
		statsUC:       statsUC,
	}
}

func (r *Router) Engine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.Use(gin.Recovery(), gin.Logger(), corsMiddleware())

	e.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// 认证（公开）
	authHandler := NewAuthHandler(r.authRegister, r.authLogin)
	authGroup := e.Group("/api/v1/auth")
	{
		authGroup.POST("/register", authHandler.HandleRegister)
		authGroup.POST("/login", authHandler.HandleLogin)
	}

	// 采集政策（公开，无需认证，让外部可查询合规承诺）
	e.GET("/api/v1/crawl-policy", NewCrawlConfigHandler(r.crawlCfgUC).HandlePolicy)

	// 公开内容站（无认证——让 AI 引擎/搜索引擎可爬取已发布内容）
	// 通过 SetPublic 注入；未注入则公开端点不注册。
	if r.publicHandler != nil {
		e.GET("/public/articles/:id", r.publicHandler.GetArticleHTML)
		e.GET("/public/sitemap.xml", r.publicHandler.GetSitemapXML)
		e.GET("/public/llms.txt", r.publicHandler.GetLLMSTxt)
		e.GET("/public/indexnow-key.txt", r.publicHandler.GetIndexNowKeyFile)
	}

	// 业务路由（受 JWT 中间件保护）
	api := e.Group("/api/v1")
	api.Use(middleware.JWTAuth(r.tokenParser))
	{
		// AI 对话（SSE 流式）
		api.POST("/chat", NewChatHandler(r.ai).HandleStream)
		// 全局工具列表（供前端查看实际可用工具，含启用状态）
		api.GET("/tools", r.handleListTools)
		// 动态启用/禁用工具（工具面板用）
		api.PUT("/tools/:name/toggle", r.handleToggleTool)
		// 仪表盘统计聚合（一次返回全量指标）
		api.GET("/stats", r.handleGetStats)
		// Agent 任务（同步执行）
		api.POST("/agents/run", NewAgentHandler(r.agentRunner).HandleRun)
		// 异步任务
		taskHandler := NewTaskHandler(r.enqueueUC)
		api.POST("/tasks", taskHandler.HandleEnqueue)
		api.GET("/tasks", r.handleListTasks)
		api.GET("/tasks/:id", r.handleGetTask)
		// 数据项（审核编排下沉到 dataItemUC）
		api.GET("/data-items", r.handleListDataItems)
		api.POST("/data-items/:id/approve", r.handleApproveItem)
		api.POST("/data-items/:id/reject", r.handleRejectItem)
		api.POST("/data-items/from-content", r.handleCreateFromContent)
		api.DELETE("/data-items/:id", r.handleDeleteItem)
		// 采集集合
		api.GET("/collections", r.handleListCollections)
		// Agent 配置
		api.GET("/agents", r.handleListAgentConfigs)
		api.POST("/agents", r.handleCreateAgentConfig)
		api.PUT("/agents/:name", r.handleUpdateAgentConfig)
		api.DELETE("/agents/:name", r.handleDeleteAgentConfig)
		// LLM 配置（独立聚合根）
		api.GET("/llm-configs", r.handleListLLMConfigs)
		api.POST("/llm-configs", r.handleCreateLLMConfig)
		api.PUT("/llm-configs/:name", r.handleUpdateLLMConfig)
		api.DELETE("/llm-configs/:name", r.handleDeleteLLMConfig)
		// 聊天会话（按用户隔离，跨设备持久化）
		convHandler := NewConversationHandler(r.conversationUC)
		api.GET("/conversations", convHandler.HandleList)
		api.POST("/conversations", convHandler.HandleCreate)
		api.GET("/conversations/:id/messages", convHandler.HandleGetMessages)
		api.POST("/conversations/:id/messages", convHandler.HandleSaveMessage)
		api.PUT("/conversations/:id", convHandler.HandleRename)
		api.DELETE("/conversations/:id", convHandler.HandleDelete)
		// 采集配置（运行时可调的速率/robots 开关）
		crawlCfgHandler := NewCrawlConfigHandler(r.crawlCfgUC)
		api.GET("/crawl-config", crawlCfgHandler.HandleGet)
		api.PUT("/crawl-config", crawlCfgHandler.HandleUpdate)
		// 外部系统推送（动态配置目标系统 + 推送 + 推送记录）
		publishHandler := NewPublishHandler(r.publishUC, r.sysCfgUC)
		api.GET("/external-systems", publishHandler.HandleListSystems)
		api.POST("/external-systems", publishHandler.HandleCreateSystem)
		api.DELETE("/external-systems/:name", publishHandler.HandleDeleteSystem)
		api.POST("/external-systems/publish", publishHandler.HandlePublish)
		api.GET("/data-items/:id/publish-records", publishHandler.HandlePublishRecords)
		// 知识搜索
		api.GET("/search", r.handleSearch)
		// 框架内容编排（图编排：探查→生成→校验→补生成，落库不推送）
		if r.orchestrateUC != nil {
			orchHandler := NewOrchestrationHandler(r.orchestrateUC)
			api.POST("/orchestrations", orchHandler.HandleOrchestrate)
		}

		// GEO 业务路由（商户端核心：品牌/关键词/监测/排行榜/内容）
		if r.geoBrandUC != nil {
			geoHandler := NewGEOHandler(r.geoBrandUC, r.geoMonitorUC, r.geoRankUC, r.geoContentUC, r.geoDiagnoseUC)
			// 注入关键词蒸馏能力（可选）
			if r.geoDistillUC != nil {
				geoHandler.SetDistillUC(r.geoDistillUC)
			}
			// 品牌 CRUD（Gin 同层 wildcard 参数名必须统一，全部用 :id）
			api.GET("/geo/brands", geoHandler.HandleListBrands)
			api.POST("/geo/brands", geoHandler.HandleCreateBrand)
			api.DELETE("/geo/brands/:id", geoHandler.HandleDeleteBrand)
			// 关键词（:id 即 brandId，handler 内用 c.Param("id") 取）
			api.GET("/geo/brands/:id/keywords", geoHandler.HandleListKeywords)
			api.POST("/geo/brands/:id/keywords", geoHandler.HandleAddKeyword)
			api.POST("/geo/brands/:id/keywords/generate", geoHandler.HandleGenerateKeywords)
			// 监测
			api.POST("/geo/monitor", geoHandler.HandleMonitor)
			api.POST("/geo/monitor-keyword", geoHandler.HandleMonitorKeyword) // 单关键词即时监测
			api.POST("/geo/monitor-multi", geoHandler.HandleMonitorMultiEngine) // 多引擎批量监测
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
			// GEO 诊断
			api.POST("/geo/brands/:id/diagnose", geoHandler.HandleDiagnose)
			// 关键词蒸馏（按来源：品牌/文本/种子/文件/网络）
			api.POST("/geo/keywords/distill", geoHandler.HandleDistillKeywords)
			// 关键词管理（跨品牌聚合列表 + 删除）
			api.GET("/geo/keywords", geoHandler.HandleListAllKeywords)
			api.DELETE("/geo/keywords/:id", geoHandler.HandleDeleteKeyword)
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

		// 管理端路由（仅 admin 角色可访问）
		if r.userRepo != nil {
			adminGroup := api.Group("/admin")
			adminGroup.Use(middleware.RequireRole("admin"))
			{
				userHandler := NewUserHandler(r.authRegister, r.userRepo)
				adminGroup.GET("/users", userHandler.HandleListUsers)
				adminGroup.POST("/users", userHandler.HandleCreateMerchant)
				adminGroup.DELETE("/users/:id", userHandler.HandleDeleteUser)
			// Tavily 搜索 API 配置（管理后台用）
			adminGroup.GET("/tavily-status", r.handleTavilyStatus)
			adminGroup.PUT("/tavily-key", r.handleUpdateTavilyKey)
			// 收录管理（运行时配置/提交日志/手动补提交）
			if r.indexingUC != nil {
				adminGroup.GET("/indexing/config", r.HandleGetIndexingConfig)
				adminGroup.PUT("/indexing/config", r.HandleUpdateIndexingConfig)
				adminGroup.GET("/indexing/logs", r.HandleListIndexingLogs)
				adminGroup.POST("/indexing/re-submit", r.HandleReSubmitAll)
			}
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
