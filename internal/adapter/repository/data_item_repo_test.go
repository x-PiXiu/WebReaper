package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
)

// sampleItem 构造测试用 DataItem（含 Tags/Metadata，覆盖 JSON 列映射）。
func sampleItem(id, title string) entity.DataItem {
	return entity.DataItem{
		ID:        id,
		Title:     title,
		Content:   "正文内容",
		Summary:   "摘要",
		Tags:      []string{"go", "后端"},
		SourceURL: "https://example.com",
		RawContent: `{"raw":true}`,
		Status:    entity.ItemStatusPendingReview,
		Metadata:  map[string]string{"crawler": "static"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// TestDataItem_Save_FindByID_RoundTrip 验证：Save→FindByID 往返，JSON 列（Tags/Metadata）正确序列化/反序列化。
func TestDataItem_Save_FindByID_RoundTrip(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormDataItemRepository(db)
	ctx := context.Background()

	item := sampleItem("item-1", "Go 工程师")
	if err := repo.Save(ctx, item); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	got, err := repo.FindByID(ctx, "item-1")
	if err != nil {
		t.Fatalf("FindByID 失败: %v", err)
	}
	if got.Title != "Go 工程师" {
		t.Errorf("Title = %q", got.Title)
	}
	// JSON 列是重点——映射错会丢数据或解析失败
	if len(got.Tags) != 2 || got.Tags[0] != "go" {
		t.Errorf("Tags = %v, want [go 后端]", got.Tags)
	}
	if got.Metadata["crawler"] != "static" {
		t.Errorf("Metadata = %v", got.Metadata)
	}
}

// TestDataItem_FindByID_NotFound 验证：查不存在返回 gorm.ErrRecordNotFound（上层靠它判断）。
func TestDataItem_FindByID_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormDataItemRepository(db)

	_, err := repo.FindByID(context.Background(), "no-such-item")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("应返回 ErrRecordNotFound，得到 %v", err)
	}
}

// TestDataItem_UpdateStatus 验证：状态流转（pending→approved）正确更新。
// 这是审核编排的核心——dataitem usecase 的 Approve/Reject 依赖它。
func TestDataItem_UpdateStatus(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormDataItemRepository(db)
	ctx := context.Background()

	_ = repo.Save(ctx, sampleItem("item-2", "测试"))

	if err := repo.UpdateStatus(ctx, "item-2", entity.ItemStatusApproved); err != nil {
		t.Fatalf("UpdateStatus 失败: %v", err)
	}

	got, _ := repo.FindByID(ctx, "item-2")
	if got.Status != entity.ItemStatusApproved {
		t.Errorf("Status = %q, want approved", got.Status)
	}
}

// TestDataItem_ListByStatus 验证：按状态过滤正确（审核台只看待审核项）。
func TestDataItem_ListByStatus(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormDataItemRepository(db)
	ctx := context.Background()

	_ = repo.Save(ctx, sampleItem("p1", "待审1"))
	_ = repo.Save(ctx, sampleItem("p2", "待审2"))
	approved := sampleItem("a1", "已过")
	approved.Status = entity.ItemStatusApproved
	_ = repo.Save(ctx, approved)

	pending, err := repo.ListByStatus(ctx, entity.ItemStatusPendingReview)
	if err != nil {
		t.Fatalf("ListByStatus 失败: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("待审核项数 = %d, want 2", len(pending))
	}

	approvedList, _ := repo.ListByStatus(ctx, entity.ItemStatusApproved)
	if len(approvedList) != 1 {
		t.Errorf("已审核项数 = %d, want 1", len(approvedList))
	}
}

// TestDataItem_ListByCollection 验证：按 collection_id 过滤。
func TestDataItem_ListByCollection(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormDataItemRepository(db)
	ctx := context.Background()

	item := sampleItem("c1-1", "项1")
	item.CollectionID = "col-A"
	_ = repo.Save(ctx, item)
	item2 := sampleItem("c2-1", "项2")
	item2.CollectionID = "col-B"
	_ = repo.Save(ctx, item2)

	got, err := repo.ListByCollection(ctx, "col-A")
	if err != nil {
		t.Fatalf("ListByCollection 失败: %v", err)
	}
	if len(got) != 1 || got[0].CollectionID != "col-A" {
		t.Errorf("col-A 的项数/归属错误: %v", got)
	}
}

// TestDataItem_SaveBatch 验证：批量保存（采集场景一次存多条）。
func TestDataItem_SaveBatch(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormDataItemRepository(db)
	ctx := context.Background()

	items := []entity.DataItem{
		sampleItem("b1", "批1"),
		sampleItem("b2", "批2"),
		sampleItem("b3", "批3"),
	}
	if err := repo.SaveBatch(ctx, items); err != nil {
		t.Fatalf("SaveBatch 失败: %v", err)
	}

	all, _ := repo.List(ctx, 10)
	if len(all) != 3 {
		t.Errorf("批量保存后总数 = %d, want 3", len(all))
	}
}
