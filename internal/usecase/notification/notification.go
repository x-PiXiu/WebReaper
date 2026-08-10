// Package notification 实现"站内通知"用例（主动唤醒）。
//
// 职责：通知的推送/查询/已读——触发点在各业务域（监测任务/复测任务/发布），
// 本包只负责通知的存储与读取，不感知业务。
package notification

import (
	"context"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// NotifyUseCase 站内通知用例。
type NotifyUseCase struct {
	repo port.NotificationRepository
}

func NewNotifyUseCase(repo port.NotificationRepository) *NotifyUseCase {
	return &NotifyUseCase{repo: repo}
}

// Push 推送一条通知（触发点调用）。
func (uc *NotifyUseCase) Push(ctx context.Context, tenantID, notifType, title, content, link string) error {
	if tenantID == "" {
		return nil // 无租户上下文（后台任务）不推送
	}
	n := entity.Notification{
		ID:        fmt.Sprintf("ntf-%d", time.Now().UnixNano()),
		TenantID:  tenantID,
		Type:      notifType,
		Title:     title,
		Content:   content,
		Link:      link,
		Read:      false,
		CreatedAt: time.Now(),
	}
	return uc.repo.Save(ctx, n)
}

// List 最近通知（顶栏铃铛/通知中心）。
func (uc *NotifyUseCase) List(ctx context.Context, tenantID string, limit int) ([]entity.Notification, error) {
	return uc.repo.ListByTenant(ctx, tenantID, limit)
}

// UnreadCount 未读数（铃铛角标）。
func (uc *NotifyUseCase) UnreadCount(ctx context.Context, tenantID string) (int, error) {
	return uc.repo.UnreadCount(ctx, tenantID)
}

// MarkRead 标记已读（id 空 = 全部已读）。
func (uc *NotifyUseCase) MarkRead(ctx context.Context, tenantID, id string) error {
	return uc.repo.MarkRead(ctx, tenantID, id)
}
