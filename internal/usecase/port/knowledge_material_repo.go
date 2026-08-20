package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// KnowledgeMaterialRepository 知识库素材仓储（双维度：品牌私有 + 行业公共池）。
//
// 设计动机（素材溯源 + 获客智能体转型）：
//   - 采集端：URL/内容指纹持久化去重（ExistsByFingerprint）——替代爬虫装饰器的内存 map
//   - 检索端：SearchSimilar 分层检索（品牌私有优先 → 行业公共池补位）
//   - 商户端：ListByBrand/DeleteByBrand 按品牌维度管理（租户隔离）
type KnowledgeMaterialRepository interface {
	// Save 保存素材（新建/更新；URLFingerprint 冲突由数据库唯一索引兜底）。
	Save(ctx context.Context, m *entity.KnowledgeMaterial) error
	// ExistsByFingerprint 持久化去重：该指纹是否已入库。
	ExistsByFingerprint(ctx context.Context, fingerprint string) (bool, error)
	// SearchSimilar 分层检索：品牌私有优先 → 行业公共池补位；余弦相似度 topK。
	// brandID 非空时先查该品牌素材，不足 limit 补行业公共池（排除该品牌已含条目）。
	// brandID 为空 = 纯行业检索（与原有行为兼容）。
	SearchSimilar(ctx context.Context, industry, brandID string, vec []float32, limit int) ([]entity.MaterialRef, error)
	// Count 统计某行业素材数（行业为空 = 全库）。
	Count(ctx context.Context, industry string) (int64, error)
	// CountByBrand 统计某品牌私有素材数。
	CountByBrand(ctx context.Context, brandID string) (int64, error)
	// ListByIndustry 分页列出某行业素材（行业为空 = 全库；created_at 降序）。
	ListByIndustry(ctx context.Context, industry string, limit, offset int) ([]entity.KnowledgeMaterial, error)
	// ListByBrand 分页列出某品牌私有素材（created_at 降序；tenantID 做隔离校验）。
	ListByBrand(ctx context.Context, tenantID, brandID string, limit, offset int) ([]entity.KnowledgeMaterial, error)
	// Delete 删除素材（admin 全局）。
	Delete(ctx context.Context, id string) error
	// DeleteByBrand 删除品牌私有素材（tenantID 隔离——只能删自己品牌的）。
	DeleteByBrand(ctx context.Context, tenantID, brandID, materialID string) error
}
