package geo

import (
	"context"
	"testing"
	"time"

	mockrepo "webreaper/internal/adapter/mock"
	"webreaper/internal/domain/entity"
)

// SetStatus 测试：状态白名单 / 租户隔离 / 幂等。
func TestContentUseCase_SetStatus(t *testing.T) {
	ctx := context.Background()
	repo := mockrepo.NewMockOptimizedContentRepository()
	uc := NewContentUseCase(nil, nil, repo)

	// 预置一条内容（tenant-A）
	oc := entity.OptimizedContent{
		ID: "oc-1", TenantID: "tenant-A", BrandID: "brand-1",
		Title: "测试内容", OptimizedText: "正文",
		Status: "draft", CreatedAt: time.Now(),
	}
	if err := repo.Save(ctx, oc); err != nil {
		t.Fatalf("save: %v", err)
	}

	t.Run("发布到 published", func(t *testing.T) {
		got, _, err := uc.SetStatus(ctx, "tenant-A", "oc-1", "published")
		if err != nil {
			t.Fatalf("SetStatus error: %v", err)
		}
		if got.Status != "published" {
			t.Errorf("status = %s, want published", got.Status)
		}
	})

	t.Run("幂等：已是 published 再设 published", func(t *testing.T) {
		if _, _, err := uc.SetStatus(ctx, "tenant-A", "oc-1", "published"); err != nil {
			t.Errorf("幂等设置不应报错: %v", err)
		}
	})

	t.Run("下线回 draft", func(t *testing.T) {
		got, _, err := uc.SetStatus(ctx, "tenant-A", "oc-1", "draft")
		if err != nil {
			t.Fatalf("SetStatus error: %v", err)
		}
		if got.Status != "draft" {
			t.Errorf("status = %s, want draft", got.Status)
		}
	})

	t.Run("非法状态被拒绝", func(t *testing.T) {
		if _, _, err := uc.SetStatus(ctx, "tenant-A", "oc-1", "approved"); err == nil {
			t.Error("approved 不应可直接设置")
		}
		if _, _, err := uc.SetStatus(ctx, "tenant-A", "oc-1", "hacked"); err == nil {
			t.Error("未知状态应报错")
		}
	})

	t.Run("租户隔离：其他租户不能改", func(t *testing.T) {
		if _, _, err := uc.SetStatus(ctx, "tenant-B", "oc-1", "published"); err == nil {
			t.Error("tenant-B 不应能改 tenant-A 的内容")
		}
	})

	t.Run("不存在的内容报错", func(t *testing.T) {
		if _, _, err := uc.SetStatus(ctx, "tenant-A", "oc-missing", "published"); err == nil {
			t.Error("不存在的内容应报错")
		}
	})
}

// mockURLSubmitter 记录提交的 URL（验证发布副作用触发）。
type mockURLSubmitter struct {
	submitted []string
}

func (m *mockURLSubmitter) SubmitURLs(_ context.Context, urls []string) error {
	m.submitted = append(m.submitted, urls...)
	return nil
}

// SetStatus 发布时应触发收录通知（IndexNow），下线不触发。
func TestContentUseCase_SetStatus_TriggersURLSubmit(t *testing.T) {
	ctx := context.Background()
	repo := mockrepo.NewMockOptimizedContentRepository()
	submitter := &mockURLSubmitter{}
	uc := NewContentUseCase(nil, nil, repo)
	uc.SetURLSubmitter(submitter)
	uc.SetPublicBaseURL("https://content.example.com")

	oc := entity.OptimizedContent{
		ID: "oc-submit", TenantID: "tenant-A", BrandID: "brand-1",
		Title: "测试", OptimizedText: "正文", Status: "draft",
		CreatedAt: time.Now(),
	}
	_ = repo.Save(ctx, oc)

	// 发布 → 触发提交，URL 正确
	if _, _, err := uc.SetStatus(ctx, "tenant-A", "oc-submit", "published"); err != nil {
		t.Fatalf("SetStatus error: %v", err)
	}
	if len(submitter.submitted) != 1 {
		t.Fatalf("应提交 1 个 URL，实际 %d", len(submitter.submitted))
	}
	want := "https://content.example.com/public/articles/oc-submit"
	if submitter.submitted[0] != want {
		t.Errorf("提交 URL = %s, want %s", submitter.submitted[0], want)
	}

	// 下线 → 不触发
	if _, _, err := uc.SetStatus(ctx, "tenant-A", "oc-submit", "draft"); err != nil {
		t.Fatalf("SetStatus error: %v", err)
	}
	if len(submitter.submitted) != 1 {
		t.Errorf("下线不应触发提交，实际提交 %d 次", len(submitter.submitted))
	}
}

// 未注入 submitter（未配 INDEXNOW_KEY）时发布不 panic。
func TestContentUseCase_SetStatus_NoSubmitter(t *testing.T) {
	ctx := context.Background()
	repo := mockrepo.NewMockOptimizedContentRepository()
	uc := NewContentUseCase(nil, nil, repo) // 未 SetURLSubmitter

	oc := entity.OptimizedContent{
		ID: "oc-nosub", TenantID: "tenant-A", BrandID: "brand-1",
		Title: "测试", OptimizedText: "正文", Status: "draft",
		CreatedAt: time.Now(),
	}
	_ = repo.Save(ctx, oc)
	if _, _, err := uc.SetStatus(ctx, "tenant-A", "oc-nosub", "published"); err != nil {
		t.Errorf("无 submitter 时发布不应报错: %v", err)
	}
}

// 低分内容（30-50）发布：允许发布但不自动提交收录，且原因随 outcome 下发——
// 回归测试"发布成功但永不收录"用户黑洞修复。
func TestContentUseCase_SetStatus_LowScoreNoSubmitWithWarning(t *testing.T) {
	ctx := context.Background()
	repo := mockrepo.NewMockOptimizedContentRepository()
	submitter := &mockURLSubmitter{}
	uc := NewContentUseCase(nil, nil, repo)
	uc.SetURLSubmitter(submitter)
	uc.SetPublicBaseURL("https://content.example.com")

	oc := entity.OptimizedContent{
		ID: "oc-low", TenantID: "tenant-A", BrandID: "brand-1",
		Title: "低分内容", OptimizedText: "正文", Status: "draft",
		Score: entity.GEOScore{Total: 40}, CreatedAt: time.Now(),
	}
	_ = repo.Save(ctx, oc)

	got, outcome, err := uc.SetStatus(ctx, "tenant-A", "oc-low", "published")
	if err != nil {
		t.Fatalf("SetStatus error: %v", err)
	}
	if got.Status != "published" {
		t.Errorf("status = %s, want published", got.Status)
	}
	if len(submitter.submitted) != 0 {
		t.Errorf("低分内容不应自动提交收录，实际提交 %d 次", len(submitter.submitted))
	}
	if outcome.IndexSubmitted {
		t.Error("低分内容 IndexSubmitted 应为 false")
	}
	if len(outcome.Warnings) == 0 {
		t.Error("低分发布应携带警告（用户需要知道为什么没提交收录）")
	}
}
