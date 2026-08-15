package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// registerCoreRoutes 基础能力路由：AI 对话 / 工具面板 / 站内通知 / 仪表盘统计 /
// Agent 配置 / LLM 配置 / 聊天会话（均挂 JWT 保护组）。
func (r *Router) registerCoreRoutes(api *gin.RouterGroup) {
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
	// Agent 配置管理
	if r.agentCfgUC != nil {
		api.GET("/agents", r.handleListAgentConfigs)
		api.POST("/agents", r.handleCreateAgentConfig)
		api.PUT("/agents/:name", r.handleUpdateAgentConfig)
		api.DELETE("/agents/:name", r.handleDeleteAgentConfig)
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
