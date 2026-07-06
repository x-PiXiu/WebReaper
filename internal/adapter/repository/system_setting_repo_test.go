package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
)

// TestSystemSetting_Get_Save_RoundTrip 回归测试：刚修复的 bug 是 Get("crawl_policy")
// 因手写 "key = ?" 触发 MySQL Error 1064。本测试验证修复后 Save→Get 往返正常。
//
// 注意：SQLite 对保留字 key 宽容（不会复现 1064），所以这个测试主要保护
// "结构化查询 + PO 映射"的正确性，防止有人改回手写 SQL。保留字类问题靠 code review 兜底。
func TestSystemSetting_Get_Save_RoundTrip(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormSystemSettingRepository(db)
	ctx := context.Background()

	setting := entity.SystemSetting{
		Key:       "crawl_policy",
		Value:     `{"interval_ms":1000}`,
		UpdatedAt: time.Now(),
	}

	// Save
	if err := repo.Save(ctx, setting); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	// Get 读回，验证字段映射正确（这是 bug 的核心：key→setting_key 列名映射）
	got, err := repo.Get(ctx, "crawl_policy")
	if err != nil {
		t.Fatalf("Get 失败（这正是之前 bug 的症状——列名映射错）: %v", err)
	}
	if got.Key != "crawl_policy" {
		t.Errorf("Key = %q, want crawl_policy", got.Key)
	}
	if got.Value != `{"interval_ms":1000}` {
		t.Errorf("Value = %q", got.Value)
	}
}

// TestSystemSetting_Get_NotFound 验证：查不存在的 key 返回 ErrNotFound（而非裸 error）。
// 这是上层降级逻辑的依赖（crawlconfig 用例靠 ErrNotFound 判断"首次启动需 seed 默认值"）。
func TestSystemSetting_Get_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormSystemSettingRepository(db)
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent_key")
	if !errors.Is(err, pkg.ErrNotFound) {
		t.Errorf("不存在的 key 应返回 ErrNotFound，得到 %v", err)
	}
}

// TestSystemSetting_Save_Overwrite 验证：同 key 再次 Save 是更新而非插入第二条。
// Save 用 GORM 的 Save（upsert 语义），主键冲突应覆盖。
func TestSystemSetting_Save_Overwrite(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormSystemSettingRepository(db)
	ctx := context.Background()

	_ = repo.Save(ctx, entity.SystemSetting{Key: "k1", Value: "v1", UpdatedAt: time.Now()})
	// 同 key 再存，值变了
	_ = repo.Save(ctx, entity.SystemSetting{Key: "k1", Value: "v2", UpdatedAt: time.Now()})

	got, err := repo.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if got.Value != "v2" {
		t.Errorf("覆盖后 Value = %q, want v2", got.Value)
	}
}
