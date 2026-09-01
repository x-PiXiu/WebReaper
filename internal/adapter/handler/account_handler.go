package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"log"
	"strings"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/adapter/agent"
	"webreaper/internal/usecase/account"
	"webreaper/internal/usecase/port"
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
	adapterRegistry port.ContentAdapterRegistry // 可选：适配预览（向导阶段⑤）
	draftCache      DraftCache // 可选：向导云草稿（Redis）
}

// DraftCache 向导云草稿存储（RedisCache 天然实现；测试可 Mock）。
type DraftCache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

// SetContentAdapters 注入内容适配器注册表（适配预览用，可选）。
func (h *AccountHandler) SetContentAdapters(r port.ContentAdapterRegistry) { h.adapterRegistry = r }

// SetDraftCache 注入向导云草稿存储（可选；未注入时前端自动降级 localStorage）。
func (h *AccountHandler) SetDraftCache(dc DraftCache) { h.draftCache = dc }

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
	// mention_rate 零值输出 null（前端用 != null 区分"未复测"——输出 0 会污染
	// 表现变化均值统计：未复测任务被计入 (0-pre) 拉低均值）
	preRate, postRate := any(nil), any(nil)
	if j.PreMentionRate != 0 {
		preRate = j.PreMentionRate
	}
	if j.PostMentionRate != 0 {
		postRate = j.PostMentionRate
	}
	// 状态派生：pending + 未来排期时间 → scheduled（前端"已排期"筛选的语义来源；
	// 库内仍存 pending——到期执行不改）
	displayStatus := j.Status
	if j.Status == entity.PublishStatusPending && !j.ScheduledAt.IsZero() && j.ScheduledAt.After(time.Now()) {
		displayStatus = "scheduled"
	}
	return gin.H{
		"id":                j.ID,
		"account_id":        j.AccountID,
		"platform":          j.Platform,
		"brand_id":          j.BrandID,
		"content_id":        j.ContentID,
		"title":             j.Title,
		"mode":              j.Mode,
		"status":            displayStatus,
		"scheduled_at":      j.ScheduledAt,
		"external_url":      j.ExternalURL,
		"error_msg":         j.ErrorMsg,
		"created_at":        j.CreatedAt,
		"published_at":      j.PublishedAt,
		"pre_mention_rate":  preRate,
		"post_mention_rate": postRate,
		"content_type":      j.ContentType,
		"media_urls":        j.MediaURLs,
		"cover_url":         j.CoverURL,
		"transport":         j.Transport,
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

// ---- 抖音 OAuth 授权绑定（已删除 2026-09-01）----
// 用户决策：所有发布操作均为 RPA 浏览器自动化（cookie 通道），OAuth token 无消费方，
// 保留仅造成误导。删除内容：HandleDouyinOAuthURL / HandleDouyinOAuthCallback / 路由 /
// 前端 douyinOauthUrl API 与回调提示。usecase 的 BuildOAuthURL/HandleOAuthCallback/
// token 刷新等保留为未接线代码（无入口不可达），将来接入官方 API 发布时恢复路由即用。

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
		// 对接向导（Plan-14）补齐的字段——此前 JSON 绑定静默丢弃导致定时发布失效/标签分区仅正文兜底
		ScheduledAt  string   `json:"scheduled_at"`  // ISO 时间（空=立即发布）
		Tags         []string `json:"tags"`          // 标签（B站独立标签框等）
		Category     string   `json:"category"`      // 平台分区（B站必选）
		Privacy      string   `json:"privacy"`       // 可见性（youtube: public/unlisted/private；空=公开）
		StoreAddress string   `json:"store_address"` // 门店地址（本地生活曝光信号）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	var scheduledAt time.Time
	if req.ScheduledAt != "" {
		t, pErr := time.Parse(time.RFC3339, req.ScheduledAt)
		if pErr != nil {
			fail(c, fmt.Errorf("scheduled_at 格式错误（需 ISO/RFC3339）: %w", pErr))
			return
		}
		scheduledAt = t
	}
	job, err := h.publishUC.Publish(c.Request.Context(), account.PublishInput{
		TenantID:     tenantID,
		BrandID:      req.BrandID,
		AccountID:    req.AccountID,
		Platform:     req.Platform,
		ContentID:    req.ContentID,
		Title:        req.Title,
		Content:      req.Content,
		Mode:         req.Mode,
		ContentType:  req.ContentType,
		MediaURLs:    req.MediaURLs,
		CoverURL:     req.CoverURL,
		ScheduledAt:  scheduledAt,
		Tags:         req.Tags,
		Privacy:      req.Privacy,
		Category:     req.Category,
		StoreAddress: req.StoreAddress,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, publishJobToView(job))
}

// HandlePreviewAdapt POST /api/v1/merchant/publish/preview —— 多平台内容适配预览
// （向导阶段⑤数据源：预览即真实适配结果——ContentAdapter 只读暴露，Plan-14 修正 #7）。
// 前端本地截断规则仅在本文档接口 404 时兜底，双端规则由此归一。
func (h *AccountHandler) HandlePreviewAdapt(c *gin.Context) {
	if h.adapterRegistry == nil {
		fail(c, fmt.Errorf("适配预览未配置"))
		return
	}
	var req struct {
		Title     string   `json:"title"`
		Content   string   `json:"content"`
		Tags      []string `json:"tags"`
		Platforms []string `json:"platforms" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	type preview struct {
		Platform       string   `json:"platform"`
		Title          string   `json:"title,omitempty"`
		Description    string   `json:"description,omitempty"`
		Tags           []string `json:"tags,omitempty"`
		CTA            string   `json:"cta,omitempty"`
		TitleTruncated bool     `json:"title_truncated,omitempty"`
		Error          string   `json:"error,omitempty"`
	}
	previews := make([]preview, 0, len(req.Platforms))
	for _, platform := range req.Platforms {
		p := preview{Platform: platform}
		adapter, err := h.adapterRegistry.Get(platform)
		if err != nil {
			p.Error = err.Error()
			previews = append(previews, p)
			continue
		}
		adapted, err := adapter.Adapt(c.Request.Context(), port.AdaptRequest{
			Platform: platform, Title: req.Title, Description: req.Content, Tags: req.Tags,
		})
		if err != nil {
			p.Error = err.Error()
			previews = append(previews, p)
			continue
		}
		p.Title, p.Description, p.Tags, p.CTA = adapted.Title, adapted.Description, adapted.Tags, adapted.CTA
		p.TitleTruncated = len([]rune(adapted.Title)) < len([]rune(req.Title))
		previews = append(previews, p)
	}
	success(c, gin.H{"previews": previews})
}

// ---- 向导云草稿（Plan-14 修正 #8：多端同步；未部署 Redis 时前端降级 localStorage）----

const publishDraftTTL = 7 * 24 * time.Hour

func (h *AccountHandler) draftKey(tenantID, brandID string) string {
	return "publish:draft:" + tenantID + ":" + brandID
}

// HandleGetPublishDraft GET /api/v1/merchant/publish/draft?brand_id=xx
func (h *AccountHandler) HandleGetPublishDraft(c *gin.Context) {
	if h.draftCache == nil {
		fail(c, fmt.Errorf("草稿服务未配置"))
		return
	}
	brandID := c.Query("brand_id")
	if brandID == "" {
		fail(c, fmt.Errorf("brand_id 必填"))
		return
	}
	// 存储结构 {draft, updated_at}（元数据随值一起存，避免多 key）
	var entry struct {
		Draft     string `json:"draft"`
		UpdatedAt string `json:"updated_at"`
	}
	if raw, ok, err := h.draftCache.Get(c.Request.Context(), h.draftKey(middleware.CurrentTenantID(c), brandID)); err == nil && ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &entry)
	}
	success(c, gin.H{"draft": entry.Draft, "updated_at": entry.UpdatedAt})
}

// HandleSavePublishDraft PUT /api/v1/merchant/publish/draft
func (h *AccountHandler) HandleSavePublishDraft(c *gin.Context) {
	if h.draftCache == nil {
		fail(c, fmt.Errorf("草稿服务未配置"))
		return
	}
	var req struct {
		BrandID string `json:"brand_id" binding:"required"`
		Draft   string `json:"draft"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	entry, _ := json.Marshal(map[string]string{
		"draft": req.Draft, "updated_at": time.Now().Format(time.RFC3339),
	})
	if err := h.draftCache.Set(c.Request.Context(), h.draftKey(middleware.CurrentTenantID(c), req.BrandID), string(entry), publishDraftTTL); err != nil {
		fail(c, fmt.Errorf("草稿保存失败: %w", err))
		return
	}
	success(c, gin.H{"saved": true})
}

// HandleDeletePublishDraft DELETE /api/v1/merchant/publish/draft?brand_id=xx
func (h *AccountHandler) HandleDeletePublishDraft(c *gin.Context) {
	if h.draftCache == nil {
		fail(c, fmt.Errorf("草稿服务未配置"))
		return
	}
	brandID := c.Query("brand_id")
	if brandID == "" {
		fail(c, fmt.Errorf("brand_id 必填"))
		return
	}
	_ = h.draftCache.Del(c.Request.Context(), h.draftKey(middleware.CurrentTenantID(c), brandID))
	success(c, gin.H{"deleted": true})
}

// HandleListPublishJobs GET /api/v1/geo/publish-jobs —— 列出发布任务记录。
func (h *AccountHandler) HandleListPublishJobs(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	jobs, err := h.publishUC.ListJobs(c.Request.Context(), tenantID, 50)
	if err != nil {
		fail(c, err)
		return
	}
	// 品牌维度过滤（品牌发布历史 Tab 传 brand_id——租户级列表混入他品牌任务的修复）
	if brandID := c.Query("brand_id"); brandID != "" {
		filtered := make([]entity.PublishJob, 0, len(jobs))
		for _, j := range jobs {
			if j.BrandID == brandID {
				filtered = append(filtered, j)
			}
		}
		jobs = filtered
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

// HandleAppealWork POST /api/v1/merchant/works/:key/appeal —— 用户申诉被处置作品
//（32号 P2 终批：防滥用一天一次；申诉中文本过机审防展示位滥用）。
func (h *AccountHandler) HandleAppealWork(c *gin.Context) {
	if h.worksUC == nil || !h.worksUC.ModerationEnabled() {
		fail(c, fmt.Errorf("申诉服务未启用"))
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := h.worksUC.AppealWork(c.Request.Context(), middleware.CurrentTenantID(c),
		strings.TrimSpace(c.Param("key")), req.Reason); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"appealed": true})
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
