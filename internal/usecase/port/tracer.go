package port

import "context"

// Span 是一个分布式追踪 span 的抽象（横切关注点的边界）。
//
// 刻意不依赖任何具体追踪库（如 go.opentelemetry.io/otel/trace.Span），
// 保持 port 层纯净。适配器层负责把 Span 映射为具体追踪库的实现。
type Span interface {
	// End 结束当前 span（必须在所有路径上 defer 调用）。
	End()
	// SetAttribute 给 span 附加一个键值属性（如 status_code=404）。
	SetAttribute(key string, value any)
	// RecordError 记录一个错误事件到 span（不影响 span 的成功/失败语义）。
	RecordError(err error)
}

// Tracer 是追踪抽象接口（与 Logger 同构的横切关注点）。
//
// 依赖倒置：接口由【用例层】声明并拥有，由【适配器层】实现（如 OTel）。
// 用例通过依赖注入获得 Tracer，从而获得 trace 能力而不直接依赖 OTel。
//
// 设计要点：
//   - StartSpan 返回带 span 的新 ctx，调用方应使用该 ctx 传播下游
//   - 方法签名不暴露任何具体追踪库类型
//   - 未启用 trace 时返回 no-op span（开销极低）
type Tracer interface {
	// StartSpan 在 ctx 上开启一个子 span，返回带 span 的 ctx 和 span 句柄。
	StartSpan(ctx context.Context, name string) (context.Context, Span)
}

// NopTracer 是空操作的 Tracer，用于不需要 trace 的场景（如单元测试、未启用时）。
type NopTracer struct{}

// NewNopTracer 创建空操作 Tracer。
func NewNopTracer() NopTracer { return NopTracer{} }

func (NopTracer) StartSpan(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, NopSpan{}
}

// NopSpan 是空操作 Span。
type NopSpan struct{}

func (NopSpan) End()                  {}
func (NopSpan) SetAttribute(string, any) {}
func (NopSpan) RecordError(error)     {}
