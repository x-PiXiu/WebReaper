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
	// FindByID 按 ID 查用户。不存在返回 ErrNotFound。
	FindByID(ctx context.Context, id string) (entity.User, error)
	// List 列出全部用户（管理端用）。
	List(ctx context.Context) ([]entity.User, error)
	// Delete 删除用户。
	Delete(ctx context.Context, id string) error
	// Count 统计用户总数（平台总览用）。
	Count(ctx context.Context) (int, error)
}
