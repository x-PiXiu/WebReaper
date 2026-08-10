package dataitem

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// 确保接口实现
var (
	_ port.DataItemRepository = (*fakeDataItemRepo)(nil)
	_ port.ItemProcessor = (*fakeProcessor)(nil)
)

// ---- fake 实现 ----

type fakeDataItemRepo struct {
	mu     sync.Mutex
	status map[string]entity.ItemStatus // itemID → status
}

func (f *fakeDataItemRepo) Save(_ context.Context, item entity.DataItem) error {
	f.mu.Lock(); defer f.mu.Unlock()
	if f.status == nil { f.status = map[string]entity.ItemStatus{} }
	f.status[item.ID] = item.Status
	return nil
}
func (f *fakeDataItemRepo) SaveBatch(context.Context, []entity.DataItem) error { return nil }
func (f *fakeDataItemRepo) FindByID(_ context.Context, id string) (entity.DataItem, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	s, ok := f.status[id]
	if !ok { return entity.DataItem{}, errors.New("not found") }
	return entity.DataItem{ID: id, Status: s}, nil
}
func (f *fakeDataItemRepo) List(context.Context, int) ([]entity.DataItem, error) { return nil, nil }
func (f *fakeDataItemRepo) ListByCollection(context.Context, string) ([]entity.DataItem, error) { return nil, nil }
func (f *fakeDataItemRepo) ListByStatus(context.Context, entity.ItemStatus) ([]entity.DataItem, error) { return nil, nil }
func (f *fakeDataItemRepo) UpdateStatus(_ context.Context, id string, s entity.ItemStatus) error {
	f.mu.Lock(); defer f.mu.Unlock()
	if f.status == nil { f.status = map[string]entity.ItemStatus{} }
	f.status[id] = s
	return nil
}
func (f *fakeDataItemRepo) Delete(_ context.Context, id string) error {
	f.mu.Lock(); defer f.mu.Unlock()
	delete(f.status, id)
	return nil
}
func (f *fakeDataItemRepo) CountByStatus(_ context.Context) (map[string]int, error) { return map[string]int{}, nil }
func (f *fakeDataItemRepo) DailyCounts(_ context.Context, days int) ([]port.DailyCount, error) { return nil, nil }
func (f *fakeDataItemRepo) GroupByMetaKey(_ context.Context, key string) ([]port.GroupCount, error) { return nil, nil }
func (f *fakeDataItemRepo) TopTags(_ context.Context, limit int) ([]port.GroupCount, error) { return nil, nil }

// fakeProcessor 记录是否被调用、用什么 itemID 调用。
type fakeProcessor struct {
	mu      sync.Mutex
	called  []string
	failErr error
}
func (f *fakeProcessor) ProcessItem(_ context.Context, itemID string) error {
	f.mu.Lock(); defer f.mu.Unlock()
	f.called = append(f.called, itemID)
	return f.failErr
}

// ---- 测试用例 ----

// TestApprove_UpdatesStatusToApproved 验证审核通过会更新状态为 approved。
func TestApprove_UpdatesStatusToApproved(t *testing.T) {
	repo := &fakeDataItemRepo{status: map[string]entity.ItemStatus{"i1": entity.ItemStatusPendingReview}}
	uc := NewDataItemUseCase(repo, nil, port.NopLogger{})

	out, err := uc.Approve(context.Background(), "i1")
	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}
	if repo.status["i1"] != entity.ItemStatusApproved {
		t.Errorf("status = %v, want approved", repo.status["i1"])
	}
	if out.ItemID != "i1" {
		t.Errorf("ItemID = %q, want i1", out.ItemID)
	}
}

// TestApprove_TriggersProcessorAsync 验证审核通过后异步触发 ItemProcessor（结构化+向量化）。
// 这是"审核→结构化→向量化"业务流可测性的核心保证。
func TestApprove_TriggersProcessorAsync(t *testing.T) {
	repo := &fakeDataItemRepo{status: map[string]entity.ItemStatus{"i1": entity.ItemStatusPendingReview}}
	proc := &fakeProcessor{}
	uc := NewDataItemUseCase(repo, proc, port.NopLogger{})

	_, _ = uc.Approve(context.Background(), "i1")

	// 异步触发，轮询等待（最多 500ms）
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		proc.mu.Lock()
		n := len(proc.called)
		proc.mu.Unlock()
		if n == 1 { break }
		time.Sleep(10 * time.Millisecond)
	}
	proc.mu.Lock()
	defer proc.mu.Unlock()
	if len(proc.called) != 1 {
		t.Fatalf("processor called %d times, want 1", len(proc.called))
	}
	if proc.called[0] != "i1" {
		t.Errorf("processor called with %q, want i1", proc.called[0])
	}
}

// TestApprove_NilProcessorDoesNotPanic 验证 processor 为 nil（降级场景）时不 panic。
func TestApprove_NilProcessorDoesNotPanic(t *testing.T) {
	repo := &fakeDataItemRepo{status: map[string]entity.ItemStatus{"i1": entity.ItemStatusPendingReview}}
	uc := NewDataItemUseCase(repo, nil, port.NopLogger{})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic with nil processor: %v", r)
		}
	}()
	_, err := uc.Approve(context.Background(), "i1")
	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}
}

// TestReject_UpdatesStatusToRejected 验证驳回更新状态。
func TestReject_UpdatesStatusToRejected(t *testing.T) {
	repo := &fakeDataItemRepo{status: map[string]entity.ItemStatus{"i1": entity.ItemStatusPendingReview}}
	uc := NewDataItemUseCase(repo, nil, port.NopLogger{})

	if err := uc.Reject(context.Background(), "i1"); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}
	if repo.status["i1"] != entity.ItemStatusRejected {
		t.Errorf("status = %v, want rejected", repo.status["i1"])
	}
}
