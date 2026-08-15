package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

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

// EmbeddingConfigProvider 向量嵌入 + 向量库的运行时配置加载器（adapter 实现）。
//
// 设计动机（管理后台动态修改、免重启）：
//   - 换向量模型/换向量库是运营高频操作，不应要求重启服务。
//   - 加载策略：system_settings（管理后台可改）优先，env（EMBEDDING_*）兜底。
//   - 配合 CachedEmbedder / VectorStoreProvider 的 TTL 缓存（30s）实现"改配置即生效"。
type EmbeddingConfigProvider interface {
	// Load 读取运行时配置（未配置返回零值——调用方按"未启用"处理）。
	Load(ctx context.Context) (entity.EmbeddingRuntimeConfig, error)
}

// EmbeddingConfigLoaderFunc 函数式适配器：main 装配把闭包直接转成接口（免写结构体）。
type EmbeddingConfigLoaderFunc func(ctx context.Context) (entity.EmbeddingRuntimeConfig, error)

// Load 实现 EmbeddingConfigProvider。
func (f EmbeddingConfigLoaderFunc) Load(ctx context.Context) (entity.EmbeddingRuntimeConfig, error) {
	return f(ctx)
}

// VectorStoreProvider 向量库工厂（按运行时配置构建；管理后台切换 vector_db 免重启）。
// 仓储依赖本接口而非具体实现——换 Milvus 不改仓储，只换 provider 实现。
type VectorStoreProvider interface {
	// Get 返回当前配置对应的向量库实现（30s TTL 缓存：配置变化后自动切换）。
	Get(ctx context.Context) (VectorStore, error)
}

// VectorStore 是向量存储的抽象接口（边界）。
// 隔离 Milvus/pgvector/MySQL(JSON+余弦) 等向量存储实现。
type VectorStore interface {
	// Store 存储一条向量及其关联 ID（metadata 为等值过滤字段，如 industry）。
	Store(ctx context.Context, id string, vector []float32, metadata map[string]string) error
	// Search 按向量相似度搜索（filter 为 metadata 等值过滤，如 {"industry":"餐饮"}；
	// 空/nil = 不过滤），返回最相似的 topK 条结果（按相似度降序）。
	Search(ctx context.Context, filter map[string]string, queryVector []float32, topK int) ([]VectorSearchResult, error)
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
