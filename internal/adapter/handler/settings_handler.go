package handler

import (
	"github.com/gin-gonic/gin"
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
