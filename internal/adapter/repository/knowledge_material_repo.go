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
	BrandID        string         `gorm:"size:64;index"` // 品牌私有素材归属（空 = 行业公共池）
	TenantID       string         `gorm:"size:64"`       // 租户隔离（品牌私有必填）
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
		ID: e.ID, BrandID: e.BrandID, TenantID: e.TenantID,
		Industry: e.Industry, SourceURL: e.SourceURL, URLFingerprint: e.URLFingerprint,
		Title: e.Title, Content: e.Content, Summary: e.Summary,
		Tags: toJSON(e.Tags), CrawlKeyword: e.CrawlKeyword,
		Embedding: toFloat32JSON(e.Embedding),
		Status:    e.Status, CreatedAt: e.CreatedAt,
	}
}

func knowledgeMaterialFromPO(p KnowledgeMaterialPO) entity.KnowledgeMaterial {
	return entity.KnowledgeMaterial{
		ID: p.ID, BrandID: p.BrandID, TenantID: p.TenantID,
		Industry: p.Industry, SourceURL: p.SourceURL, URLFingerprint: p.URLFingerprint,
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

// SearchSimilar 分层向量检索（获客智能体转型）：
// brandID 非空 → 先查品牌私有素材（brand_id = ?），不足 limit 补行业公共池（brand_id = '' AND industry = ?）。
// brandID 为空 → 纯行业检索（与原有行为完全兼容）。
func (r *GormKnowledgeMaterialRepository) SearchSimilar(ctx context.Context, industry, brandID string, vec []float32, limit int) ([]entity.MaterialRef, error) {
	if r.vecStore == nil {
		return nil, nil // 无向量能力：返回空（调用方降级）
	}
	store, err := r.vecStore.Get(ctx)
	if err != nil {
		return nil, err
	}

	// 第一层：品牌私有素材
	var refs []entity.MaterialRef
	if brandID != "" {
		refs, err = r.searchWithFilter(ctx, store, map[string]string{"brand_id": brandID}, vec, limit)
		if err != nil {
			return nil, err
		}
	}
	// 第二层：行业公共池补位（品牌素材不足 limit 时）
	if len(refs) < limit {
		remaining := limit - len(refs)
		var filter map[string]string
		if industry != "" {
			filter = map[string]string{"industry": industry}
		}
		// 公共池排除品牌已有的（去重：按 title 匹配近似——简化处理）
		seen := make(map[string]bool, len(refs))
		for _, ref := range refs {
			seen[ref.Title] = true
		}
		pubRefs, err := r.searchWithFilter(ctx, store, filter, vec, remaining+3)
		if err != nil {
			return nil, err
		}
		for _, ref := range pubRefs {
			if !seen[ref.Title] && len(refs) < limit {
				refs = append(refs, ref)
			}
		}
	}
	return refs, nil
}

// searchWithFilter 按过滤器做向量检索并回查素材组装 MaterialRef。
func (r *GormKnowledgeMaterialRepository) searchWithFilter(ctx context.Context, store port.VectorStore, filter map[string]string, vec []float32, limit int) ([]entity.MaterialRef, error) {
	if limit <= 0 {
		return nil, nil
	}
	results, err := store.Search(ctx, filter, vec, limit*3)
	if err != nil || len(results) == 0 {
		return nil, err
	}
	ids := make([]string, 0, len(results))
	for _, res := range results {
		ids = append(ids, res.ID)
	}
	var pos []KnowledgeMaterialPO
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&pos).Error; err != nil {
		return nil, err
	}
	byID := make(map[string]KnowledgeMaterialPO, len(pos))
	for i := range pos {
		byID[pos[i].ID] = pos[i]
	}
	refs := make([]entity.MaterialRef, 0, len(results))
	for _, res := range results {
		po, ok := byID[res.ID]
		if !ok {
			continue
		}
		refs = append(refs, entity.MaterialRef{
			Title:     po.Title,
			Summary:   po.Summary,
			SourceURL: po.SourceURL,
			Score:     res.Score,
		})
		if len(refs) >= limit {
			break
		}
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

// CountByBrand 统计某品牌私有素材数。
func (r *GormKnowledgeMaterialRepository) CountByBrand(ctx context.Context, brandID string) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&KnowledgeMaterialPO{}).
		Where("brand_id = ?", brandID).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// ListByBrand 分页列出某品牌私有素材（created_at 降序；tenantID 做隔离校验——只能看自己品牌的）。
func (r *GormKnowledgeMaterialRepository) ListByBrand(ctx context.Context, tenantID, brandID string, limit, offset int) ([]entity.KnowledgeMaterial, error) {
	var pos []KnowledgeMaterialPO
	if err := r.db.WithContext(ctx).
		Where("brand_id = ? AND tenant_id = ?", brandID, tenantID).
		Order("created_at DESC").Limit(limit).Offset(offset).
		Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.KnowledgeMaterial, 0, len(pos))
	for i := range pos {
		out = append(out, knowledgeMaterialFromPO(pos[i]))
	}
	return out, nil
}

// DeleteByBrand 删除品牌私有素材（tenantID 隔离——只能删自己品牌的）。
func (r *GormKnowledgeMaterialRepository) DeleteByBrand(ctx context.Context, tenantID, brandID, materialID string) error {
	if err := r.db.WithContext(ctx).
		Where("id = ? AND brand_id = ? AND tenant_id = ?", materialID, brandID, tenantID).
		Delete(&KnowledgeMaterialPO{}).Error; err != nil {
		return err
	}
	if r.vecStore != nil {
		if store, err := r.vecStore.Get(ctx); err == nil {
			_ = store.Delete(ctx, materialID)
		}
	}
	return nil
}
