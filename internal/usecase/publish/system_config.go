package publish

import (
	"context"
	"fmt"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// SystemConfigUseCase 外部系统配置管理用例（CRUD）。
type SystemConfigUseCase struct {
	repo port.ExternalSystemRepository
}

func NewSystemConfigUseCase(repo port.ExternalSystemRepository) *SystemConfigUseCase {
	return &SystemConfigUseCase{repo: repo}
}

// CreateInput 创建外部系统的输入。
type CreateInput struct {
	Name         string
	Description  string
	Endpoint     string
	Method       string
	Headers      string // JSON
	Mode         string // raw / mapping
	FieldMapping string // JSON（mapping 模式必填）
	BodyTemplate string // raw 模式的请求体示例（可选，用于 UI 提示）
	ContentType  string
	Enabled      *bool // 指针区分"未传"和"传 false"
}

// Create 创建外部系统配置。
func (uc *SystemConfigUseCase) Create(ctx context.Context, in CreateInput) (entity.ExternalSystem, error) {
	mode := in.Mode
	if mode == "" {
		mode = entity.PublishModeRaw
	}
	sys := entity.ExternalSystem{
		Name: in.Name, Description: in.Description, Endpoint: in.Endpoint,
		Method: in.Method, Headers: in.Headers, Mode: mode,
		FieldMapping: in.FieldMapping, BodyTemplate: in.BodyTemplate,
		ContentType: in.ContentType, Enabled: true,
	}
	if in.Enabled != nil {
		sys.Enabled = *in.Enabled
	}
	if sys.Method == "" {
		sys.Method = "POST"
	}
	if !sys.IsValid() {
		return entity.ExternalSystem{}, fmt.Errorf("外部系统配置无效：name/endpoint 必填；mapping 模式需 field_mapping")
	}
	if err := uc.repo.Save(ctx, sys); err != nil {
		return entity.ExternalSystem{}, fmt.Errorf("save: %w", err)
	}
	return sys, nil
}

// List 列出全部外部系统。
func (uc *SystemConfigUseCase) List(ctx context.Context) ([]entity.ExternalSystem, error) {
	return uc.repo.List(ctx)
}

// Delete 删除外部系统。
func (uc *SystemConfigUseCase) Delete(ctx context.Context, name string) error {
	return uc.repo.Delete(ctx, name)
}

// ListRecords 查询某数据项的推送记录。
func (uc *SystemConfigUseCase) ListRecords(ctx context.Context, contentID string, recRepo port.PublishRecordRepository) ([]entity.PublishRecord, error) {
	return recRepo.ListByContent(ctx, contentID)
}
