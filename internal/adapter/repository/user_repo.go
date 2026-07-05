package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// GormUserRepository 是 port.UserRepository 的 GORM 实现。
type GormUserRepository struct {
	db *gorm.DB
}

// 编译期断言：实现 port.UserRepository。
var _ port.UserRepository = (*GormUserRepository)(nil)

func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) Save(ctx context.Context, user entity.User) error {
	po := userToPO(user)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormUserRepository) FindByUsername(ctx context.Context, username string) (entity.User, error) {
	var po UserPO
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.User{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.User{}, err
	}
	return userFromPO(po), nil
}
