package provider

import (
	"context"
	"strings"
	"testing"

	"webreaper/internal/usecase/port"
)

// realStub 可区分的"真实"provider 桩（Name=real，Poll 恒 processing）。
type realStub struct{ *MockGenerationProvider }

func (realStub) Name() string { return "real" }
func (realStub) Poll(ctx context.Context, taskID string) (port.GenerationStatus, error) {
	return port.GenerationStatus{State: "processing"}, nil
}

func newRealStub() realStub { return realStub{NewMockGenerationProvider()} }

func TestSwitchingProviderKeyRouting(t *testing.T) {
	key := ""
	sp := NewSwitchingProvider(newRealStub(), NewMockGenerationProvider(), func() (string, bool) { return key, true })

	// 无 Key → mock
	if sp.Name() != "mock" {
		t.Errorf("无 Key 应委派 mock，得到 %q", sp.Name())
	}
	// mock 模式下提交一个任务（在途 mock 任务——稍后验证切真实后仍路由回 mock）
	res, err := sp.Submit(context.Background(), "/ent/v2/text2video", map[string]any{"prompt": "x"})
	if err != nil {
		t.Fatalf("mock 提交失败: %v", err)
	}
	if !strings.HasPrefix(res.TaskID, "mock-") {
		t.Fatalf("mock 提交应返回 mock- 前缀任务 ID，得到 %q", res.TaskID)
	}
	// 有 Key → 真实（UpdateAPIKey 即时刷新缓存——管理后台保存路径）
	sp.UpdateAPIKey("tok-1")
	if sp.Name() != "real" {
		t.Errorf("保存 Key 后应委派 real，得到 %q", sp.Name())
	}
	// mock- 前缀任务（Key 切换前提交的在途任务）固定路由回 mock——
	// 否则会拿 mock id 去真实 API 查 404 空转到 2h 超时
	if _, err := sp.Poll(context.Background(), res.TaskID); err != nil {
		t.Errorf("mock- 前缀任务应路由回 mock（其自身 Poll 可推进），err=%v", err)
	}
	// 真实任务 ID 走 real
	if st, err := sp.Poll(context.Background(), "real-abc"); err != nil || st.State != "processing" {
		t.Errorf("真实任务应路由 real（processing），得到 state=%q err=%v", st.State, err)
	}
}

func TestSwitchingProviderEnabledSwitch(t *testing.T) {
	// 停用厂商（enabled=false）：Key 存在也应回 mock——
	// 此前 Enabled 开关保存后无任何地方消费（死开关），现在由切换器语义化
	sp := NewSwitchingProvider(newRealStub(), NewMockGenerationProvider(), func() (string, bool) { return "tok", false })
	if sp.Name() != "mock" {
		t.Error("停用厂商（enabled=false）应委派 mock")
	}
}
