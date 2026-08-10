package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// TenantSettingRepository 租户级设置仓储（多租户）。
type TenantSettingRepository interface {
	// Get 读租户设置；不存在返回 ErrNotFound。
	Get(ctx context.Context, tenantID, key string) (entity.TenantSetting, error)
	// Save 写租户设置（upsert）。
	Save(ctx context.Context, setting entity.TenantSetting) error
}
