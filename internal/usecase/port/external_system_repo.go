package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// ExternalSystemRepository 外部系统配置持久化接口。
type ExternalSystemRepository interface {
	Save(ctx context.Context, sys entity.ExternalSystem) error
	FindByName(ctx context.Context, name string) (entity.ExternalSystem, error)
	List(ctx context.Context) ([]entity.ExternalSystem, error)
	Delete(ctx context.Context, name string) error
}

// PublishRecordRepository 推送记录持久化接口。
type PublishRecordRepository interface {
	Save(ctx context.Context, rec entity.PublishRecord) error
	ListByContent(ctx context.Context, contentID string) ([]entity.PublishRecord, error)
	FindDedup(ctx context.Context, contentID, systemName string) (entity.PublishRecord, error) // 去重查询
}
