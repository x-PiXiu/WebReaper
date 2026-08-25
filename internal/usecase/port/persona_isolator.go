package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// PersonaIsolator 人设隔离器接口（装饰器模式）。
//
// 多账号发布时，每个账号可以有不同的人设风格（活泼/专业/亲切/沉稳）。
// 通过此接口对内容进行风格化处理，实现多账号内容差异化。
type PersonaIsolator interface {
	// Isolate 隔离内容（添加人设风格）。
	// 根据 personaID 查找人设配置，对内容进行风格化处理：
	// - 禁用词过滤
	// - 语气风格调整
	// - 偏好标签追加
	Isolate(ctx context.Context, content string, personaID string) (string, error)
	// GetPersona 获取人设配置。
	GetPersona(ctx context.Context, personaID string) (*entity.Persona, error)
}

// PersonaRepository 人设仓储接口。
type PersonaRepository interface {
	// FindByID 根据ID查找人设。
	FindByID(ctx context.Context, tenantID, id string) (*entity.Persona, error)
	// ListByTenant 列出租户下所有人设。
	ListByTenant(ctx context.Context, tenantID string) ([]entity.Persona, error)
	// Save 保存人设。
	Save(ctx context.Context, persona *entity.Persona) error
	// Delete 删除人设。
	Delete(ctx context.Context, tenantID, id string) error
}
