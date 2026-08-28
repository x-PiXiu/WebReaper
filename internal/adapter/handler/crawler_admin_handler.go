package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/inspiration"
	"webreaper/internal/usecase/port"
)

// CrawlerAdminHandler 爬虫管理 API（管理后台，需要 admin 角色）。
type CrawlerAdminHandler struct {
	uc          *inspiration.UseCase
	configRepo  port.CrawlerConfigRepository
	accountRepo port.CrawlerAccountRepository
	taskLogRepo port.CrawlerTaskLogRepository
	vault       port.CookieVault // cookie 加解密（健康检测解密 / 手动添加加密）
}

func NewCrawlerAdminHandler(
	uc *inspiration.UseCase,
	configRepo port.CrawlerConfigRepository,
	accountRepo port.CrawlerAccountRepository,
	taskLogRepo port.CrawlerTaskLogRepository,
	vault port.CookieVault,
) *CrawlerAdminHandler {
	return &CrawlerAdminHandler{
		uc:          uc,
		configRepo:  configRepo,
		accountRepo: accountRepo,
		taskLogRepo: taskLogRepo,
		vault:       vault,
	}
}

// ---- 平台方账号管理 ----

// HandleListAccounts GET /admin/crawler-accounts —— 列出所有平台方账号。
func (h *CrawlerAdminHandler) HandleListAccounts(c *gin.Context) {
	if h.accountRepo == nil {
		fail(c, fmt.Errorf("账号仓储未配置"))
		return
	}

	accounts, err := h.accountRepo.ListAll(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}

	// 脱敏：隐藏 Cookie
	for i := range accounts {
		accounts[i].CookieEncrypted = "***"
	}

	success(c, gin.H{"accounts": accounts})
}

// HandleCreateAccount POST /admin/crawler-accounts —— 添加平台方账号。
func (h *CrawlerAdminHandler) HandleCreateAccount(c *gin.Context) {
	if h.accountRepo == nil {
		fail(c, fmt.Errorf("账号仓储未配置"))
		return
	}

	var req struct {
		Platform       string `json:"platform" binding:"required"`
		AccountName    string `json:"account_name" binding:"required"`
		Cookie         string `json:"cookie" binding:"required"`
		UserAgent      string `json:"user_agent"`
		ProxyAddress   string `json:"proxy_address"`
		DailyUsageLimit int   `json:"daily_usage_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	if req.DailyUsageLimit <= 0 {
		req.DailyUsageLimit = 50
	}

	// cookie 加密落库（与扫码登录保存路径一致——pickCookie 读取时按密文解密，
	// 明文存储的记录解密必失败，会被账号池静默跳过）
	encCookie := req.Cookie
	if h.vault != nil {
		enc, encErr := h.vault.Encrypt(req.Cookie)
		if encErr != nil {
			fail(c, fmt.Errorf("cookie 加密失败: %w", encErr))
			return
		}
		encCookie = enc
	}

	account := entity.CrawlerAccount{
		Platform:         req.Platform,
		AccountName:      req.AccountName,
		CookieEncrypted:  encCookie,
		UserAgent:        req.UserAgent,
		ProxyAddress:     req.ProxyAddress,
		Status:           entity.CrawlerAccountActive,
		HealthCheckResult: entity.HealthUnknown,
		DailyUsageLimit:  req.DailyUsageLimit,
	}

	if err := h.accountRepo.Save(c.Request.Context(), account); err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{"msg": "账号添加成功"})
}

// HandleUpdateAccount PUT /admin/crawler-accounts/:id —— 更新账号。
func (h *CrawlerAdminHandler) HandleUpdateAccount(c *gin.Context) {
	if h.accountRepo == nil {
		fail(c, fmt.Errorf("账号仓储未配置"))
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, fmt.Errorf("无效的账号 ID"))
		return
	}

	existing, err := h.accountRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		fail(c, fmt.Errorf("账号不存在"))
		return
	}

	var req struct {
		AccountName     *string `json:"account_name"`
		Cookie          *string `json:"cookie"`
		UserAgent       *string `json:"user_agent"`
		ProxyAddress    *string `json:"proxy_address"`
		Status          *string `json:"status"`
		DailyUsageLimit *int    `json:"daily_usage_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	if req.AccountName != nil {
		existing.AccountName = *req.AccountName
	}
	if req.Cookie != nil {
		existing.CookieEncrypted = *req.Cookie // TODO: 加密
	}
	if req.UserAgent != nil {
		existing.UserAgent = *req.UserAgent
	}
	if req.ProxyAddress != nil {
		existing.ProxyAddress = *req.ProxyAddress
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}
	if req.DailyUsageLimit != nil {
		existing.DailyUsageLimit = *req.DailyUsageLimit
	}

	if err := h.accountRepo.Save(c.Request.Context(), existing); err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{"msg": "账号已更新"})
}

// HandleCheckAccountHealth POST /admin/crawler-accounts/:id/health —— 手动触发健康检查。
//
// 按账号检测：解密该账号自己的 cookie 跑平台登录态验证。
//（旧实现调平台级 IsPlatformAlive——从账号池选"健康"账号检测，与被点账号无关，
//  且唯一账号被标 unhealthy 后池子选不出账号，检测永远失败——死锁。）
func (h *CrawlerAdminHandler) HandleCheckAccountHealth(c *gin.Context) {
	if h.accountRepo == nil || h.uc == nil {
		fail(c, fmt.Errorf("服务未配置"))
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, fmt.Errorf("无效的账号 ID"))
		return
	}

	acc, err := h.accountRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		fail(c, fmt.Errorf("账号不存在"))
		return
	}

	// 解密该账号自己的 cookie（密钥变更/明文脏数据 → 解密失败即不健康，原因可解释）
	alive := false
	reason := ""
	if h.vault == nil {
		reason = "加密服务未配置"
	} else {
		cookie, decErr := h.vault.Decrypt(acc.CookieEncrypted)
		if decErr != nil {
			reason = "cookie 解密失败（非加密格式或加密密钥已变更）"
		} else {
			alive, reason = h.uc.CheckAccountAlive(c.Request.Context(), acc.Platform, cookie)
		}
	}

	result := entity.HealthHealthy
	if !alive {
		result = entity.HealthUnhealthy
	}

	if err := h.accountRepo.UpdateHealth(c.Request.Context(), id, result); err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{
		"account_id": id,
		"platform":   acc.Platform,
		"healthy":    alive,
		"result":     result,
		"reason":     reason,
	})
}

// HandleDeleteAccount DELETE /admin/crawler-accounts/:id —— 删除账号。
func (h *CrawlerAdminHandler) HandleDeleteAccount(c *gin.Context) {
	if h.accountRepo == nil {
		fail(c, fmt.Errorf("账号仓储未配置"))
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, fmt.Errorf("无效的账号 ID"))
		return
	}

	if err := h.accountRepo.Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{"msg": "账号已删除"})
}

// ---- 爬虫配置管理 ----

// HandleListConfigs GET /admin/crawlers —— 列出所有爬虫配置。
func (h *CrawlerAdminHandler) HandleListConfigs(c *gin.Context) {
	if h.configRepo == nil {
		fail(c, fmt.Errorf("配置仓储未配置"))
		return
	}

	configs, err := h.configRepo.ListAll(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{"configs": configs})
}

// HandleGetConfig GET /admin/crawlers/:platform —— 获取平台爬虫配置。
func (h *CrawlerAdminHandler) HandleGetConfig(c *gin.Context) {
	if h.configRepo == nil {
		fail(c, fmt.Errorf("配置仓储未配置"))
		return
	}

	config, err := h.configRepo.FindByPlatform(c.Request.Context(), c.Param("platform"))
	if err != nil {
		fail(c, err)
		return
	}

	success(c, config)
}

// HandleUpdateConfig PUT /admin/crawlers/:platform —— 更新平台爬虫配置。
func (h *CrawlerAdminHandler) HandleUpdateConfig(c *gin.Context) {
	if h.configRepo == nil {
		fail(c, fmt.Errorf("配置仓储未配置"))
		return
	}

	platform := c.Param("platform")
	existing, err := h.configRepo.FindByPlatform(c.Request.Context(), platform)
	if err != nil {
		fail(c, fmt.Errorf("配置不存在"))
		return
	}

	var req struct {
		Enabled              *bool    `json:"enabled"`
		SearchKeywords       []string `json:"search_keywords"`
		ExtraKeywords        []string `json:"extra_keywords"`
		CrawlIntervalMinutes *int     `json:"crawl_interval_minutes"`
		MaxResults           *int     `json:"max_results"`
		SortBy               string   `json:"sort_by"`
		PublishTime          string   `json:"publish_time"`
		EnableComments       *bool    `json:"enable_comments"`
		EnableRefresh        *bool    `json:"enable_refresh"`
		RefreshIntervalHours *int     `json:"refresh_interval_hours"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	// 更新非空字段
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.SearchKeywords != nil {
		existing.SearchKeywords = req.SearchKeywords
	}
	if req.ExtraKeywords != nil {
		existing.ExtraKeywords = req.ExtraKeywords
	}
	if req.CrawlIntervalMinutes != nil {
		existing.CrawlIntervalMinutes = *req.CrawlIntervalMinutes
	}
	if req.MaxResults != nil {
		existing.MaxResults = *req.MaxResults
	}
	if req.SortBy != "" {
		existing.SortBy = req.SortBy
	}
	if req.PublishTime != "" {
		existing.PublishTime = req.PublishTime
	}
	if req.EnableComments != nil {
		existing.EnableComments = *req.EnableComments
	}
	if req.EnableRefresh != nil {
		existing.EnableRefresh = *req.EnableRefresh
	}
	if req.RefreshIntervalHours != nil {
		existing.RefreshIntervalHours = *req.RefreshIntervalHours
	}

	if err := h.configRepo.Save(c.Request.Context(), existing); err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{"msg": "配置已更新"})
}

// ---- 任务监控 ----

// HandleListTasks GET /admin/crawlers/tasks —— 采集任务列表。
func (h *CrawlerAdminHandler) HandleListTasks(c *gin.Context) {
	if h.taskLogRepo == nil {
		fail(c, fmt.Errorf("任务日志仓储未配置"))
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	platform := c.Query("platform")

	var tasks []entity.CrawlerTaskLog
	var err error
	if platform != "" {
		tasks, err = h.taskLogRepo.ListByPlatform(c.Request.Context(), platform, limit)
	} else {
		tasks, err = h.taskLogRepo.ListAll(c.Request.Context(), limit)
	}
	if err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{"tasks": tasks})
}

// HandleGetTask GET /admin/crawlers/tasks/:id —— 任务详情。
func (h *CrawlerAdminHandler) HandleGetTask(c *gin.Context) {
	if h.taskLogRepo == nil {
		fail(c, fmt.Errorf("任务日志仓储未配置"))
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, fmt.Errorf("无效的任务 ID"))
		return
	}

	task, err := h.taskLogRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}

	success(c, task)
}

// ---- 手动触发 ----

// HandleTriggerCrawl POST /admin/crawlers/:platform/trigger —— 手动触发采集。
func (h *CrawlerAdminHandler) HandleTriggerCrawl(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("灵感服务未配置"))
		return
	}

	platform := c.Param("platform")
	var req struct {
		BrandID  string   `json:"brand_id" binding:"required"`
		Keywords []string `json:"keywords" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	result, err := h.uc.CrawlBrand(c.Request.Context(), platform, req.BrandID, req.Keywords)
	if err != nil {
		fail(c, err)
		return
	}

	success(c, result)
}

// HandleTestConnection POST /admin/crawlers/:platform/test —— 测试平台连接。
func (h *CrawlerAdminHandler) HandleTestConnection(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("灵感服务未配置"))
		return
	}

	platform := c.Param("platform")
	alive := h.uc.IsPlatformAlive(c.Request.Context(), platform)

	success(c, gin.H{
		"platform": platform,
		"alive":    alive,
	})
}

// HandleRefreshMetrics POST /admin/crawlers/:platform/refresh-metrics —— 刷新视频指标。
//
// 调用详情 API 补充搜索 API 不返回的字段（play_count/duration/collect_count 等）。
func (h *CrawlerAdminHandler) HandleRefreshMetrics(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("灵感服务未配置"))
		return
	}

	platform := c.Param("platform")
	var req struct {
		VideoIDs []string `json:"video_ids"` // 为空则刷新该平台所有视频
		Limit    int      `json:"limit"`     // 最大刷新数量（默认 20）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 没有 body 也可以，使用默认值
		req.Limit = 20
	}

	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}

	// 如果没有指定 video_ids，从数据库获取该平台的视频
	if len(req.VideoIDs) == 0 {
		// 获取该平台最近的视频
		videos, _, err := h.uc.List(c.Request.Context(), "", platform, "", "created_at", 1, req.Limit)
		if err != nil {
			fail(c, err)
			return
		}
		for _, v := range videos {
			req.VideoIDs = append(req.VideoIDs, v.PlatformVideoID)
		}
	}

	if len(req.VideoIDs) == 0 {
		success(c, gin.H{"msg": "没有需要刷新的视频", "updated": 0})
		return
	}

	// 调用详情 API 刷新指标
	updated, err := h.uc.RefreshMetrics(c.Request.Context(), platform, req.VideoIDs)
	if err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{
		"platform":    platform,
		"total":       len(req.VideoIDs),
		"updated":     updated,
		"video_ids":   req.VideoIDs,
	})
}
