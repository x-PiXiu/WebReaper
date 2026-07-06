package task

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/domain/valueobject"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ---- 假实现 ----

// fakeHandler 假的任务处理器。
type fakeHandler struct {
	taskType entity.TaskType
	output   string
	err      error
	called   bool
	mu       sync.Mutex
}

func (h *fakeHandler) TaskType() entity.TaskType { return h.taskType }
func (h *fakeHandler) Handle(_ context.Context, _ string) (string, error) {
	h.mu.Lock()
	h.called = true
	h.mu.Unlock()
	return h.output, h.err
}

// fakeQueue 假的任务队列（可预置任务）。
type fakeQueue struct {
	tasks chan entity.Task
	acked []string
	mu    sync.Mutex
}

func newFakeQueue(tasks ...entity.Task) *fakeQueue {
	q := &fakeQueue{tasks: make(chan entity.Task, 10)}
	for _, t := range tasks {
		q.tasks <- t
	}
	return q
}
func (q *fakeQueue) Enqueue(_ context.Context, t entity.Task) error {
	q.tasks <- t
	return nil
}
func (q *fakeQueue) Dequeue(ctx context.Context) (entity.Task, error) {
	select {
	case <-ctx.Done():
		return entity.Task{}, ctx.Err()
	case t := <-q.tasks:
		return t, nil
	}
}
func (q *fakeQueue) Ack(_ context.Context, taskID string, _ string) error {
	q.mu.Lock()
	q.acked = append(q.acked, taskID)
	q.mu.Unlock()
	return nil
}
func (q *fakeQueue) Close() error { return nil }

// fakeTaskRepo 假的任务仓储（记录状态更新）。
type fakeTaskRepo struct {
	mu       sync.Mutex
	statuses map[string]valueobject.TaskStatus
}

func newFakeTaskRepo() *fakeTaskRepo {
	return &fakeTaskRepo{statuses: make(map[string]valueobject.TaskStatus)}
}
func (r *fakeTaskRepo) Save(_ context.Context, t entity.Task) error {
	r.mu.Lock()
	r.statuses[t.ID] = t.Status
	r.mu.Unlock()
	return nil
}
func (r *fakeTaskRepo) FindByID(_ context.Context, id string) (entity.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.statuses[id]
	if !ok {
		return entity.Task{}, pkg.ErrNotFound
	}
	return entity.Task{ID: id, Status: st}, nil
}
func (r *fakeTaskRepo) UpdateStatus(_ context.Context, id string, status valueobject.TaskStatus, _ string) error {
	r.mu.Lock()
	r.statuses[id] = status
	r.mu.Unlock()
	return nil
}
func (r *fakeTaskRepo) List(_ context.Context, _ int) ([]entity.Task, error) { return nil, nil }
func (r *fakeTaskRepo) UpdateOutput(_ context.Context, _ string, _ string) error { return nil }
func (r *fakeTaskRepo) UpdateProgress(_ context.Context, _ string, _ string) error { return nil }

// ---- DispatchUseCase 测试 ----

func TestDispatch_Success(t *testing.T) {
	reg := NewHandlerRegistry()
	reg.Register(&fakeHandler{taskType: "test_type", output: `{"ok":true}`})
	uc := NewDispatchUseCase(reg, nil)

	task := entity.Task{ID: "t1", Type: "test_type", Input: "{}"}
	out, err := uc.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != `{"ok":true}` {
		t.Errorf("output = %q", out)
	}
}

func TestDispatch_HandlerNotRegistered(t *testing.T) {
	reg := NewHandlerRegistry() // 空注册表
	uc := NewDispatchUseCase(reg, nil)

	_, err := uc.Execute(context.Background(), entity.Task{Type: "unknown"})
	if !errors.Is(err, pkg.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestDispatch_HandlerError(t *testing.T) {
	reg := NewHandlerRegistry()
	reg.Register(&fakeHandler{taskType: "test_type", err: errors.New("handler failed")})
	uc := NewDispatchUseCase(reg, nil)

	_, err := uc.Execute(context.Background(), entity.Task{Type: "test_type"})
	if err == nil {
		t.Fatal("expected error from handler")
	}
}

// ---- EnqueueUseCase 测试 ----

func TestEnqueue_Success(t *testing.T) {
	q := newFakeQueue()
	repo := newFakeTaskRepo()
	uc := NewEnqueueUseCase(q, repo)

	out, err := uc.Execute(context.Background(), EnqueueTaskInput{
		Type:  entity.TaskTypeAgentRun,
		Input: map[string]any{"raw_text": "hello"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.TaskID == "" {
		t.Error("TaskID should not be empty")
	}
	// 应已持久化为 pending
	task, _ := repo.FindByID(context.Background(), out.TaskID)
	if task.Status != valueobject.TaskStatusPending {
		t.Errorf("Status = %q, want pending", task.Status)
	}
}

func TestEnqueue_EmptyType(t *testing.T) {
	uc := NewEnqueueUseCase(newFakeQueue(), newFakeTaskRepo())
	_, err := uc.Execute(context.Background(), EnqueueTaskInput{Type: ""})
	if !errors.Is(err, pkg.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

// ---- Worker 测试 ----

func TestWorker_ProcessesTask(t *testing.T) {
	// 准备：一个 pending 任务 + 成功的 handler
	task := entity.Task{
		ID: "w1", Type: "test_type", Input: "{}",
		Status: valueobject.TaskStatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	q := newFakeQueue(task)
	repo := newFakeTaskRepo()
	_ = repo.Save(context.Background(), task)

	reg := NewHandlerRegistry()
	reg.Register(&fakeHandler{taskType: "test_type", output: "{}"})
	dispatch := NewDispatchUseCase(reg, nil)

	worker := NewWorker(q, repo, dispatch, nil)

	// 启动 worker，1 秒后取消（足够处理一个任务）
	ctx, cancel := context.WithCancel(context.Background())
	go worker.Start(ctx)
	time.Sleep(300 * time.Millisecond)
	cancel()

	// 验证：任务状态应变为 succeeded
	done := repo.statuses["w1"]
	if done != valueobject.TaskStatusSucceeded {
		t.Errorf("Status = %q, want succeeded", done)
	}
}

func TestWorker_FailedTask(t *testing.T) {
	task := entity.Task{
		ID: "w2", Type: "test_type", Input: "{}",
		Status: valueobject.TaskStatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	q := newFakeQueue(task)
	repo := newFakeTaskRepo()
	_ = repo.Save(context.Background(), task)

	reg := NewHandlerRegistry()
	reg.Register(&fakeHandler{taskType: "test_type", err: errors.New("boom")})
	dispatch := NewDispatchUseCase(reg, nil)

	worker := NewWorker(q, repo, dispatch, nil).WithMaxRetries(1) // 测试：不重试，快速失败
	ctx, cancel := context.WithCancel(context.Background())
	go worker.Start(ctx)
	time.Sleep(300 * time.Millisecond)
	cancel()

	if repo.statuses["w2"] != valueobject.TaskStatusFailed {
		t.Errorf("Status = %q, want failed", repo.statuses["w2"])
	}
}

// 编译期断言：确保假实现满足接口。
var _ port.TaskQueue = (*fakeQueue)(nil)
var _ port.TaskRepository = (*fakeTaskRepo)(nil)
var _ TaskHandler = (*fakeHandler)(nil)
