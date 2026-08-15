package knowledge

import (
	"context"
	"strings"
	"testing"
)

// fakeMilvusClient 内存 fake（记录调用与表达式；Search 按相似度返回）。
type fakeMilvusClient struct {
	exists       bool
	createCalled bool
	rows         map[string]struct {
		industry string
		vec      []float32
	}
	lastExpr string
	err      error
}

func newFakeMilvusClient() *fakeMilvusClient {
	return &fakeMilvusClient{rows: map[string]struct {
		industry string
		vec      []float32
	}{}}
}

func (f *fakeMilvusClient) HasCollection(context.Context, string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.exists, nil
}
func (f *fakeMilvusClient) CreateCollection(context.Context, string, int) error {
	if f.err != nil {
		return f.err
	}
	f.createCalled = true
	f.exists = true
	return nil
}
func (f *fakeMilvusClient) Insert(_ context.Context, _ string, id, industry string, vec []float32) error {
	if f.err != nil {
		return f.err
	}
	f.rows[id] = struct {
		industry string
		vec      []float32
	}{industry: industry, vec: vec}
	return nil
}
func (f *fakeMilvusClient) Search(_ context.Context, _ string, expr string, vec []float32, topK int) ([]milvusHit, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.lastExpr = expr
	// 余弦 topK（fake 实现——真实余弦由 Milvus 计算）
	hits := make([]milvusHit, 0)
	for id, row := range f.rows {
		if expr != "" && !strings.Contains(expr, `"`+row.industry+`"`) {
			continue // 表达式未命中该行业 → 跳过（简化匹配）
		}
		hits = append(hits, milvusHit{ID: id, Score: cosine(vec, row.vec)})
	}
	// 降序 topK
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].Score > hits[i].Score {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits, nil
}
func (f *fakeMilvusClient) DeleteByExpr(_ context.Context, _ string, expr string) error {
	if f.err != nil {
		return f.err
	}
	f.lastExpr = expr
	for id := range f.rows {
		if strings.Contains(expr, `"`+id+`"`) {
			delete(f.rows, id)
		}
	}
	return nil
}
func (f *fakeMilvusClient) Close() error { return nil }

var _ milvusClient = (*fakeMilvusClient)(nil)

// TestMilvusVectorStore_StoreSearchDelete 懒建集 → 插入 → 检索（filter 表达式）→ 删除。
func TestMilvusVectorStore_StoreSearchDelete(t *testing.T) {
	fake := newFakeMilvusClient()
	s := NewMilvusVectorStore(fake, "kb_materials")
	ctx := context.Background()

	// 首次 Store 触发建集（集合不存在）
	if err := s.Store(ctx, "m1", []float32{1, 0, 0}, map[string]string{"industry": "餐饮"}); err != nil {
		t.Fatalf("Store 失败: %v", err)
	}
	if !fake.createCalled {
		t.Error("首次 Store 应触发建集")
	}
	_ = s.Store(ctx, "m2", []float32{0.9, 0.1, 0}, map[string]string{"industry": "餐饮"})
	_ = s.Store(ctx, "m3", []float32{1, 0, 0}, map[string]string{"industry": "美业"})

	// 行业过滤检索（表达式传递）
	results, err := s.Search(ctx, map[string]string{"industry": "餐饮"}, []float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if len(results) != 2 || results[0].ID != "m1" {
		t.Errorf("餐饮应返回 m1/m2 且 m1 第一: %+v", results)
	}
	if !strings.Contains(fake.lastExpr, `industry == "餐饮"`) {
		t.Errorf("filter 应转 Milvus 表达式: %s", fake.lastExpr)
	}

	// 空 filter = 全库
	results, _ = s.Search(ctx, nil, []float32{1, 0, 0}, 5)
	if len(results) != 3 {
		t.Errorf("空 filter 应全库检索: %+v", results)
	}

	// 删除（主键表达式）——先断言表达式，再检索验证
	if err := s.Delete(ctx, "m1"); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if !strings.Contains(fake.lastExpr, `id in ["m1"]`) {
		t.Errorf("删除表达式错误: %s", fake.lastExpr)
	}
	results, _ = s.Search(ctx, map[string]string{"industry": "餐饮"}, []float32{1, 0, 0}, 5)
	if len(results) != 1 || results[0].ID != "m2" {
		t.Errorf("删除 m1 后应只剩 m2: %+v", results)
	}
}

// TestMilvusVectorStore_EnsureErrors 建集失败/客户端错误 → 明确报错。
func TestMilvusVectorStore_EnsureErrors(t *testing.T) {
	// 建集失败
	fake := newFakeMilvusClient()
	fake.err = context.DeadlineExceeded
	s := NewMilvusVectorStore(fake, "kb")
	if err := s.Store(context.Background(), "m1", []float32{1}, nil); err == nil {
		t.Error("建集失败应报错")
	}
}

// TestMilvusFilterExpr filter 表达式转换（白名单 + 转义）。
func TestMilvusFilterExpr(t *testing.T) {
	if got := milvusFilterExpr(map[string]string{"industry": "餐饮"}); got != `industry == "餐饮"` {
		t.Errorf("表达式错误: %s", got)
	}
	if got := milvusFilterExpr(nil); got != "" {
		t.Errorf("空 filter 应为空表达式: %q", got)
	}
	// 白名单外键忽略 + 值内引号转义
	if got := milvusFilterExpr(map[string]string{"industry": `a"b`, "evil": "x"}); got != `industry == "a\"b"` {
		t.Errorf("白名单/转义错误: %s", got)
	}
}
