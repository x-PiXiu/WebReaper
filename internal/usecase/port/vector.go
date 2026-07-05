package port

import "context"

// Embedder 是文本向量化的抽象接口（边界）。
// 隔离 Embedding API（智谱/OpenAI/硅基流动等）。
type Embedder interface {
	// Embed 把文本转为向量。
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch 批量向量化（效率更高）。
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension 返回向量的维度。
	Dimension() int
}

// VectorStore 是向量存储的抽象接口（边界）。
// 隔离 Milvus/pgvector 等向量数据库。
type VectorStore interface {
	// Store 存储一条向量及其关联的 DataItem ID。
	Store(ctx context.Context, id string, vector []float32, metadata map[string]string) error
	// Search 按向量相似度搜索，返回最相似的 N 条结果的 DataItem ID。
	Search(ctx context.Context, queryVector []float32, topK int) ([]VectorSearchResult, error)
	// Delete 删除指定 ID 的向量。
	Delete(ctx context.Context, id string) error
	// IsAvailable 检查向量库是否可用（用于降级判断）。
	IsAvailable() bool
}

// VectorSearchResult 向量搜索结果。
type VectorSearchResult struct {
	ID       string
	Score    float32 // 相似度分数（0-1，越高越相似）
	Metadata map[string]string
}
