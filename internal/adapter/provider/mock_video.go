// MockVideoProvider 模拟视频生成进度（未配置 Vidu API Key 时用于开发/演示）。
//
// 行为：Submit 直接返回任务 ID；Poll 第一次返回进行中，第二次返回完成
//（成片 URL 为占位地址）。使视频工作台在前端可完整演示而不消耗真实积分。
package provider

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockVideoProvider 模拟视频生成提供方。
type MockVideoProvider struct {
	mu      sync.Mutex
	polls   map[string]int // taskID → 已轮询次数
}

func NewMockVideoProvider() *MockVideoProvider {
	return &MockVideoProvider{polls: make(map[string]int)}
}

func (p *MockVideoProvider) Name() string { return "mock" }

func (p *MockVideoProvider) Submit(_ context.Context, mode, prompt, materialURL string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := fmt.Sprintf("mock-video-%d", time.Now().UnixNano())
	p.polls[id] = 0
	return id, nil
}

// Poll 模拟异步进度：第一次进行中，第二次完成。
func (p *MockVideoProvider) Poll(_ context.Context, taskID string) (float64, bool, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := p.polls[taskID]
	p.polls[taskID] = n + 1
	if n < 1 {
		return 0.5, false, "", nil // 进行中
	}
	return 1, true, "https://example.com/mock-video-" + taskID + ".mp4", nil
}
