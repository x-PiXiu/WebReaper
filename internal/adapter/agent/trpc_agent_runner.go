package agent

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent/builtin"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"

	llmadapter "webreaper/internal/adapter/llm"
	"webreaper/internal/adapter/telemetry"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// 默认 LLM 配置名：Agent 未指定时回退到此配置。
const defaultLLMConfigName = "default"

// TrpcAgentRunner 是基于 trpc-agent-go 的 Agent 执行器。
//
// 它根据 AgentConfig 构建 ReAct Agent（LLM + SystemPrompt + Tools），
// 执行任务并返回结果。Agent 自主决定调用哪些爬虫工具、怎么调用。
//
// LLM 客户端按 AgentConfig.LLMConfigName 从 LLMConfigRepository 解析（空则 default）。
type TrpcAgentRunner struct {
	llmCfgRepo   port.LLMConfigRepository
	registry     *port.ToolRegistry
	dataItemRepo port.DataItemRepository
	logger       port.Logger
}

// 编译期断言：TrpcAgentRunner 同时实现
//   - port.AgentSyncRunner（供 HTTP /agents/run 同步端点依赖接口）
//   - task.AgentRunner（供异步 worker 调用 RunTask）
var _ port.AgentSyncRunner = (*TrpcAgentRunner)(nil)

// NewTrpcAgentRunner 创建 Agent 执行器（注入 LLMConfigRepository 用于按 Agent 选 LLM，
// 注入 DataItemRepo 用于工具结果落库，注入 Logger 用于工具落库失败日志）。
func NewTrpcAgentRunner(llmCfgRepo port.LLMConfigRepository, registry *port.ToolRegistry, dataItemRepo port.DataItemRepository, logger port.Logger) *TrpcAgentRunner {
	return &TrpcAgentRunner{llmCfgRepo: llmCfgRepo, registry: registry, dataItemRepo: dataItemRepo, logger: logger}
}

// resolveLLM 按 AgentConfig.LLMConfigName 解析 LLM 配置并构建客户端。
// 空名回退到 "default"；找不到配置时返回错误。
func (r *TrpcAgentRunner) resolveLLM(ctx context.Context, llmConfigName string) (*openai.Model, error) {
	name := llmConfigName
	if name == "" {
		name = defaultLLMConfigName
	}
	cfg, err := r.llmCfgRepo.FindByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("LLM 配置 %q 不存在: %w", name, err)
	}
	return llmadapter.Build(cfg), nil
}

// RunInput 是执行 Agent 的输入。
type RunInput struct {
	Task       string         // 用户给 Agent 的任务描述（如"采集会话xxx的内容并总结"）
	AgentConfig entity.AgentConfig // Agent 配置（提示词+工具）
}

// RunOutput 是 Agent 执行的输出。
type RunOutput struct {
	Response     string            // Agent 的最终回复
	CrawlResults []entity.DataItem // Agent 调用爬虫产生的结果（供存储）
	Tokens       TokenUsage        // 本次任务的 token 消耗统计
}

// TokenUsage 记录一次 Agent 任务的 LLM token 消耗。
//
// 设计说明：
//   - 独立定义（不复用 trpc-agent-go 的 model.Usage），避免把框架类型泄露到上层。
//   - 从 runner 事件 channel 的 evt.Response.Usage 累加，仅计 ObjectTypeChatCompletion
//     事件（chunk 事件 usage 为 nil，框架自身也这么做——见 chainagent.go）。
//   - LLMCalls 反映 ReAct 迭代次数：一次 Agent 任务多次调 LLM 属正常，
//     若接近 MaxIterations 上限说明 Agent 陷入循环，需告警。
type TokenUsage struct {
	PromptTokens     int // 输入 token（含历史上下文，逐轮递增）
	CompletionTokens int // 输出 token
	TotalTokens      int // 合计
	LLMCalls         int // LLM 调用次数（观察 ReAct 迭代深度）
}

// Run 执行 Agent 任务。
//
// 流程：
//  1. 从 ToolRegistry 获取 AgentConfig 允许的工具
//  2. 构建 trpc-agent-go 的 LLM Agent（带工具 + 系统提示词）
//  3. 用 runner 执行，Agent 自主 ReAct 循环
//  4. 收集最终回复 + 工具调用产生的 CrawlResult
func (r *TrpcAgentRunner) Run(ctx context.Context, in RunInput) (RunOutput, error) {
	ctx, span := telemetry.StartSpan(ctx, "agent.run")
	defer span.End()
	span.SetAttributes(attribute.String("agent_name", in.AgentConfig.Name))

	// 1. 获取允许的工具（注入 DataItemRepo 用于落库）
	crawlers := r.registry.GetByNames(in.AgentConfig.Tools)
	tools := ConvertTools(crawlers, r.dataItemRepo, r.logger)

	// 2. 按 Agent 引用的 LLMConfigName 解析并构建 LLM 客户端
	llm, err := r.resolveLLM(ctx, in.AgentConfig.LLMConfigName)
	if err != nil {
		return RunOutput{}, fmt.Errorf("resolve llm: %w", err)
	}

	// 3. 构建 Agent。
	// 有工具时用 builtin.NewExplorer（支持 ReAct 工具调用循环）；
	// 无工具时用纯 llmagent（纯 LLM 对话）。
	var ag agent.Agent
	maxIter := in.AgentConfig.MaxIterations
	if maxIter <= 0 {
		maxIter = 10
	}

	if len(tools) > 0 {
		// 有工具：用 explorer（ReAct 循环，LLM 自主决定调用哪个工具）
		ag = builtin.NewExplorer(
			builtin.WithModel(llm),
			builtin.WithTools(tools),
			builtin.WithLLMAgentOptions(
				llmagent.WithInstruction(in.AgentConfig.SystemPrompt),
				llmagent.WithMaxToolIterations(maxIter),
			),
		)
	} else {
		// 无工具：纯 LLM Agent
		ag = llmagent.New(in.AgentConfig.Name,
			llmagent.WithModel(llm),
			llmagent.WithInstruction(in.AgentConfig.SystemPrompt),
		)
	}

	// 4. 用 runner 执行
	rn := runner.NewRunner("webreaper", ag)
	events, err := rn.Run(ctx, "agent-user", "default",
		model.Message{Role: model.RoleUser, Content: in.Task},
	)
	if err != nil {
		return RunOutput{}, fmt.Errorf("agent run: %w", err)
	}

	// 5. 收集响应（拼接流式输出 + explorer 最终结果）
	var sb strings.Builder
	var tokens TokenUsage
	for evt := range events {
		if evt.IsError() {
			if evt.Error != nil {
				reportTokenUsage(ctx, r.logger, tokens)
				return RunOutput{Response: sb.String(), Tokens: tokens}, fmt.Errorf("agent error: %v", evt.Error)
			}
			break
		}
		// 流式增量
		if evt.Object == model.ObjectTypeChatCompletionChunk && evt.Response != nil {
			for _, choice := range evt.Response.Choices {
				if choice.Delta.Content != "" {
					sb.WriteString(choice.Delta.Content)
				}
			}
		}
		// chat completion（含 Message.Content + Usage）
		if evt.Object == model.ObjectTypeChatCompletion && evt.Response != nil {
			for _, choice := range evt.Response.Choices {
				if choice.Delta.Content != "" {
					sb.WriteString(choice.Delta.Content)
				}
				if choice.Message.Content != "" && sb.Len() == 0 {
					sb.WriteString(choice.Message.Content)
				}
			}
			// 累加 token 用量（chunk 事件 usage 为 nil，只在 completion 事件计）
			tokens = accumulateUsage(tokens, evt.Response.Usage)
		}
		// runner completion（explorer 工具调用后的最终结果）
		if evt.Object == model.ObjectTypeRunnerCompletion && evt.Response != nil {
			for _, choice := range evt.Response.Choices {
				if choice.Message.Content != "" {
					sb.WriteString(choice.Message.Content)
				}
			}
		}
	}

	// 上报 token 消耗（日志 + trace span 属性）
	reportTokenUsage(ctx, r.logger, tokens)
	span.SetAttributes(
		attribute.Int("token.prompt", tokens.PromptTokens),
		attribute.Int("token.completion", tokens.CompletionTokens),
		attribute.Int("token.total", tokens.TotalTokens),
		attribute.Int("token.llm_calls", tokens.LLMCalls),
	)

	return RunOutput{Response: sb.String(), Tokens: tokens}, nil
}

// reportTokenUsage 通过日志上报 token 消耗，便于成本核算。
// 即使任务失败也调用（在错误 return 前上报），确保每次实际消耗都被记录。
func reportTokenUsage(_ context.Context, logger port.Logger, t TokenUsage) {
	if logger == nil {
		return
	}
	logger.Info("token 消耗",
		port.Int("prompt_tokens", t.PromptTokens),
		port.Int("completion_tokens", t.CompletionTokens),
		port.Int("total_tokens", t.TotalTokens),
		port.Int("llm_calls", t.LLMCalls),
	)
}

// accumulateUsage 把单次 LLM 调用的 usage 累加到累计值。
// usage 为 nil（chunk 事件或缺失）时跳过，返回原值。
//
// 抽成纯函数便于单元测试（事件循环本身依赖 trpc-agent-go 难以单测，
// 这是谦卑对象模式——把可测的累加逻辑从难测的事件循环中分离）。
func accumulateUsage(acc TokenUsage, usage *model.Usage) TokenUsage {
	if usage == nil {
		return acc
	}
	acc.PromptTokens += usage.PromptTokens
	acc.CompletionTokens += usage.CompletionTokens
	acc.TotalTokens += usage.TotalTokens
	acc.LLMCalls++
	return acc
}

// runSyncInternal 是 Run 的同步便捷方法（直接返回最终文本），供内部和 RunTask 复用。
func (r *TrpcAgentRunner) runSyncInternal(ctx context.Context, task string, cfg entity.AgentConfig) (string, error) {
	out, err := r.Run(ctx, RunInput{Task: task, AgentConfig: cfg})
	if err != nil {
		return "", err
	}
	return out.Response, nil
}

// RunSync 实现 port.AgentSyncRunner 接口（供 HTTP handler 依赖接口而非具体 struct）。
// 把 port 层 DTO 转成内部 RunInput，复用 Run。
func (r *TrpcAgentRunner) RunSync(ctx context.Context, in port.AgentRunInput) (port.AgentRunOutput, error) {
	prompt := in.SystemPrompt
	if prompt == "" {
		prompt = entity.DefaultSystemPrompt
	}
	cfg := entity.AgentConfig{
		Name:          "api-agent",
		SystemPrompt:  prompt,
		Tools:         in.Tools,
		LLMConfigName: in.LLMConfigName,
	}.FillDefaults()
	resp, err := r.runSyncInternal(ctx, in.Task, cfg)
	if err != nil {
		return port.AgentRunOutput{}, err
	}
	return port.AgentRunOutput{Response: resp}, nil
}

// RunTask 实现 task.AgentRunner 接口（供 AgentHandler 异步调用）。
func (r *TrpcAgentRunner) RunTask(ctx context.Context, taskStr string, tools []string, systemPrompt string) (string, error) {
	prompt := systemPrompt
	if prompt == "" {
		prompt = entity.DefaultSystemPrompt
	}
	return r.runSyncInternal(ctx, taskStr, entity.AgentConfig{
		Name:          "async-agent",
		SystemPrompt:  prompt,
		Tools:         tools,
	}.FillDefaults())
}
