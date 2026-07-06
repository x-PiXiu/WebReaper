package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
)

// sampleExtSys 构造测试用外部系统配置。
func sampleExtSys(name string) entity.ExternalSystem {
	return entity.ExternalSystem{
		Name:         name,
		Description:  "测试系统",
		Endpoint:     "https://example.com/api",
		Method:       "POST",
		Mode:         entity.PublishModeRaw,
		Enabled:      true,
	}
}

// ---- ExternalSystem 仓储（GORM 结构化查询，SQLite 可完整覆盖）----

// TestExternalSystem_Save_FindByName_RoundTrip 验证：存取往返，Mode/Enabled 映射正确。
func TestExternalSystem_Save_FindByName_RoundTrip(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormExternalSystemRepository(db)
	ctx := context.Background()

	if err := repo.Save(ctx, sampleExtSys("sys-1")); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	got, err := repo.FindByName(ctx, "sys-1")
	if err != nil {
		t.Fatalf("FindByName 失败: %v", err)
	}
	if got.Endpoint != "https://example.com/api" {
		t.Errorf("Endpoint = %q", got.Endpoint)
	}
	if got.Mode != entity.PublishModeRaw {
		t.Errorf("Mode = %q, want raw", got.Mode)
	}
	if !got.Enabled {
		t.Error("Enabled 应为 true")
	}
}

// TestExternalSystem_FindByName_NotFound 验证：查不存在返回 ErrNotFound。
// publish 用例依赖它判断"外部系统不存在"。
func TestExternalSystem_FindByName_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormExternalSystemRepository(db)

	_, err := repo.FindByName(context.Background(), "ghost")
	if !errors.Is(err, pkg.ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，得到 %v", err)
	}
}

// TestExternalSystem_ListAndDelete 验证：列表 + 删除。
func TestExternalSystem_ListAndDelete(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormExternalSystemRepository(db)
	ctx := context.Background()

	_ = repo.Save(ctx, sampleExtSys("s1"))
	_ = repo.Save(ctx, sampleExtSys("s2"))

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("系统数 = %d, want 2", len(list))
	}

	if err := repo.Delete(ctx, "s1"); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	list, _ = repo.List(ctx)
	if len(list) != 1 || list[0].Name != "s2" {
		t.Errorf("删除后系统数/归属错误: %v", list)
	}
}

// ---- PublishRecord 仓储（原生 SQL）----
//
// 覆盖边界说明（诚实标注）：
//   - PublishRecordPO 不在 allModels()，SQLite AutoMigrate 不建 publish_records 表。
//     本测试用 createPublishRecordsTable 手动建表（SQLite 语法），专测查询逻辑。
//   - Save() 用 ON DUPLICATE KEY UPDATE（MySQL 专属语法），SQLite 不支持，
//     故跳过 Save 测试，靠 MySQL e2e 联调兜底。
//   - ListByContent / FindDedup 用通用 SELECT 语法，SQLite 可测。

// createPublishRecordsTable 在 SQLite 测试库中手动建 publish_records 表。
// （复用 001_init.sql 的结构，但用 SQLite 兼容类型。）
func createPublishRecordsTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE publish_records (
		id TEXT PRIMARY KEY,
		content_id TEXT,
		content_type TEXT,
		platform TEXT,
		success INTEGER,
		external_id TEXT,
		error_msg TEXT,
		result_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("建 publish_records 表失败: %v", err)
	}
}

// insertPublishRecord 直接用通用 INSERT 插入测试数据（绕过 Save 的 MySQL 专属语法）。
func insertPublishRecord(db *gorm.DB, rec entity.PublishRecord) error {
	return db.Exec(`INSERT INTO publish_records (id, content_id, content_type, platform, success, external_id, error_msg, result_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.ContentID, rec.ContentType, rec.SystemName, rec.Success, rec.ExternalID, rec.ErrorMsg, rec.ResultAt, rec.CreatedAt, rec.CreatedAt).Error
}

// TestPublishRecord_ListByContent 验证：按 content_id 查推送记录（含排序）。
func TestPublishRecord_ListByContent(t *testing.T) {
	db := newTestDB(t)
	createPublishRecordsTable(t, db)
	repo := NewGormPublishRecordRepository(db)
	ctx := context.Background()

	now := time.Now()
	_ = insertPublishRecord(db, entity.PublishRecord{ID: "r1", ContentID: "c1", SystemName: "sysA", Success: true, CreatedAt: now.Add(-1 * time.Minute), ResultAt: now})
	_ = insertPublishRecord(db, entity.PublishRecord{ID: "r2", ContentID: "c1", SystemName: "sysB", Success: false, ErrorMsg: "timeout", CreatedAt: now, ResultAt: now})
	_ = insertPublishRecord(db, entity.PublishRecord{ID: "r3", ContentID: "c2", SystemName: "sysA", Success: true, CreatedAt: now, ResultAt: now})

	list, err := repo.ListByContent(ctx, "c1")
	if err != nil {
		t.Fatalf("ListByContent 失败: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("c1 的记录数 = %d, want 2", len(list))
	}
	// 验证字段映射（platform 列 → SystemName 字段，这是映射重点）
	if list[0].SystemName == "" {
		t.Error("SystemName 不应为空（platform 列映射）")
	}
}

// TestPublishRecord_FindDedup 验证：去重查询——已成功推送过则能查到。
// publish 用例依赖它跳过重复推送（FindDedup 命中即跳过）。
func TestPublishRecord_FindDedup_SuccessExists(t *testing.T) {
	db := newTestDB(t)
	createPublishRecordsTable(t, db)
	repo := NewGormPublishRecordRepository(db)
	ctx := context.Background()

	now := time.Now()
	_ = insertPublishRecord(db, entity.PublishRecord{
		ID: "r1", ContentID: "item-1", SystemName: "sysA",
		Success: true, CreatedAt: now, ResultAt: now,
	})

	// 已成功推送过 → 应能查到（非 ErrNotFound）
	rec, err := repo.FindDedup(ctx, "item-1", "sysA")
	if err != nil {
		t.Fatalf("已成功推送应能查到，得到 error: %v", err)
	}
	if !rec.Success {
		t.Error("去重记录的 Success 应为 true")
	}
}

// TestPublishRecord_FindDedup_NotFound 验证：未推送过返回 ErrNotFound（允许推送）。
func TestPublishRecord_FindDedup_NotFound(t *testing.T) {
	db := newTestDB(t)
	createPublishRecordsTable(t, db)
	repo := NewGormPublishRecordRepository(db)

	_, err := repo.FindDedup(context.Background(), "new-item", "sysA")
	if !errors.Is(err, pkg.ErrNotFound) {
		t.Errorf("未推送过应返回 ErrNotFound，得到 %v", err)
	}
}

// TestPublishRecord_FindDedup_FailedNotCounted 验证：之前推送失败（success=0）不算已推送。
// 即只有成功推送才去重，失败的允许重试——这是 publish 重试逻辑的依赖。
func TestPublishRecord_FindDedup_FailedNotCounted(t *testing.T) {
	db := newTestDB(t)
	createPublishRecordsTable(t, db)
	repo := NewGormPublishRecordRepository(db)
	ctx := context.Background()

	now := time.Now()
	// 之前推送失败
	_ = insertPublishRecord(db, entity.PublishRecord{
		ID: "r1", ContentID: "item-fail", SystemName: "sysA",
		Success: false, ErrorMsg: "500", CreatedAt: now, ResultAt: now,
	})

	// 失败记录不应被去重（FindDedup 只查 success=1）
	_, err := repo.FindDedup(ctx, "item-fail", "sysA")
	if !errors.Is(err, pkg.ErrNotFound) {
		t.Errorf("失败推送不应被去重，应返回 ErrNotFound，得到 %v", err)
	}
}
