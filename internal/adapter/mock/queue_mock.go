package mock

import (
	"context"
	"sync"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
)

// MockTaskQueue 是 TaskQueue 接口的进程内实现（Go channel）。
// 对应"进程内协程 + 内存队列"的起步决策。
// 未来可被 Redis/asynq 等分布式实现替换，用例层零修改。
type MockTaskQueue struct {
	mu     sync.Mutex
	queue  chan entity.Task
	closed bool
}

func NewMockTaskQueue(bufferSize int) *MockTaskQueue {
	return &MockTaskQueue{queue: make(chan entity.Task, bufferSize)}
}

func (q *MockTaskQueue) Enqueue(_ context.Context, t entity.Task) error {
	q.mu.Lock()
	closed := q.closed
	q.mu.Unlock()
	if closed {
		return pkg.ErrTaskNotExecutable
	}
	q.queue <- t
	return nil
}

func (q *MockTaskQueue) Dequeue(ctx context.Context) (entity.Task, error) {
	select {
	case <-ctx.Done():
		return entity.Task{}, ctx.Err()
	case t, ok := <-q.queue:
		if !ok {
			return entity.Task{}, pkg.ErrNotFound
		}
		return t, nil
	}
}

func (q *MockTaskQueue) Ack(_ context.Context, _ string, _ string) error {
	// 内存队列消费即确认，无额外动作
	return nil
}

func (q *MockTaskQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		close(q.queue)
		q.closed = true
	}
	return nil
}
