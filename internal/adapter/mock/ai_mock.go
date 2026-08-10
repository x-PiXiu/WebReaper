package mock

import (
	"context"
	"fmt"

	"webreaper/internal/usecase/port"
)

// MockAIGenerator 是 AIGenerator 接口的假实现（降级用）。
// 返回模拟回复，让 Agent 链路在无 LLM API Key 时也能跑通。
type MockAIGenerator struct{}

func NewMockAIGenerator() *MockAIGenerator { return &MockAIGenerator{} }

// ChatStream 模拟流式对话（降级用），逐字返回示例回复。
func (m *MockAIGenerator) ChatStream(_ context.Context, _ string, _ string, messages []port.ChatMessage, onDelta func(delta string)) (string, error) {
	lastUser := ""
	if len(messages) > 0 {
		lastUser = messages[len(messages)-1].Content
	}
	reply := fmt.Sprintf("这是 Mock AI 的模拟回复。你说了：%s\n\n在真实模式下，这里会调用 LLM 进行智能回复。要启用真实 AI，请配置 LLM_API_KEY。", lastUser)
	for _, r := range []rune(reply) {
		if onDelta != nil {
			onDelta(string(r))
		}
	}
	return reply, nil
}

// RunWithTools Mock 实现：模拟工具调用过程。
func (m *MockAIGenerator) RunWithTools(_ context.Context, _ string, _ string, task string, _ string, _ []string, onEvent func(event port.ToolEvent)) error {
	if onEvent != nil {
		onEvent(port.ToolEvent{Type: "tool-call", ToolName: "search_crawler", ToolArgs: fmt.Sprintf(`{"query":"%s"}`, task)})
		onEvent(port.ToolEvent{Type: "tool-result", ToolResult: "Mock 搜索结果：找到 3 条相关链接"})
		onEvent(port.ToolEvent{Type: "text-delta", Text: fmt.Sprintf("根据搜索结果，关于「%s」的信息如下：\n\n（Mock 模式，配置 LLM_API_KEY 启用真实采集）", task)})
		onEvent(port.ToolEvent{Type: "finish"})
	}
	return nil
}
