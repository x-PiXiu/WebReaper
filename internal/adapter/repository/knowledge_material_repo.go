package repository

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- KnowledgeMaterialPO（平台知识库素材，表 kb_materials）----
// 与领域实体分离（ADR-003），通过 mapper 双向转换。
// 平台级表（无 tenant_id）：按行业组织，多租户共享检索。

// KnowledgeMaterialPO 知识库素材。
type KnowledgeMaterialPO struct {
	ID             string         `gorm:"primaryKey;size:64"`
	Industry       string         `gorm:"size:64;index"`
	SourceURL      string         `gorm:"size:1024"`
	URLFingerprint string         `gorm:"size:64;uniqueIndex"`
	Title          string         `gorm:"size:512"`
	Content        string         `gorm:"type:mediumtext"`
	Summary        string         `gorm:"type:text"`
	Tags           datatypes.JSON `gorm:"type:json"`
	CrawlKeyword   string         `gorm:"size:256"`
	Embedding      datatypes.JSON `gorm:"type:json"` // []float32 的 JSON 序列化
	Status         string         `gorm:"size:16;default:active"`
	CreatedAt      time.Time
}

func (KnowledgeMaterialPO) TableName() string { return "kb_materials" }

// ---- mapper ----

func knowledgeMaterialToPO(e entity.KnowledgeMaterial) KnowledgeMaterialPO {
	return KnowledgeMaterialPO{
		ID: e.ID, Industry: e.Industry, SourceURL: e.SourceURL, URLFingerprint: e.URLFingerprint,
		Title: e.Title, Content: e.Content, Summary: e.Summary,
		Tags: toJSON(e.Tags), CrawlKeyword: e.CrawlKeyword,
		Embedding: toFloat32JSON(e.Embedding),
		Status:    e.Status, CreatedAt: e.CreatedAt,
	}
}

func knowledgeMaterialFromPO(p KnowledgeMaterialPO) entity.KnowledgeMaterial {
	return entity.KnowledgeMaterial{
		ID: p.ID, Industry: p.Industry, SourceURL: p.SourceURL, URLFingerprint: p.URLFingerprint,
		Title: p.Title, Content: p.Content, Summary: p.Summary,
		Tags: toStringSlice(p.Tags), CrawlKeyword: p.CrawlKeyword,
		Embedding: fromFloat32JSON(p.Embedding),
		Status:    p.Status, CreatedAt: p.CreatedAt,
	}
}

// toFloat32JSON []float32 → datatypes.JSON（nil/空切片 → "null"，读出为 nil）。
func toFloat32JSON(v []float32) datatypes.JSON {
	if len(v) == 0 {
		return nil
	}
	b, _ := json.Marshal(v)
	return datatypes.JSON(b)
}

// fromFloat32JSON datatypes.JSON → []float32（nil/非法 → nil，调用方按"无向量"处理）。
func fromFloat32JSON(j datatypes.JSON) []float32 {
	if len(j) == 0 {
		return nil
	}
	var out []float32
	if err := json.Unmarshal(j, &out); err != nil {
		return nil
	}
	return out
}

// ---- GormKnowledgeMaterialRepository ----

var _ port.KnowledgeMaterialRepository = (*GormKnowledgeMaterialRepository)(nil)

// GormKnowledgeMaterialRepository 知识库素材仓储（GORM 实现）。
//
// 向量存储解耦（开闭原则）：余弦计算在注入的 port.VectorStore 实现里
// （MySQLVectorStore / 未来 MilvusVectorStore），向量库按运行时配置切换
// （port.VectorStoreProvider）——换向量库不改仓储。
type GormKnowledgeMaterialRepository struct {
	db       *gorm.DB
	vecStore port.VectorStoreProvider // 向量库工厂（可选：nil = 无向量能力，素材照常 CRUD）
}

// NewGormKnowledgeMaterialRepository 创建知识库素材仓储。
// vecStore 传 nil 可禁用向量检索（素材仍可入库/管理）。
func NewGormKnowledgeMaterialRepository(db *gorm.DB, vecStore port.VectorStoreProvider) *GormKnowledgeMaterialRepository {
	return &GormKnowledgeMaterialRepository{db: db, vecStore: vecStore}
}

// Save 保存素材（主键 upsert；URLFingerprint 唯一索引兜底防重）。
// 素材带向量时同步写入向量库（失败不阻断——降级为无向量素材，不影响入库）。
func (r *GormKnowledgeMaterialRepository) Save(ctx context.Context, m *entity.KnowledgeMaterial) error {
	po := knowledgeMaterialToPO(*m)
	if err := r.db.WithContext(ctx).Save(&po).Error; err != nil {
		return err
	}
	if r.vecStore != nil && len(m.Embedding) > 0 {
		// 向量存储失败不阻断（embedding 是增强能力，素材本体已入库）
		if store, err := r.vecStore.Get(ctx); err == nil {
			_ = store.Store(ctx, m.ID, m.Embedding, map[string]string{"industry": m.Industry})
		}
	}
	return nil
}

// ExistsByFingerprint 持久化去重：URL 指纹是否已入库。
func (r *GormKnowledgeMaterialRepository) ExistsByFingerprint(ctx context.Context, fingerprint string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&KnowledgeMaterialPO{}).
		Where("url_fingerprint = ?", fingerprint).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// SearchSimilar 向量检索：委托注入的 VectorStoreProvider（按配置取当前向量库：
// 行业过滤 + 余弦 topK），再按结果 ID 回查素材组装 MaterialRef（带来源——溯源需求）。
func (r *GormKnowledgeMaterialRepository) SearchSimilar(ctx context.Context, industry string, vec []float32, limit int) ([]entity.MaterialRef, error) {
	if r.vecStore == nil {
		return nil, nil // 无向量能力：返回空（调用方降级）
	}
	store, err := r.vecStore.Get(ctx)
	if err != nil {
		return nil, err
	}
	var filter map[string]string
	if industry != "" {
		filter = map[string]string{"industry": industry}
	}
	// 预取 num×3（阈值过滤后可能不足 num），由 VectorStore 按余弦降序返回
	results, err := store.Search(ctx, filter, vec, limit*3)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	// 按 ID 回查素材信息
	ids := make([]string, 0, len(results))
	for _, res := range results {
		ids = append(ids, res.ID)
	}
	var pos []KnowledgeMaterialPO
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&pos).Error; err != nil {
		return nil, err
	}
	// 按搜索结果顺序组装（VectorStore 已按相似度降序）
	byID := make(map[string]KnowledgeMaterialPO, len(pos))
	for i := range pos {
		byID[pos[i].ID] = pos[i]
	}
	refs := make([]entity.MaterialRef, 0, len(results))
	for _, res := range results {
		po, ok := byID[res.ID]
		if !ok {
			continue // 素材被删（残留向量）——跳过
		}
		refs = append(refs, entity.MaterialRef{
			Title:     po.Title,
			Summary:   po.Summary,
			SourceURL: po.SourceURL,
			Score:     res.Score,
		})
	}
	if len(refs) > limit {
		refs = refs[:limit]
	}
	return refs, nil
}

// Count 统计素材数（行业为空 = 全库）。
func (r *GormKnowledgeMaterialRepository) Count(ctx context.Context, industry string) (int64, error) {
	q := r.db.WithContext(ctx).Model(&KnowledgeMaterialPO{})
	if industry != "" {
		q = q.Where("industry = ?", industry)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// ListByIndustry 分页列出素材（行业为空 = 全库；created_at 降序）。
func (r *GormKnowledgeMaterialRepository) ListByIndustry(ctx context.Context, industry string, limit, offset int) ([]entity.KnowledgeMaterial, error) {
	q := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Offset(offset)
	if industry != "" {
		q = q.Where("industry = ?", industry)
	}
	var pos []KnowledgeMaterialPO
	if err := q.Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.KnowledgeMaterial, 0, len(pos))
	for i := range pos {
		out = append(out, knowledgeMaterialFromPO(pos[i]))
	}
	return out, nil
}

// Delete 删除素材（同步删除向量；向量删除失败不阻断——残留向量检索时按 ID 回查自然跳过）。
func (r *GormKnowledgeMaterialRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&KnowledgeMaterialPO{}).Error; err != nil {
		return err
	}
	if r.vecStore != nil {
		if store, err := r.vecStore.Get(ctx); err == nil {
			_ = store.Delete(ctx, id)
		}
	}
	return nil
}
