package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// KnowledgeMaterialRepository 知识库素材仓储（平台级：按行业组织，无租户维度）。
//
// 设计动机（素材溯源）：
//   - 采集端：URL 指纹持久化去重（ExistsByFingerprint）——替代爬虫装饰器的内存 map，
//     重启不丢、多实例不重爬。
//   - 检索端：SearchSimilar 按行业过滤 + 向量余弦相似度 topK，
//     返回带来源（SourceURL）的引用——生成注入"有据可查"。
type KnowledgeMaterialRepository interface {
	// Save 保存素材（新建/更新；URLFingerprint 冲突由数据库唯一索引兜底）。
	Save(ctx context.Context, m *entity.KnowledgeMaterial) error
	// ExistsByFingerprint 持久化去重：该 URL 指纹是否已入库。
	ExistsByFingerprint(ctx context.Context, fingerprint string) (bool, error)
	// SearchSimilar 按行业过滤 + 余弦相似度 topK（仅 active 且带向量的素材）。
	// 行业为空 = 全行业检索；返回按相似度降序。
	SearchSimilar(ctx context.Context, industry string, vec []float32, limit int) ([]entity.MaterialRef, error)
	// Count 统计某行业素材数（行业为空 = 全库）。
	Count(ctx context.Context, industry string) (int64, error)
	// ListByIndustry 分页列出某行业素材（行业为空 = 全库；created_at 降序）。
	ListByIndustry(ctx context.Context, industry string, limit, offset int) ([]entity.KnowledgeMaterial, error)
	// Delete 删除素材。
	Delete(ctx context.Context, id string) error
}
