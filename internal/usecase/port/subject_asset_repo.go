// subject_asset_repo.go 主体资产仓储接口（26 号计划）。
//
// 整洁架构：接口归用例层声明，适配器层实现（GORM）。
// 物化钩子写入、读路径查询均通过本接口。
package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// SubjectAssetRepository 主体资产仓储。
type SubjectAssetRepository interface {
	// Upsert 按 server_id 唯一键幂等写入（物化钩子调用——存在则更新，不存在则插入）。
	Upsert(ctx context.Context, asset entity.SubjectAsset) error
	// FindByID 按 ID 查询。
	FindByID(ctx context.Context, id string) (entity.SubjectAsset, error)
	// FindByServerID 按 Vidu 主体 ID 查询（物化去重 + 形象视频回填定位）。
	FindByServerID(ctx context.Context, serverID string) (entity.SubjectAsset, error)
	// ListByTenant 按租户分页查询（scope/person/scene 过滤）。
	ListByTenant(ctx context.Context, tenantID, scope, kind string, limit, offset int) ([]entity.SubjectAsset, int64, error)
	// UpdateAvatarVideoURL 回填形象视频 URL（链式形象视频成功后调用）。
	UpdateAvatarVideoURL(ctx context.Context, serverID, avatarVideoURL string) error
	// UpdateStatus 更新状态（官方主体上下架）。
	UpdateStatus(ctx context.Context, id, status string) error
	// Delete 删除资产（仅管理后台——谨慎使用）。
	Delete(ctx context.Context, id string) error
}
