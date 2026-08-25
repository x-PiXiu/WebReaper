package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
)

// ---- 品牌发布配置仓储 ----

// BrandPublishConfigPO 品牌发布配置持久化对象
type BrandPublishConfigPO struct {
	ID              int64  `gorm:"primaryKey;autoIncrement"`
	TenantID        string `gorm:"size:64;not null"`
	BrandID         string `gorm:"size:64;not null"`
	Platform        string `gorm:"size:32;not null"`
	AccountIDsJSON  string `gorm:"column:account_ids;type:json"`
	RateLimitJSON   string `gorm:"column:rate_limit;type:json"`
	DefaultTagsJSON string `gorm:"column:default_tags;type:json"`
	DefaultPersona  string `gorm:"size:64"`
	IsActive        bool   `gorm:"default:true"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (BrandPublishConfigPO) TableName() string { return "brand_publish_configs" }

// GormBrandPublishConfigRepository 品牌发布配置仓储 GORM 实现
type GormBrandPublishConfigRepository struct {
	db *gorm.DB
}

// NewGormBrandPublishConfigRepository 创建品牌发布配置仓储
func NewGormBrandPublishConfigRepository(db *gorm.DB) *GormBrandPublishConfigRepository {
	return &GormBrandPublishConfigRepository{db: db}
}

func (r *GormBrandPublishConfigRepository) FindByBrand(ctx context.Context, tenantID, brandID string) ([]entity.BrandPublishConfig, error) {
	var pos []BrandPublishConfigPO
	q := r.db.WithContext(ctx).Where("brand_id = ?", brandID)
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.BrandPublishConfig, 0, len(pos))
	for _, po := range pos {
		out = append(out, r.fromPO(po))
	}
	return out, nil
}

func (r *GormBrandPublishConfigRepository) FindByPlatform(ctx context.Context, tenantID, brandID, platform string) (*entity.BrandPublishConfig, error) {
	var po BrandPublishConfigPO
	q := r.db.WithContext(ctx).Where("brand_id = ? AND platform = ?", brandID, platform)
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	config := r.fromPO(po)
	return &config, nil
}

func (r *GormBrandPublishConfigRepository) Save(ctx context.Context, config *entity.BrandPublishConfig) error {
	po := r.toPO(config)
	return r.db.WithContext(ctx).Where("tenant_id = ? AND brand_id = ? AND platform = ?", po.TenantID, po.BrandID, po.Platform).
		Assign(map[string]any{
			"account_ids":    po.AccountIDsJSON,
			"rate_limit":     po.RateLimitJSON,
			"default_tags":   po.DefaultTagsJSON,
			"default_persona": po.DefaultPersona,
			"is_active":      po.IsActive,
			"updated_at":     time.Now(),
		}).FirstOrCreate(&po).Error
}

func (r *GormBrandPublishConfigRepository) Delete(ctx context.Context, tenantID, brandID, platform string) error {
	q := r.db.WithContext(ctx).Where("brand_id = ? AND platform = ?", brandID, platform)
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	return q.Delete(&BrandPublishConfigPO{}).Error
}

func (r *GormBrandPublishConfigRepository) fromPO(po BrandPublishConfigPO) entity.BrandPublishConfig {
	config := entity.BrandPublishConfig{
		ID:             formatInt64(po.ID),
		TenantID:       po.TenantID,
		BrandID:        po.BrandID,
		Platform:       po.Platform,
		DefaultPersona: po.DefaultPersona,
		IsActive:       po.IsActive,
		CreatedAt:      po.CreatedAt,
		UpdatedAt:      po.UpdatedAt,
	}
	json.Unmarshal([]byte(po.AccountIDsJSON), &config.AccountIDs)
	json.Unmarshal([]byte(po.RateLimitJSON), &config.RateLimit)
	json.Unmarshal([]byte(po.DefaultTagsJSON), &config.DefaultTags)
	return config
}

func (r *GormBrandPublishConfigRepository) toPO(config *entity.BrandPublishConfig) BrandPublishConfigPO {
	accountIDsJSON, _ := json.Marshal(config.AccountIDs)
	rateLimitJSON, _ := json.Marshal(config.RateLimit)
	defaultTagsJSON, _ := json.Marshal(config.DefaultTags)
	return BrandPublishConfigPO{
		TenantID:        config.TenantID,
		BrandID:         config.BrandID,
		Platform:        config.Platform,
		AccountIDsJSON:  string(accountIDsJSON),
		RateLimitJSON:   string(rateLimitJSON),
		DefaultTagsJSON: string(defaultTagsJSON),
		DefaultPersona:  config.DefaultPersona,
		IsActive:        config.IsActive,
	}
}

// ---- 账号品牌绑定仓储 ----

// AccountBrandBindingPO 账号品牌绑定持久化对象
type AccountBrandBindingPO struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	TenantID  string `gorm:"size:64;not null"`
	AccountID string `gorm:"size:64;not null"`
	BrandID   string `gorm:"size:64;not null"`
	Platform  string `gorm:"size:32;not null"`
	IsDefault bool   `gorm:"default:false"`
	CreatedAt time.Time
}

func (AccountBrandBindingPO) TableName() string { return "account_brand_bindings" }

// GormAccountBrandBindingRepository 账号品牌绑定仓储 GORM 实现
type GormAccountBrandBindingRepository struct {
	db *gorm.DB
}

// NewGormAccountBrandBindingRepository 创建账号品牌绑定仓储
func NewGormAccountBrandBindingRepository(db *gorm.DB) *GormAccountBrandBindingRepository {
	return &GormAccountBrandBindingRepository{db: db}
}

func (r *GormAccountBrandBindingRepository) FindByBrand(ctx context.Context, tenantID, brandID string) ([]entity.AccountBrandBinding, error) {
	var pos []AccountBrandBindingPO
	q := r.db.WithContext(ctx).Where("brand_id = ?", brandID)
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.AccountBrandBinding, 0, len(pos))
	for _, po := range pos {
		out = append(out, r.fromPO(po))
	}
	return out, nil
}

func (r *GormAccountBrandBindingRepository) FindByAccount(ctx context.Context, accountID string) ([]entity.AccountBrandBinding, error) {
	var pos []AccountBrandBindingPO
	if err := r.db.WithContext(ctx).Where("account_id = ?", accountID).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.AccountBrandBinding, 0, len(pos))
	for _, po := range pos {
		out = append(out, r.fromPO(po))
	}
	return out, nil
}

func (r *GormAccountBrandBindingRepository) Bind(ctx context.Context, binding *entity.AccountBrandBinding) error {
	po := r.toPO(binding)
	return r.db.WithContext(ctx).Where("account_id = ? AND brand_id = ?", po.AccountID, po.BrandID).
		Assign(map[string]any{
			"tenant_id":  po.TenantID,
			"platform":   po.Platform,
			"is_default": po.IsDefault,
		}).FirstOrCreate(&po).Error
}

func (r *GormAccountBrandBindingRepository) Unbind(ctx context.Context, tenantID, accountID, brandID string) error {
	q := r.db.WithContext(ctx).Where("account_id = ? AND brand_id = ?", accountID, brandID)
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	return q.Delete(&AccountBrandBindingPO{}).Error
}

func (r *GormAccountBrandBindingRepository) fromPO(po AccountBrandBindingPO) entity.AccountBrandBinding {
	return entity.AccountBrandBinding{
		ID:        formatInt64(po.ID),
		TenantID:  po.TenantID,
		AccountID: po.AccountID,
		BrandID:   po.BrandID,
		Platform:  po.Platform,
		IsDefault: po.IsDefault,
		CreatedAt: po.CreatedAt,
	}
}

func (r *GormAccountBrandBindingRepository) toPO(binding *entity.AccountBrandBinding) AccountBrandBindingPO {
	return AccountBrandBindingPO{
		TenantID:  binding.TenantID,
		AccountID: binding.AccountID,
		BrandID:   binding.BrandID,
		Platform:  binding.Platform,
		IsDefault: binding.IsDefault,
	}
}

// ---- 发布使用量仓储 ----

// PublishUsageStatPO 发布使用量统计持久化对象
type PublishUsageStatPO struct {
	ID            int64  `gorm:"primaryKey;autoIncrement"`
	TenantID      string `gorm:"size:64;not null"`
	BrandID       string `gorm:"size:64;not null"`
	Platform      string `gorm:"size:32;not null"`
	PublishDate   string `gorm:"size:10;not null"`
	UsageCount    int    `gorm:"default:0"`
	LastPublishAt *time.Time
}

func (PublishUsageStatPO) TableName() string { return "publish_usage_stats" }

// GormPublishUsageRepository 发布使用量仓储 GORM 实现
type GormPublishUsageRepository struct {
	db *gorm.DB
}

// NewGormPublishUsageRepository 创建发布使用量仓储
func NewGormPublishUsageRepository(db *gorm.DB) *GormPublishUsageRepository {
	return &GormPublishUsageRepository{db: db}
}

func (r *GormPublishUsageRepository) GetDailyUsage(ctx context.Context, tenantID, brandID, platform string) (int, error) {
	var stat PublishUsageStatPO
	today := time.Now().Format("2006-01-02")
	q := r.db.WithContext(ctx).Where("brand_id = ? AND platform = ? AND publish_date = ?", brandID, platform, today)
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.First(&stat).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	return stat.UsageCount, nil
}

func (r *GormPublishUsageRepository) GetHourlyUsage(ctx context.Context, tenantID, brandID, platform string) (int, error) {
	// 统计最近1小时内的发布次数
	hourAgo := time.Now().Add(-1 * time.Hour)
	q := r.db.WithContext(ctx).Model(&PublishUsageStatPO{}).
		Where("brand_id = ? AND platform = ? AND last_publish_at >= ?", brandID, platform, hourAgo)
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	var stat PublishUsageStatPO
	if err := q.First(&stat).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	// 如果最近1小时内有发布，返回1（基于 publish_usage_stats 按天统计的简化实现）
	return 1, nil
}

func (r *GormPublishUsageRepository) GetLastPublishTime(ctx context.Context, tenantID, brandID, platform string) (*time.Time, error) {
	var stat PublishUsageStatPO
	q := r.db.WithContext(ctx).Where("brand_id = ? AND platform = ?", brandID, platform)
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.Order("last_publish_at DESC").First(&stat).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return stat.LastPublishAt, nil
}

func (r *GormPublishUsageRepository) IncrementUsage(ctx context.Context, tenantID, brandID, platform string) error {
	today := time.Now().Format("2006-01-02")
	now := time.Now()
	return r.db.WithContext(ctx).Where("tenant_id = ? AND brand_id = ? AND platform = ? AND publish_date = ?", tenantID, brandID, platform, today).
		Assign(map[string]any{
			"usage_count":     gorm.Expr("usage_count + 1"),
			"last_publish_at": now,
		}).FirstOrCreate(&PublishUsageStatPO{
		TenantID:    tenantID,
		BrandID:     brandID,
		Platform:    platform,
		PublishDate: today,
	}).Error
}

// formatInt64 格式化 int64 为字符串
func formatInt64(n int64) string {
	return fmt.Sprintf("%d", n)
}
