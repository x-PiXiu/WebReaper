package knowledge

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"webreaper/internal/usecase/port"
)

// MilvusVectorStore 是 port.VectorStore 的 Milvus 实现。
//
// 设计要点：
//   - 依赖 milvusClient 内部接口（sdk 细节在 milvus_client.go 隔离）——测试注入 fake。
//   - 集合懒创建（首次 Store/Search 时按向量维度建集+索引+加载），配置切换零额外步骤。
//   - filter（industry）转 Milvus 表达式 `industry == "xx"`；空 = 全库检索。
//   - 与 MySQLVectorStore 语义一致：余弦度量 + 相似度由 Milvus 计算（返回原始分数，
//     业务阈值由检索器过滤；注意 Milvus 余弦范围 [-1,1]，与 MySQL 实现同源）。
type MilvusVectorStore struct {
	client milvusClient
	coll   string
	once   sync.Once
	initErr error
}

// NewMilvusVectorStore 创建 Milvus 向量存储（collection 已存在则复用，不存在则懒建）。
func NewMilvusVectorStore(client milvusClient, coll string) *MilvusVectorStore {
	return &MilvusVectorStore{client: client, coll: coll}
}

// Store 插入向量（首次调用触发建集）。
func (s *MilvusVectorStore) Store(ctx context.Context, id string, vector []float32, metadata map[string]string) error {
	if err := s.ensure(ctx, len(vector)); err != nil {
		return err
	}
	return s.client.Insert(ctx, s.coll, id, metadata["industry"], vector)
}

// Search 向量相似度 topK（filter 仅支持 industry——键白名单防注入）。
func (s *MilvusVectorStore) Search(ctx context.Context, filter map[string]string, queryVector []float32, topK int) ([]port.VectorSearchResult, error) {
	if err := s.ensure(ctx, len(queryVector)); err != nil {
		return nil, err
	}
	expr := milvusFilterExpr(filter)
	hits, err := s.client.Search(ctx, s.coll, expr, queryVector, topK)
	if err != nil {
		return nil, err
	}
	out := make([]port.VectorSearchResult, 0, len(hits))
	for _, h := range hits {
		out = append(out, port.VectorSearchResult{ID: h.ID, Score: h.Score})
	}
	return out, nil
}

// Delete 按主键删除（`id in ["x"]`）。
func (s *MilvusVectorStore) Delete(ctx context.Context, id string) error {
	if err := s.ensure(ctx, 1); err != nil {
		return err
	}
	// 表达式值转义（主键含引号场景）
	escaped := strings.ReplaceAll(id, `"`, `\"`)
	return s.client.DeleteByExpr(ctx, s.coll, fmt.Sprintf(`id in ["%s"]`, escaped))
}

// IsAvailable 客户端已装配即可用（连接状态由 Provider 建连时保证）。
func (s *MilvusVectorStore) IsAvailable() bool { return s.client != nil }

// ensure 懒初始化：集合不存在时按向量维度建集（幂等——once 只执行一次，失败记入 initErr）。
func (s *MilvusVectorStore) ensure(ctx context.Context, dim int) error {
	s.once.Do(func() {
		ok, err := s.client.HasCollection(ctx, s.coll)
		if err != nil {
			s.initErr = fmt.Errorf("milvus 检查集合失败: %w", err)
			return
		}
		if !ok {
			if err := s.client.CreateCollection(ctx, s.coll, dim); err != nil {
				s.initErr = err
				return
			}
		}
	})
	return s.initErr
}

// milvusFilterExpr 把 metadata 过滤转为 Milvus 表达式（白名单键；空/未知键 = 空表达式）。
func milvusFilterExpr(filter map[string]string) string {
	parts := make([]string, 0, len(filter))
	for k, v := range filter {
		if !vectorFilterKeys[k] {
			continue // 白名单外键忽略（防注入）
		}
		escaped := strings.ReplaceAll(v, `"`, `\"`)
		parts = append(parts, fmt.Sprintf(`%s == "%s"`, k, escaped))
	}
	return strings.Join(parts, " && ")
}
