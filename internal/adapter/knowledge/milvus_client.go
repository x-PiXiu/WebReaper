package knowledge

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// milvusClient 是 Milvus 客户端的内部抽象（依赖倒置：MilvusVectorStore 只依赖本接口，
// 测试注入 fake，生产用 MilvusSDKClient 包装 SDK——sdk 细节不出本文件）。
type milvusClient interface {
	// HasCollection 集合是否存在。
	HasCollection(ctx context.Context, name string) (bool, error)
	// CreateCollection 建集（含 industry 列 + 向量索引 + 加载）。
	CreateCollection(ctx context.Context, name string, dim int) error
	// Insert 插入一条（id + industry + 向量）。
	Insert(ctx context.Context, coll, id, industry string, vec []float32) error
	// Search 向量相似度 topK（expr 为过滤表达式，如 `industry == "餐饮"`；空=不过滤）。
	Search(ctx context.Context, coll, expr string, vec []float32, topK int) ([]milvusHit, error)
	// DeleteByExpr 按表达式删除（如 `id in ["x"]`）。
	DeleteByExpr(ctx context.Context, coll, expr string) error
	// Close 释放连接。
	Close() error
}

// milvusHit 搜索结果（ID + 相似度）。
type milvusHit struct {
	ID    string
	Score float32
}

// MilvusSDKClient 是 milvusClient 的 milvus-sdk-go 实现。
type MilvusSDKClient struct {
	cli client.Client
}

// NewMilvusSDKClient 包装已连接的 SDK 客户端。
func NewMilvusSDKClient(cli client.Client) *MilvusSDKClient {
	return &MilvusSDKClient{cli: cli}
}

// HasCollection 实现 milvusClient。
func (c *MilvusSDKClient) HasCollection(ctx context.Context, name string) (bool, error) {
	return c.cli.HasCollection(ctx, name)
}

// CreateCollection 建集：id（VarChar 主键）+ industry（VarChar 过滤列）+ embedding
// （FloatVector）+ IVF_FLAT 余弦索引 + 加载（LoadCollection 后立即可查）。
func (c *MilvusSDKClient) CreateCollection(ctx context.Context, name string, dim int) error {
	schema := entity.NewSchema().
		WithName(name).
		WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(64).WithIsPrimaryKey(true)).
		WithField(entity.NewField().WithName("industry").WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(64)).
		WithField(entity.NewField().WithName("embedding").WithDataType(entity.FieldTypeFloatVector).
			WithDim(int64(dim)))
	if err := c.cli.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
		return fmt.Errorf("milvus 建集失败: %w", err)
	}
	idx := entity.NewGenericIndex("embedding_index", "IVF_FLAT", map[string]string{
		"metric_type": string(entity.COSINE),
		"nlist":       "1024",
	})
	if err := c.cli.CreateIndex(ctx, name, "embedding", idx, false); err != nil {
		return fmt.Errorf("milvus 建索引失败: %w", err)
	}
	if err := c.cli.LoadCollection(ctx, name, false); err != nil {
		return fmt.Errorf("milvus 加载集合失败: %w", err)
	}
	return nil
}

// Insert 实现 milvusClient（column 式插入）。
func (c *MilvusSDKClient) Insert(ctx context.Context, coll, id, industry string, vec []float32) error {
	_, err := c.cli.Insert(ctx, coll, "",
		entity.NewColumnVarChar("id", []string{id}),
		entity.NewColumnVarChar("industry", []string{industry}),
		entity.NewColumnFloatVector("embedding", len(vec), [][]float32{vec}),
	)
	if err != nil {
		return fmt.Errorf("milvus 插入失败: %w", err)
	}
	return nil
}

// Search 实现 milvusClient（IVF_FLAT 索引对应 IVF search param）。
func (c *MilvusSDKClient) Search(ctx context.Context, coll, expr string, vec []float32, topK int) ([]milvusHit, error) {
	sp, err := entity.NewIndexIvfFlatSearchParam(16)
	if err != nil {
		return nil, fmt.Errorf("milvus 检索参数错误: %w", err)
	}
	results, err := c.cli.Search(ctx, coll, nil, expr, nil,
		[]entity.Vector{entity.FloatVector(vec)}, "embedding", entity.COSINE, topK, sp)
	if err != nil {
		return nil, fmt.Errorf("milvus 检索失败: %w", err)
	}
	if len(results) == 0 || results[0].ResultCount == 0 {
		return nil, nil
	}
	r := results[0]
	hits := make([]milvusHit, 0, r.ResultCount)
	for i := 0; i < r.ResultCount; i++ {
		id, err := r.IDs.GetAsString(i)
		if err != nil {
			continue
		}
		hits = append(hits, milvusHit{ID: id, Score: r.Scores[i]})
	}
	return hits, nil
}

// DeleteByExpr 实现 milvusClient（按主键表达式删除）。
func (c *MilvusSDKClient) DeleteByExpr(ctx context.Context, coll, expr string) error {
	if err := c.cli.Delete(ctx, coll, "", expr); err != nil {
		return fmt.Errorf("milvus 删除失败: %w", err)
	}
	return nil
}

// Close 实现 milvusClient。
func (c *MilvusSDKClient) Close() error { return c.cli.Close() }
