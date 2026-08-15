package generation

import (
	"context"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- 自动重试/卡死超时回归测试（F-fix：RetryDue 补齐 ClassifyError 的执行闭环）----

type retryRepo struct {
	port.GenerationTaskRepository
	active []entity.GenerationTask
	failed []entity.GenerationTask
	saved  []entity.GenerationTask
}

func (r *retryRepo) ListActive(_ context.Context, _ int) ([]entity.GenerationTask, error) {
	return r.active, nil
}
func (r *retryRepo) ListFailed(_ context.Context, _ int) ([]entity.GenerationTask, error) {
	return r.failed, nil
}
func (r *retryRepo) Save(_ context.Context, t entity.GenerationTask) error {
	r.saved = append(r.saved, t)
	return nil
}

// 卡死超时：processing 超 2h 无更新 → PollDue 判失败（不再无限轮询）
func TestPollDueStuckTimeout(t *testing.T) {
	repo := &retryRepo{active: []entity.GenerationTask{{
		ID: "gen-stuck", State: entity.TaskStateProcessing, ProviderTaskID: "up-1",
		UpdatedAt: time.Now().Add(-3 * time.Hour),
	}}}
	uc := NewGenerationUseCase(nil, nil, repo)
	n, err := uc.PollDue(context.Background(), 10)
	if err != nil || n != 1 {
		t.Fatalf("PollDue = %d, %v; want 1, nil", n, err)
	}
	saved := repo.saved[len(repo.saved)-1]
	if saved.State != entity.TaskStateFailed || saved.ErrCode != "LocalStuckTimeout" {
		t.Errorf("卡死任务应判失败, got state=%s err=%s", saved.State, saved.ErrCode)
	}
}

// 卡死超时分类为不可自动重试——RetryDue 不碰它
func TestRetryDueSkipsStuckAndManual(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	repo := &retryRepo{failed: []entity.GenerationTask{
		{ID: "gen-stuck", State: entity.TaskStateFailed, ErrCode: "LocalStuckTimeout", FinishedAt: &old},
		{ID: "gen-manual", State: entity.TaskStateFailed, ErrCode: "CreditInsufficient", FinishedAt: &old},
	}}
	uc := NewGenerationUseCase(nil, nil, repo)
	// provider/registry 为 nil → RetryDue 直接放行（未配置不重试）
	if n, _ := uc.RetryDue(context.Background(), 10); n != 0 {
		t.Errorf("未配置 provider 不应重试, got %d", n)
	}
	if len(repo.saved) != 0 {
		t.Errorf("不应有任何保存动作, got %d", len(repo.saved))
	}
	// 分类层面再验证：卡死/人工类不满足 CanAutoRetry
	if CanAutoRetry("LocalStuckTimeout", 0) || CanAutoRetry("CreditInsufficient", 0) {
		t.Error("卡死超时与人工类错误不应自动重试")
	}
	// 退避窗口：RetryAuto 类刚失败（未到 1 分钟）不重试
	if CanAutoRetry("TooManyRequests", 0) != true {
		t.Error("限流类应可自动重试")
	}
}

// 退避表与注释口径一致（1/5/30 分钟）
func TestRetryBackoffTable(t *testing.T) {
	want := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute}
	for i, w := range want {
		if retryBackoff[i] != w {
			t.Errorf("retryBackoff[%d] = %v, want %v", i, retryBackoff[i], w)
		}
	}
}
