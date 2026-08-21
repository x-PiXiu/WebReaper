package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// VideoMetricPO 视频互动数据快照（数据回读时间序列）。
type VideoMetricPO struct {
	ID          string    `gorm:"primaryKey;size:64"`
	TenantID    string    `gorm:"size:64;index"`
	JobID       string    `gorm:"size:64"`
	Platform    string    `gorm:"size:32"`
	VideoID     string    `gorm:"size:64"`
	Views       int64
	Likes       int64
	Comments    int64
	Shares      int64
	CollectedAt time.Time `gorm:"index"`
}

func (VideoMetricPO) TableName() string { return "video_metrics" }

func videoMetricToPO(m entity.VideoMetric) VideoMetricPO {
	return VideoMetricPO{ID: m.ID, TenantID: m.TenantID, JobID: m.JobID, Platform: m.Platform,
		VideoID: m.VideoID, Views: m.Views, Likes: m.Likes, Comments: m.Comments, Shares: m.Shares,
		CollectedAt: m.CollectedAt}
}

func videoMetricFromPO(p VideoMetricPO) entity.VideoMetric {
	return entity.VideoMetric{ID: p.ID, TenantID: p.TenantID, JobID: p.JobID, Platform: p.Platform,
		VideoID: p.VideoID, Views: p.Views, Likes: p.Likes, Comments: p.Comments, Shares: p.Shares,
		CollectedAt: p.CollectedAt}
}

// GormVideoMetricRepository port.VideoMetricRepository 实现。
type GormVideoMetricRepository struct{ db *gorm.DB }

func NewGormVideoMetricRepository(db *gorm.DB) *GormVideoMetricRepository {
	return &GormVideoMetricRepository{db: db}
}

func (r *GormVideoMetricRepository) Save(ctx context.Context, m entity.VideoMetric) error {
	po := videoMetricToPO(m)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormVideoMetricRepository) ListByJob(ctx context.Context, tenantID, jobID string, limit int) ([]entity.VideoMetric, error) {
	q := r.db.WithContext(ctx).Where("tenant_id = ? AND job_id = ?", tenantID, jobID).
		Order("collected_at ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var pos []VideoMetricPO
	if err := q.Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.VideoMetric, 0, len(pos))
	for _, p := range pos {
		out = append(out, videoMetricFromPO(p))
	}
	return out, nil
}

// LatestByTenant 每作品最新一条（collected_at DESC 全量拉取后 Go 去重——
// 单租户作品量级小，避免窗口函数依赖）。
func (r *GormVideoMetricRepository) LatestByTenant(ctx context.Context, tenantID string) ([]entity.VideoMetric, error) {
	var pos []VideoMetricPO
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).
		Order("collected_at DESC").Limit(2000).Find(&pos).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	out := make([]entity.VideoMetric, 0)
	for _, p := range pos {
		if seen[p.JobID] {
			continue
		}
		seen[p.JobID] = true
		out = append(out, videoMetricFromPO(p))
	}
	return out, nil
}

// 编译期断言：实现 port.VideoMetricRepository。
var _ port.VideoMetricRepository = (*GormVideoMetricRepository)(nil)
