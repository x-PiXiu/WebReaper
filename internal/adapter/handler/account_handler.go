package handler

import (
	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/account"
)

// AccountHandler 多平台发布账号域 HTTP 适配器（账号绑定 + 半自动发布）。
//
// 多租户：所有请求从 JWT 取 tenant_id（merchant 只能看自己的，admin 看全局）。
type AccountHandler struct {
	accountUC *account.AccountUseCase
	publishUC *account.PublishUseCase
}

func NewAccountHandler(au *account.AccountUseCase, pu *account.PublishUseCase) *AccountHandler {
	return &AccountHandler{accountUC: au, publishUC: pu}
}

// ---- DTO 转换（实体 → API 响应，PascalCase → snake_case）----

func accountToView(a entity.Account) gin.H {
	return gin.H{
		"id":           a.ID,
		"tenant_id":    a.TenantID,
		"platform":     a.Platform,
		"display_name": a.DisplayName,
		"health":       a.Health,
		"login_method": a.LoginMethod,
		"expires_at":   a.ExpiresAt,
		"bound_at":     a.BoundAt,
		"last_used_at": a.LastUsedAt,
		// 注意：cookie_encrypted 绝不返回前端
	}
}

func accountsToView(as []entity.Account) []gin.H {
	out := make([]gin.H, 0, len(as))
	for _, a := range as {
		out = append(out, accountToView(a))
	}
	return out
}

func publishJobToView(j entity.PublishJob) gin.H {
	return gin.H{
		"id":                j.ID,
		"account_id":        j.AccountID,
		"platform":          j.Platform,
		"content_id":        j.ContentID,
		"title":             j.Title,
		"mode":              j.Mode,
		"status":            j.Status,
		"external_url":      j.ExternalURL,
		"error_msg":         j.ErrorMsg,
		"created_at":        j.CreatedAt,
		"published_at":      j.PublishedAt,
		"pre_mention_rate":  j.PreMentionRate,
		"post_mention_rate": j.PostMentionRate,
	}
}

func publishJobsToView(js []entity.PublishJob) []gin.H {
	out := make([]gin.H, 0, len(js))
	for _, j := range js {
		out = append(out, publishJobToView(j))
	}
	return out
}

// ---- 账号管理端点 ----

// HandleListAccounts GET /api/v1/geo/accounts —— 列出当前租户的全部平台账号。
func (h *AccountHandler) HandleListAccounts(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	accounts, err := h.accountUC.List(c.Request.Context(), tenantID)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, accountsToView(accounts))
}

// HandleStartQRLogin POST /api/v1/geo/accounts/qr-login —— 启动扫码登录，返回会话 ID。
func (h *AccountHandler) HandleStartQRLogin(c *gin.Context) {
	var req struct {
		Platform string `json:"platform" binding:"required"`
		Method   string `json:"method"` // 登录方式：空=默认, wechat/qq/weibo
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	sessionID, err := h.accountUC.StartQRLogin(c.Request.Context(), req.Platform, req.Method)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{
		"session_id": sessionID,
		"platform":   req.Platform,
		"method":     req.Method,
	})
}

// HandlePollQRLogin GET /api/v1/geo/accounts/qr-login/:sessionId —— 轮询扫码状态 + 二维码图片。
func (h *AccountHandler) HandlePollQRLogin(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	sessionID := c.Param("sessionId")
	platform := c.Query("platform")
	method := c.Query("method")

	result, err := h.accountUC.PollQRLogin(c.Request.Context(), tenantID, sessionID, platform, method)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{
		"status":       result.Status,
		"qr_image":     result.QRImage,
		"account_id":   result.AccountID,
		"account_name": result.AccountName,
		"expires_at":   result.ExpiresAt,
	})
}

// HandleCancelQRLogin DELETE /api/v1/geo/accounts/qr-login/:sessionId —— 取消扫码（关闭浏览器）。
func (h *AccountHandler) HandleCancelQRLogin(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if err := h.accountUC.CleanupSession(c.Request.Context(), sessionID); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"status": "cancelled"})
}

// HandleDeleteAccount DELETE /api/v1/geo/accounts/:id —— 解绑账号。
func (h *AccountHandler) HandleDeleteAccount(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	id := c.Param("id")
	if err := h.accountUC.Delete(c.Request.Context(), tenantID, id); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"id": id})
}

// ---- 发布管理端点 ----

// HandlePublish POST /api/v1/geo/publish —— 半自动发布（生成预填链接）。
func (h *AccountHandler) HandlePublish(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	var req struct {
		AccountID string `json:"account_id"`
		Platform  string `json:"platform" binding:"required"`
		ContentID string `json:"content_id"`
		BrandID   string `json:"brand_id"`
		Title     string `json:"title"`
		Content   string `json:"content"`
		Mode      string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	job, err := h.publishUC.Publish(c.Request.Context(), account.PublishInput{
		TenantID:  tenantID,
		BrandID:   req.BrandID,
		AccountID: req.AccountID,
		Platform:  req.Platform,
		ContentID: req.ContentID,
		Title:     req.Title,
		Content:   req.Content,
		Mode:      req.Mode,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, publishJobToView(job))
}

// HandleListPublishJobs GET /api/v1/geo/publish-jobs —— 列出发布任务记录。
func (h *AccountHandler) HandleListPublishJobs(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	jobs, err := h.publishUC.ListJobs(c.Request.Context(), tenantID, 50)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, publishJobsToView(jobs))
}

// HandleMarkPublished POST /api/v1/geo/publish-jobs/:id/published —— 标记任务为已发布。
func (h *AccountHandler) HandleMarkPublished(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	jobID := c.Param("id")
	if err := h.publishUC.MarkPublished(c.Request.Context(), tenantID, jobID); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"id": jobID, "status": "published"})
}

// HandleGetJobStatus GET /api/v1/geo/publish-jobs/:id/status —— 查询发布任务状态（前端轮询用）。
func (h *AccountHandler) HandleGetJobStatus(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	jobID := c.Param("id")
	job, err := h.publishUC.GetJobStatus(c.Request.Context(), tenantID, jobID)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{
		"id":           job.ID,
		"status":       job.Status,
		"external_url": job.ExternalURL,
		"error_msg":    job.ErrorMsg,
		"platform":     job.Platform,
	})
}
