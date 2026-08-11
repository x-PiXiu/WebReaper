package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ---- GEO 仓储的 GORM 实现 ----
// 所有查询强制带 tenant_id 过滤（多租户隔离）。
// tenantID 为空时（admin 看全局）跳过过滤。

func applyTenantScope(db *gorm.DB, tenantID string) *gorm.DB {
	if tenantID == "" {
		return db
	}
	return db.Where("tenant_id = ?", tenantID)
}

// ============ BrandRepository ============

type GormBrandRepository struct{ db *gorm.DB }

var _ port.BrandRepository = (*GormBrandRepository)(nil)

func NewGormBrandRepository(db *gorm.DB) *GormBrandRepository {
	return &GormBrandRepository{db: db}
}

func (r *GormBrandRepository) Save(ctx context.Context, b entity.Brand) error {
	po := brandToPO(b)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormBrandRepository) FindByID(ctx context.Context, tenantID, id string) (entity.Brand, error) {
	var po BrandPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	err := q.Where("id = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.Brand{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.Brand{}, err
	}
	return brandFromPO(po), nil
}

// FindPublishedByID 公开查询：按 ID 查品牌（不限租户——公开站渲染用）。
func (r *GormBrandRepository) FindPublishedByID(ctx context.Context, id string) (entity.Brand, error) {
	var po BrandPO
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.Brand{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.Brand{}, err
	}
	return brandFromPO(po), nil
}

func (r *GormBrandRepository) ListByTenant(ctx context.Context, tenantID string) ([]entity.Brand, error) {
	var pos []BrandPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	if err := q.Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.Brand, 0, len(pos))
	for _, p := range pos {
		out = append(out, brandFromPO(p))
	}
	return out, nil
}

func (r *GormBrandRepository) Delete(ctx context.Context, tenantID, id string) error {
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	return q.Where("id = ?", id).Delete(&BrandPO{}).Error
}

func (r *GormBrandRepository) Count(ctx context.Context) (int, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&BrandPO{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

// ListAll 全平台品牌列表（admin 旁路——无租户过滤，仅管理后台全局端点调用）。
func (r *GormBrandRepository) ListAll(ctx context.Context) ([]entity.Brand, error) {
	var pos []BrandPO
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.Brand, 0, len(pos))
	for _, p := range pos {
		out = append(out, brandFromPO(p))
	}
	return out, nil
}

// ============ KeywordRepository ============

type GormKeywordRepository struct{ db *gorm.DB }

var _ port.KeywordRepository = (*GormKeywordRepository)(nil)

func NewGormKeywordRepository(db *gorm.DB) *GormKeywordRepository {
	return &GormKeywordRepository{db: db}
}

func (r *GormKeywordRepository) Save(ctx context.Context, k entity.Keyword) error {
	po := keywordToPO(k)
	return r.db.WithContext(ctx).Save(&po).Error
}

// FindByID 直接按 ID 查关键词（带租户隔离）。
func (r *GormKeywordRepository) FindByID(ctx context.Context, tenantID, id string) (entity.Keyword, error) {
	var po KeywordPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	err := q.Where("id = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.Keyword{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.Keyword{}, err
	}
	return keywordFromPO(po), nil
}

func (r *GormKeywordRepository) ListByBrand(ctx context.Context, tenantID, brandID string) ([]entity.Keyword, error) {
	var pos []KeywordPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	if err := q.Where("brand_id = ?", brandID).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.Keyword, 0, len(pos))
	for _, p := range pos {
		out = append(out, keywordFromPO(p))
	}
	return out, nil
}

// ListByTenant 跨品牌查租户所有关键词（关键词管理页用）。
func (r *GormKeywordRepository) ListByTenant(ctx context.Context, tenantID string) ([]entity.Keyword, error) {
	var pos []KeywordPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	if err := q.Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.Keyword, 0, len(pos))
	for _, p := range pos {
		out = append(out, keywordFromPO(p))
	}
	return out, nil
}

func (r *GormKeywordRepository) Delete(ctx context.Context, tenantID, id string) error {
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	return q.Where("id = ?", id).Delete(&KeywordPO{}).Error
}

func (r *GormKeywordRepository) Count(ctx context.Context) (int, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&KeywordPO{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

// ============ StoreLocationRepository ============

type GormStoreLocationRepository struct{ db *gorm.DB }

var _ port.StoreLocationRepository = (*GormStoreLocationRepository)(nil)

func NewGormStoreLocationRepository(db *gorm.DB) *GormStoreLocationRepository {
	return &GormStoreLocationRepository{db: db}
}

func (r *GormStoreLocationRepository) Save(ctx context.Context, s entity.StoreLocation) error {
	po := storeLocationToPO(s)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormStoreLocationRepository) FindByID(ctx context.Context, tenantID, id string) (entity.StoreLocation, error) {
	var po StoreLocationPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	err := q.Where("id = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.StoreLocation{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.StoreLocation{}, err
	}
	return storeLocationFromPO(po), nil
}

func (r *GormStoreLocationRepository) ListByBrand(ctx context.Context, tenantID, brandID string) ([]entity.StoreLocation, error) {
	var pos []StoreLocationPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	if err := q.Where("brand_id = ?", brandID).Order("created_at ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.StoreLocation, 0, len(pos))
	for _, p := range pos {
		out = append(out, storeLocationFromPO(p))
	}
	return out, nil
}

// FindPrimaryByBrand 公开查询：取品牌主门店（最早创建的；不限租户——
// 公开内容站 NAP 注入/周边搜索中心点用，不暴露其他租户数据）。
func (r *GormStoreLocationRepository) FindPrimaryByBrand(ctx context.Context, brandID string) (entity.StoreLocation, error) {
	var po StoreLocationPO
	err := r.db.WithContext(ctx).Where("brand_id = ?", brandID).Order("created_at ASC").First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.StoreLocation{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.StoreLocation{}, err
	}
	return storeLocationFromPO(po), nil
}

func (r *GormStoreLocationRepository) Delete(ctx context.Context, tenantID, id string) error {
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	return q.Where("id = ?", id).Delete(&StoreLocationPO{}).Error
}

// ============ MonitoringResultRepository ============

type GormMonitoringResultRepository struct{ db *gorm.DB }

var _ port.MonitoringResultRepository = (*GormMonitoringResultRepository)(nil)

func NewGormMonitoringResultRepository(db *gorm.DB) *GormMonitoringResultRepository {
	return &GormMonitoringResultRepository{db: db}
}

func (r *GormMonitoringResultRepository) Save(ctx context.Context, m entity.MonitoringResult) error {
	po := monitoringResultToPO(m)
	return r.db.WithContext(ctx).Save(&po).Error
}

// LatestByKeyword 取某关键词在各引擎的最新监测结果。
// 每个 engine_name 取最新一条（子查询取最大 probed_at）。
func (r *GormMonitoringResultRepository) LatestByKeyword(ctx context.Context, tenantID, keywordID string) ([]entity.MonitoringResult, error) {
	var pos []MonitoringResultPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	// 取该关键词全部结果，按引擎+时间倒序，Go 层去重保留每个引擎最新一条
	if err := q.Where("keyword_id = ?", keywordID).Order("engine_name, probed_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []entity.MonitoringResult
	for _, p := range pos {
		if seen[p.EngineName] {
			continue
		}
		seen[p.EngineName] = true
		out = append(out, monitoringResultFromPO(p))
	}
	return out, nil
}

// LatestByBrand 取某品牌下所有关键词的最新监测结果（关键词一览页用）。
// 每个关键词 × 每个引擎只保留最新一条。
func (r *GormMonitoringResultRepository) LatestByBrand(ctx context.Context, tenantID, brandID string) ([]entity.MonitoringResult, error) {
	var pos []MonitoringResultPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	if err := q.Where("brand_id = ?", brandID).Order("keyword_id, engine_name, probed_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	// 去重：key = keywordID + engineName，保留最新（已按时间倒序，第一条即最新）
	seen := make(map[string]bool)
	var out []entity.MonitoringResult
	for _, p := range pos {
		key := p.KeywordID + "|" + p.EngineName
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, monitoringResultFromPO(p))
	}
	return out, nil
}

// LatestByTenant 取租户下所有关键词的最新监测结果（关键词一览页用）。
// 每个关键词 × 每个引擎只保留最新一条。不依赖品牌筛选。
func (r *GormMonitoringResultRepository) LatestByTenant(ctx context.Context, tenantID string) ([]entity.MonitoringResult, error) {
	var pos []MonitoringResultPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	// 按时间倒序取全部（限 500 条防膨胀），Go 层按 (keyword, engine) 分组保留最近 5 条——
	// 修复：仅保留 1 条会导致"变化对比 delta"永远算不出、详情无历史记录。
	// 前端按 keyword_id 分组后自行排序（sortByTime），组内顺序无要求。
	if err := q.Order("probed_at DESC").Limit(500).Find(&pos).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]int)
	var out []entity.MonitoringResult
	for _, p := range pos {
		key := p.KeywordID + "|" + p.EngineName
		if seen[key] >= 5 { // 每组保留最近 5 条（趋势/对比足够，防详情爆炸）
			continue
		}
		seen[key]++
		out = append(out, monitoringResultFromPO(p))
	}
	return out, nil
}
func (r *GormMonitoringResultRepository) Trend(ctx context.Context, tenantID, brandID string, limit int) ([]entity.MonitoringResult, error) {
	var pos []MonitoringResultPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	if limit <= 0 {
		limit = 30
	}
	if err := q.Where("brand_id = ?", brandID).Order("probed_at DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.MonitoringResult, 0, len(pos))
	for _, p := range pos {
		out = append(out, monitoringResultFromPO(p))
	}
	return out, nil
}

func (r *GormMonitoringResultRepository) Count(ctx context.Context) (int, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&MonitoringResultPO{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

// ============ OptimizedContentRepository ============

type GormOptimizedContentRepository struct{ db *gorm.DB }

var _ port.OptimizedContentRepository = (*GormOptimizedContentRepository)(nil)

func NewGormOptimizedContentRepository(db *gorm.DB) *GormOptimizedContentRepository {
	return &GormOptimizedContentRepository{db: db}
}

func (r *GormOptimizedContentRepository) Save(ctx context.Context, c entity.OptimizedContent) error {
	po := optimizedContentToPO(c)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormOptimizedContentRepository) ListByBrand(ctx context.Context, tenantID, brandID string) ([]entity.OptimizedContent, error) {
	var pos []OptimizedContentPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	if err := q.Where("brand_id = ?", brandID).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.OptimizedContent, 0, len(pos))
	for _, p := range pos {
		out = append(out, optimizedContentFromPO(p))
	}
	return out, nil
}

func (r *GormOptimizedContentRepository) FindByID(ctx context.Context, tenantID, id string) (entity.OptimizedContent, error) {
	var po OptimizedContentPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	err := q.Where("id = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.OptimizedContent{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.OptimizedContent{}, err
	}
	return optimizedContentFromPO(po), nil
}

func (r *GormOptimizedContentRepository) FindMaxVersion(ctx context.Context, tenantID, brandID, keywordID string) (int, error) {
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	q = q.Where("brand_id = ?", brandID)
	if keywordID != "" {
		q = q.Where("keyword_id = ?", keywordID)
	}
	var maxVersion int
	err := q.Model(&OptimizedContentPO{}).Select("COALESCE(MAX(version), 0)").Scan(&maxVersion).Error
	if err != nil {
		return 0, err
	}
	return maxVersion, nil
}

// FindPublishedByID 公开查询：按 ID 查已发布内容（不限定租户、不限定 status 之外的任何条件）。
// 未发布（draft）内容对公网不可见——返回 pkg.ErrNotFound。
// Delete 删除优化内容（物理删除；管理后台/内容工作台用）。
func (r *GormOptimizedContentRepository) Delete(ctx context.Context, tenantID, id string) error {
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	return q.Where("id = ?", id).Delete(&OptimizedContentPO{}).Error
}

func (r *GormOptimizedContentRepository) FindPublishedByID(ctx context.Context, id string) (entity.OptimizedContent, error) {	var po OptimizedContentPO
	err := r.db.WithContext(ctx).Where("id = ? AND status = ?", id, "published").First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.OptimizedContent{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.OptimizedContent{}, err
	}
	return optimizedContentFromPO(po), nil
}

// ListPublished 公开查询：列出全部已发布内容（sitemap/llms.txt 用）。
func (r *GormOptimizedContentRepository) ListPublished(ctx context.Context) ([]entity.OptimizedContent, error) {
	var pos []OptimizedContentPO
	if err := r.db.WithContext(ctx).
		Where("status = ?", "published").
		Order("created_at DESC").
		Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.OptimizedContent, 0, len(pos))
	for _, p := range pos {
		out = append(out, optimizedContentFromPO(p))
	}
	return out, nil
}

// UpdateIndexStatus 更新内容收录状态（收录验证任务用；tenantID 限定租户防越权）。
func (r *GormOptimizedContentRepository) UpdateIndexStatus(ctx context.Context, tenantID, id, status string, indexedAt time.Time) error {
	return applyTenantScope(r.db.WithContext(ctx), tenantID).
		Model(&OptimizedContentPO{}).
		Where("id = ?", id).
		Updates(map[string]any{"index_status": status, "indexed_at": indexedAt}).Error
}

func (r *GormOptimizedContentRepository) Count(ctx context.Context) (int, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&OptimizedContentPO{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *GormOptimizedContentRepository) CountPublished(ctx context.Context) (int, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&OptimizedContentPO{}).
		Where("status = ?", "published").Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

// ListAll 全平台内容列表（admin 旁路——无租户过滤，仅管理后台全局端点调用）。
func (r *GormOptimizedContentRepository) ListAll(ctx context.Context, status string, limit int) ([]entity.OptimizedContent, error) {
	q := r.db.WithContext(ctx).Order("created_at DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	var pos []OptimizedContentPO
	if err := q.Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.OptimizedContent, 0, len(pos))
	for _, p := range pos {
		out = append(out, optimizedContentFromPO(p))
	}
	return out, nil
}
