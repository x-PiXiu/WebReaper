package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// GormExternalSystemRepository 是 port.ExternalSystemRepository 的 GORM 实现。
type GormExternalSystemRepository struct{ db *gorm.DB }

var _ port.ExternalSystemRepository = (*GormExternalSystemRepository)(nil)

func NewGormExternalSystemRepository(db *gorm.DB) *GormExternalSystemRepository {
	return &GormExternalSystemRepository{db: db}
}

func (r *GormExternalSystemRepository) Save(ctx context.Context, sys entity.ExternalSystem) error {
	po := externalSystemToPO(sys)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormExternalSystemRepository) FindByName(ctx context.Context, name string) (entity.ExternalSystem, error) {
	var po ExternalSystemPO
	err := r.db.WithContext(ctx).First(&po, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.ExternalSystem{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.ExternalSystem{}, err
	}
	return externalSystemFromPO(po), nil
}

func (r *GormExternalSystemRepository) List(ctx context.Context) ([]entity.ExternalSystem, error) {
	var pos []ExternalSystemPO
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.ExternalSystem, 0, len(pos))
	for _, p := range pos {
		result = append(result, externalSystemFromPO(p))
	}
	return result, nil
}

func (r *GormExternalSystemRepository) Delete(ctx context.Context, name string) error {
	return r.db.WithContext(ctx).Where("name = ?", name).Delete(&ExternalSystemPO{}).Error
}

// ---- PublishRecord 仓储（复用 001_init.sql 的 publish_records 表）----
// 该表字段：id/content_id/content_type/platform/success/external_id/error_msg/result_at/created_at/updated_at

type GormPublishRecordRepository struct{ db *gorm.DB }

var _ port.PublishRecordRepository = (*GormPublishRecordRepository)(nil)

func NewGormPublishRecordRepository(db *gorm.DB) *GormPublishRecordRepository {
	return &GormPublishRecordRepository{db: db}
}

func (r *GormPublishRecordRepository) Save(ctx context.Context, rec entity.PublishRecord) error {
	now := time.Now()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.ResultAt.IsZero() {
		rec.ResultAt = now
	}
	// 用 map 写入，避免 GORM 自动建表（复用旧表，列名 platform 对应 system_name）
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO publish_records (id, content_id, content_type, platform, success, external_id, error_msg, result_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE success = VALUES(success), external_id = VALUES(external_id), error_msg = VALUES(error_msg), result_at = VALUES(result_at), updated_at = VALUES(updated_at)`,
		rec.ID, rec.ContentID, rec.ContentType, rec.SystemName, rec.Success, rec.ExternalID, rec.ErrorMsg, rec.ResultAt, rec.CreatedAt, now,
	).Error
}

func (r *GormPublishRecordRepository) ListByContent(ctx context.Context, contentID string) ([]entity.PublishRecord, error) {
	rows, err := r.db.WithContext(ctx).Raw(
		`SELECT id, content_id, content_type, platform, success, external_id, error_msg, result_at, created_at FROM publish_records WHERE content_id = ? ORDER BY created_at DESC`,
		contentID,
	).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPublishRecords(rows)
}

func (r *GormPublishRecordRepository) FindDedup(ctx context.Context, contentID, systemName string) (entity.PublishRecord, error) {
	var success bool
	err := r.db.WithContext(ctx).Raw(
		`SELECT success FROM publish_records WHERE content_id = ? AND platform = ? AND success = 1 LIMIT 1`,
		contentID, systemName,
	).Row().Scan(&success)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.PublishRecord{}, pkg.ErrNotFound
		}
		return entity.PublishRecord{}, err
	}
	return entity.PublishRecord{ContentID: contentID, SystemName: systemName, Success: true}, nil
}

func scanPublishRecords(rows *sql.Rows) ([]entity.PublishRecord, error) {
	var result []entity.PublishRecord
	for rows.Next() {
		var r entity.PublishRecord
		var success bool
		if err := rows.Scan(&r.ID, &r.ContentID, &r.ContentType, &r.SystemName, &success, &r.ExternalID, &r.ErrorMsg, &r.ResultAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Success = success
		result = append(result, r)
	}
	return result, nil
}
