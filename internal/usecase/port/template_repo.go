package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// TemplateRepository 模板仓储接口（整洁架构·Port层）。
//
// 设计动机：
//   - 模板存储在数据库中，管理后台可以动态增删改查
//   - 用例层只依赖本接口，不感知具体存储实现（GORM/MySQL）
//   - 换存储实现 = 新增 adapter，用例零改动
type TemplateRepository interface {
	// Save 保存模板（新增或更新）。
	Save(ctx context.Context, template entity.GenerationTemplate) error

	// FindByID 根据ID查询模板。
	FindByID(ctx context.Context, id string) (entity.GenerationTemplate, error)

	// ListByTenant 查询租户可用模板（全局模板 + 租户私有模板）。
	ListByTenant(ctx context.Context, tenantID string) ([]entity.GenerationTemplate, error)

	// ListAll 查询所有模板（管理后台用）。
	ListAll(ctx context.Context) ([]entity.GenerationTemplate, error)

	// Delete 删除模板。
	Delete(ctx context.Context, id string) error
}
