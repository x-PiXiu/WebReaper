package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// ---- 提示词模板仓储（用例层声明，适配器实现）----

// PromptTemplateRepository 提示词模板仓储。
type PromptTemplateRepository interface {
	// Get 按 Key 取模板。不存在返回 ErrNotFound。
	Get(ctx context.Context, key string) (entity.PromptTemplate, error)
	// Save 保存模板（Key 存在则覆盖并递增版本）。
	Save(ctx context.Context, t entity.PromptTemplate) error
	// List 全部模板（管理后台编辑用）。
	List(ctx context.Context) ([]entity.PromptTemplate, error)
}
