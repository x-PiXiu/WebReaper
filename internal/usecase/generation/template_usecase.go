package generation

import (
	"context"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// TemplateUseCase 模板管理用例（整洁架构·Usecase层）。
//
// 设计动机：
//   - 模板存储在数据库中，管理后台可以动态增删改查
//   - 用例层只依赖 port 接口，不感知具体存储实现
//   - 业务规则内聚在用例层（如：全局模板只能由admin管理）
type TemplateUseCase struct {
	repo port.TemplateRepository
}

func NewTemplateUseCase(repo port.TemplateRepository) *TemplateUseCase {
	return &TemplateUseCase{repo: repo}
}

// List 查询租户可用模板。
func (uc *TemplateUseCase) List(ctx context.Context, tenantID string) ([]entity.GenerationTemplate, error) {
	return uc.repo.ListByTenant(ctx, tenantID)
}

// ListAll 查询所有模板（管理后台用）。
func (uc *TemplateUseCase) ListAll(ctx context.Context) ([]entity.GenerationTemplate, error) {
	return uc.repo.ListAll(ctx)
}

// Get 根据ID查询模板。
func (uc *TemplateUseCase) Get(ctx context.Context, id string) (entity.GenerationTemplate, error) {
	return uc.repo.FindByID(ctx, id)
}

// CreateInput 创建模板输入。
type CreateTemplateInput struct {
	ID                string         `json:"id"`
	TenantID          string         `json:"tenant_id"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	Icon              string         `json:"icon"`
	SubType           string         `json:"sub_type"`
	DefaultParams     map[string]any `json:"default_params"`
	RequiredMaterials []string       `json:"required_materials"`
	OptionalMaterials []string       `json:"optional_materials"`
	SortOrder         int            `json:"sort_order"`
}

// Create 创建模板。
func (uc *TemplateUseCase) Create(ctx context.Context, input CreateTemplateInput) (entity.GenerationTemplate, error) {
	// 参数校验
	if input.ID == "" {
		return entity.GenerationTemplate{}, fmt.Errorf("模板ID不能为空")
	}
	if input.Name == "" {
		return entity.GenerationTemplate{}, fmt.Errorf("模板名称不能为空")
	}
	if input.SubType == "" {
		return entity.GenerationTemplate{}, fmt.Errorf("端点类型不能为空")
	}

	// 检查ID是否已存在
	if _, err := uc.repo.FindByID(ctx, input.ID); err == nil {
		return entity.GenerationTemplate{}, fmt.Errorf("模板ID已存在: %s", input.ID)
	}

	now := time.Now()
	template := entity.GenerationTemplate{
		ID:                input.ID,
		TenantID:          input.TenantID,
		Name:              input.Name,
		Description:       input.Description,
		Icon:              input.Icon,
		SubType:           input.SubType,
		DefaultParams:     input.DefaultParams,
		RequiredMaterials: input.RequiredMaterials,
		OptionalMaterials: input.OptionalMaterials,
		SortOrder:         input.SortOrder,
		Enabled:           true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if !template.IsValid() {
		return entity.GenerationTemplate{}, fmt.Errorf("模板信息不完整")
	}

	if err := uc.repo.Save(ctx, template); err != nil {
		return entity.GenerationTemplate{}, fmt.Errorf("保存模板失败: %w", err)
	}

	return template, nil
}

// UpdateInput 更新模板输入。
type UpdateTemplateInput struct {
	Name              *string         `json:"name"`
	Description       *string         `json:"description"`
	Icon              *string         `json:"icon"`
	DefaultParams     *map[string]any `json:"default_params"`
	RequiredMaterials *[]string       `json:"required_materials"`
	OptionalMaterials *[]string       `json:"optional_materials"`
	SortOrder         *int            `json:"sort_order"`
	Enabled           *bool           `json:"enabled"`
}

// Update 更新模板。
func (uc *TemplateUseCase) Update(ctx context.Context, id string, input UpdateTemplateInput) (entity.GenerationTemplate, error) {
	template, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return entity.GenerationTemplate{}, fmt.Errorf("模板不存在: %w", err)
	}

	// 更新字段
	if input.Name != nil {
		template.Name = *input.Name
	}
	if input.Description != nil {
		template.Description = *input.Description
	}
	if input.Icon != nil {
		template.Icon = *input.Icon
	}
	if input.DefaultParams != nil {
		template.DefaultParams = *input.DefaultParams
	}
	if input.RequiredMaterials != nil {
		template.RequiredMaterials = *input.RequiredMaterials
	}
	if input.OptionalMaterials != nil {
		template.OptionalMaterials = *input.OptionalMaterials
	}
	if input.SortOrder != nil {
		template.SortOrder = *input.SortOrder
	}
	if input.Enabled != nil {
		template.Enabled = *input.Enabled
	}

	template.UpdatedAt = time.Now()

	if err := uc.repo.Save(ctx, template); err != nil {
		return entity.GenerationTemplate{}, fmt.Errorf("更新模板失败: %w", err)
	}

	return template, nil
}

// Delete 删除模板。
func (uc *TemplateUseCase) Delete(ctx context.Context, id string) error {
	// 检查模板是否存在
	if _, err := uc.repo.FindByID(ctx, id); err != nil {
		return fmt.Errorf("模板不存在: %w", err)
	}

	return uc.repo.Delete(ctx, id)
}
