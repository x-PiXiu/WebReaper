package port

import (
	"context"
	"time"

	"webreaper/internal/domain/entity"
)

// ---- GEO 仓储接口（用例层声明，适配器实现）----
// 所有方法都带 tenantID，强制多租户隔离（admin 传空 = 看全局）。

// BrandRepository 品牌资产仓储。
type BrandRepository interface {
	Save(ctx context.Context, b entity.Brand) error
	FindByID(ctx context.Context, tenantID, id string) (entity.Brand, error)
	// FindPublishedByID 公开查询：按 ID 查品牌（公开站用，不限定租户）。
	FindPublishedByID(ctx context.Context, id string) (entity.Brand, error)
	ListByTenant(ctx context.Context, tenantID string) ([]entity.Brand, error)
	Delete(ctx context.Context, tenantID, id string) error
	// Count 统计品牌总数（平台总览用，admin 看全局）。
	Count(ctx context.Context) (int, error)
	// ListAll 全平台品牌列表（admin 旁路——仅管理后台全局管理端点调用，
	// 商户上下文一律走 ListByTenant，杜绝空租户越权）。
	ListAll(ctx context.Context) ([]entity.Brand, error)
}

// KeywordRepository 关键词仓储。
type KeywordRepository interface {
	Save(ctx context.Context, k entity.Keyword) error
	FindByID(ctx context.Context, tenantID, id string) (entity.Keyword, error) // 直接按 ID 查（避免 N+1）
	ListByBrand(ctx context.Context, tenantID, brandID string) ([]entity.Keyword, error)
	ListByTenant(ctx context.Context, tenantID string) ([]entity.Keyword, error)
	Delete(ctx context.Context, tenantID, id string) error
	// Count 统计关键词总数（平台总览用，admin 看全局）。
	Count(ctx context.Context) (int, error)
}

// StoreLocationRepository 门店档案仓储（本地生活 GEO 地基）。
type StoreLocationRepository interface {
	Save(ctx context.Context, s entity.StoreLocation) error
	FindByID(ctx context.Context, tenantID, id string) (entity.StoreLocation, error)
	ListByBrand(ctx context.Context, tenantID, brandID string) ([]entity.StoreLocation, error)
	// FindPrimaryByBrand 取品牌主门店（公开内容站 NAP 注入/周边搜索中心点用；
	// 不限租户——公开查询，取该品牌最早创建的门店）。
	FindPrimaryByBrand(ctx context.Context, brandID string) (entity.StoreLocation, error)
	Delete(ctx context.Context, tenantID, id string) error
}

// MonitoringResultRepository 监测结果仓储（核心数据资产）。
type MonitoringResultRepository interface {
	Save(ctx context.Context, r entity.MonitoringResult) error
	// LatestByKeyword 取某关键词在各引擎的最新监测结果。
	LatestByKeyword(ctx context.Context, tenantID, keywordID string) ([]entity.MonitoringResult, error)
	// LatestByBrand 取某品牌下所有关键词的最新监测结果。
	LatestByBrand(ctx context.Context, tenantID, brandID string) ([]entity.MonitoringResult, error)
	// LatestByTenant 取租户下所有关键词的最新监测结果（关键词管理页一览用，不依赖品牌筛选）。
	LatestByTenant(ctx context.Context, tenantID string) ([]entity.MonitoringResult, error)
	// Trend 取某品牌的提及率趋势（时间序列）。
	Trend(ctx context.Context, tenantID, brandID string, limit int) ([]entity.MonitoringResult, error)
	// Count 统计监测结果总数（平台总览用，admin 看全局）。
	Count(ctx context.Context) (int, error)
}

// OptimizedContentRepository 优化内容仓储。
type OptimizedContentRepository interface {
	Save(ctx context.Context, c entity.OptimizedContent) error
	ListByBrand(ctx context.Context, tenantID, brandID string) ([]entity.OptimizedContent, error)
	FindByID(ctx context.Context, tenantID, id string) (entity.OptimizedContent, error)
	FindMaxVersion(ctx context.Context, tenantID, brandID, keywordID string) (int, error)
	// FindPublishedByID 公开查询：按 ID 查已发布内容（公开站点用，不限定租户，
	// 仅返回 status=published——未发布内容对公网不可见）。
	FindPublishedByID(ctx context.Context, id string) (entity.OptimizedContent, error)
	// ListPublished 公开查询：列出全部已发布内容（sitemap/llms.txt 用）。
	ListPublished(ctx context.Context) ([]entity.OptimizedContent, error)
	// Count 统计优化内容总数（平台总览用，admin 看全局）。
	Count(ctx context.Context) (int, error)
	// CountPublished 统计已发布内容数（平台总览/公开站点规模用）。
	CountPublished(ctx context.Context) (int, error)
	// Delete 删除优化内容（管理后台/内容工作台用）。
	Delete(ctx context.Context, tenantID, id string) error
	// ListAll 全平台内容列表（admin 旁路——仅管理后台全局管理端点调用；
	// 可按状态过滤，limit<=0 返回全部）。
	ListAll(ctx context.Context, status string, limit int) ([]entity.OptimizedContent, error)
	// UpdateIndexStatus 更新内容收录状态（收录验证任务用；status 为空表示
	// 从未验证过——发布后初始写入 pending）。
	UpdateIndexStatus(ctx context.Context, tenantID, id, status string, indexedAt time.Time) error
}
