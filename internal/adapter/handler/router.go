package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	authadapter "webreaper/internal/adapter/auth"
	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/usecase/agentconfig"
	"webreaper/internal/usecase/auth"
	"webreaper/internal/usecase/conversation"
	"webreaper/internal/usecase/crawlconfig"
	"webreaper/internal/usecase/dataitem"
	"webreaper/internal/usecase/llmconfig"
	"webreaper/internal/usecase/orchestrate"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/publish"
	"webreaper/internal/usecase/stats"
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
		api.DELETE("/agents/:name", r.handleDeleteAgentConfig)
		// LLM 配置（独立聚合根）
		api.GET("/llm-configs", r.handleListLLMConfigs)
		api.POST("/llm-configs", r.handleCreateLLMConfig)
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
	}
	return e
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
