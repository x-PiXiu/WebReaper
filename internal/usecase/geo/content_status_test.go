package geo

import (
	"context"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	mockrepo "webreaper/internal/adapter/mock"
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
		got, err := uc.SetStatus(ctx, "tenant-A", "oc-1", "published")
		if err != nil {
			t.Fatalf("SetStatus error: %v", err)
		}
		if got.Status != "published" {
			t.Errorf("status = %s, want published", got.Status)
		}
	})

	t.Run("幂等：已是 published 再设 published", func(t *testing.T) {
		if _, err := uc.SetStatus(ctx, "tenant-A", "oc-1", "published"); err != nil {
			t.Errorf("幂等设置不应报错: %v", err)
		}
	})

	t.Run("下线回 draft", func(t *testing.T) {
		got, err := uc.SetStatus(ctx, "tenant-A", "oc-1", "draft")
		if err != nil {
			t.Fatalf("SetStatus error: %v", err)
		}
		if got.Status != "draft" {
			t.Errorf("status = %s, want draft", got.Status)
		}
	})

	t.Run("非法状态被拒绝", func(t *testing.T) {
		if _, err := uc.SetStatus(ctx, "tenant-A", "oc-1", "approved"); err == nil {
			t.Error("approved 不应可直接设置")
		}
		if _, err := uc.SetStatus(ctx, "tenant-A", "oc-1", "hacked"); err == nil {
			t.Error("未知状态应报错")
		}
	})

	t.Run("租户隔离：其他租户不能改", func(t *testing.T) {
		if _, err := uc.SetStatus(ctx, "tenant-B", "oc-1", "published"); err == nil {
			t.Error("tenant-B 不应能改 tenant-A 的内容")
		}
	})

	t.Run("不存在的内容报错", func(t *testing.T) {
		if _, err := uc.SetStatus(ctx, "tenant-A", "oc-missing", "published"); err == nil {
			t.Error("不存在的内容应报错")
		}
	})
}
