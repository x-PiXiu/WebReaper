package knowledge

import (
	"context"
	"testing"

	"webreaper/internal/adapter/repository"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// newMySQLVectorStoreTest 建 sqlite 内存库 + MySQLVectorStore。
func newMySQLVectorStoreTest(t *testing.T) *MySQLVectorStore {
	t.Helper()
	db, err := repository.NewSQLiteDB(":memory:")
	if err != nil {
		t.Fatalf("创建测试 DB 失败: %v", err)
	}
	return NewMySQLVectorStore(db)
}

// TestMySQLVectorStore_StoreSearchDelete 存储 → 检索（行业过滤/余弦排序）→ 删除。
func TestMySQLVectorStore_StoreSearchDelete(t *testing.T) {
	s := newMySQLVectorStoreTest(t)
	ctx := context.Background()

	// 通过仓储插入素材（含行业）；向量库走当前实现（MySQLVectorStore 同时是存储与驱动）
	load := func(ctx context.Context) (entity.EmbeddingRuntimeConfig, error) {
		return entity.EmbeddingRuntimeConfig{}, nil
	}
	repo := repository.NewGormKnowledgeMaterialRepository(s.db,
		NewVectorStoreProvider(port.EmbeddingConfigLoaderFunc(load), s))
	_ = repo.Save(ctx, &entity.KnowledgeMaterial{
		ID: "m1", Industry: "餐饮", SourceURL: "https://a.com/1", URLFingerprint: "f1",
		Title: "餐饮营销", Status: entity.MaterialStatusActive,
	})
	_ = repo.Save(ctx, &entity.KnowledgeMaterial{
		ID: "m2", Industry: "餐饮", SourceURL: "https://a.com/2", URLFingerprint: "f2",
		Title: "餐饮获客", Status: entity.MaterialStatusActive,
	})
	_ = repo.Save(ctx, &entity.KnowledgeMaterial{
		ID: "m3", Industry: "美业", SourceURL: "https://b.com/3", URLFingerprint: "f3",
		Title: "美容院运营", Status: entity.MaterialStatusActive,
	})

	// Store 向量（m1 与查询同向）
	_ = s.Store(ctx, "m1", []float32{1, 0, 0}, map[string]string{"industry": "餐饮"})
	_ = s.Store(ctx, "m2", []float32{0.8, 0.2, 0}, map[string]string{"industry": "餐饮"})
	_ = s.Store(ctx, "m3", []float32{1, 0, 0}, map[string]string{"industry": "美业"})

	// 行业过滤 + 余弦排序
	results, err := s.Search(ctx, map[string]string{"industry": "餐饮"}, []float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if len(results) != 2 || results[0].ID != "m1" {
		t.Errorf("餐饮应返回 2 条且 m1 第一: %+v", results)
	}
	// 美业隔离
	results, _ = s.Search(ctx, map[string]string{"industry": "美业"}, []float32{1, 0, 0}, 5)
	if len(results) != 1 || results[0].ID != "m3" {
		t.Errorf("美业应只返回 m3: %+v", results)
	}
	// 无过滤全行业
	results, _ = s.Search(ctx, nil, []float32{1, 0, 0}, 5)
	if len(results) != 3 {
		t.Errorf("无过滤应返回 3 条: %+v", results)
	}
	// topK 截断
	results, _ = s.Search(ctx, map[string]string{"industry": "餐饮"}, []float32{1, 0, 0}, 1)
	if len(results) != 1 {
		t.Errorf("topK=1 应截断: %+v", results)
	}

	// Delete 向量后不可检索
	_ = s.Delete(ctx, "m1")
	results, _ = s.Search(ctx, map[string]string{"industry": "餐饮"}, []float32{1, 0, 0}, 5)
	if len(results) != 1 || results[0].ID != "m2" {
		t.Errorf("删除 m1 向量后应只剩 m2: %+v", results)
	}
	if !s.IsAvailable() {
		t.Error("MySQL 驱动应常驻可用")
	}
}
