// Package ai 提供 trpc-agent-go 的 LLM 调用实现（适配器层）。
//
// 接入框架的 session service，实现真正的多轮对话（不再字符串拼接）。
// 每个 sessionID 对应一段独立的对话历史，框架自动维护上下文。
package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent/builtin"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
	"trpc.group/trpc-go/trpc-agent-go/tool"

	agentadapter "webreaper/internal/adapter/agent"
	llmadapter "webreaper/internal/adapter/llm"
	"webreaper/internal/adapter/telemetry"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// 默认 LLM 配置名：未指定 llmConfigName 时回退到此配置。
const DefaultLLMConfigName = "default"

// TrpcAgentGenerator 是 port.AIGenerator 的 trpc-agent-go 实现。
// 使用框架的 session service 管理多轮对话历史。
//
// LLM 客户端不再在启动时固定创建，而是按请求的 llmConfigName
// 从 LLMConfigRepository 取配置、经工厂创建（带缓存，避免重复建）。
// llmCacheEntry 是 LLM 客户端缓存项（带过期时间）。
// 设计动机：LLM 配置（API Key/BaseURL/Model）可被用户随时修改，
// 缓存若不过期，改了配置要重启才生效——违反"运行时可调"。
// 用 TTL 兜底（30s），避免跨对象耦合（无需 handler 改完后回调通知）。
type llmCacheEntry struct {
	llm      *openai.Model
	cachedAt time.Time
}

// llmCacheTTL LLM 客户端缓存有效期。过期后重新从 DB 取配置构建。
// 30s：兼顾性能（热点模型不重复建连）与配置生效时效。
const llmCacheTTL = 30 * time.Second

type TrpcAgentGenerator struct {
	llmCfgRepo   port.LLMConfigRepository
	capResolver  port.CapabilityResolver  // 能力路由（新表优先，旧表兜底）
	mu           sync.RWMutex
	runners      map[string]runner.Runner
	sessionSvc   *inmemory.SessionService
	toolRegistry *port.ToolRegistry      // 全局工具注册表（所有爬虫）
	llmCache     sync.Map                // map[string]*llmCacheEntry（按 LLMConfigName 缓存，带 TTL）
	memory       port.ConversationMemory // 会话历史（重启后从 DB 恢复上下文）；nil 则不 seed 历史
	logger       port.Logger             // 日志（注入给 toolAdapter 替代 fmt.Printf）
	usage        port.UsageRecorder      // LLM 用量计量（可选注入，nil=不记录）——经济系统基础
	metrics      port.MetricsCollector   // R3 运营指标（可选；nil=不采集）
}

// SetMetrics 注入指标采集器（可选，R3——LLM 调用成功率埋点）。
func (g *TrpcAgentGenerator) SetMetrics(m port.MetricsCollector) {
	g.metrics = m
}

// trackLLMCall LLM 调用埋点（成功/失败/慢调用）。
func (g *TrpcAgentGenerator) trackLLMCall(err error, elapsed time.Duration) {
	if g.metrics == nil {
		return
	}
	ctx := context.Background()
	_ = g.metrics.Incr(ctx, port.MetricLLMCalls)
	if err != nil {
		_ = g.metrics.Incr(ctx, port.MetricLLMErrors)
	}
	if elapsed > 30*time.Second {
		_ = g.metrics.Incr(ctx, port.MetricLLMSlow)
	}
}

// 编译期断言：实现 port.AIGenerator。
var _ port.AIGenerator = (*TrpcAgentGenerator)(nil)

// NewTrpcAgentGenerator 创建生成器。
// memory 可为 nil（不启用历史恢复）；非 nil 时首次创建会话 runner 会从 DB seed 历史。
func NewTrpcAgentGenerator(llmCfgRepo port.LLMConfigRepository, toolRegistry *port.ToolRegistry, memory port.ConversationMemory, logger port.Logger) (*TrpcAgentGenerator, error) {
	g := &TrpcAgentGenerator{
		llmCfgRepo:   llmCfgRepo,
		runners:      make(map[string]runner.Runner),
		toolRegistry: toolRegistry,
		memory:       memory,
		logger:       logger,
	}
	g.sessionSvc = inmemory.NewSessionService()
	return g, nil
}

// SetMemory 注入会话记忆（支持后装配，用于解决循环依赖或延迟初始化）。
func (g *TrpcAgentGenerator) SetMemory(m port.ConversationMemory) {
	g.memory = m
}

// SetUsageRecorder 注入 LLM 用量记录器（可选；nil 或未注入 = 不计量，行为不变）。
// 计量上下文（租户/场景）从 ctx 读取（port.WithUsageContext 注入）。
func (g *TrpcAgentGenerator) SetUsageRecorder(r port.UsageRecorder) {
	g.usage = r
}

// SetCapResolver 注入能力路由解析器（可选——未注入时 LLM 配置只走旧表 llm_configs）。
func (g *TrpcAgentGenerator) SetCapResolver(r port.CapabilityResolver) {
	g.capResolver = r
}

// resolveLLM 按 llmConfigName 解析并（带 TTL 缓存地）构建 LLM 客户端。
// 空名回退到 CapabilityResolver 的 llm-chat 能力路由（新表优先），
// 再回退到 llm_configs 表的 "default" 配置。
// 缓存 TTL 30s：用户改了 LLM 配置后，最多 30s 内旧客户端被淘汰、新配置生效。
func (g *TrpcAgentGenerator) resolveLLM(ctx context.Context, llmConfigName string) (*openai.Model, error) {
	name := llmConfigName
	if name == "" {
		name = DefaultLLMConfigName
	}
	// 缓存命中且未过期
	if v, ok := g.llmCache.Load(name); ok {
		entry := v.(*llmCacheEntry)
		if time.Since(entry.cachedAt) < llmCacheTTL {
			return entry.llm, nil
		}
		// 过期：删除旧条目，走重建
		g.llmCache.Delete(name)
	}
	// ① 旧表：按 llmConfigName 查 llm_configs
	cfg, err := g.llmCfgRepo.FindByName(ctx, name)
	if err != nil && llmConfigName == "" && g.capResolver != nil {
		// ② 新表兜底：llmConfigName 为空时走 CapabilityResolver（能力路由）
		if cap, capErr := g.capResolver.Resolve(ctx, "llm-chat"); capErr == nil && cap.APIKey != "" {
			cfg = entity.LLMConfig{
				Name:    "auto",
				Provider: cap.VendorID,
				APIKey:  cap.APIKey,
				BaseURL: cap.BaseURL,
				Model:   cap.Model,
			}
			err = nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("LLM 配置 %q 不存在: %w", name, err)
	}
	llm := llmadapter.Build(cfg)
	g.llmCache.Store(name, &llmCacheEntry{llm: llm, cachedAt: time.Now()})
	return llm, nil
}

// InvalidateLLMCache 显式失效指定 LLM 配置的缓存（name 空则清空全部）。
// 当前 TTL 已能保证最终一致，此方法留作未来"配置改完即时生效"的扩展点。
func (g *TrpcAgentGenerator) InvalidateLLMCache(name string) {
	if name == "" {
		g.llmCache.Range(func(k, _ any) bool {
			g.llmCache.Delete(k)
			return true
		})
		return
	}
	g.llmCache.Delete(name)
}

// getOrCreateRunner 获取或创建指定 sessionID 的 runner。
// 同一个 sessionID 的多次调用共享对话历史（多轮对话）。
// 会话隔离：sessionID 即 conversationID，不同会话天然隔离。
// 注意：同一会话若中途切换 LLM，会复用已有 runner（带旧 LLM）。
// 这是有意为之——一个会话的对话历史应连续，不应因换模型而断开。
func (g *TrpcAgentGenerator) getOrCreateRunner(sessionID string, systemPrompt string, llm *openai.Model) (runner.Runner, bool) {
	g.mu.RLock()
	if r, ok := g.runners[sessionID]; ok {
		g.mu.RUnlock()
		return r, false // 已存在，非新建
	}
	g.mu.RUnlock()

	g.mu.Lock()
	defer g.mu.Unlock()
	// double check
	if r, ok := g.runners[sessionID]; ok {
		return r, false
	}

	// 为这个 session 创建带系统提示词的 Agent
	agentOpts := []llmagent.Option{llmagent.WithModel(llm)}
	if systemPrompt != "" {
		agentOpts = append(agentOpts, llmagent.WithInstruction(systemPrompt))
	}
	ag := llmagent.New("webreaper-chat", agentOpts...)

	// 创建带 session service 的 runner（关键：WithSessionService 启用多轮对话）
	r := runner.NewRunner("webreaper", ag,
		runner.WithSessionService(g.sessionSvc),
	)
	g.runners[sessionID] = r
	return r, true // 新建
}

// ChatStream 实现 port.AIGenerator：流式对话（支持多轮上下文）。
// conversationID 是会话隔离的关键：直接作为 sessionID，根治"会话间记忆串台"。
func (g *TrpcAgentGenerator) ChatStream(ctx context.Context, conversationID string, llmConfigName string, messages []port.ChatMessage, onDelta func(delta string)) (string, error) {
	return g.ChatStreamWithOptions(ctx, port.ChatStreamInput{
		ConversationID: conversationID,
		LLMConfigName:  llmConfigName,
		Messages:       messages,
		OnDelta:        onDelta,
	})
}

// ChatStreamWithOptions 实现 port.OptionsAwareGenerator：带控制选项的对话。
//
// 选项映射（适配器层职责——业务零感知厂商差异）：
//   - ResponseFormat=json + SchemaExample：注入框架原生结构化输出
//     （agent.WithStructuredOutputJSON → OpenAI response_format json_schema；
//     DeepSeek 变体由框架自动降级为 json_object）
//   - DisableThinking：厂商专属请求参数（如 DeepSeek enable_thinking=false）——
//     框架 Request.ExtraFields 已预留字段（model/request.go），但当前版本无
//     per-request 入口；降级为提示词层禁令（在 system prompt 追加），
//     待框架暴露 RequestOption 后一行接入请求参数层（纵深防御第二层）。
func (g *TrpcAgentGenerator) ChatStreamWithOptions(ctx context.Context, in port.ChatStreamInput) (result string, retErr error) {
	ctx, span := telemetry.StartSpan(ctx, "ai.chat_stream")
	defer span.End()
	start := time.Now()
	defer func() { g.trackLLMCall(retErr, time.Since(start)) }() // R3 埋点

	if len(in.Messages) == 0 {
		return "", fmt.Errorf("no messages")
	}

	// 解析 LLM 客户端（按 llmConfigName，空则 default）
	llm, err := g.resolveLLM(ctx, in.LLMConfigName)
	if err != nil {
		return "", err
	}

	// 提取 system prompt（如果有）和最后一条 user 消息
	var systemPrompt string
	lastUser := in.Messages[len(in.Messages)-1]
	for _, msg := range in.Messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
		}
	}

	// DisableThinking 降级：提示词层禁令（模型无关兜底——请求参数层待框架
	// RequestOption 暴露后接入厂商专属字段，届时此处删除）
	if in.Options.DisableThinking {
		systemPrompt = "严禁输出任何思考过程、推理过程或 <think> 内容，只输出最终结果。\n" + systemPrompt
	}

	// 会话隔离：sessionID 直接用 conversationID（根治串台）。
	// 后台编排路径（conversationID 为空）退化为按内容 hash，保持原行为。
	sessionID := in.ConversationID
	if sessionID == "" {
		sessionID = fmt.Sprintf("adhoc-%d", hashString(systemPrompt+":"+in.LLMConfigName))
	}

	r, isNew := g.getOrCreateRunner(sessionID, systemPrompt, llm)

	// 记忆恢复：新建会话 runner 时，若注入了 memory，从 DB 取历史 seed 进框架。
	// 框架的 seedSessionHistory 只在空 session 时执行（isNew=true 即首次），
	// 后续多轮走内存累积，不重复 seed。system 消息已通过 Instruction 注入，这里不传。
	// 如此实现"重启后旧会话续聊仍带历史上下文"。
	if isNew && g.memory != nil && in.ConversationID != "" {
		if history, hErr := g.memory.History(ctx, in.ConversationID); hErr == nil && len(history) > 0 {
			seedMsgs := make([]model.Message, 0, len(history)+1)
			for _, hm := range history {
				role := model.RoleUser
				if hm.Role == "assistant" {
					role = model.RoleAssistant
				}
				seedMsgs = append(seedMsgs, model.Message{Role: role, Content: hm.Content})
			}
			// 本次 user 消息放最后，框架会合并去重（mergeCurrentTurnMessagesIntoSeed）
			seedMsgs = append(seedMsgs, model.Message{Role: model.RoleUser, Content: lastUser.Content})
			events, err := runner.RunWithMessages(ctx, r, "webreaper-user", sessionID, seedMsgs, g.structuredOutputRunOption(in.Options)...)
			if err != nil {
				return "", fmt.Errorf("runner run with messages: %w", err)
			}
			return g.drainChatStreamEvents(ctx, events, in.OnDelta, in.LLMConfigName)
		}
	}

	events, err := r.Run(ctx, "webreaper-user", sessionID,
		model.Message{Role: model.RoleUser, Content: lastUser.Content},
		g.structuredOutputRunOption(in.Options)...,
	)
	if err != nil {
		return "", fmt.Errorf("runner run: %w", err)
	}
	return g.drainChatStreamEvents(ctx, events, in.OnDelta, in.LLMConfigName)
}

// structuredOutputRunOption 按选项构造框架结构化输出 RunOption（无则返回空）。
// 框架原生：agent.WithStructuredOutputJSON 自动推断 schema（DeepSeek 变体自动降级 json_object）。
func (g *TrpcAgentGenerator) structuredOutputRunOption(opts port.ChatOptions) []agent.RunOption {
	if opts.ResponseFormat != "json" {
		return nil
	}
	if opts.SchemaExample != nil {
		return []agent.RunOption{agent.WithStructuredOutputJSON(opts.SchemaExample, true, opts.SchemaDescription)}
	}
	if opts.JSONSchema != nil {
		return []agent.RunOption{agent.WithStructuredOutputJSONSchema("wr_output", opts.JSONSchema, true, opts.SchemaDescription)}
	}
	return nil
}

// drainChatStreamEvents 消费 runner 的事件流，把文本增量回调出去，并统计 token 用量。
// 抽出为独立方法，避免 ChatStream 在 seed/非 seed 两条路径上重复事件消费逻辑。
// （谦卑对象模式：事件循环依赖框架难单测，但可测的累加逻辑已在 accumulateUsage 等
// 纯函数中；此处只是 IO 编排。）
func (g *TrpcAgentGenerator) drainChatStreamEvents(ctx context.Context, events <-chan *event.Event, onDelta func(delta string), llmConfigName string) (string, error) {
	_, span := telemetry.StartSpan(ctx, "ai.chat_stream.drain")
	defer span.End()

	var sb strings.Builder
	var promptTokens, completionTokens, totalTokens, llmCalls int
	for evt := range events {
		if evt.IsError() {
			if evt.Error != nil {
				return sb.String(), fmt.Errorf("llm error: %v", evt.Error)
			}
			break
		}
		if evt.Object == model.ObjectTypeChatCompletionChunk && evt.Response != nil {
			for _, choice := range evt.Response.Choices {
				if choice.Delta.Content != "" {
					sb.WriteString(choice.Delta.Content)
					if onDelta != nil {
						onDelta(choice.Delta.Content)
					}
				}
			}
		}
		if evt.Object == model.ObjectTypeChatCompletion && evt.Response != nil {
			for _, choice := range evt.Response.Choices {
				if choice.Message.Content != "" && sb.Len() == 0 {
					sb.WriteString(choice.Message.Content)
					if onDelta != nil {
						onDelta(choice.Message.Content)
					}
				}
			}
			if evt.Response.Usage != nil {
				promptTokens += evt.Response.Usage.PromptTokens
				completionTokens += evt.Response.Usage.CompletionTokens
				totalTokens += evt.Response.Usage.TotalTokens
				llmCalls++
			}
		}
	}
	if llmCalls > 0 {
		g.logger.Info("token 消耗",
			port.Int("prompt_tokens", promptTokens),
			port.Int("completion_tokens", completionTokens),
			port.Int("total_tokens", totalTokens),
			port.Int("llm_calls", llmCalls),
			port.String("path", "chat_stream"),
		)
		span.SetAttributes(
			attribute.Int("token.prompt", promptTokens),
			attribute.Int("token.completion", completionTokens),
			attribute.Int("token.total", totalTokens),
			attribute.Int("token.llm_calls", llmCalls),
		)
		// 用量计量落库（经济系统基础）：租户/场景从 ctx 取（调用方 WithUsageContext 注入），
		// 后台任务 ctx 无租户则记空（平台消耗）。失败不阻断主流程。
		if g.usage != nil {
			_ = g.usage.RecordUsage(ctx, entity.UsageRecord{
				TenantID:         port.UsageTenantFrom(ctx),
				Scene:            port.UsageSceneFrom(ctx),
				LLMConfigName:    llmConfigName,
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      totalTokens,
				LLMCalls:         llmCalls,
			})
		}
	}
	return sb.String(), nil
}

// RunWithTools 实现 port.AIGenerator：带工具的流式执行（ReAct 循环）。
// 所有爬虫工具全局可用（不按 Agent 配置过滤），LLM 自主决定调哪个。
// conversationID 作为 sessionID 实现会话隔离（带工具模式也按会话隔离，根治串台）。
func (g *TrpcAgentGenerator) RunWithTools(ctx context.Context, conversationID string, llmConfigName string, task string, systemPrompt string, toolNames []string, onEvent func(event port.ToolEvent)) error {
	ctx, span := telemetry.StartSpan(ctx, "ai.run_with_tools")
	defer span.End()

	// 解析 LLM 客户端（按 llmConfigName，空则 default）
	llm, err := g.resolveLLM(ctx, llmConfigName)
	if err != nil {
		if onEvent != nil {
			onEvent(port.ToolEvent{Type: "error", Error: err.Error()})
		}
		return err
	}

	// 构建工具：按调用方指定的 toolNames 过滤。
	// 如果 toolNames 为空（前端聊天），用全部工具；如果非空（GEO 监测），只用指定工具。
	var tools []tool.Tool
	if g.toolRegistry != nil {
		var selectedCrawlers []port.CrawlerTool
		if len(toolNames) == 0 {
			selectedCrawlers = g.toolRegistry.All() // 聊天模式：全部工具
		} else {
			selectedCrawlers = g.toolRegistry.GetByNames(toolNames) // 监测模式：只用搜索工具
		}
		adapterTools := agentadapter.ConvertTools(selectedCrawlers)
		tools = adapterTools
	}

	// 构建 explorer Agent（有工具用 explorer，无工具用 llmagent）
	var ag agent.Agent
	agOpts := []llmagent.Option{llmagent.WithModel(llm)}
	if systemPrompt != "" {
		agOpts = append(agOpts, llmagent.WithInstruction(systemPrompt))
	}

	if len(tools) > 0 {
		ag = builtin.NewExplorer(
			builtin.WithModel(llm),
			builtin.WithTools(tools),
			builtin.WithLLMAgentOptions(agOpts...),
		)
	} else {
		ag = llmagent.New("chat-no-tools", agOpts...)
	}

	r := runner.NewRunner("webreaper", ag,
		runner.WithSessionService(g.sessionSvc),
	)

	// 会话隔离：sessionID 用 conversationID（带工具模式也按会话隔离）。
	// 后台编排路径（conversationID 为空）退化为一次性 session，避免串台。
	sessionID := conversationID
	if sessionID == "" {
		sessionID = fmt.Sprintf("adhoc-tools-%d", hashString(task+":"+llmConfigName))
	}
	events, err := r.Run(ctx, "chat-user", sessionID, model.Message{Role: model.RoleUser, Content: task})
	if err != nil {
		return fmt.Errorf("runner run: %w", err)
	}

	for evt := range events {
		if evt.IsError() {
			errMsg := "unknown"
			if evt.Error != nil {
				errMsg = evt.Error.Error()
			}
			if onEvent != nil {
				onEvent(port.ToolEvent{Type: "error", Error: errMsg})
			}
			return fmt.Errorf("agent error: %s", errMsg)
		}

		// 文本增量
		if (evt.Object == model.ObjectTypeChatCompletionChunk || evt.Object == model.ObjectTypeChatCompletion) && evt.Response != nil {
			for _, choice := range evt.Response.Choices {
				if choice.Delta.Content != "" && onEvent != nil {
					onEvent(port.ToolEvent{Type: "text-delta", Text: choice.Delta.Content})
				}
				if choice.Message.Content != "" && onEvent != nil {
					onEvent(port.ToolEvent{Type: "text-delta", Text: choice.Message.Content})
				}
				// 工具调用检测
				for _, tc := range choice.Delta.ToolCalls {
					if onEvent != nil && tc.Function.Name != "" {
						onEvent(port.ToolEvent{Type: "tool-call", ToolName: tc.Function.Name, ToolArgs: string(tc.Function.Arguments)})
					}
				}
				for _, tc := range choice.Message.ToolCalls {
					if onEvent != nil && tc.Function.Name != "" {
						onEvent(port.ToolEvent{Type: "tool-call", ToolName: tc.Function.Name, ToolArgs: string(tc.Function.Arguments)})
					}
				}
			}
		}

		// 工具返回结果
		if evt.Object == model.ObjectTypeToolResponse && evt.Response != nil && onEvent != nil {
			result := ""
			if len(evt.Response.Choices) > 0 {
				result = evt.Response.Choices[0].Message.Content
			}
			onEvent(port.ToolEvent{Type: "tool-result", ToolResult: truncateStr(result, 500)})
		}

		// Runner 完成：explorer 的最终总结在这里。
		// 注意：这部分内容是一整块返回的（非流式增量），直接发出会让前端看到"一次性输出"。
		// 改为按句/段落切片，逐块流式发送（打字机效果），提升体验。
		if evt.Object == model.ObjectTypeRunnerCompletion && evt.Response != nil {
			for _, choice := range evt.Response.Choices {
				if choice.Message.Content != "" && onEvent != nil {
					streamChunked(ctx, choice.Message.Content, onEvent)
				}
			}
		}
	}

	if onEvent != nil {
		onEvent(port.ToolEvent{Type: "finish"})
	}
	return nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (g *TrpcAgentGenerator) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, r := range g.runners {
		_ = r.Close()
	}
	g.runners = make(map[string]runner.Runner)
	return nil
}

// 辅助函数
func hashString(s string) uint32 {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}

// streamChunked 把一整块文本按句/段落切片，逐块流式发送（打字机效果）。
//
// 设计动机：explorer 的 RunnerCompletion 返回的是完整总结（非流式增量），
// 直接发出会让前端看到"一次性刷出"。切片后逐块发送，模拟流式体验。
//
// 切片策略：按换行/句号/问号/感叹号切分，每块不超过 120 字符。
// 间隔 15ms（足够快不影响总时长，又足够慢让眼睛看到渐进感）。
// ctx 取消时立即停止（支持用户点"停止生成"）。
func streamChunked(ctx context.Context, text string, onEvent func(port.ToolEvent)) {
	if text == "" || onEvent == nil {
		return
	}
	chunks := splitForStreaming(text)
	for _, ch := range chunks {
		// ctx 取消则停止（用户点了停止生成）
		select {
		case <-ctx.Done():
			return
		default:
		}
		onEvent(port.ToolEvent{Type: "text-delta", Text: ch})
		time.Sleep(15 * time.Millisecond)
	}
}

// splitForStreaming 把文本切成适合流式发送的小块。
// 按自然句边界（换行、句号、问号、感叹号）切，超长块强制按长度切。
func splitForStreaming(text string) []string {
	const maxLen = 120
	var chunks []string
	// 先按换行分段
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if line == "" {
			chunks = append(chunks, "\n")
			continue
		}
		// 再按句号/问号/感叹号分句
		sentences := splitSentences(line)
		for _, s := range sentences {
			s = strings.TrimRight(s, " \t")
			if s == "" {
				continue
			}
			// 超长句按 maxLen 强制切
			for len(s) > maxLen {
				chunks = append(chunks, s[:maxLen])
				s = s[maxLen:]
			}
			if s != "" {
				chunks = append(chunks, s)
			}
		}
		chunks = append(chunks, "\n") // 保留换行
	}
	// 去掉末尾多余的换行
	if n := len(chunks); n > 0 && chunks[n-1] == "\n" {
		chunks = chunks[:n-1]
	}
	return chunks
}

// splitSentences 按句末标点切分，保留标点。
func splitSentences(line string) []string {
	var result []string
	current := strings.Builder{}
	for _, r := range line {
		current.WriteRune(r)
		// 中英文句末标点作为分割点
		if r == '.' || r == '!' || r == '?' || r == '。' || r == '！' || r == '？' || r == '；' || r == ';' {
			result = append(result, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}
