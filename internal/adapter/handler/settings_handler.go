package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
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

// HandleGetTenantAutoMonitor GET /api/v1/geo/monitor-auto —— 商户端自动盯盘状态。
// 返回：租户开关（可自控）+ 平台总闸（管理员控制）。
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
	success(c, gin.H{
		"tenant_enabled":   tenantEnabled,   // 我的品牌是否参与自动监测（可切换）
		"platform_enabled": platformEnabled, // 平台总闸（管理员控制，只读）
	})
}

// HandleSetTenantAutoMonitor PUT /api/v1/geo/monitor-auto —— 商户端自动盯盘开关。
// 关闭后调度器跳过该租户的品牌（节省 LLM 额度）；平台总闸关闭时本开关无效。
func (r *Router) HandleSetTenantAutoMonitor(c *gin.Context) {
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
	tenantID := middleware.CurrentTenantID(c)
	if tenantID == "" {
		fail(c, fmt.Errorf("缺少租户上下文"))
		return
	}
	if err := r.settingsUC.SetTenantAutoMonitor(c.Request.Context(), tenantID, req.Enabled); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"tenant_enabled": req.Enabled})
}
