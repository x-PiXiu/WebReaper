package handler

import (
	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
)

// HandleListNotifications GET /api/v1/notifications —— 最近通知（铃铛列表）。
func (r *Router) HandleListNotifications(c *gin.Context) {
	if r.notifyUC == nil {
		success(c, []any{})
		return
	}
	list, err := r.notifyUC.List(c.Request.Context(), middleware.CurrentTenantID(c), 30)
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(list))
	for _, n := range list {
		views = append(views, gin.H{
			"id": n.ID, "type": n.Type, "title": n.Title, "content": n.Content,
			"link": n.Link, "read": n.Read, "created_at": n.CreatedAt,
		})
	}
	success(c, views)
}

// HandleNotificationUnread GET /api/v1/notifications/unread-count —— 未读数（铃铛角标）。
func (r *Router) HandleNotificationUnread(c *gin.Context) {
	if r.notifyUC == nil {
		success(c, gin.H{"unread": 0})
		return
	}
	n, err := r.notifyUC.UnreadCount(c.Request.Context(), middleware.CurrentTenantID(c))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"unread": n})
}

// HandleMarkNotificationRead POST /api/v1/notifications/:id/read —— 标记已读（空 id=全部）。
func (r *Router) HandleMarkNotificationRead(c *gin.Context) {
	if r.notifyUC == nil {
		fail(c, errNotConfigured("通知"))
		return
	}
	if err := r.notifyUC.MarkRead(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"ok": true})
}
