package handler

import (
	"fmt"
	"time"
	"log"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/adapter/agent"
	"webreaper/internal/usecase/account"
	"webreaper/internal/usecase/works"
)

// AccountHandler 多平台发布账号域 HTTP 适配器（账号绑定 + 半自动发布）。
//
// 多租户：所有请求从 JWT 取 tenant_id（merchant 只能看自己的，admin 看全局）。
type AccountHandler struct {
	accountUC      *account.AccountUseCase
	publishUC      *account.PublishUseCase
	worksUC        *works.WorksUseCase // 可选：作品库聚合
	pendingStore   *agent.PendingPublishStore // 可选：发布计划暂存（硬确认）
	frontendBaseURL string // OAuth 回调完成后 302 跳回的前端地址
}

// SetWorksUC 注入作品库聚合用例（可选）。
func (h *AccountHandler) SetWorksUC(uc *works.WorksUseCase) { h.worksUC = uc }

// SetPendingPublishStore 注入发布计划暂存（主 Agent 硬确认卡片用）。
func (h *AccountHandler) SetPendingPublishStore(ps *agent.PendingPublishStore) { h.pendingStore = ps }

// HandleConfirmPublish POST /merchant/publish-plans/:planID/confirm —— 硬确认执行：
// 用户在确认卡片点「确认发布」→ 从 pending 取出计划执行（支持 scheduled_at 定时——
// pending 层同时服务立即确认与定时发布）。确认动作走 REST 与 Agent 对话链路分离。
func (h *AccountHandler) HandleConfirmPublish(c *gin.Context) {
	if h.pendingStore == nil {
		fail(c, fmt.Errorf("发布计划功能未启用"))
		return
	}
	var req struct {
		ScheduledAt string `json:"scheduled_at"` // ISO 时间（可选=定时发布）
	}
	_ = c.ShouldBindJSON(&req)
	input, title, ok := h.pendingStore.Take(c.Param("planID"))
	if !ok {
		fail(c, fmt.Errorf("发布计划不存在或已过期（10 分钟有效），请重新发起"))
		return
	}
	if req.ScheduledAt != "" {
		t, pErr := time.Parse(time.RFC3339, req.ScheduledAt)
		if pErr != nil {
			fail(c, fmt.Errorf("定时时间格式错误"))
			return
		}
		input.ScheduledAt = t
	}
	job, err := h.publishUC.Publish(c.Request.Context(), input)
	if err != nil {
		fail(c, err)
		return
	}
	log.Printf("[PublishPlan] 硬确认执行：plan=%s《%s》→ %s", c.Param("planID"), title, job.Platform)
	success(c, publishJobToView(job))
}

// HandleCancelPublish POST /merchant/publish-plans/:planID/cancel —— 取消（取出即丢弃）。
func (h *AccountHandler) HandleCancelPublish(c *gin.Context) {
	if h.pendingStore != nil {
		_, _, _ = h.pendingStore.Take(c.Param("planID")) // 消费掉即取消
	}
	success(c, gin.H{"status": "cancelled"})
}

func NewAccountHandler(au *account.AccountUseCase, pu *account.PublishUseCase) *AccountHandler {
	return &AccountHandler{accountUC: au, publishUC: pu, frontendBaseURL: "http://localhost:5173"}
}

// SetFrontendBaseURL 注入前端地址（OAuth 回调 302 目标；main 装配时调用）。
func (h *AccountHandler) SetFrontendBaseURL(baseURL string) {
	if baseURL != "" {
		h.frontendBaseURL = baseURL
	}
}

// ---- DTO 转换（实体 → API 响应，PascalCase → snake_case）----

func accountToView(a entity.Account) gin.H {
	authType := a.AuthType
	if authType == "" {
		authType = entity.AccountAuthCookie
	}
	return gin.H{
		"id":           a.ID,
		"tenant_id":    a.TenantID,
		"platform":     a.Platform,
		"display_name": a.DisplayName,
		"health":       a.Health,
		"login_method": a.LoginMethod,
		"auth_type":    authType, // cookie（浏览器通道）/ oauth（官方 API 通道）
		"expires_at":   a.ExpiresAt,
		"bound_at":     a.BoundAt,
		"last_used_at": a.LastUsedAt,
		// 注意：cookie_encrypted / token 密文绝不返回前端
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
		"content_type":      j.ContentType,
		"media_urls":        j.MediaURLs,
		"cover_url":         j.CoverURL,
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
//
// Query 参数：
//   - platform: 平台（douyin/kuaishou/bilibili/xiaohongshu）
//   - method: 登录方式（cookie）
//   - scene: 场景（account=用户发布 / crawler=平台方爬虫）
func (h *AccountHandler) HandlePollQRLogin(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	sessionID := c.Param("sessionId")
	platform := c.Query("platform")
	method := c.Query("method")
	scene := c.DefaultQuery("scene", "account")

	var result account.PollQRLoginResult
	var err error

	if scene == "crawler" {
		result, err = h.accountUC.PollQRLoginWithScene(c.Request.Context(), tenantID, sessionID, platform, method, "crawler")
	} else {
		result, err = h.accountUC.PollQRLogin(c.Request.Context(), tenantID, sessionID, platform, method)
	}

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

// ---- 官方 OAuth 授权绑定（抖音开放平台 API 通道）----

// HandleDouyinOAuthURL GET /api/v1/geo/accounts/douyin/oauth/url —— 生成抖音官方授权页地址。
// 前端新窗口打开该地址（PC 端展示扫码二维码），授权后抖音回调服务端公开端点自动完成绑定。
func (h *AccountHandler) HandleDouyinOAuthURL(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	userID := middleware.CurrentUserID(c)
	url, err := h.accountUC.BuildOAuthURL(tenantID, userID)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"url": url})
}

// HandleDouyinOAuthCallback GET /api/v1/geo/accounts/douyin/oauth/callback —— 抖音授权回调（公开）。
// 浏览器从抖音授权页重定向至此（无 JWT）——state 签名携带租户上下文，
// 验签后换 token 落库，最后 302 跳回前端分发页并带结果参数。
func (h *AccountHandler) HandleDouyinOAuthCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	frontend := h.frontendBaseURL
	accountID, name, err := h.accountUC.HandleOAuthCallback(c.Request.Context(), code, state)
	if err != nil {
		// 商户端路由挂在前缀 /m/ 下（App.tsx），重定向路径必须带 /m
		c.Redirect(http.StatusFound, frontend+"/m/distribution?douyin_oauth=failed&reason="+url.QueryEscape(err.Error()))
		return
	}
	log.Printf("[DouyinOAuth] 授权绑定成功：account=%s name=%s", accountID, name)
	c.Redirect(http.StatusFound, frontend+"/m/distribution?douyin_oauth=success&name="+url.QueryEscape(name))
}

// ---- 发布管理端点 ----

// HandleListChannels GET /api/v1/geo/publish/channels —— 发布通道能力清单。
// 前端能力驱动的数据源：选内容形态 → 过滤可用平台 → 按约束动态生成检查清单。
// 新平台注册即自动出现（规则归适配器声明，前端零硬编码）。
func (h *AccountHandler) HandleListChannels(c *gin.Context) {
	success(c, gin.H{"channels": h.publishUC.ChannelCapabilities()})
}

// HandlePublish POST /api/v1/geo/publish —— 半自动发布（生成预填链接）。
func (h *AccountHandler) HandlePublish(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	var req struct {
		AccountID   string   `json:"account_id"`
		Platform    string   `json:"platform" binding:"required"`
		ContentID   string   `json:"content_id"`
		BrandID     string   `json:"brand_id"`
		Title       string   `json:"title"`
		Content     string   `json:"content"`
		Mode        string   `json:"mode"`
		ContentType string   `json:"content_type"` // image/video/article/audio
		MediaURLs   []string `json:"media_urls"`   // 媒体文件 URL（图文=图片）
		CoverURL    string   `json:"cover_url"`    // 封面图
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	job, err := h.publishUC.Publish(c.Request.Context(), account.PublishInput{
		TenantID:    tenantID,
		BrandID:     req.BrandID,
		AccountID:   req.AccountID,
		Platform:    req.Platform,
		ContentID:   req.ContentID,
		Title:       req.Title,
		Content:     req.Content,
		Mode:        req.Mode,
		ContentType: req.ContentType,
		MediaURLs:   req.MediaURLs,
		CoverURL:    req.CoverURL,
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

// HandleListWorks GET /api/v1/merchant/works —— 作品库三源聚合（我的作品页）。
func (h *AccountHandler) HandleListWorks(c *gin.Context) {
	if h.worksUC == nil {
		fail(c, fmt.Errorf("作品库未启用"))
		return
	}
	items, err := h.worksUC.ListWorks(c.Request.Context(), middleware.CurrentTenantID(c))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, items)
}

// HandleAnalyticsSummary GET /api/v1/merchant/works/analytics-summary —— 作品数据页聚合
// （指标卡 + 近14天趋势 + 已发布作品列表；互动数据由回读上线后填充）。
func (h *AccountHandler) HandleAnalyticsSummary(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	summary, err := h.publishUC.AnalyticsSummary(c.Request.Context(), tenantID)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, summary)
}

// HandleRefreshJobMetrics POST /api/v1/merchant/publish-jobs/:id/refresh-metrics
// 手动回读单作品互动数据（详情 Drawer「立即刷新」）。
func (h *AccountHandler) HandleRefreshJobMetrics(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	m, err := h.publishUC.RefreshJobMetrics(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{
		"job_id": m.JobID, "views": m.Views, "likes": m.Likes,
		"comments": m.Comments, "shares": m.Shares, "collected_at": m.CollectedAt,
	})
}

// HandleGetJobMetrics GET /api/v1/merchant/publish-jobs/:id/metrics —— 单作品指标时间序列（详情趋势）。
func (h *AccountHandler) HandleGetJobMetrics(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	ms, err := h.publishUC.ListJobMetrics(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(ms))
	for _, m := range ms {
		out = append(out, gin.H{
			"views": m.Views, "likes": m.Likes, "comments": m.Comments,
			"shares": m.Shares, "collected_at": m.CollectedAt,
		})
	}
	success(c, out)
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

// HandleReMonitor POST /api/v1/geo/publish-jobs/:id/re-monitor
// 发布效果复测：重新触发品牌监测，更新发布后提及率（建议收录周期后使用）。
func (h *AccountHandler) HandleReMonitor(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	jobID := c.Param("id")
	job, err := h.publishUC.ReMonitor(c.Request.Context(), tenantID, jobID)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, publishJobToView(job))
}
