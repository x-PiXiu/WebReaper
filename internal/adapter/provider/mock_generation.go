package provider

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// MockGenerationProvider 是 port.GenerationProvider 的模拟实现（开发/演示）。
//
// 行为：提交立即返回 task_id；轮询按时间推进状态（created → processing(2s) →
// success(5s) 带模拟生成物 URL）；未配置 VIDU_API_KEY 时装配此实现，前端全流程可演示。
type MockGenerationProvider struct {
	mu    sync.Mutex
	tasks map[string]time.Time // taskID → 提交时间
}

// NewMockGenerationProvider 创建模拟生成服务商。
func NewMockGenerationProvider() *MockGenerationProvider {
	return &MockGenerationProvider{tasks: map[string]time.Time{}}
}

func (m *MockGenerationProvider) Name() string { return "mock" }

func (m *MockGenerationProvider) Submit(ctx context.Context, endpoint string, body map[string]any) (string, int, error) {
	id := fmt.Sprintf("mock-%d", time.Now().UnixNano())
	m.mu.Lock()
	m.tasks[id] = time.Now()
	m.mu.Unlock()
	credits := 4
	if v, ok := body["duration"].(int); ok && v > 8 {
		credits = 8
	}
	return id, credits, nil
}

func (m *MockGenerationProvider) Poll(ctx context.Context, taskID string) (port.GenerationStatus, error) {
	m.mu.Lock()
	started, ok := m.tasks[taskID]
	m.mu.Unlock()
	if !ok {
		return port.GenerationStatus{}, fmt.Errorf("任务不存在: %s", taskID)
	}
	elapsed := time.Since(started)
	switch {
	case elapsed > 4*time.Second:
		return port.GenerationStatus{
			State: entity.TaskStateSuccess,
			Creations: []entity.CreationItem{{
				ID:  taskID + "-c1",
				URL: "https://mock.vidu.local/creations/" + taskID + ".mp4",
			}},
		}, nil
	case elapsed > 2*time.Second:
		return port.GenerationStatus{State: entity.TaskStateProcessing}, nil
	default:
		return port.GenerationStatus{State: entity.TaskStateQueueing}, nil
	}
}

func (m *MockGenerationProvider) Cancel(ctx context.Context, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tasks, taskID)
	return nil
}

func (m *MockGenerationProvider) VerifyCallback(ctx context.Context, header http.Header, body []byte) error {
	return nil // mock 无验签
}

func (m *MockGenerationProvider) QueryCredits(ctx context.Context) (int, error) {
	return 1000, nil
}

func (m *MockGenerationProvider) TranslateError(code string) string {
	if code == "" {
		return ""
	}
	return "生成失败（" + code + "）"
}
