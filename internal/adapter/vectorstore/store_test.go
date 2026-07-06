package vectorstore

import (
	"context"
	"strings"
	"testing"

	"webreaper/internal/usecase/port"
)

// TestNewMilvusVectorStore_ReturnsError 验证诚实降级：
// 真实 SDK 未接入时，NewMilvusVectorStore 必须返回显式 error，
// 而非静默返回无操作的 nop（违反 ADR-002 双实现降级原则）。
//
// 这保证 main.go 的「失败则降级内存向量存储」分支可达、可被日志感知。
func TestNewMilvusVectorStore_ReturnsError(t *testing.T) {
	vs, err := NewMilvusVectorStore("127.0.0.1", "19530")
	if err == nil {
		t.Fatal("期望返回 error（诚实降级），得到 nil——配置会被静默忽略")
	}
	if vs != nil {
		t.Errorf("未接入时不应返回可用的 store，得到 %T", vs)
	}
	if !strings.Contains(err.Error(), "Milvus") {
		t.Errorf("错误信息应说明是 Milvus，得到: %v", err)
	}
}

// TestMemoryVectorStore_StoreAndSearch 验证内存向量存储的真实可用性
// （这是 Milvus 不可用时的降级实现，必须真的能存能搜）。
func TestMemoryVectorStore_StoreAndSearch(t *testing.T) {
	s := NewMemoryVectorStore()
	ctx := context.Background()

	// 存两个向量
	if err := s.Store(ctx, "a", []float32{1, 0, 0}, map[string]string{"t": "A"}); err != nil {
		t.Fatalf("Store a: %v", err)
	}
	if err := s.Store(ctx, "b", []float32{0, 1, 0}, map[string]string{"t": "B"}); err != nil {
		t.Fatalf("Store b: %v", err)
	}

	// 搜与 a 最相似的，应返回 a 排第一
	results, err := s.Search(ctx, []float32{1, 0, 0}, 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("应有 1 个结果，得到 %d", len(results))
	}
	if results[0].ID != "a" {
		t.Errorf("最相似的应是 a，得到 %q", results[0].ID)
	}
}

// TestMemoryVectorStore_ImplementsInterface 编译期断言 MemoryVectorStore 实现接口。
func TestMemoryVectorStore_ImplementsInterface(t *testing.T) {
	var _ port.VectorStore = (*MemoryVectorStore)(nil)
	var _ port.VectorStore = (*NopVectorStore)(nil)
}

// TestNopVectorStore 验证降级实现的安全性（静默跳过，Search 返回空）。
func TestNopVectorStore(t *testing.T) {
	s := NewNopVectorStore()
	ctx := context.Background()

	if err := s.Store(ctx, "x", []float32{1}, nil); err != nil {
		t.Errorf("Nop Store 应静默成功: %v", err)
	}
	results, err := s.Search(ctx, []float32{1}, 5)
	if err != nil {
		t.Errorf("Nop Search 应静默成功: %v", err)
	}
	if results != nil {
		t.Errorf("Nop Search 应返回空结果，得到 %v", results)
	}
	if s.IsAvailable() {
		t.Error("NopVectorStore.IsAvailable 应为 false")
	}
}
