package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
)

// GormNotificationRepository port.NotificationRepository 的 GORM 实现。
type GormNotificationRepository struct {
	db *gorm.DB
}

func NewGormNotificationRepository(db *gorm.DB) *GormNotificationRepository {
	return &GormNotificationRepository{db: db}
}

func (r *GormNotificationRepository) Save(ctx context.Context, n entity.Notification) error {
	po := NotificationPO{
		ID: n.ID, TenantID: n.TenantID, Type: n.Type,
		Title: n.Title, Content: n.Content, Link: n.Link,
		IsRead: n.Read, CreatedAt: n.CreatedAt,
	}
	return r.db.WithContext(ctx).Create(&po).Error
}

func (r *GormNotificationRepository) ListByTenant(ctx context.Context, tenantID string, limit int) ([]entity.Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var pos []NotificationPO
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.Notification, 0, len(pos))
	for _, p := range pos {
		out = append(out, notificationFromPO(p))
	}
	return out, nil
}

func (r *GormNotificationRepository) UnreadCount(ctx context.Context, tenantID string) (int, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&NotificationPO{}).
		Where("tenant_id = ? AND is_read = ?", tenantID, false).Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *GormNotificationRepository) MarkRead(ctx context.Context, tenantID, id string) error {
	q := r.db.WithContext(ctx).Model(&NotificationPO{}).Where("tenant_id = ?", tenantID)
	if id != "" {
		q = q.Where("id = ?", id)
	}
	return q.Update("is_read", true).Error
}

// NotificationPO 站内通知持久化对象（notifications 表，AutoMigrate 建表）。
type NotificationPO struct {
	ID        string    `gorm:"primaryKey;size:64"`
	TenantID  string    `gorm:"size:64;index"`
	Type      string    `gorm:"size:32;index"`
	Title     string    `gorm:"size:256"`
	Content   string    `gorm:"type:text"`
	Link      string    `gorm:"size:256"`
	IsRead    bool      `gorm:"column:is_read;default:false;index"`
	CreatedAt time.Time `gorm:"index"`
}

func (NotificationPO) TableName() string { return "notifications" }

func notificationFromPO(p NotificationPO) entity.Notification {
	return entity.Notification{
		ID: p.ID, TenantID: p.TenantID, Type: p.Type,
		Title: p.Title, Content: p.Content, Link: p.Link,
		Read: p.IsRead, CreatedAt: p.CreatedAt,
	}
}
