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
}

func NewCrawlerAdminHandler(
	uc *inspiration.UseCase,
	configRepo port.CrawlerConfigRepository,
	accountRepo port.CrawlerAccountRepository,
) *CrawlerAdminHandler {
	return &CrawlerAdminHandler{
		uc:          uc,
		configRepo:  configRepo,
		accountRepo: accountRepo,
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

	account := entity.CrawlerAccount{
		Platform:         req.Platform,
		AccountName:      req.AccountName,
		CookieEncrypted:  req.Cookie, // TODO: 加密存储
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
	// TODO: 实现任务日志查询
	success(c, gin.H{"tasks": []interface{}{}})
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
