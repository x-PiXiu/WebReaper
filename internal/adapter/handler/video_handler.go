package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/usecase/video"
)

// VideoHandler 视频生成工作台 API（多租户：tenant_id 来自 JWT）。
type VideoHandler struct {
	uc *video.VideoUseCase
}

func NewVideoHandler(uc *video.VideoUseCase) *VideoHandler {
	return &VideoHandler{uc: uc}
}

// HandleSubmit POST /api/v1/video/tasks —— 提交视频生成任务（立即返回，后台流水线驱动）。
func (h *VideoHandler) HandleSubmit(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	var req struct {
		BrandID     string `json:"brand_id"`
		Mode        string `json:"mode"`
		Prompt      string `json:"prompt"`
		MaterialURL string `json:"material_url"`
		VoiceText   string `json:"voice_text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数解析失败: " + err.Error()})
		return
	}
	if req.Mode == "" {
		req.Mode = "text"
	}
	task, err := h.uc.Submit(c.Request.Context(), video.SubmitInput{
		TenantID:    tenantID,
		BrandID:     req.BrandID,
		Mode:        req.Mode,
		Prompt:      req.Prompt,
		MaterialURL: req.MaterialURL,
		VoiceText:   req.VoiceText,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// HandleGet GET /api/v1/video/tasks/:id —— 查询任务详情（前端轮询）。
func (h *VideoHandler) HandleGet(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	task, err := h.uc.Get(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// HandleList GET /api/v1/video/tasks —— 任务列表。
func (h *VideoHandler) HandleList(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	tasks, err := h.uc.List(c.Request.Context(), tenantID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// HandlePublish POST /api/v1/video/tasks/publish —— 发布成片到视频平台。
func (h *VideoHandler) HandlePublish(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	var req struct {
		TaskID    string `json:"task_id"`
		Platform  string `json:"platform"`
		AccountID string `json:"account_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数解析失败: " + err.Error()})
		return
	}
	if req.TaskID == "" || req.Platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task_id 和 platform 必填"})
		return
	}
	job, err := h.uc.Publish(c.Request.Context(), tenantID, req.TaskID, req.Platform, req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

// HandleListJobs GET /api/v1/video/jobs —— 视频发布任务列表。
func (h *VideoHandler) HandleListJobs(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	jobs, err := h.uc.ListJobs(c.Request.Context(), tenantID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, jobs)
}
