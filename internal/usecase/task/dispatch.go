// Package task 实现"异步任务"用例：任务投递（Enqueue）、任务分派（Dispatch）、
// 后台消费（Worker）。
//
// 这是用例层：编排"队列消费 → 按 Type 分派 → 调业务用例 → 更新状态"流程，
// 只依赖 domain 实体和 port 接口，不依赖任何 HTTP/队列框架实现。
//
// 架构演进：项目已转向「单一 Agent + N 工具」模型，采集/加工由 Agent 通过
// 工具调用完成。当前任务子系统服务于 Agent 异步执行（AgentHandler）。
// 关键设计（避免循环依赖）：
// task 包不直接 import 业务用例，而是通过 TaskHandler 接口解耦——
// 每种任务类型注册一个 Handler，dispatch 按 TaskType 查找 Handler。
// 与 SpiderRegistry / ToolRegistry 完全同构（开闭原则）。
package task

import (
	"context"
	"fmt"
	"sync"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// TaskHandler 处理一种类型的异步任务。
// 当前实现：AgentHandler（处理 Agent 异步执行）。
// inputJSON 是 Task.Input（业务用例 Input 的 JSON 序列化），由 Handler 反序列化。
type TaskHandler interface {
	// TaskType 返回本处理器支持的任务类型，用于注册表分派。
	TaskType() entity.TaskType
	// Handle 执行任务。inputJSON 是序列化后的输入参数。
	Handle(ctx context.Context, inputJSON string) (outputJSON string, err error)
}

// HandlerRegistry 是任务处理器的注册表（策略模式 + 注册表）。
// 与 SpiderRegistry / PlatformRegistry 完全同构：
// 新增任务类型只需实现 TaskHandler 并注册，dispatch 用例零修改（开闭原则）。
type HandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[entity.TaskType]TaskHandler
}

// NewHandlerRegistry 创建空的处理器注册表。
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{handlers: make(map[entity.TaskType]TaskHandler)}
}

// Register 注册一个任务处理器。重复注册会覆盖。
func (r *HandlerRegistry) Register(h TaskHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[h.TaskType()] = h
}

// Lookup 按 TaskType 查找已注册的处理器。
func (r *HandlerRegistry) Lookup(t entity.TaskType) (TaskHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[t]
	return h, ok
}

// DispatchUseCase 任务分派用例。
// 按 Task.Type 查找对应 Handler 执行，不关心具体业务逻辑。
type DispatchUseCase struct {
	registry *HandlerRegistry
	logger   port.Logger
}

func NewDispatchUseCase(registry *HandlerRegistry, logger port.Logger) *DispatchUseCase {
	if logger == nil {
		logger = port.NopLogger{}
	}
	return &DispatchUseCase{
		registry: registry,
		logger:   logger.With(port.String("component", "dispatch")),
	}
}

// Execute 分派并执行一个任务。
// 返回 (输出 JSON, 错误)。未注册的类型返回 ErrInvalidArgument。
func (uc *DispatchUseCase) Execute(ctx context.Context, t entity.Task) (string, error) {
	handler, ok := uc.registry.Lookup(t.Type)
	if !ok {
		uc.logger.Error("未注册的任务类型", port.String("type", string(t.Type)))
		return "", fmt.Errorf("%w: no handler for task type %q", pkg.ErrInvalidArgument, t.Type)
	}

	uc.logger.Info("开始执行任务",
		port.String("task_id", t.ID),
		port.String("type", string(t.Type)),
	)
	output, err := handler.Handle(ctx, t.Input)
	if err != nil {
		uc.logger.Error("任务执行失败",
			port.String("task_id", t.ID),
			port.Err(err),
		)
		return "", err
	}
	uc.logger.Info("任务执行完成", port.String("task_id", t.ID))
	return output, nil
}
