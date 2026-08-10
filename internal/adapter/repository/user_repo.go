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

func (r *GormUserRepository) FindByID(ctx context.Context, id string) (entity.User, error) {
	var po UserPO
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.User{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.User{}, err
	}
	return userFromPO(po), nil
}

func (r *GormUserRepository) List(ctx context.Context) ([]entity.User, error) {
	var pos []UserPO
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.User, 0, len(pos))
	for _, p := range pos {
		result = append(result, userFromPO(p))
	}
	return result, nil
}

func (r *GormUserRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&UserPO{}).Error
}

func (r *GormUserRepository) Count(ctx context.Context) (int, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&UserPO{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}
