package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// SystemSettingRepository 系统配置持久化接口（key-value 单例语义）。
type SystemSettingRepository interface {
	Get(ctx context.Context, key string) (entity.SystemSetting, error)
	Save(ctx context.Context, setting entity.SystemSetting) error
}
