package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- IndexingSubmitLogPO 收录提交日志持久化对象 ----

type IndexingSubmitLogPO struct {
	ID          string    `gorm:"primaryKey;size:64"`
	Channel     string    `gorm:"size:16;index"`
	URL         string    `gorm:"size:512"`
	Status      string    `gorm:"size:16;index"`
	ErrorMsg    string    `gorm:"type:text"`
	SubmittedAt time.Time `gorm:"index"`
}

func (IndexingSubmitLogPO) TableName() string { return "indexing_submit_logs" }

func indexingLogToPO(e entity.IndexingSubmitLog) IndexingSubmitLogPO {
	return IndexingSubmitLogPO{
		ID: e.ID, Channel: string(e.Channel), URL: e.URL,
		Status: e.Status, ErrorMsg: e.ErrorMsg, SubmittedAt: e.SubmittedAt,
	}
}

func indexingLogFromPO(p IndexingSubmitLogPO) entity.IndexingSubmitLog {
	return entity.IndexingSubmitLog{
		ID: p.ID, Channel: entity.IndexingChannel(p.Channel), URL: p.URL,
		Status: p.Status, ErrorMsg: p.ErrorMsg, SubmittedAt: p.SubmittedAt,
	}
}

// GormIndexingLogRepository 是 port.IndexingLogRepository 的 GORM 实现。
type GormIndexingLogRepository struct {
	db *gorm.DB
}

var _ port.IndexingLogRepository = (*GormIndexingLogRepository)(nil)

func NewGormIndexingLogRepository(db *gorm.DB) *GormIndexingLogRepository {
	return &GormIndexingLogRepository{db: db}
}

func (r *GormIndexingLogRepository) Save(ctx context.Context, log entity.IndexingSubmitLog) error {
	po := indexingLogToPO(log)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormIndexingLogRepository) ListRecent(ctx context.Context, limit int) ([]entity.IndexingSubmitLog, error) {
	var pos []IndexingSubmitLogPO
	if err := r.db.WithContext(ctx).Order("submitted_at DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.IndexingSubmitLog, 0, len(pos))
	for _, p := range pos {
		out = append(out, indexingLogFromPO(p))
	}
	return out, nil
}
