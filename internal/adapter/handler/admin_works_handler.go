package handler

// admin_works_handler.go —— 32号：作品管理与内容安全（admin 侧）。
// 谦卑对象：HTTP ↔ 用例搬运。巡查流（成片+处置状态）/ 下架 / 恢复。
// 文章类管理复用既有 /admin/contents；处置对文章 key（c-{contentID}）同样生效
//（发布拦截与用户端过滤按 key 消费，不区分来源）。

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/pkg"
)

// HandleAdminWorksList GET /admin/works?limit= —— 跨租户作品巡查流（最近成片+处置状态）。
func (r *Router) HandleAdminWorksList(c *gin.Context) {
	if r.worksUC == nil {
		fail(c, pkg.ErrTaskNotExecutable)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	items, err := r.worksUC.ListRecentForAdmin(c.Request.Context(), limit)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"items": items, "total": len(items)})
}

// HandleAdminWorkHide POST /admin/works/:key/hide —— 下架/逻辑删除（reason 必填）。
// body: {kind?, tenant_id?, action?("hidden"|"deleted"，默认 hidden), reason}
func (r *Router) HandleAdminWorkHide(c *gin.Context) {
	if r.worksUC == nil || !r.worksUC.ModerationEnabled() {
		fail(c, pkg.ErrTaskNotExecutable)
		return
	}
	key := strings.TrimSpace(c.Param("key"))
	var req struct {
		Kind     string `json:"kind"`
		TenantID string `json:"tenant_id"`
		Action   string `json:"action"`
		Reason   string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if req.Action == "" {
		req.Action = "hidden"
	}
	if key == "" || !strings.HasPrefix(key, "g-") && !strings.HasPrefix(key, "c-") {
		fail(c, pkg.ErrInvalidArgument)
		return
	}
	if req.Kind == "" {
		if strings.HasPrefix(key, "c-") {
			req.Kind = "article"
		} else {
			req.Kind = "video"
		}
	}
	if err := r.worksUC.HideWork(c.Request.Context(), key, req.Kind, req.TenantID, req.Action,
		strings.TrimSpace(req.Reason), middleware.CurrentUserID(c)); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"work_key": key, "action": req.Action})
}

// HandleAdminWorkRestore POST /admin/works/:key/restore —— 恢复作品（清除处置记录）。
func (r *Router) HandleAdminWorkRestore(c *gin.Context) {
	if r.worksUC == nil || !r.worksUC.ModerationEnabled() {
		fail(c, pkg.ErrTaskNotExecutable)
		return
	}
	key := strings.TrimSpace(c.Param("key"))
	if err := r.worksUC.RestoreWork(c.Request.Context(), key); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"work_key": key, "restored": true})
}

// HandleAdminWorksFlagged GET /admin/works/flagged —— 机审待复核队列（32号 P2）。
func (r *Router) HandleAdminWorksFlagged(c *gin.Context) {
	if r.worksUC == nil || !r.worksUC.ModerationEnabled() {
		fail(c, pkg.ErrTaskNotExecutable)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	items, err := r.worksUC.ListFlaggedForAdmin(c.Request.Context(), limit)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"items": items, "total": len(items)})
}
