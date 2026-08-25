package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// PublishConfigHandler 品牌发布配置 API
type PublishConfigHandler struct {
	configRepo  port.BrandPublishConfigRepository
	bindingRepo port.AccountBrandBindingRepository
	usageRepo   port.PublishUsageRepository
}

// NewPublishConfigHandler 创建品牌发布配置处理器
func NewPublishConfigHandler(configRepo port.BrandPublishConfigRepository, bindingRepo port.AccountBrandBindingRepository, usageRepo port.PublishUsageRepository) *PublishConfigHandler {
	return &PublishConfigHandler{
		configRepo:  configRepo,
		bindingRepo: bindingRepo,
		usageRepo:   usageRepo,
	}
}

// HandleGetBrandPublishConfig GET /api/v1/merchant/brands/:id/publish-config
func (h *PublishConfigHandler) HandleGetBrandPublishConfig(c *gin.Context) {
	if h.configRepo == nil {
		fail(c, fmt.Errorf("发布配置服务未配置"))
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	brandID := c.Param("id")

	configs, err := h.configRepo.FindByBrand(c.Request.Context(), tenantID, brandID)
	if err != nil {
		fail(c, err)
		return
	}

	if configs == nil {
		configs = []entity.BrandPublishConfig{}
	}
	success(c, configs)
}

// HandleUpdateBrandPublishConfig PUT /api/v1/merchant/brands/:id/publish-config
func (h *PublishConfigHandler) HandleUpdateBrandPublishConfig(c *gin.Context) {
	if h.configRepo == nil {
		fail(c, fmt.Errorf("发布配置服务未配置"))
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	brandID := c.Param("id")

	var req struct {
		Platform       string           `json:"platform" binding:"required"`
		AccountIDs     []string         `json:"account_ids"`
		RateLimit      entity.RateLimit `json:"rate_limit"`
		DefaultTags    []string         `json:"default_tags"`
		DefaultPersona string           `json:"default_persona"`
		IsActive       *bool            `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	config := &entity.BrandPublishConfig{
		TenantID:       tenantID,
		BrandID:        brandID,
		Platform:       req.Platform,
		AccountIDs:     req.AccountIDs,
		RateLimit:      req.RateLimit,
		DefaultTags:    req.DefaultTags,
		DefaultPersona: req.DefaultPersona,
		IsActive:       isActive,
	}

	if err := h.configRepo.Save(c.Request.Context(), config); err != nil {
		fail(c, err)
		return
	}

	success(c, config)
}

// HandleDeleteBrandPublishConfig DELETE /api/v1/merchant/brands/:id/publish-config/:platform
func (h *PublishConfigHandler) HandleDeleteBrandPublishConfig(c *gin.Context) {
	if h.configRepo == nil {
		fail(c, fmt.Errorf("发布配置服务未配置"))
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	brandID := c.Param("id")
	platform := c.Param("platform")

	if err := h.configRepo.Delete(c.Request.Context(), tenantID, brandID, platform); err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{"message": "删除成功"})
}

// HandleBindAccount POST /api/v1/merchant/brands/:id/publish-config/bindings
func (h *PublishConfigHandler) HandleBindAccount(c *gin.Context) {
	if h.bindingRepo == nil {
		fail(c, fmt.Errorf("绑定服务未配置"))
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	brandID := c.Param("id")

	var req struct {
		AccountID string `json:"account_id" binding:"required"`
		Platform  string `json:"platform" binding:"required"`
		IsDefault bool   `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	binding := &entity.AccountBrandBinding{
		TenantID:  tenantID,
		AccountID: req.AccountID,
		BrandID:   brandID,
		Platform:  req.Platform,
		IsDefault: req.IsDefault,
	}

	if err := h.bindingRepo.Bind(c.Request.Context(), binding); err != nil {
		fail(c, err)
		return
	}

	success(c, binding)
}

// HandleUnbindAccount DELETE /api/v1/merchant/brands/:id/publish-config/bindings/:accountId
func (h *PublishConfigHandler) HandleUnbindAccount(c *gin.Context) {
	if h.bindingRepo == nil {
		fail(c, fmt.Errorf("绑定服务未配置"))
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	brandID := c.Param("id")
	accountID := c.Param("accountId")

	if err := h.bindingRepo.Unbind(c.Request.Context(), tenantID, accountID, brandID); err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{"message": "解绑成功"})
}

// defaultDailyQuota 各平台默认日上限（品牌未配置限流时的保守默认——Plan-14 8.3）。
var defaultDailyQuota = map[string]int{
	"xiaohongshu": 3, "douyin": 5, "kuaishou": 5, "weixin": 5, "bilibili": 3, "zhihu": 2,
}

// HandleGetPublishStats GET /api/v1/merchant/brands/:id/publish-stats
// 响应 quotas 数组是向导限流拦截的数据源（此前缺失导致"今日已达上限"永不触发）。
func (h *PublishConfigHandler) HandleGetPublishStats(c *gin.Context) {
	if h.usageRepo == nil {
		fail(c, fmt.Errorf("统计服务未配置"))
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	brandID := c.Param("id")

	// 品牌发布配置（取每平台自定义日上限；无配置走保守默认）
	configs, _ := h.configRepo.FindByBrand(c.Request.Context(), tenantID, brandID)
	maxByPlatform := make(map[string]int)
	if configs != nil {
		for _, cfg := range configs {
			if cfg.RateLimit.MaxPerDay > 0 {
				maxByPlatform[cfg.Platform] = cfg.RateLimit.MaxPerDay
			}
		}
	}

	platforms := []string{"douyin", "kuaishou", "xiaohongshu", "weixin", "bilibili", "zhihu"}
	stats := make(map[string]int, len(platforms))
	type quota struct {
		Platform  string `json:"platform"`
		UsedToday int    `json:"used_today"`
		MaxPerDay int    `json:"max_per_day"`
		Remaining int    `json:"remaining"`
		AtLimit   bool   `json:"at_limit"`
	}
	quotas := make([]quota, 0, len(platforms))
	for _, platform := range platforms {
		usage, err := h.usageRepo.GetDailyUsage(c.Request.Context(), tenantID, brandID, platform)
		if err != nil {
			continue
		}
		stats[platform] = usage
		maxDay := maxByPlatform[platform]
		if maxDay <= 0 {
			maxDay = defaultDailyQuota[platform]
		}
		remaining := maxDay - usage
		if remaining < 0 {
			remaining = 0
		}
		quotas = append(quotas, quota{
			Platform: platform, UsedToday: usage, MaxPerDay: maxDay,
			Remaining: remaining, AtLimit: usage >= maxDay,
		})
	}

	success(c, gin.H{
		"brand_id":    brandID,
		"daily_usage": stats,
		"quotas":      quotas,
	})
}
