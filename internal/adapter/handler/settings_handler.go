package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
)

// HandleGetAutoMonitor GET /api/v1/admin/settings/auto-monitor —— 读自动盯盘开关。
func (r *Router) HandleGetAutoMonitor(c *gin.Context) {
	if r.settingsUC == nil {
		fail(c, errNotConfigured("系统设置"))
		return
	}
	enabled, err := r.settingsUC.GetAutoMonitor(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"auto_monitor_enabled": enabled})
}

// HandleSetAutoMonitor PUT /api/v1/admin/settings/auto-monitor —— 写自动盯盘开关。
// 开启后调度器每日对全平台品牌自动监测（趋势自动生长，付费卖点）。
func (r *Router) HandleSetAutoMonitor(c *gin.Context) {
	if r.settingsUC == nil {
		fail(c, errNotConfigured("系统设置"))
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := r.settingsUC.SetAutoMonitor(c.Request.Context(), req.Enabled); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"auto_monitor_enabled": req.Enabled})
}

// HandleGetBrowserHeaded GET /api/v1/admin/settings/browser-headed —— 读浏览器可见性。
func (r *Router) HandleGetBrowserHeaded(c *gin.Context) {
	if r.settingsUC == nil {
		fail(c, errNotConfigured("系统设置"))
		return
	}
	headed, _ := r.settingsUC.GetBrowserHeaded(c.Request.Context())
	success(c, gin.H{"headed": headed})
}

// HandleSetBrowserHeaded PUT /api/v1/admin/settings/browser-headed —— 写浏览器可见性。
// true=显示浏览器窗口（调试/扫码可见）；false=headless（生产默认）。即时生效。
func (r *Router) HandleSetBrowserHeaded(c *gin.Context) {
	if r.settingsUC == nil {
		fail(c, errNotConfigured("系统设置"))
		return
	}
	var req struct {
		Headed bool `json:"headed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := r.settingsUC.SetBrowserHeaded(c.Request.Context(), req.Headed); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"headed": req.Headed})
}

// HandleGetTenantAutoMonitor GET /api/v1/geo/monitor-auto —— 商户端自动盯盘状态 + 配置。
// 返回：租户开关（可自控）+ 平台总闸（管理员控制）+ 盯盘配置（频率/采样/通知阈值）。
func (r *Router) HandleGetTenantAutoMonitor(c *gin.Context) {
	if r.settingsUC == nil {
		fail(c, errNotConfigured("系统设置"))
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	tenantEnabled, err := r.settingsUC.GetTenantAutoMonitor(c.Request.Context(), tenantID)
	if err != nil {
		fail(c, err)
		return
	}
	platformEnabled, _ := r.settingsUC.GetAutoMonitor(c.Request.Context())
	cfg, _ := r.settingsUC.GetTenantAutoMonitorConfig(c.Request.Context(), tenantID)
	success(c, gin.H{
		"tenant_enabled":   tenantEnabled,   // 我的品牌是否参与自动监测（可切换）
		"platform_enabled": platformEnabled, // 平台总闸（管理员控制，只读）
		"config":           cfg,             // 盯盘配置（频率/采样/引擎/通知阈值）
	})
}

// HandleSetTenantAutoMonitor PUT /api/v1/geo/monitor-auto —— 商户端自动盯盘开关 + 配置。
// 关闭后调度器跳过该租户的品牌（节省 LLM 额度）；平台总闸关闭时本开关无效。
func (r *Router) HandleSetTenantAutoMonitor(c *gin.Context) {
	if r.settingsUC == nil {
		fail(c, errNotConfigured("系统设置"))
		return
	}
	var req struct {
		Enabled bool                     `json:"enabled"`
		Config  *entity.AutoMonitorConfig `json:"config"` // 可选：同时更新盯盘配置
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	if tenantID == "" {
		fail(c, fmt.Errorf("缺少租户上下文"))
		return
	}
	if err := r.settingsUC.SetTenantAutoMonitor(c.Request.Context(), tenantID, req.Enabled); err != nil {
		fail(c, err)
		return
	}
	if req.Config != nil {
		if err := r.settingsUC.SetTenantAutoMonitorConfig(c.Request.Context(), tenantID, *req.Config); err != nil {
			fail(c, err)
			return
		}
	}
	cfg, _ := r.settingsUC.GetTenantAutoMonitorConfig(c.Request.Context(), tenantID)
	success(c, gin.H{"tenant_enabled": req.Enabled, "config": cfg})
}

// ---- 提示词模板管理（admin）----

// HandleListPromptTemplates GET /api/v1/admin/prompt-templates —— 全部模板列表。
func (r *Router) HandleListPromptTemplates(c *gin.Context) {
	if r.promptTemplateRepo == nil {
		fail(c, errNotConfigured("提示词模板"))
		return
	}
	list, err := r.promptTemplateRepo.List(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"templates": list})
}

// HandleUpdatePromptTemplate PUT /api/v1/admin/prompt-templates/:key —— 更新模板内容
// （保存后版本 +1，内容生成即时生效——热更新，无需发版）。
func (r *Router) HandleUpdatePromptTemplate(c *gin.Context) {
	if r.promptTemplateRepo == nil {
		fail(c, errNotConfigured("提示词模板"))
		return
	}
	key := c.Param("key")
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content 必填"})
		return
	}
	// 读取当前版本（不存在则从 0 起）——版本递增由仓储 Save 处理
	if err := r.promptTemplateRepo.Save(c.Request.Context(), entity.PromptTemplate{Key: key, Content: req.Content}); err != nil {
		fail(c, err)
		return
	}
	t, err := r.promptTemplateRepo.Get(c.Request.Context(), key)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, t)
}
