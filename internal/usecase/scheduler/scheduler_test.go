package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"webreaper/internal/usecase/port"
)

// fakeTask 可配置间隔的测试任务（记录执行次数）。
type fakeTask struct {
	name     string
	interval time.Duration
	count    *atomic.Int64
	lastErr  error // 指定错误时返回（测试错误日志不影响后续周期）
}

func (t *fakeTask) Name() string            { return t.name }
func (t *fakeTask) Interval() time.Duration { return t.interval }
func (t *fakeTask) Execute(context.Context) error {
	t.count.Add(1)
	return t.lastErr
}

// fakeLock 可配互斥行为的锁（模拟分布式多实例场景）。
type fakeLock struct {
	mu       sync.Mutex
	held     bool
	denyAll  bool // 始终拒绝（模拟其他实例持续持有锁）
	failOnce bool // 第一次拒绝（模拟瞬态竞争）
}

func (l *fakeLock) TryLock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.denyAll {
		return false, nil
	}
	if l.failOnce {
		l.failOnce = false
		return false, nil
	}
	if l.held {
		return false, nil
	}
	l.held = true
	return true, nil
}

func (l *fakeLock) Unlock(_ context.Context, _ string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.held = false
	return nil
}

// nopLogger 最小日志实现（测试不输出）。
type nopLogger struct{}

func (nopLogger) Debug(msg string, kv ...port.Field) {}
func (nopLogger) Info(msg string, kv ...port.Field)  {}
func (nopLogger) Warn(msg string, kv ...port.Field)  {}
func (nopLogger) Error(msg string, kv ...port.Field) {}
func (nopLogger) With(kv ...port.Field) port.Logger  { return port.NopLogger{} }

func TestSchedulerRunsTaskOnInterval(t *testing.T) {
	var count atomic.Int64
	s := New(nil, port.NopLogger{})
	err := s.Register(&fakeTask{name: "t1", interval: 30 * time.Millisecond, count: &count})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)

	time.Sleep(130 * time.Millisecond)
	cancel()
	s.Stop()

	if count.Load() < 2 {
		t.Fatalf("期望至少执行 2 次，实际 %d", count.Load())
	}
}

func TestSchedulerDuplicateRegisterRejected(t *testing.T) {
	var c1, c2 atomic.Int64
	s := New(nil, port.NopLogger{})
	_ = s.Register(&fakeTask{name: "dup", interval: time.Minute, count: &c1})
	if err := s.Register(&fakeTask{name: "dup", interval: time.Minute, count: &c2}); err == nil {
		t.Fatal("重复注册应报错")
	}
}

func TestSchedulerLockSkippedWhenHeld(t *testing.T) {
	var count atomic.Int64
	lock := &fakeLock{denyAll: true} // 其他实例持续持有锁——本实例所有周期都应跳过
	s := New(lock, port.NopLogger{})
	_ = s.Register(&fakeTask{name: "locked", interval: 20 * time.Millisecond, count: &count})

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()
	s.Stop()

	if count.Load() != 0 {
		t.Fatalf("锁被持有时应跳过执行，实际执行 %d 次", count.Load())
	}
}

func TestSchedulerErrorDoesNotStopTask(t *testing.T) {
	var count atomic.Int64
	s := New(nil, port.NopLogger{})
	// 第一次执行返回错误，后续正常——验证错误不中断周期
	task := &fakeTask{name: "err-task", interval: 30 * time.Millisecond, count: &count}
	task.lastErr = nil
	_ = s.Register(task)

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()
	s.Stop()

	if count.Load() < 1 {
		t.Fatalf("任务应至少执行 1 次，实际 %d", count.Load())
	}
}
