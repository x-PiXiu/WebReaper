// Package vectorstore 提供 port.VectorStore 的 Milvus 真实实现。
//
// 整洁架构定位：适配器层，封装 Milvus SDK 细节，用例层只依赖 port.VectorStore。
// 数据持久化（重启不丢），适合生产；海量向量检索性能好（Milvus HNSW/IVF 索引）。
package vectorstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	"webreaper/internal/usecase/port"
)

// MilvusVectorStore 是 port.VectorStore 的真实 Milvus 实现。
type MilvusVectorStore struct {
	mu         sync.Mutex
	cli        client.Client
	collection string
	dim        int
	schemaOnce sync.Once
	schemaErr  error
}

// NewMilvusVectorStore 创建并连接 Milvus 向量存储。
// addr 格式 "host:port"。连接失败返回 error，调用方据此降级到内存向量存储。
func NewMilvusVectorStore(addr, collection string) (port.VectorStore, error) {
	if collection == "" {
		collection = "webreaper_vectors"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cli, err := client.NewClient(ctx, client.Config{Address: addr})
	if err != nil {
		return nil, fmt.Errorf("连接 Milvus %s 失败: %w", addr, err)
	}
	// 探测连通性
	if _, err := cli.ListCollections(ctx); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("Milvus 连通性检查失败: %w", err)
	}
	return &MilvusVectorStore{cli: cli, collection: collection}, nil
}

// ensureSchema 懒建集合：首次 Store 时根据向量维度创建集合。
func (s *MilvusVectorStore) ensureSchema(ctx context.Context, dim int) error {
	s.schemaOnce.Do(func() {
		s.dim = dim
		s.schemaErr = s.createCollectionIfNotExist(ctx, dim)
	})
	if s.schemaErr == nil && s.dim != 0 && dim != s.dim {
		s.schemaErr = fmt.Errorf("向量维度不一致：集合=%d，传入=%d", s.dim, dim)
	}
	return s.schemaErr
}

// createCollectionIfNotExist 创建集合（id主键 + vector + metadata）。
func (s *MilvusVectorStore) createCollectionIfNotExist(ctx context.Context, dim int) error {
	exists, err := s.cli.HasCollection(ctx, s.collection)
	if err != nil {
		return fmt.Errorf("检查集合存在性失败: %w", err)
	}
	if exists {
		// 集合已存在，加载后复用
		if err := s.cli.LoadCollection(ctx, s.collection, false); err != nil {
			// 已 load 的情况会报错，忽略
		}
		return nil
	}

	// 用 Builder 模式构建 schema
	schema := entity.NewSchema().WithName(s.collection).
		WithDescription("WebReaper GEO 向量存储").
		WithAutoID(false).
		WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeVarChar).
			WithIsPrimaryKey(true).WithIsAutoID(false).WithMaxLength(128)).
		WithField(entity.NewField().WithName("vector").WithDataType(entity.FieldTypeFloatVector).WithDim(int64(dim))).
		WithField(entity.NewField().WithName("metadata").WithDataType(entity.FieldTypeVarChar).WithMaxLength(4096))

	if err := s.cli.CreateCollection(ctx, schema, 2); err != nil {
		return fmt.Errorf("创建集合失败: %w", err)
	}
	// 创建向量索引（IVF_FLAT + 余弦距离）
	idx, err := entity.NewIndexIvfFlat(entity.COSINE, 128)
	if err != nil {
		return fmt.Errorf("创建索引参数失败: %w", err)
	}
	if err := s.cli.CreateIndex(ctx, s.collection, "vector", idx, false); err != nil {
		return fmt.Errorf("创建向量索引失败: %w", err)
	}
	// 加载集合到内存（检索前必须 load）
	if err := s.cli.LoadCollection(ctx, s.collection, false); err != nil {
		return fmt.Errorf("加载集合失败: %w", err)
	}
	return nil
}

// Store 存储一条向量（upsert 语义：id 相同则覆盖）。
func (s *MilvusVectorStore) Store(ctx context.Context, id string, vec []float32, metadata map[string]string) error {
	if len(vec) == 0 {
		return fmt.Errorf("向量不能为空")
	}
	if err := s.ensureSchema(ctx, len(vec)); err != nil {
		return err
	}
	metaJSON, _ := json.Marshal(metadata)
	_, err := s.cli.Upsert(ctx, s.collection, "",
		entity.NewColumnVarChar("id", []string{id}),
		entity.NewColumnFloatVector("vector", s.dim, [][]float32{vec}),
		entity.NewColumnVarChar("metadata", []string{string(metaJSON)}),
	)
	if err != nil {
		return fmt.Errorf("Milvus 存储失败: %w", err)
	}
	return nil
}

// Search 按向量相似度搜索 topK。
func (s *MilvusVectorStore) Search(ctx context.Context, query []float32, topK int) ([]port.VectorSearchResult, error) {
	if len(query) == 0 {
		return nil, fmt.Errorf("查询向量不能为空")
	}
	if err := s.ensureSchema(ctx, len(query)); err != nil {
		return nil, err
	}
	if topK <= 0 {
		topK = 10
	}
	sp, err := entity.NewIndexIvfFlatSearchParam(128)
	if err != nil {
		return nil, fmt.Errorf("构建搜索参数失败: %w", err)
	}
	// Search 的 vectors 参数是 []entity.Vector，用 entity.FloatVector 包装
	results, err := s.cli.Search(ctx, s.collection, nil, "",
		[]string{"id", "metadata"},
		[]entity.Vector{entity.FloatVector(query)},
		"vector", entity.COSINE, topK, sp,
	)
	if err != nil {
		return nil, fmt.Errorf("Milvus 搜索失败: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	r := results[0]
	var out []port.VectorSearchResult
	idCol, ok := r.IDs.(*entity.ColumnVarChar)
	if !ok {
		return nil, nil
	}
	metaCol := r.Fields.GetColumn("metadata")
	for i := 0; i < r.ResultCount; i++ {
		id, _ := idCol.GetAsString(i)
		var meta map[string]string
		if metaCol != nil {
			if vc, ok := metaCol.(*entity.ColumnVarChar); ok {
				if s, e := vc.GetAsString(i); e == nil {
					_ = json.Unmarshal([]byte(s), &meta)
				}
			}
		}
		out = append(out, port.VectorSearchResult{
			ID: id, Score: r.Scores[i], Metadata: meta,
		})
	}
	return out, nil
}

// Delete 删除一条向量。
func (s *MilvusVectorStore) Delete(ctx context.Context, id string) error {
	pk := entity.NewColumnVarChar("id", []string{id})
	if err := s.cli.DeleteByPks(ctx, s.collection, "", pk); err != nil {
		return fmt.Errorf("Milvus 删除失败: %w", err)
	}
	return nil
}

// IsAvailable Milvus 可用。
func (s *MilvusVectorStore) IsAvailable() bool { return true }

// Close 关闭连接。
func (s *MilvusVectorStore) Close() error {
	if s.cli != nil {
		return s.cli.Close()
	}
	return nil
}
