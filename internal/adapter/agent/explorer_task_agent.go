package agent

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent/builtin"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"

	llmadapter "webreaper/internal/adapter/llm"
	"webreaper/internal/adapter/telemetry"
	"webreaper/internal/usecase/port"
)

// defaultTaskSystemPrompt 通用任务的默认系统提示词。
// 引导 Agent 自主规划：分析任务 → 选工具 → 执行 → 自我判断完成。
const defaultTaskSystemPrompt = `你是一个能自主完成任意任务的 Agent。
面对任务时：
1. 先分析任务需要什么信息/操作
2. 选择合适的工具调用（采集、生成内容、保存、推送等）
3. 根据工具返回结果继续推理，必要时调用更多工具
4. 任务真正完成后，给出清晰的总结回复
不要在任务未完成时就停止。工具调用失败时尝试其他方式。`

// ExplorerTaskAgent 是 port.TaskAgent 的 Explorer 实现（通用任务 Agent）。
//
// 与 TrpcAgentRunner 的区别：
//   - TrpcAgentRunner 绑定 AgentConfig（数据库配置的固定提示词/工具），配置驱动。
//   - ExplorerTaskAgent 接受任意 TaskInput（任务描述+工具列表），任务驱动。
//
// 自主性来源：用 builtin.NewExplorer（ReAct 循环）——LLM 自己决定调哪个工具、
// 调几次、何时完成。调用方只给任务，不规定步骤。
//
// 整洁架构：实现 port.TaskAgent 接口，框架细节（trpc-agent-go）封装在本包。
type ExplorerTaskAgent struct {
	llmCfgRepo   port.LLMConfigRepository
	registry     *port.ToolRegistry
	dataItemRepo port.DataItemRepository
	logger       port.Logger
	maxIter      int // ReAct 最大工具调用轮数（安全阀，默认 20）
}

// 编译期断言：实现 port.TaskAgent。
var _ port.TaskAgent = (*ExplorerTaskAgent)(nil)

// NewExplorerTaskAgent 创建通用任务 Agent。
func NewExplorerTaskAgent(
	llmCfgRepo port.LLMConfigRepository,
	registry *port.ToolRegistry,
	dataItemRepo port.DataItemRepository,
	logger port.Logger,
) *ExplorerTaskAgent {
	if logger == nil {
		logger = port.NopLogger{}
	}
	return &ExplorerTaskAgent{
		llmCfgRepo:   llmCfgRepo,
		registry:     registry,
		dataItemRepo: dataItemRepo,
		logger:       logger,
		maxIter:      20,
	}
}

// Execute 执行任意任务，Agent 自主 ReAct 循环直到完成。
func (a *ExplorerTaskAgent) Execute(ctx context.Context, in port.TaskInput, onEvent func(port.TaskEvent)) (port.TaskResult, error) {
	if in.Task == "" {
		return port.TaskResult{}, fmt.Errorf("task is required")
	}
	ctx, span := telemetry.StartSpan(ctx, "task_agent.execute")
	defer span.End()
	span.SetAttributes(attribute.String("task", truncateForLLM(in.Task, 100)))

	emit := func(e port.TaskEvent) {
		if onEvent != nil {
			onEvent(e)
		}
	}

	// 1. 解析 LLM 客户端（用 default 配置）
	llm, err := a.resolveLLM(ctx, "")
	if err != nil {
		return port.TaskResult{}, fmt.Errorf("resolve llm: %w", err)
	}

	// 2. 获取工具（空列表=全部已注册）
	var tools []port.CrawlerTool
	if len(in.Tools) == 0 && a.registry != nil {
		tools = a.registry.All()
	} else if a.registry != nil {
		tools = a.registry.GetByNames(in.Tools)
	}
	adapterTools := ConvertTools(tools, a.dataItemRepo, a.logger)

	// 3. 系统提示词（空用默认）
	prompt := in.SystemPrompt
	if prompt == "" {
		prompt = defaultTaskSystemPrompt
	}

	// 4. 构建 Explorer Agent（ReAct 循环：LLM 自主决定调工具）
	ag := builtin.NewExplorer(
		builtin.WithModel(llm),
		builtin.WithTools(adapterTools),
		builtin.WithLLMAgentOptions(
			llmagent.WithInstruction(prompt),
			llmagent.WithMaxToolIterations(a.maxIter),
		),
	)

	// 5. 执行
	rn := runner.NewRunner("webreaper-task", ag)
	events, err := rn.Run(ctx, "task-user", "default",
		model.Message{Role: model.RoleUser, Content: in.Task},
	)
	if err != nil {
		return port.TaskResult{}, fmt.Errorf("agent run: %w", err)
	}

	// 6. 收集回复 + 透传事件
	var sb []byte
	for evt := range events {
		if evt.IsError() {
			if evt.Error != nil {
				emit(port.TaskEvent{Type: "error", Error: evt.Error.Error()})
				return port.TaskResult{Response: string(sb)}, fmt.Errorf("agent error: %v", evt.Error)
			}
			break
		}
		// 流式增量文本
		if evt.Object == model.ObjectTypeChatCompletionChunk && evt.Response != nil {
			for _, choice := range evt.Response.Choices {
				if choice.Delta.Content != "" {
					sb = append(sb, choice.Delta.Content...)
					emit(port.TaskEvent{Type: "text-delta", Text: choice.Delta.Content})
				}
			}
		}
		// 完整回复
		if evt.Object == model.ObjectTypeChatCompletion && evt.Response != nil {
			for _, choice := range evt.Response.Choices {
				if choice.Message.Content != "" && len(sb) == 0 {
					sb = append(sb, choice.Message.Content...)
				}
			}
		}
		// runner 完成（explorer 工具调用后的最终结果）
		if evt.Object == model.ObjectTypeRunnerCompletion && evt.Response != nil {
			for _, choice := range evt.Response.Choices {
				if choice.Message.Content != "" {
					sb = append(sb, choice.Message.Content...)
				}
			}
		}
	}

	return port.TaskResult{Response: string(sb)}, nil
}

// resolveLLM 复用 TrpcAgentRunner 的 LLM 解析逻辑（按配置名，空用 default）。
func (a *ExplorerTaskAgent) resolveLLM(ctx context.Context, llmConfigName string) (*openai.Model, error) {
	name := llmConfigName
	if name == "" {
		name = defaultLLMConfigName
	}
	cfg, err := a.llmCfgRepo.FindByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("LLM 配置 %q 不存在: %w", name, err)
	}
	return llmadapter.Build(cfg), nil
}

// 确保未使用 import 的编译期检查通过（agent 包用于类型引用）
var _ agent.Agent = (agent.Agent)(nil)
