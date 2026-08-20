package repository

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// fakeVectorStore 本地向量存储（内存 map）：余弦/排序逻辑在 knowledge 包测试覆盖，
// 这里只验证仓储的"委托 + 回查组装"链路（industry filter 传递 / ID 回查 / limit 截断）。
type fakeVectorStore struct {
	vecs        map[string][]float32
	industries  map[string]string // id → industry（Store 时记录，Search 时过滤）
	filters     []map[string]string // 记录 Search 收到的 filter
	searchErr   error
}

func newFakeVectorStore() *fakeVectorStore {
	return &fakeVectorStore{vecs: map[string][]float32{}, industries: map[string]string{}}
}

func (f *fakeVectorStore) Store(_ context.Context, id string, vec []float32, meta map[string]string) error {
	f.vecs[id] = vec
	f.industries[id] = meta["industry"]
	return nil
}

func (f *fakeVectorStore) Search(_ context.Context, filter map[string]string, _ []float32, topK int) ([]port.VectorSearchResult, error) {
	f.filters = append(f.filters, filter)
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	results := make([]port.VectorSearchResult, 0, len(f.vecs))
	for id := range f.vecs {
		if industry := filter["industry"]; industry != "" && f.industries[id] != industry {
			continue
		}
		results = append(results, port.VectorSearchResult{ID: id, Score: 0.9})
	}
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func (f *fakeVectorStore) Delete(_ context.Context, id string) error {
	delete(f.vecs, id)
	return nil
}

func (f *fakeVectorStore) IsAvailable() bool { return true }

// Get 实现 VectorStoreProvider：返回自身（单驱动场景）。
func (f *fakeVectorStore) Get(context.Context) (port.VectorStore, error) { return f, nil }

var _ port.VectorStore = (*fakeVectorStore)(nil)
var _ port.VectorStoreProvider = (*fakeVectorStore)(nil)

// newMaterial 构造测试素材（指纹/向量可覆盖）。
func newMaterial(id, industry, url string, emb []float32) *entity.KnowledgeMaterial {
	return &entity.KnowledgeMaterial{
		ID: id, Industry: industry, SourceURL: url, URLFingerprint: "fp-" + id,
		Title: "素材" + id, Content: "正文内容" + id, Summary: "摘要" + id,
		Status: entity.MaterialStatusActive, Embedding: emb,
	}
}

// TestKnowledgeMaterial_SaveAndExists 入库 + 指纹去重 + 向量同步存储。
func TestKnowledgeMaterial_SaveAndExists(t *testing.T) {
	db := newTestDB(t)
	vec := newFakeVectorStore()
	repo := NewGormKnowledgeMaterialRepository(db, vec)
	ctx := context.Background()

	m := newMaterial("m1", "餐饮", "https://a.com/x", []float32{1, 0, 0})
	if err := repo.Save(ctx, m); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	ok, err := repo.ExistsByFingerprint(ctx, "fp-m1")
	if err != nil || !ok {
		t.Fatalf("指纹应存在: ok=%v err=%v", ok, err)
	}
	ok, _ = repo.ExistsByFingerprint(ctx, "fp-none")
	if ok {
		t.Fatal("不存在的指纹应返回 false")
	}
	// 带向量素材应同步进向量库
	if _, exists := vec.vecs["m1"]; !exists {
		t.Error("Save 应同步向量到 VectorStore")
	}
	// 无向量素材：不调向量库
	_ = repo.Save(ctx, newMaterial("m2", "餐饮", "https://a.com/y", nil))
	if _, exists := vec.vecs["m2"]; exists {
		t.Error("无向量素材不应进向量库")
	}
}

// TestKnowledgeMaterial_SearchSimilar 检索委托：industry 过滤传递 + ID 回查组装（带来源）。
func TestKnowledgeMaterial_SearchSimilar(t *testing.T) {
	db := newTestDB(t)
	vec := newFakeVectorStore()
	repo := NewGormKnowledgeMaterialRepository(db, vec)
	ctx := context.Background()

	_ = repo.Save(ctx, newMaterial("m1", "餐饮", "https://a.com/1", []float32{1, 0, 0}))
	_ = repo.Save(ctx, newMaterial("m2", "餐饮", "https://a.com/2", []float32{0.9, 0.1, 0}))
	_ = repo.Save(ctx, newMaterial("m3", "美业", "https://b.com/3", []float32{1, 0, 0}))

	refs, err := repo.SearchSimilar(ctx, "餐饮", "", []float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatalf("检索失败: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("应返回 2 条，实际 %d: %+v", len(refs), refs)
	}
	if refs[0].Title != "素材m1" || refs[0].SourceURL != "https://a.com/1" {
		t.Errorf("top1 应为 m1（带来源）: %+v", refs[0])
	}
	// industry 过滤应传递给 VectorStore
	if len(vec.filters) == 0 || vec.filters[0]["industry"] != "餐饮" {
		t.Errorf("industry 过滤未传递: %+v", vec.filters)
	}

	// 无行业 → filter 为 nil（全行业）
	_, _ = repo.SearchSimilar(ctx, "", "", []float32{1, 0, 0}, 5)
	if len(vec.filters) < 2 || vec.filters[1] != nil {
		t.Errorf("空行业应传 nil filter: %+v", vec.filters)
	}

	// limit 截断
	refs, _ = repo.SearchSimilar(ctx, "餐饮", "", []float32{1, 0, 0}, 1)
	if len(refs) != 1 {
		t.Errorf("limit=1 应只返回 1 条: %d", len(refs))
	}
}

// TestKnowledgeMaterial_SearchSimilar_NoVectorStore 未注入向量库 → 返回空（降级）。
func TestKnowledgeMaterial_SearchSimilar_NoVectorStore(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormKnowledgeMaterialRepository(db, nil)
	ctx := context.Background()
	_ = repo.Save(ctx, newMaterial("m1", "餐饮", "https://a.com/1", []float32{1, 0, 0}))
	refs, err := repo.SearchSimilar(ctx, "餐饮", "", []float32{1, 0, 0}, 5)
	if err != nil || refs != nil {
		t.Errorf("无向量库应返回 nil: %v %v", refs, err)
	}
}

// TestKnowledgeMaterial_CountListDelete 统计 / 分页 / 删除（同步删向量）。
func TestKnowledgeMaterial_CountListDelete(t *testing.T) {
	db := newTestDB(t)
	vec := newFakeVectorStore()
	repo := NewGormKnowledgeMaterialRepository(db, vec)
	ctx := context.Background()

	_ = repo.Save(ctx, newMaterial("m1", "餐饮", "https://a.com/1", []float32{1}))
	_ = repo.Save(ctx, newMaterial("m2", "美业", "https://b.com/2", []float32{1}))
	_ = repo.Save(ctx, newMaterial("m3", "餐饮", "https://a.com/3", []float32{1}))

	n, err := repo.Count(ctx, "餐饮")
	if err != nil || n != 2 {
		t.Errorf("餐饮应有 2 条: n=%d err=%v", n, err)
	}
	n, _ = repo.Count(ctx, "")
	if n != 3 {
		t.Errorf("全库应有 3 条: n=%d", n)
	}

	list, err := repo.ListByIndustry(ctx, "餐饮", 10, 0)
	if err != nil || len(list) != 2 {
		t.Fatalf("分页列表错误: len=%d err=%v", len(list), err)
	}

	if err := repo.Delete(ctx, "m1"); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	n, _ = repo.Count(ctx, "餐饮")
	if n != 1 {
		t.Errorf("删除后餐饮应剩 1 条: n=%d", n)
	}
	if _, exists := vec.vecs["m1"]; exists {
		t.Error("Delete 应同步删除向量")
	}
}
