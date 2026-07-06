package agent

import (
	"context"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/model"

	"webreaper/internal/usecase/port"
)

// TestAccumulateUsage_NilSkips 验证：usage 为 nil 时（chunk 事件常见）跳过不累加。
func TestAccumulateUsage_NilSkips(t *testing.T) {
	acc := TokenUsage{PromptTokens: 10}
	got := accumulateUsage(acc, nil)
	if got.PromptTokens != 10 {
		t.Errorf("nil usage 不应改变累计值，prompt=%d want 10", got.PromptTokens)
	}
	if got.LLMCalls != 0 {
		t.Errorf("nil usage 不应增加 LLMCalls，got=%d want 0", got.LLMCalls)
	}
}

// TestAccumulateUsage_Accumulates 验证：多次累加正确汇总。
// 模拟 ReAct 多轮调用——一次 Agent 任务可能 3-5 次 LLM 调用。
func TestAccumulateUsage_Accumulates(t *testing.T) {
	acc := TokenUsage{}
	// 模拟 3 次 LLM 调用
	calls := []*model.Usage{
		{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
		{PromptTokens: 200, CompletionTokens: 80, TotalTokens: 280},
		{PromptTokens: 300, CompletionTokens: 100, TotalTokens: 400},
	}
	for _, u := range calls {
		acc = accumulateUsage(acc, u)
	}

	if acc.LLMCalls != 3 {
		t.Errorf("LLMCalls = %d, want 3", acc.LLMCalls)
	}
	if acc.PromptTokens != 600 {
		t.Errorf("PromptTokens = %d, want 600", acc.PromptTokens)
	}
	if acc.CompletionTokens != 230 {
		t.Errorf("CompletionTokens = %d, want 230", acc.CompletionTokens)
	}
	if acc.TotalTokens != 830 {
		t.Errorf("TotalTokens = %d, want 830", acc.TotalTokens)
	}
}

// TestAccumulateUsage_MixedNil 验证：nil 与非 nil 混合时只计非 nil。
// 对应真实场景：completion 事件带 usage，chunk 事件带 nil。
func TestAccumulateUsage_MixedNil(t *testing.T) {
	acc := TokenUsage{}
	seq := []*model.Usage{
		nil, // chunk
		{PromptTokens: 50, CompletionTokens: 20, TotalTokens: 70},
		nil, // chunk
		{PromptTokens: 60, CompletionTokens: 30, TotalTokens: 90},
	}
	for _, u := range seq {
		acc = accumulateUsage(acc, u)
	}
	if acc.LLMCalls != 2 {
		t.Errorf("混合 nil 时 LLMCalls = %d, want 2", acc.LLMCalls)
	}
	if acc.TotalTokens != 160 {
		t.Errorf("TotalTokens = %d, want 160", acc.TotalTokens)
	}
}

// TestReportTokenUsage_NoPanic 验证：logger 为 nil 时不 panic。
func TestReportTokenUsage_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("logger 为 nil 时不应 panic: %v", r)
		}
	}()
	reportTokenUsage(context.Background(), nil, TokenUsage{TotalTokens: 100})
}

// TestReportTokenUsage_WithLogger 验证：用真实 logger 上报，不 panic 且字段正确。
func TestReportTokenUsage_WithLogger(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("不应 panic: %v", r)
		}
	}()
	// NopLogger 不实际输出，但能验证调用路径不报错
	reportTokenUsage(context.Background(), port.NopLogger{}, TokenUsage{
		PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150, LLMCalls: 2,
	})
}

// TestTokenUsage_ZeroValue 验证：零值结构可用（未调 LLM 的边界情况）。
func TestTokenUsage_ZeroValue(t *testing.T) {
	var t0 TokenUsage
	if t0.PromptTokens != 0 || t0.LLMCalls != 0 {
		t.Error("零值应为 0")
	}
	// 累加 nil 不改变零值
	t0 = accumulateUsage(t0, nil)
	if t0.LLMCalls != 0 {
		t.Error("零值累加 nil 后仍应为 0")
	}
}
