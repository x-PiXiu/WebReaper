package repository

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- 平台方账号仓储 ----

// CrawlerAccountPO 平台方账号持久化对象。
type CrawlerAccountPO struct {
	ID                int64      `gorm:"primaryKey;autoIncrement"`
	Platform          string     `gorm:"size:32;not null"`
	AccountName       string     `gorm:"size:128;not null"`
	CookieEncrypted   string     `gorm:"type:text;not null"`
	UserAgent         string     `gorm:"size:512;not null;default:''"`
	ProxyAddress      string     `gorm:"size:256;not null;default:''"`
	Status            string     `gorm:"size:16;not null;default:'active'"`
	LastUsedAt        *time.Time `gorm:"column:last_used_at"`
	LastHealthCheckAt *time.Time `gorm:"column:last_health_check_at"`
	HealthCheckResult string     `gorm:"size:16;not null;default:'unknown'"`
	DailyUsageCount   int        `gorm:"not null;default:0"`
	DailyUsageLimit   int        `gorm:"not null;default:50"`
	CreatedAt         time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP(3)"`
	UpdatedAt         time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP(3)"`
}

func (CrawlerAccountPO) TableName() string { return "crawler_accounts" }

func crawlerAccountToPO(a entity.CrawlerAccount) CrawlerAccountPO {
	return CrawlerAccountPO{
		ID:                a.ID,
		Platform:          a.Platform,
		AccountName:       a.AccountName,
		CookieEncrypted:   a.CookieEncrypted,
		UserAgent:         a.UserAgent,
		ProxyAddress:      a.ProxyAddress,
		Status:            a.Status,
		LastUsedAt:        a.LastUsedAt,
		LastHealthCheckAt: a.LastHealthCheckAt,
		HealthCheckResult: a.HealthCheckResult,
		DailyUsageCount:   a.DailyUsageCount,
		DailyUsageLimit:   a.DailyUsageLimit,
		CreatedAt:         a.CreatedAt,
		UpdatedAt:         a.UpdatedAt,
	}
}

func crawlerAccountFromPO(po CrawlerAccountPO) entity.CrawlerAccount {
	return entity.CrawlerAccount{
		ID:                po.ID,
		Platform:          po.Platform,
		AccountName:       po.AccountName,
		CookieEncrypted:   po.CookieEncrypted,
		UserAgent:         po.UserAgent,
		ProxyAddress:      po.ProxyAddress,
		Status:            po.Status,
		LastUsedAt:        po.LastUsedAt,
		LastHealthCheckAt: po.LastHealthCheckAt,
		HealthCheckResult: po.HealthCheckResult,
		DailyUsageCount:   po.DailyUsageCount,
		DailyUsageLimit:   po.DailyUsageLimit,
		CreatedAt:         po.CreatedAt,
		UpdatedAt:         po.UpdatedAt,
	}
}

// GormCrawlerAccountRepository 是 port.CrawlerAccountRepository 的 GORM 实现。
type GormCrawlerAccountRepository struct {
	db *gorm.DB
}

func NewGormCrawlerAccountRepository(db *gorm.DB) *GormCrawlerAccountRepository {
	return &GormCrawlerAccountRepository{db: db}
}

func (r *GormCrawlerAccountRepository) Save(ctx context.Context, a entity.CrawlerAccount) error {
	po := crawlerAccountToPO(a)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormCrawlerAccountRepository) FindByID(ctx context.Context, id int64) (entity.CrawlerAccount, error) {
	var po CrawlerAccountPO
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&po).Error; err != nil {
		return entity.CrawlerAccount{}, err
	}
	return crawlerAccountFromPO(po), nil
}

func (r *GormCrawlerAccountRepository) ListByPlatform(ctx context.Context, platform string) ([]entity.CrawlerAccount, error) {
	var pos []CrawlerAccountPO
	if err := r.db.WithContext(ctx).Where("platform = ?", platform).Order("id ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.CrawlerAccount, 0, len(pos))
	for _, po := range pos {
		out = append(out, crawlerAccountFromPO(po))
	}
	return out, nil
}

func (r *GormCrawlerAccountRepository) ListAll(ctx context.Context) ([]entity.CrawlerAccount, error) {
	var pos []CrawlerAccountPO
	if err := r.db.WithContext(ctx).Order("platform ASC, id ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.CrawlerAccount, 0, len(pos))
	for _, po := range pos {
		out = append(out, crawlerAccountFromPO(po))
	}
	return out, nil
}

func (r *GormCrawlerAccountRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&CrawlerAccountPO{}).Error
}

func (r *GormCrawlerAccountRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	return r.db.WithContext(ctx).Model(&CrawlerAccountPO{}).Where("id = ?", id).Update("status", status).Error
}

func (r *GormCrawlerAccountRepository) UpdateHealth(ctx context.Context, id int64, result string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&CrawlerAccountPO{}).Where("id = ?", id).
		Updates(map[string]any{
			"health_check_result":  result,
			"last_health_check_at": now,
		}).Error
}

func (r *GormCrawlerAccountRepository) IncrementUsage(ctx context.Context, id int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&CrawlerAccountPO{}).Where("id = ?", id).
		Updates(map[string]any{
			"daily_usage_count": gorm.Expr("daily_usage_count + 1"),
			"last_used_at":      now,
		}).Error
}

func (r *GormCrawlerAccountRepository) ResetDailyUsage(ctx context.Context) error {
	return r.db.WithContext(ctx).Model(&CrawlerAccountPO{}).
		Where("daily_usage_count > 0").
		Update("daily_usage_count", 0).Error
}

// SelectAvailable 选择一个可用账号（负载均衡：使用次数最少的）。
func (r *GormCrawlerAccountRepository) SelectAvailable(ctx context.Context, platform string) (*entity.CrawlerAccount, error) {
	var po CrawlerAccountPO
	err := r.db.WithContext(ctx).
		Where("platform = ? AND status = ? AND health_check_result != ? AND daily_usage_count < daily_usage_limit",
			platform, entity.CrawlerAccountActive, entity.HealthUnhealthy).
		Order("daily_usage_count ASC").
		First(&po).Error
	if err != nil {
		return nil, err
	}
	account := crawlerAccountFromPO(po)
	return &account, nil
}

// ---- 爬虫配置仓储 ----

// CrawlerConfigPO 爬虫配置持久化对象。
type CrawlerConfigPO struct {
	ID                   int64     `gorm:"primaryKey;autoIncrement"`
	Platform             string    `gorm:"size:32;not null"`
	TenantID             string    `gorm:"size:64;not null;default:''"`
	Enabled              bool      `gorm:"not null;default:1"`
	SearchKeywords       string    `gorm:"type:json"`
	ExtraKeywords        string    `gorm:"type:json"`
	CrawlIntervalMinutes int       `gorm:"not null;default:15"`
	MaxResults           int       `gorm:"not null;default:20"`
	SortBy               string    `gorm:"size:32;not null;default:'popular'"`
	PublishTime          string    `gorm:"size:16;not null;default:'week'"`
	EnableComments       bool      `gorm:"not null;default:0"`
	EnableRefresh        bool      `gorm:"not null;default:1"`
	RefreshIntervalHours int       `gorm:"not null;default:12"`
	RateLimitPerMin      int       `gorm:"not null;default:10"`
	ProxyEnabled         bool      `gorm:"not null;default:0"`
	MaxRetryCount        int       `gorm:"not null;default:3"`
	LastCrawledAt        *time.Time
	LastError            string    `gorm:"type:text"`
	CreatedAt            time.Time `gorm:"not null;default:CURRENT_TIMESTAMP(3)"`
	UpdatedAt            time.Time `gorm:"not null;default:CURRENT_TIMESTAMP(3)"`
}

func (CrawlerConfigPO) TableName() string { return "crawler_configs" }

func crawlerConfigToPO(c entity.CrawlerConfig) CrawlerConfigPO {
	keywordsJSON, _ := json.Marshal(c.SearchKeywords)
	extraJSON, _ := json.Marshal(c.ExtraKeywords)
	return CrawlerConfigPO{
		ID:                   c.ID,
		Platform:             c.Platform,
		TenantID:             c.TenantID,
		Enabled:              c.Enabled,
		SearchKeywords:       string(keywordsJSON),
		ExtraKeywords:        string(extraJSON),
		CrawlIntervalMinutes: c.CrawlIntervalMinutes,
		MaxResults:           c.MaxResults,
		SortBy:               c.SortBy,
		PublishTime:          c.PublishTime,
		EnableComments:       c.EnableComments,
		EnableRefresh:        c.EnableRefresh,
		RefreshIntervalHours: c.RefreshIntervalHours,
		RateLimitPerMin:      c.RateLimitPerMin,
		ProxyEnabled:         c.ProxyEnabled,
		MaxRetryCount:        c.MaxRetryCount,
		LastCrawledAt:        c.LastCrawledAt,
		LastError:            c.LastError,
		CreatedAt:            c.CreatedAt,
		UpdatedAt:            c.UpdatedAt,
	}
}

func crawlerConfigFromPO(po CrawlerConfigPO) entity.CrawlerConfig {
	var keywords []string
	var extra []string
	_ = json.Unmarshal([]byte(po.SearchKeywords), &keywords)
	_ = json.Unmarshal([]byte(po.ExtraKeywords), &extra)
	return entity.CrawlerConfig{
		ID:                   po.ID,
		Platform:             po.Platform,
		TenantID:             po.TenantID,
		Enabled:              po.Enabled,
		SearchKeywords:       keywords,
		ExtraKeywords:        extra,
		CrawlIntervalMinutes: po.CrawlIntervalMinutes,
		MaxResults:           po.MaxResults,
		SortBy:               po.SortBy,
		PublishTime:          po.PublishTime,
		EnableComments:       po.EnableComments,
		EnableRefresh:        po.EnableRefresh,
		RefreshIntervalHours: po.RefreshIntervalHours,
		RateLimitPerMin:      po.RateLimitPerMin,
		ProxyEnabled:         po.ProxyEnabled,
		MaxRetryCount:        po.MaxRetryCount,
		LastCrawledAt:        po.LastCrawledAt,
		LastError:            po.LastError,
		CreatedAt:            po.CreatedAt,
		UpdatedAt:            po.UpdatedAt,
	}
}

// GormCrawlerConfigRepository 是 port.CrawlerConfigRepository 的 GORM 实现。
type GormCrawlerConfigRepository struct {
	db *gorm.DB
}

func NewGormCrawlerConfigRepository(db *gorm.DB) *GormCrawlerConfigRepository {
	return &GormCrawlerConfigRepository{db: db}
}

func (r *GormCrawlerConfigRepository) Save(ctx context.Context, c entity.CrawlerConfig) error {
	po := crawlerConfigToPO(c)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormCrawlerConfigRepository) FindByPlatform(ctx context.Context, platform string) (entity.CrawlerConfig, error) {
	var po CrawlerConfigPO
	if err := r.db.WithContext(ctx).Where("platform = ?", platform).First(&po).Error; err != nil {
		return entity.CrawlerConfig{}, err
	}
	return crawlerConfigFromPO(po), nil
}

func (r *GormCrawlerConfigRepository) ListAll(ctx context.Context) ([]entity.CrawlerConfig, error) {
	var pos []CrawlerConfigPO
	if err := r.db.WithContext(ctx).Order("platform ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.CrawlerConfig, 0, len(pos))
	for _, po := range pos {
		out = append(out, crawlerConfigFromPO(po))
	}
	return out, nil
}

func (r *GormCrawlerConfigRepository) Delete(ctx context.Context, platform string) error {
	return r.db.WithContext(ctx).Where("platform = ?", platform).Delete(&CrawlerConfigPO{}).Error
}

func (r *GormCrawlerConfigRepository) UpdateLastCrawled(ctx context.Context, platform string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&CrawlerConfigPO{}).Where("platform = ?", platform).
		Update("last_crawled_at", now).Error
}

func (r *GormCrawlerConfigRepository) UpdateLastError(ctx context.Context, platform string, errMsg string) error {
	return r.db.WithContext(ctx).Model(&CrawlerConfigPO{}).Where("platform = ?", platform).
		Update("last_error", errMsg).Error
}

// ---- 采集任务日志仓储 ----

// CrawlerTaskLogPO 采集任务日志持久化对象。
type CrawlerTaskLogPO struct {
	ID            int64      `gorm:"primaryKey;autoIncrement"`
	TaskID        string     `gorm:"size:64;not null"`
	Platform      string     `gorm:"size:32;not null"`
	BrandID       string     `gorm:"size:64;not null;default:''"`
	TriggerType   string     `gorm:"size:16;not null;default:'scheduled'"`
	Status        string     `gorm:"size:16;not null;default:'running'"`
	KeywordsUsed  string     `gorm:"type:json"`
	VideosFound   int        `gorm:"not null;default:0"`
	VideosNew     int        `gorm:"not null;default:0"`
	VideosUpdated int        `gorm:"not null;default:0"`
	ErrorMessage  string     `gorm:"type:text"`
	StartedAt     time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP(3)"`
	FinishedAt    *time.Time
	DurationMs    int        `gorm:"not null;default:0"`
}

func (CrawlerTaskLogPO) TableName() string { return "crawler_task_logs" }

func crawlerTaskLogToPO(l entity.CrawlerTaskLog) CrawlerTaskLogPO {
	keywordsJSON, _ := json.Marshal(l.KeywordsUsed)
	return CrawlerTaskLogPO{
		ID:            l.ID,
		TaskID:        l.TaskID,
		Platform:      l.Platform,
		BrandID:       l.BrandID,
		TriggerType:   l.TriggerType,
		Status:        l.Status,
		KeywordsUsed:  string(keywordsJSON),
		VideosFound:   l.VideosFound,
		VideosNew:     l.VideosNew,
		VideosUpdated: l.VideosUpdated,
		ErrorMessage:  l.ErrorMessage,
		StartedAt:     l.StartedAt,
		FinishedAt:    l.FinishedAt,
		DurationMs:    l.DurationMs,
	}
}

func crawlerTaskLogFromPO(po CrawlerTaskLogPO) entity.CrawlerTaskLog {
	var keywords []string
	_ = json.Unmarshal([]byte(po.KeywordsUsed), &keywords)
	return entity.CrawlerTaskLog{
		ID:            po.ID,
		TaskID:        po.TaskID,
		Platform:      po.Platform,
		BrandID:       po.BrandID,
		TriggerType:   po.TriggerType,
		Status:        po.Status,
		KeywordsUsed:  keywords,
		VideosFound:   po.VideosFound,
		VideosNew:     po.VideosNew,
		VideosUpdated: po.VideosUpdated,
		ErrorMessage:  po.ErrorMessage,
		StartedAt:     po.StartedAt,
		FinishedAt:    po.FinishedAt,
		DurationMs:    po.DurationMs,
	}
}

// GormCrawlerTaskLogRepository 是 port.CrawlerTaskLogRepository 的 GORM 实现。
type GormCrawlerTaskLogRepository struct {
	db *gorm.DB
}

func NewGormCrawlerTaskLogRepository(db *gorm.DB) *GormCrawlerTaskLogRepository {
	return &GormCrawlerTaskLogRepository{db: db}
}

func (r *GormCrawlerTaskLogRepository) Save(ctx context.Context, l entity.CrawlerTaskLog) error {
	po := crawlerTaskLogToPO(l)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormCrawlerTaskLogRepository) FindByID(ctx context.Context, id int64) (entity.CrawlerTaskLog, error) {
	var po CrawlerTaskLogPO
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&po).Error; err != nil {
		return entity.CrawlerTaskLog{}, err
	}
	return crawlerTaskLogFromPO(po), nil
}

func (r *GormCrawlerTaskLogRepository) ListByPlatform(ctx context.Context, platform string, limit int) ([]entity.CrawlerTaskLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var pos []CrawlerTaskLogPO
	if err := r.db.WithContext(ctx).Where("platform = ?", platform).
		Order("started_at DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.CrawlerTaskLog, 0, len(pos))
	for _, po := range pos {
		out = append(out, crawlerTaskLogFromPO(po))
	}
	return out, nil
}

func (r *GormCrawlerTaskLogRepository) ListAll(ctx context.Context, limit int) ([]entity.CrawlerTaskLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var pos []CrawlerTaskLogPO
	if err := r.db.WithContext(ctx).Order("started_at DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.CrawlerTaskLog, 0, len(pos))
	for _, po := range pos {
		out = append(out, crawlerTaskLogFromPO(po))
	}
	return out, nil
}

func (r *GormCrawlerTaskLogRepository) UpdateStatus(ctx context.Context, id int64, status string, errMsg string) error {
	now := time.Now()
	updates := map[string]any{
		"status":     status,
		"finished_at": now,
	}
	if errMsg != "" {
		updates["error_message"] = errMsg
	}
	return r.db.WithContext(ctx).Model(&CrawlerTaskLogPO{}).Where("id = ?", id).Updates(updates).Error
}

func (r *GormCrawlerTaskLogRepository) UpdateResult(ctx context.Context, id int64, found, new, updated int) error {
	return r.db.WithContext(ctx).Model(&CrawlerTaskLogPO{}).Where("id = ?", id).
		Updates(map[string]any{
			"videos_found":   found,
			"videos_new":     new,
			"videos_updated": updated,
		}).Error
}

// ---- 灵感视频仓储 ----

// InspirationVideoPO 灵感视频持久化对象。
type InspirationVideoPO struct {
	ID              string     `gorm:"primaryKey;size:64"`
	Platform        string     `gorm:"size:16;not null;default:'douyin'"`
	PlatformVideoID string     `gorm:"size:128;not null;default:''"`
	Title           string     `gorm:"size:512;not null;default:''"`
	Description     string     `gorm:"type:text"`
	CoverURL        string     `gorm:"size:1024;not null;default:''"`
	VideoURL        string     `gorm:"size:1024;not null;default:''"`
	Author          string     `gorm:"size:128;not null;default:''"`
	AuthorAvatar    string     `gorm:"size:1024;not null;default:''"`
	Duration        int        `gorm:"not null;default:0"`
	PublishTime     time.Time
	PlayCount       int64      `gorm:"not null;default:0"`
	DiggCount       int64      `gorm:"not null;default:0"`
	CommentCount    int64      `gorm:"not null;default:0"`
	ShareCount      int64      `gorm:"not null;default:0"`
	CollectCount    int64      `gorm:"not null;default:0"`
	Topics          string     `gorm:"type:json"`
	MusicName       string     `gorm:"size:256;not null;default:''"`
	MusicAuthor     string     `gorm:"size:256;not null;default:''"`
	Sentiment       string     `gorm:"size:16;not null;default:'neutral'"`
	ViralScore      float64    `gorm:"not null;default:0"`
	IsPinned        bool       `gorm:"not null;default:0"`
	IsRecommended   bool       `gorm:"not null;default:0"`
	AdminNote       string     `gorm:"size:512;not null;default:''"`
	CreatedAt       time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP(3)"`
	UpdatedAt       time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP(3)"`
	LastRefreshedAt *time.Time
}

func (InspirationVideoPO) TableName() string { return "inspiration_videos" }

func inspirationVideoToPO(v entity.InspirationVideo) InspirationVideoPO {
	topicsJSON, _ := json.Marshal(v.Topics)
	return InspirationVideoPO{
		ID:              v.ID,
		Platform:        v.Platform,
		PlatformVideoID: v.PlatformVideoID,
		Title:           v.Title,
		Description:     v.Description,
		CoverURL:        v.CoverURL,
		VideoURL:        v.VideoURL,
		Author:          v.Author,
		AuthorAvatar:    v.AuthorAvatar,
		Duration:        v.Duration,
		PublishTime:     v.PublishTime,
		PlayCount:       v.PlayCount,
		DiggCount:       v.DiggCount,
		CommentCount:    v.CommentCount,
		ShareCount:      v.ShareCount,
		CollectCount:    v.CollectCount,
		Topics:          string(topicsJSON),
		MusicName:       v.MusicName,
		MusicAuthor:     v.MusicAuthor,
		Sentiment:       v.Sentiment,
		ViralScore:      v.ViralScore,
		IsPinned:        v.IsPinned,
		IsRecommended:   v.IsRecommended,
		AdminNote:       v.AdminNote,
		CreatedAt:       v.CreatedAt,
		UpdatedAt:       v.UpdatedAt,
		LastRefreshedAt: v.LastRefreshedAt,
	}
}

func inspirationVideoFromPO(po InspirationVideoPO) entity.InspirationVideo {
	var topics []string
	_ = json.Unmarshal([]byte(po.Topics), &topics)
	return entity.InspirationVideo{
		ID:              po.ID,
		Platform:        po.Platform,
		PlatformVideoID: po.PlatformVideoID,
		Title:           po.Title,
		Description:     po.Description,
		CoverURL:        po.CoverURL,
		VideoURL:        po.VideoURL,
		Author:          po.Author,
		AuthorAvatar:    po.AuthorAvatar,
		Duration:        po.Duration,
		PublishTime:     po.PublishTime,
		PlayCount:       po.PlayCount,
		DiggCount:       po.DiggCount,
		CommentCount:    po.CommentCount,
		ShareCount:      po.ShareCount,
		CollectCount:    po.CollectCount,
		Topics:          topics,
		MusicName:       po.MusicName,
		MusicAuthor:     po.MusicAuthor,
		Sentiment:       po.Sentiment,
		ViralScore:      po.ViralScore,
		IsPinned:        po.IsPinned,
		IsRecommended:   po.IsRecommended,
		AdminNote:       po.AdminNote,
		CreatedAt:       po.CreatedAt,
		UpdatedAt:       po.UpdatedAt,
		LastRefreshedAt: po.LastRefreshedAt,
	}
}

// GormInspirationVideoRepository 是 port.InspirationVideoRepository 的 GORM 实现。
type GormInspirationVideoRepository struct {
	db *gorm.DB
}

func NewGormInspirationVideoRepository(db *gorm.DB) *GormInspirationVideoRepository {
	return &GormInspirationVideoRepository{db: db}
}

// SaveBatch 批量保存视频（去重：按 platform + platform_video_id）。
func (r *GormInspirationVideoRepository) SaveBatch(ctx context.Context, videos []entity.InspirationVideo) (int, error) {
	newCount := 0
	for _, v := range videos {
		po := inspirationVideoToPO(v)
		// 使用 ON DUPLICATE KEY UPDATE 实现幂等
		result := r.db.WithContext(ctx).
			Where("platform = ? AND platform_video_id = ?", v.Platform, v.PlatformVideoID).
			Assign(map[string]any{
				"title":            po.Title,
				"description":      po.Description,
				"cover_url":        po.CoverURL,
				"video_url":        po.VideoURL,
				"author":           po.Author,
				"play_count":       po.PlayCount,
				"digg_count":       po.DiggCount,
				"comment_count":    po.CommentCount,
				"share_count":      po.ShareCount,
				"collect_count":    po.CollectCount,
				"viral_score":      po.ViralScore,
				"topics":           po.Topics,
				"last_refreshed_at": time.Now(),
			}).
			FirstOrCreate(&po)
		if result.Error != nil {
			return newCount, result.Error
		}
		if result.RowsAffected > 0 {
			newCount++
		}
	}
	return newCount, nil
}

// List 查询灵感视频列表（支持分页、排序、筛选）。
func (r *GormInspirationVideoRepository) List(ctx context.Context, brandID, platform, keyword, sortBy string, page, pageSize int) ([]entity.InspirationVideo, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	q := r.db.WithContext(ctx).Model(&InspirationVideoPO{})

	// 品牌筛选：通过 brand_inspirations 关联
	if brandID != "" {
		q = q.Joins("JOIN brand_inspirations bi ON bi.video_id = inspiration_videos.id").
			Where("bi.brand_id = ?", brandID)
	}
	if platform != "" {
		q = q.Where("platform = ?", platform)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("(title LIKE ? OR author LIKE ?)", like, like)
	}

	// 排序
	switch sortBy {
	case "play_count":
		q = q.Order("play_count DESC")
	case "digg_count":
		q = q.Order("digg_count DESC")
	case "publish_time":
		q = q.Order("publish_time DESC")
	default:
		q = q.Order("viral_score DESC")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var pos []InspirationVideoPO
	if err := q.Offset(offset).Limit(pageSize).Find(&pos).Error; err != nil {
		return nil, 0, err
	}

	out := make([]entity.InspirationVideo, 0, len(pos))
	for _, po := range pos {
		out = append(out, inspirationVideoFromPO(po))
	}
	return out, int(total), nil
}

func (r *GormInspirationVideoRepository) FindByID(ctx context.Context, id string) (entity.InspirationVideo, error) {
	var po InspirationVideoPO
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&po).Error; err != nil {
		return entity.InspirationVideo{}, err
	}
	return inspirationVideoFromPO(po), nil
}

func (r *GormInspirationVideoRepository) UpdateMetrics(ctx context.Context, videoID string, metrics entity.MetricsUpdate) error {
	return r.db.WithContext(ctx).Model(&InspirationVideoPO{}).Where("id = ?", videoID).
		Updates(map[string]any{
			"play_count":        metrics.PlayCount,
			"digg_count":        metrics.DiggCount,
			"comment_count":     metrics.CommentCount,
			"share_count":       metrics.ShareCount,
			"collect_count":     metrics.CollectCount,
			"last_refreshed_at": time.Now(),
		}).Error
}

func (r *GormInspirationVideoRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&InspirationVideoPO{}).Error
}

func (r *GormInspirationVideoRepository) CountByPlatform(ctx context.Context) ([]port.PlatformCount, error) {
	var results []struct {
		Platform string
		Count    int
	}
	if err := r.db.WithContext(ctx).Model(&InspirationVideoPO{}).
		Select("platform, COUNT(*) as count").
		Group("platform").Find(&results).Error; err != nil {
		return nil, err
	}
	out := make([]port.PlatformCount, 0, len(results))
	for _, r := range results {
		out = append(out, port.PlatformCount{Platform: r.Platform, Count: r.Count})
	}
	return out, nil
}

func (r *GormInspirationVideoRepository) CountByBrand(ctx context.Context) ([]port.BrandCount, error) {
	var results []struct {
		BrandID   string
		BrandName string
		Count     int
		AvgScore  float64
	}
	if err := r.db.WithContext(ctx).Table("brand_inspirations bi").
		Select("bi.brand_id, '' as brand_name, COUNT(*) as count, AVG(iv.viral_score) as avg_score").
		Joins("JOIN inspiration_videos iv ON iv.id = bi.video_id").
		Group("bi.brand_id").Find(&results).Error; err != nil {
		return nil, err
	}
	out := make([]port.BrandCount, 0, len(results))
	for _, r := range results {
		out = append(out, port.BrandCount{
			BrandID:   r.BrandID,
			BrandName: r.BrandName,
			Count:     r.Count,
			AvgScore:  r.AvgScore,
		})
	}
	return out, nil
}

// ---- 品牌-视频关联仓储 ----

// BrandInspirationPO 品牌-视频关联持久化对象。
type BrandInspirationPO struct {
	ID            int64     `gorm:"primaryKey;autoIncrement"`
	BrandID       string    `gorm:"size:64;not null"`
	VideoID       string    `gorm:"size:64;not null"`
	SearchKeyword string    `gorm:"size:256;not null;default:''"`
	RelevanceScore float64  `gorm:"not null;default:0"`
	CreatedAt     time.Time `gorm:"not null;default:CURRENT_TIMESTAMP(3)"`
}

func (BrandInspirationPO) TableName() string { return "brand_inspirations" }

// GormBrandInspirationRepository 是 port.BrandInspirationRepository 的 GORM 实现。
type GormBrandInspirationRepository struct {
	db *gorm.DB
}

func NewGormBrandInspirationRepository(db *gorm.DB) *GormBrandInspirationRepository {
	return &GormBrandInspirationRepository{db: db}
}

func (r *GormBrandInspirationRepository) Link(ctx context.Context, brandID, videoID, keyword string) error {
	po := BrandInspirationPO{
		BrandID:       brandID,
		VideoID:       videoID,
		SearchKeyword: keyword,
	}
	return r.db.WithContext(ctx).
		Where("brand_id = ? AND video_id = ?", brandID, videoID).
		FirstOrCreate(&po).Error
}

func (r *GormBrandInspirationRepository) Unlink(ctx context.Context, brandID, videoID string) error {
	return r.db.WithContext(ctx).
		Where("brand_id = ? AND video_id = ?", brandID, videoID).
		Delete(&BrandInspirationPO{}).Error
}

func (r *GormBrandInspirationRepository) ListByBrand(ctx context.Context, brandID string) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).Model(&BrandInspirationPO{}).
		Where("brand_id = ?", brandID).
		Pluck("video_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *GormBrandInspirationRepository) ListByVideo(ctx context.Context, videoID string) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).Model(&BrandInspirationPO{}).
		Where("video_id = ?", videoID).
		Pluck("brand_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
