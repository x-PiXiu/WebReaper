package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// WorkModerationRepository 作品处置仓储（32号：管理与内容安全）。
type WorkModerationRepository interface {
	// FindByKey 按作品键查处置记录（聚合过滤与发布拦截消费；无记录返回 ErrNotFound）。
	FindByKey(ctx context.Context, workKey string) (entity.WorkModeration, error)
	// Upsert 按 work_key 幂等写入（重复处置覆盖 action/reason/operator）。
	Upsert(ctx context.Context, m entity.WorkModeration) error
	// Delete 清除处置记录（restore 恢复）。
	Delete(ctx context.Context, workKey string) error
	// ListByTenant 租户在效处置集合（用户端聚合过滤：hidden/deleted 的 work_key）。
	ListByTenant(ctx context.Context, tenantID string) ([]entity.WorkModeration, error)
	// ListRecent 全平台处置记录倒序（管理端"已处置"列表/审计）。
	ListRecent(ctx context.Context, limit int) ([]entity.WorkModeration, error)
}
