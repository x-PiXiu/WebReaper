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

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent/builtin"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
	"trpc.group/trpc-go/trpc-agent-go/tool"

	agentadapter "webreaper/internal/adapter/agent"
	llmadapter "webreaper/internal/adapter/llm"
	"webreaper/internal/adapter/telemetry"
	"webreaper/internal/usecase/port"
)

// 默认 LLM 配置名：未指定 llmConfigName 时回退到此配置。
const DefaultLLMConfigName = "default"

// TrpcAgentGenerator 是 port.AIGenerator 的 trpc-agent-go 实现。
// 使用框架的 session service 管理多轮对话历史。
//
// LLM 客户端不再在启动时固定创建，而是按请求的 llmConfigName
// 从 LLMConfigRepository 取配置、经工厂创建（带缓存，避免重复建）。
type TrpcAgentGenerator struct {
	llmCfgRepo   port.LLMConfigRepository
	mu           sync.RWMutex
	runners      map[string]runner.Runner
	sessionSvc   *inmemory.SessionService
	toolRegistry *port.ToolRegistry // 全局工具注册表（所有爬虫）
	llmCache     sync.Map           // map[string]*openai.Model（按 LLMConfigName 缓存客户端）
	logger       port.Logger        // 日志（注入给 toolAdapter 替代 fmt.Printf）
}

// 编译期断言：实现 port.AIGenerator。
var _ port.AIGenerator = (*TrpcAgentGenerator)(nil)

// NewTrpcAgentGenerator 创建生成器（注入 LLMConfigRepository 用于运行时解析 LLM 配置）。
func NewTrpcAgentGenerator(llmCfgRepo port.LLMConfigRepository, toolRegistry *port.ToolRegistry, logger port.Logger) (*TrpcAgentGenerator, error) {
	g := &TrpcAgentGenerator{
		llmCfgRepo:   llmCfgRepo,
		runners:      make(map[string]runner.Runner),
		toolRegistry: toolRegistry,
		logger:       logger,
	}
	g.sessionSvc = inmemory.NewSessionService()
	return g, nil
}

// resolveLLM 按 llmConfigName 解析并（带缓存地）构建 LLM 客户端。
// 空名回退到 "default"；找不到配置时返回错误。
func (g *TrpcAgentGenerator) resolveLLM(ctx context.Context, llmConfigName string) (*openai.Model, error) {
	name := llmConfigName
	if name == "" {
		name = DefaultLLMConfigName
	}
	// 缓存命中
	if v, ok := g.llmCache.Load(name); ok {
		return v.(*openai.Model), nil
	}
	cfg, err := g.llmCfgRepo.FindByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("LLM 配置 %q 不存在: %w", name, err)
	}
	llm := llmadapter.Build(cfg)
	g.llmCache.Store(name, llm)
	return llm, nil
}

// getOrCreateRunner 获取或创建指定 sessionID 的 runner。
// 同一个 sessionID 的多次调用共享对话历史（多轮对话）。
// 注意：不同 LLM 的 runner 不应共享 sessionID，因此把 llm 纳入 sessionID 计算。
func (g *TrpcAgentGenerator) getOrCreateRunner(sessionID string, systemPrompt string, llm *openai.Model) runner.Runner {
	g.mu.RLock()
	if r, ok := g.runners[sessionID]; ok {
		g.mu.RUnlock()
		return r
	}
	g.mu.RUnlock()

	g.mu.Lock()
	defer g.mu.Unlock()
	// double check
	if r, ok := g.runners[sessionID]; ok {
		return r
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
	return r
}

// ChatStream 实现 port.AIGenerator：流式对话（支持多轮上下文）。
// sessionID 由调用方传入，同一 sessionID 共享对话历史。
func (g *TrpcAgentGenerator) ChatStream(ctx context.Context, llmConfigName string, messages []port.ChatMessage, onDelta func(delta string)) (string, error) {
	ctx, span := telemetry.StartSpan(ctx, "ai.chat_stream")
	defer span.End()

	if len(messages) == 0 { return "", fmt.Errorf("no messages") }

	// 解析 LLM 客户端（按 llmConfigName，空则 default）
	llm, err := g.resolveLLM(ctx, llmConfigName)
	if err != nil { return "", err }

	// 提取 system prompt（如果有）和最后一条 user 消息
	var systemPrompt string
	lastUser := messages[len(messages)-1]
	for _, msg := range messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
		}
	}

	// 用消息内容的 hash + LLM 名 作为 sessionID（同一对话流、同一 LLM 复用 runner）
	sessionID := fmt.Sprintf("chat-%d", hashString(systemPrompt+":"+llmConfigName))

	r := g.getOrCreateRunner(sessionID, systemPrompt, llm)

	events, err := r.Run(ctx, "webreaper-user", sessionID,
		model.Message{Role: model.RoleUser, Content: lastUser.Content},
	)
	if err != nil { return "", fmt.Errorf("runner run: %w", err) }

	var sb strings.Builder
	for evt := range events {
		if evt.IsError() {
			if evt.Error != nil { return sb.String(), fmt.Errorf("llm error: %v", evt.Error) }
			break
		}
		if evt.Object == model.ObjectTypeChatCompletionChunk && evt.Response != nil {
			for _, choice := range evt.Response.Choices {
				if choice.Delta.Content != "" {
					sb.WriteString(choice.Delta.Content)
					if onDelta != nil { onDelta(choice.Delta.Content) }
				}
			}
		}
		if evt.Object == model.ObjectTypeChatCompletion && evt.Response != nil {
			for _, choice := range evt.Response.Choices {
				if choice.Message.Content != "" && sb.Len() == 0 {
					sb.WriteString(choice.Message.Content)
					if onDelta != nil { onDelta(choice.Message.Content) }
				}
			}
		}
	}
	return sb.String(), nil
}

// RunWithTools 实现 port.AIGenerator：带工具的流式执行（ReAct 循环）。
// 所有爬虫工具全局可用（不按 Agent 配置过滤），LLM 自主决定调哪个。
func (g *TrpcAgentGenerator) RunWithTools(ctx context.Context, llmConfigName string, task string, systemPrompt string, _ []string, onEvent func(event port.ToolEvent)) error {
	// 解析 LLM 客户端（按 llmConfigName，空则 default）
	llm, err := g.resolveLLM(ctx, llmConfigName)
	if err != nil {
		if onEvent != nil { onEvent(port.ToolEvent{Type: "error", Error: err.Error()}) }
		return err
	}

	// 构建工具（全部注册，不再按 toolNames 过滤——工具全局化）
	var tools []tool.Tool
	if g.toolRegistry != nil {
		allCrawlers := g.toolRegistry.All()
		adapterTools := agentadapter.ConvertTools(allCrawlers, nil, g.logger) // nil = 聊天模式不落库
		tools = adapterTools
	}

	// 构建 explorer Agent（有工具用 explorer，无工具用 llmagent）
	var ag agent.Agent
	agOpts := []llmagent.Option{llmagent.WithModel(llm)}
	if systemPrompt != "" { agOpts = append(agOpts, llmagent.WithInstruction(systemPrompt)) }

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

	events, err := r.Run(ctx, "chat-user", "chat-tools", model.Message{Role: model.RoleUser, Content: task})
	if err != nil { return fmt.Errorf("runner run: %w", err) }

	for evt := range events {
		if evt.IsError() {
			errMsg := "unknown"
			if evt.Error != nil { errMsg = evt.Error.Error() }
			if onEvent != nil { onEvent(port.ToolEvent{Type: "error", Error: errMsg}) }
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

	if onEvent != nil { onEvent(port.ToolEvent{Type: "finish"}) }
	return nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen { return s }
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
