package handler

import (
	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/conversation"
)

// ConversationHandler 聊天会话的 HTTP 适配器（薄 handler）。
//
// userID 从 JWT 中间件注入的 gin.Context 读取（按用户隔离会话）。
type ConversationHandler struct {
	uc *conversation.ConversationUseCase
}

func NewConversationHandler(uc *conversation.ConversationUseCase) *ConversationHandler {
	return &ConversationHandler{uc: uc}
}

// currentUserID 从 gin.Context 取 JWT 注入的 user_id。
func currentUserID(c *gin.Context) string {
	if v, ok := c.Get("user_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func conversationToView(c entity.Conversation) gin.H {
	return gin.H{
		"id":         c.ID,
		"title":      c.Title,
		"agent_name": c.AgentName,
		"user_id":    c.UserID,
		"created_at": c.CreatedAt,
		"updated_at": c.UpdatedAt,
	}
}

func messageToView(m entity.Message) gin.H {
	return gin.H{
		"id":              m.ID,
		"conversation_id": m.ConversationID,
		"role":            m.Role,
		"content":         m.Content,
		"tool_calls":      m.ToolCallsJSON,
		"created_at":      m.CreatedAt,
	}
}

// HandleList GET /api/v1/conversations —— 列出当前用户会话
func (h *ConversationHandler) HandleList(c *gin.Context) {
	convs, err := h.uc.List(c.Request.Context(), currentUserID(c))
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(convs))
	for _, cv := range convs {
		views = append(views, conversationToView(cv))
	}
	success(c, views)
}

// HandleCreate POST /api/v1/conversations —— 创建会话
func (h *ConversationHandler) HandleCreate(c *gin.Context) {
	var req struct {
		ID        string `json:"id" binding:"required"`
		Title     string `json:"title"`
		AgentName string `json:"agent_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	conv, err := h.uc.Create(c.Request.Context(), conversation.CreateInput{
		ID: req.ID, Title: req.Title, AgentName: req.AgentName, UserID: currentUserID(c),
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, conversationToView(conv))
}

// HandleGetMessages GET /api/v1/conversations/:id/messages —— 拉取会话消息
func (h *ConversationHandler) HandleGetMessages(c *gin.Context) {
	id := c.Param("id")
	msgs, err := h.uc.GetMessages(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(msgs))
	for _, m := range msgs {
		views = append(views, messageToView(m))
	}
	success(c, views)
}

// HandleSaveMessage POST /api/v1/conversations/:id/messages —— 保存一条消息
func (h *ConversationHandler) HandleSaveMessage(c *gin.Context) {
	convID := c.Param("id")
	var req struct {
		ID            string `json:"id" binding:"required"`
		Role          string `json:"role" binding:"required"`
		Content       string `json:"content"`
		ToolCallsJSON string `json:"tool_calls"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	msg := entity.Message{
		ID: req.ID, ConversationID: convID, Role: req.Role,
		Content: req.Content, ToolCallsJSON: req.ToolCallsJSON,
	}
	if err := h.uc.SaveMessage(c.Request.Context(), msg); err != nil {
		fail(c, err)
		return
	}
	success(c, messageToView(msg))
}

// HandleRename PUT /api/v1/conversations/:id —— 重命名会话
func (h *ConversationHandler) HandleRename(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Title string `json:"title" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := h.uc.Rename(c.Request.Context(), id, req.Title); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"id": id, "title": req.Title})
}

// HandleDelete DELETE /api/v1/conversations/:id —— 删除会话（级联删消息）
func (h *ConversationHandler) HandleDelete(c *gin.Context) {
	id := c.Param("id")
	if err := h.uc.Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": id})
}
