package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// NotificationRepository 站内通知仓储（多租户）。
type NotificationRepository interface {
	Save(ctx context.Context, n entity.Notification) error
	// ListByTenant 最近通知（limit<=0 默认 30）。
	ListByTenant(ctx context.Context, tenantID string, limit int) ([]entity.Notification, error)
	// UnreadCount 未读数（顶栏铃铛角标）。
	UnreadCount(ctx context.Context, tenantID string) (int, error)
	// MarkRead 标记已读（id 为空 = 全部已读）。
	MarkRead(ctx context.Context, tenantID, id string) error
}
