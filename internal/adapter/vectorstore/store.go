// Package vectorstore 提供 port.VectorStore 的实现（适配器层）。
package vectorstore

import (
	"context"
	"fmt"
	"sync"

	"webreaper/internal/usecase/port"
)

// NopVectorStore 是空操作的向量存储（Milvus 不可用时的降级实现）。
// 所有操作返回 nil（成功但不做任何事），Search 返回空结果。
// 这样结构化流程不会因 Milvus 断连而中断。
type NopVectorStore struct{}

func NewNopVectorStore() *NopVectorStore { return &NopVectorStore{} }

func (s *NopVectorStore) Store(_ context.Context, _ string, _ []float32, _ map[string]string) error {
	return nil // 静默跳过
}
func (s *NopVectorStore) Search(_ context.Context, _ []float32, _ int) ([]port.VectorSearchResult, error) {
	return nil, nil // 返回空结果
}
func (s *NopVectorStore) Delete(_ context.Context, _ string) error { return nil }
func (s *NopVectorStore) IsAvailable() bool { return false }

// MemoryVectorStore 是内存向量存储（开发/测试用，支持余弦相似度搜索）。
type MemoryVectorStore struct {
	mu      sync.RWMutex
	vectors map[string][]float32
	meta    map[string]map[string]string
}

func NewMemoryVectorStore() *MemoryVectorStore {
	return &MemoryVectorStore{
		vectors: make(map[string][]float32),
		meta:    make(map[string]map[string]string),
	}
}

func (s *MemoryVectorStore) Store(_ context.Context, id string, vec []float32, metadata map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vectors[id] = vec
	s.meta[id] = metadata
	return nil
}

func (s *MemoryVectorStore) Search(_ context.Context, query []float32, topK int) ([]port.VectorSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type scored struct {
		id    string
		score float32
		meta  map[string]string
	}
	var results []scored
	for id, vec := range s.vectors {
		score := cosineSimilarity(query, vec)
		results = append(results, scored{id: id, score: score, meta: s.meta[id]})
	}

	// 简单排序取 topK（数据量小，不需要优化）
	for i := 0; i < len(results) && i < topK; i++ {
		maxIdx := i
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[maxIdx].score {
				maxIdx = j
			}
		}
		results[i], results[maxIdx] = results[maxIdx], results[i]
	}

	if topK > len(results) {
		topK = len(results)
	}
	out := make([]port.VectorSearchResult, 0, topK)
	for i := 0; i < topK; i++ {
		out = append(out, port.VectorSearchResult{
			ID: results[i].id, Score: results[i].score, Metadata: results[i].meta,
		})
	}
	return out, nil
}

func (s *MemoryVectorStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.vectors, id)
	delete(s.meta, id)
	return nil
}

func (s *MemoryVectorStore) IsAvailable() bool { return true }

// cosineSimilarity 计算两个向量的余弦相似度。
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (sqrt(normA) * sqrt(normB)))
}

func sqrt(x float64) float64 {
	if x <= 0 { return 0 }
	z := x
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// 确保实现接口
var _ port.VectorStore = (*NopVectorStore)(nil)
var _ port.VectorStore = (*MemoryVectorStore)(nil)

// MilvusVectorStore 占位（真实 Milvus 接入尚未实现）。
//
// 设计说明（诚实降级原则）：
// 真实 Milvus SDK（github.com/milvus-io/milvus-sdk-go/v2）尚未接入。
// 为避免「装作成功却静默丢弃数据」的反模式，NewMilvusVectorStore 返回显式 error，
// 让 main.go 的「失败则降级内存向量存储」分支真正可达、可被日志感知。
// 这符合项目 ADR-002「双实现降级」——降级必须诚实，不能装成功。
//
// 后续接入步骤：
//  1. go get github.com/milvus-io/milvus-sdk-go/v2
//  2. 实现 Store/Search/Delete/IsAvailable（参考 MemoryVectorStore 的余弦相似度）
//  3. NewMilvusVectorStore 真正建连，连不上才返回 error 走降级
type MilvusVectorStore struct {
	*nopFallback
}

type nopFallback struct{}

func (nopFallback) Store(context.Context, string, []float32, map[string]string) error { return nil }
func (nopFallback) Search(context.Context, []float32, int) ([]port.VectorSearchResult, error) { return nil, nil }
func (nopFallback) Delete(context.Context, string) error { return nil }
func (nopFallback) IsAvailable() bool { return false }

// NewMilvusVectorStore 创建 Milvus 向量存储。
//
// 当前真实 SDK 未接入，始终返回 error，提示调用方降级到内存向量存储。
// 等真实实现就位后，本函数改为真正建连（连不上才 error）。
func NewMilvusVectorStore(host, port string) (*MilvusVectorStore, error) {
	_ = host
	_ = port
	return nil, fmt.Errorf("Milvus 真实实现尚未接入（SDK 未引入），请使用内存向量存储；配置已忽略")
}

var _ = fmt.Sprintf // 防止 fmt 未使用
