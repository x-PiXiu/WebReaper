package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ---- 模板仓储的 GORM 实现 ----

// TemplatePO 模板持久化对象。
type TemplatePO struct {
	ID                string    `gorm:"primaryKey;column:id"`
	TenantID          string    `gorm:"column:tenant_id;index"`
	Name              string    `gorm:"column:name"`
	Description       string    `gorm:"column:description"`
	Icon              string    `gorm:"column:icon"`
	SubType           string    `gorm:"column:sub_type;index"`
	DefaultParams     string    `gorm:"column:default_params"` // JSON
	RequiredMaterials string    `gorm:"column:required_materials"` // JSON
	OptionalMaterials string    `gorm:"column:optional_materials"` // JSON
	SortOrder         int       `gorm:"column:sort_order"`
	Enabled           bool      `gorm:"column:enabled;index"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (TemplatePO) TableName() string { return "generation_templates" }

// templateToPO 领域实体 → 持久化对象。
func templateToPO(t entity.GenerationTemplate) TemplatePO {
	defaultParamsJSON, _ := json.Marshal(t.DefaultParams)
	requiredJSON, _ := json.Marshal(t.RequiredMaterials)
	optionalJSON, _ := json.Marshal(t.OptionalMaterials)

	return TemplatePO{
		ID:                t.ID,
		TenantID:          t.TenantID,
		Name:              t.Name,
		Description:       t.Description,
		Icon:              t.Icon,
		SubType:           t.SubType,
		DefaultParams:     string(defaultParamsJSON),
		RequiredMaterials: string(requiredJSON),
		OptionalMaterials: string(optionalJSON),
		SortOrder:         t.SortOrder,
		Enabled:           t.Enabled,
		CreatedAt:         t.CreatedAt,
		UpdatedAt:         t.UpdatedAt,
	}
}

// templateFromPO 持久化对象 → 领域实体。
func templateFromPO(po TemplatePO) entity.GenerationTemplate {
	var defaultParams map[string]any
	var requiredMaterials []string
	var optionalMaterials []string

	json.Unmarshal([]byte(po.DefaultParams), &defaultParams)
	json.Unmarshal([]byte(po.RequiredMaterials), &requiredMaterials)
	json.Unmarshal([]byte(po.OptionalMaterials), &optionalMaterials)

	return entity.GenerationTemplate{
		ID:                po.ID,
		TenantID:          po.TenantID,
		Name:              po.Name,
		Description:       po.Description,
		Icon:              po.Icon,
		SubType:           po.SubType,
		DefaultParams:     defaultParams,
		RequiredMaterials: requiredMaterials,
		OptionalMaterials: optionalMaterials,
		SortOrder:         po.SortOrder,
		Enabled:           po.Enabled,
		CreatedAt:         po.CreatedAt,
		UpdatedAt:         po.UpdatedAt,
	}
}

// GormTemplateRepository 模板仓储的GORM实现。
type GormTemplateRepository struct {
	db *gorm.DB
}

var _ port.TemplateRepository = (*GormTemplateRepository)(nil)

func NewGormTemplateRepository(db *gorm.DB) *GormTemplateRepository {
	return &GormTemplateRepository{db: db}
}

// Save 保存模板（新增或更新）。
func (r *GormTemplateRepository) Save(ctx context.Context, t entity.GenerationTemplate) error {
	po := templateToPO(t)
	return r.db.WithContext(ctx).Save(&po).Error
}

// FindByID 根据ID查询模板。
func (r *GormTemplateRepository) FindByID(ctx context.Context, id string) (entity.GenerationTemplate, error) {
	var po TemplatePO
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.GenerationTemplate{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.GenerationTemplate{}, err
	}
	return templateFromPO(po), nil
}

// ListByTenant 查询租户可用模板（全局模板 + 租户私有模板）。
func (r *GormTemplateRepository) ListByTenant(ctx context.Context, tenantID string) ([]entity.GenerationTemplate, error) {
	var pos []TemplatePO
	query := r.db.WithContext(ctx).Where("enabled = ?", true)

	if tenantID != "" {
		// 租户可以看到：全局模板 + 自己的私有模板
		query = query.Where("(tenant_id = '' OR tenant_id = ?)", tenantID)
	} else {
		// 空租户只能看到全局模板
		query = query.Where("tenant_id = ''")
	}

	err := query.Order("sort_order ASC").Find(&pos).Error
	if err != nil {
		return nil, err
	}

	out := make([]entity.GenerationTemplate, 0, len(pos))
	for _, po := range pos {
		out = append(out, templateFromPO(po))
	}
	return out, nil
}

// ListAll 查询所有模板（管理后台用）。
func (r *GormTemplateRepository) ListAll(ctx context.Context) ([]entity.GenerationTemplate, error) {
	var pos []TemplatePO
	err := r.db.WithContext(ctx).Order("sort_order ASC").Find(&pos).Error
	if err != nil {
		return nil, err
	}

	out := make([]entity.GenerationTemplate, 0, len(pos))
	for _, po := range pos {
		out = append(out, templateFromPO(po))
	}
	return out, nil
}

// Delete 删除模板。
func (r *GormTemplateRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&TemplatePO{}).Error
}
