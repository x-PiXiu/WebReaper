package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// UserRepository 用户持久化接口（边界）。
// 用例层声明、适配器层实现（如 GORM）。隔离存储细节。
type UserRepository interface {
	// Save 创建或更新用户。
	Save(ctx context.Context, user entity.User) error
	// FindByUsername 按用户名查用户（登录时用）。不存在返回 ErrNotFound。
	FindByUsername(ctx context.Context, username string) (entity.User, error)
}
