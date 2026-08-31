// generation_handler.go GenerationHandler 依赖的接口（27号优化——Handler接口化）。
//
// 设计动机：
//   - GenerationHandler 直接依赖 *generation.GenerationUseCase 具体类型
//   - 改为依赖接口，提升可测试性（mock 接口而非整个 usecase）
//   - 接口只暴露 Handler 实际使用的方法
package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// GenerationHandlerUseCase GenerationHandler 依赖的用例接口。
type GenerationHandlerUseCase interface {
	// 统一提交
	UnifiedSubmit(ctx context.Context, in GenerationSubmitInput) (entity.GenerationTask, error)
	// 查询
	Get(ctx context.Context, tenantID, id string) (entity.GenerationTask, error)
	List(ctx context.Context, tenantID string, limit int) ([]entity.GenerationTask, error)
	// 操作
	Cancel(ctx context.Context, tenantID, id string) error
	DeleteTask(ctx context.Context, tenantID, id string) error
	// 类型查询
	Types() []string
	Capabilities(ctx context.Context, subType string) ([]entity.ModelCapability, error)
	// 主体库
	ListOfficialSubjects(ctx context.Context, pageToken string, count int) (SubjectListResult, error)
	ListPersonalSubjects(ctx context.Context, tenantID string, limit int) (SubjectListResult, error)
	// 链式形象视频
	RetryAvatarVideo(ctx context.Context, tenantID, taskID string) (entity.GenerationTask, error)
}

// GenerationSubmitInput 统一提交输入（与 generation.UnifiedSubmitInput 对齐）。
type GenerationSubmitInput struct {
	TenantID    string
	BrandID     string
	Text        string
	Materials   []string
	Template    string
	Type        string
	Duration    int
	Quality     string
	AspectRatio string
	Params      map[string]any
	Refs        []entity.PromptRef
	Watermark   bool
	OffPeak     bool
	SubType     string
}
