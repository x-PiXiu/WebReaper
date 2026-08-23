package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/usecase/agent"
)

// AgentHandler 智能体 API。
type AgentHandler struct {
	orchestrator *agent.AgentOrchestrator
}

func NewAgentHandler(orchestrator *agent.AgentOrchestrator) *AgentHandler {
	return &AgentHandler{orchestrator: orchestrator}
}

// HandleChat POST /api/v1/agent/chat —— 智能体对话。
//
// 请求体：
//   - message：用户消息
//   - permission_level：权限级别（可选，默认 semi_auto）
//
// 响应：
//   - reply：AI回复
//   - tools_used：使用的工具列表
func (h *AgentHandler) HandleChat(c *gin.Context) {
	if h.orchestrator == nil {
		fail(c, fmt.Errorf("智能体未配置"))
		return
	}

	var req struct {
		Message         string `json:"message" binding:"required"`
		PermissionLevel string `json:"permission_level"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	// 默认权限级别
	if req.PermissionLevel == "" {
		req.PermissionLevel = "semi_auto"
	}

	tenantID := middleware.CurrentTenantID(c)

	reply, err := h.orchestrator.Execute(c.Request.Context(), tenantID, req.PermissionLevel, req.Message)
	if err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{
		"reply": reply,
	})
}
