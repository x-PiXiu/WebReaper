package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
)

// PromptTemplatePO 提示词模板持久化对象（prompt_templates 表）。
type PromptTemplatePO struct {
	Key       string    `gorm:"primaryKey;size:64"`
	Version   int       `gorm:"default:1"`
	Content   string    `gorm:"type:longtext"`
	UpdatedAt time.Time
}

func (PromptTemplatePO) TableName() string { return "prompt_templates" }

// GormPromptTemplateRepository 是 port.PromptTemplateRepository 的 GORM 实现。
type GormPromptTemplateRepository struct {
	db *gorm.DB
}

func NewGormPromptTemplateRepository(db *gorm.DB) *GormPromptTemplateRepository {
	return &GormPromptTemplateRepository{db: db}
}

func (r *GormPromptTemplateRepository) Get(ctx context.Context, key string) (entity.PromptTemplate, error) {
	var po PromptTemplatePO
	err := r.db.WithContext(ctx).Where("`key` = ?", key).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.PromptTemplate{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.PromptTemplate{}, err
	}
	return promptTemplateFromPO(po), nil
}

func (r *GormPromptTemplateRepository) Save(ctx context.Context, t entity.PromptTemplate) error {
	var po PromptTemplatePO
	err := r.db.WithContext(ctx).Where("`key` = ?", t.Key).First(&po).Error
	switch {
	case err == nil:
		t.Version = po.Version + 1 // 覆盖时版本递增（可回滚追溯）
	case errors.Is(err, gorm.ErrRecordNotFound):
		if t.Version <= 0 {
			t.Version = 1
		}
	default:
		return err
	}
	t.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(promptTemplateToPO(t)).Error
}

func (r *GormPromptTemplateRepository) List(ctx context.Context) ([]entity.PromptTemplate, error) {
	var pos []PromptTemplatePO
	if err := r.db.WithContext(ctx).Order("`key` ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.PromptTemplate, 0, len(pos))
	for _, p := range pos {
		out = append(out, promptTemplateFromPO(p))
	}
	return out, nil
}

func promptTemplateToPO(t entity.PromptTemplate) PromptTemplatePO {
	return PromptTemplatePO{Key: t.Key, Version: t.Version, Content: t.Content, UpdatedAt: t.UpdatedAt}
}

func promptTemplateFromPO(p PromptTemplatePO) entity.PromptTemplate {
	return entity.PromptTemplate{Key: p.Key, Version: p.Version, Content: p.Content, UpdatedAt: p.UpdatedAt}
}
