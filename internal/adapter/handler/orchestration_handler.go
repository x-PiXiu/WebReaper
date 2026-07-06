package handler

import (
	"github.com/gin-gonic/gin"

	"webreaper/internal/usecase/orchestrate"
)

// OrchestrationHandler 是框架内容编排的 HTTP 适配器（薄 handler）。
//
// 依赖 OrchestratorUseCase（而非 adapter 的 ContentOrchestrator 实现），
// 遵循 handler→usecase 的依赖方向。
type OrchestrationHandler struct {
	uc *orchestrate.OrchestratorUseCase
}

func NewOrchestrationHandler(uc *orchestrate.OrchestratorUseCase) *OrchestrationHandler {
	return &OrchestrationHandler{uc: uc}
}

// OrchestrateRequest POST /api/v1/orchestrations
type OrchestrateRequest struct {
	Topic       string `json:"topic" binding:"required"`        // 主题，如 "trpc-agent-go 框架"
	ContentType string `json:"content_type"`                    // "interview_questions"（默认）
}

// HandleOrchestrate POST /api/v1/orchestrations —— 编排生成内容并落库（不推送）
//
// 首版同步返回。由于图编排可能耗时较长（多轮 LLM 调用），调用方需设较长超时。
// 后续可改为异步任务 + SSE 进度上报。
func (h *OrchestrationHandler) HandleOrchestrate(c *gin.Context) {
	var req OrchestrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if req.ContentType == "" {
		req.ContentType = "interview_questions"
	}

	out, err := h.uc.Execute(c.Request.Context(), orchestrate.OrchestrateInput{
		Topic:       req.Topic,
		ContentType: req.ContentType,
	}, nil) // 首版不上报进度（同步模式无流式通道）
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{
		"item_ids": out.ItemIDs,
		"count":    out.Count,
	})
}
