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

// HandleGetPublishStats GET /api/v1/merchant/brands/:id/publish-stats
func (h *PublishConfigHandler) HandleGetPublishStats(c *gin.Context) {
	if h.usageRepo == nil {
		fail(c, fmt.Errorf("统计服务未配置"))
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	brandID := c.Param("id")

	// 获取各平台今日使用量
	platforms := []string{"douyin", "kuaishou", "xiaohongshu", "weixin", "bilibili"}
	stats := make(map[string]int)
	for _, platform := range platforms {
		usage, err := h.usageRepo.GetDailyUsage(c.Request.Context(), tenantID, brandID, platform)
		if err != nil {
			continue
		}
		stats[platform] = usage
	}

	success(c, gin.H{
		"brand_id":     brandID,
		"daily_usage":  stats,
	})
}
