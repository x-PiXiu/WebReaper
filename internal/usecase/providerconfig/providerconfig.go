package providerconfig

import (
	"context"
	"errors"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// UseCase 厂商配置用例（管理后台：按厂商设置 API Key / 启用开关）。
type UseCase struct {
	repo port.ProviderConfigRepository
}

func NewUseCase(repo port.ProviderConfigRepository) *UseCase {
	return &UseCase{repo: repo}
}

// List 全部厂商配置（API 层负责掩码脱敏）。
func (uc *UseCase) List(ctx context.Context) ([]entity.ProviderConfig, error) {
	return uc.repo.List(ctx)
}

// Upsert 保存厂商配置。
func (uc *UseCase) Upsert(ctx context.Context, cfg entity.ProviderConfig) error {
	if cfg.Provider == "" {
		return errors.New("厂商名不能为空")
	}
	return uc.repo.Upsert(ctx, cfg)
}

// MaskKey API Key 脱敏（abcd****wxyz；空返回空）。
func MaskKey(key string) string {
	if len(key) <= 8 {
		if key == "" {
			return ""
		}
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
