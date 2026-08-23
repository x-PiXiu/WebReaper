package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"webreaper/internal/domain/entity"
)

type HotVideoPO struct {
	ID          int64      `gorm:"primaryKey;autoIncrement"`
	TenantID    string     `gorm:"column:tenant_id;size:64;not null;default:''"`
	BrandID     string     `gorm:"column:brand_id;size:64;not null;default:''"`
	Title       string     `gorm:"size:512"`
	URL         string     `gorm:"size:1024"`
	Platform    string     `gorm:"size:32"`
	HotPoint    string     `gorm:"column:hot_point;size:1024"`
	Topic       string     `gorm:"size:512"`
	CoverURL    string     `gorm:"column:cover_url;size:1024"`
	Author      string     `gorm:"size:128"`
	PlayCount   int64      `gorm:"column:play_count"`
	DiggCount   int64      `gorm:"column:digg_count"`
	CommentCount int64     `gorm:"column:comment_count"`
	PublishTime *time.Time `gorm:"column:publish_time"`
	Source      string     `gorm:"size:32;default:'search'"`
	CreatedAt   time.Time
}

func (HotVideoPO) TableName() string { return "hot_videos" }

type GormHotVideoRepository struct{ db *gorm.DB }

func NewGormHotVideoRepository(db *gorm.DB) *GormHotVideoRepository {
	return &GormHotVideoRepository{db: db}
}

func (r *GormHotVideoRepository) SaveBatch(ctx context.Context, videos []entity.HotVideo) (int, error) {
	if len(videos) == 0 {
		return 0, nil
	}
	pos := make([]HotVideoPO, 0, len(videos))
	for _, v := range videos {
		pos = append(pos, hotVideoToPO(v))
	}
	// 批量 upsert（brand_id + url 去重——同品牌同视频不重复入库）
	const batchSize = 50
	saved := 0
	for i := 0; i < len(pos); i += batchSize {
		end := i + batchSize
		if end > len(pos) {
			end = len(pos)
		}
		res := r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "brand_id"}, {Name: "url"}},
			DoNothing: true,
		}).Create(pos[i:end])
		if res.Error != nil {
			return saved, res.Error
		}
		saved += int(res.RowsAffected)
	}
	return saved, nil
}

// List 按品牌列出热门视频（支持平台/关键词筛选 + 排序）。
func (r *GormHotVideoRepository) List(ctx context.Context, brandID string, opts entity.HotVideoListOptions) ([]entity.HotVideo, int, error) {
	q := r.db.WithContext(ctx).Where("brand_id = ?", brandID)
	if opts.Platform != "" {
		q = q.Where("platform = ?", opts.Platform)
	}
	if opts.Keyword != "" {
		like := "%" + opts.Keyword + "%"
		q = q.Where("(title LIKE ? OR hot_point LIKE ? OR topic LIKE ? OR author LIKE ?)", like, like, like, like)
	}
	// 总数（筛选后）
	var total int64
	q.Model(&HotVideoPO{}).Count(&total)

	// 排序
	order := "created_at DESC"
	switch opts.SortBy {
	case "publish_time":
		order = "publish_time DESC"
	case "digg_count":
		order = "digg_count DESC"
	case "play_count":
		order = "play_count DESC"
	case "comment_count":
		order = "comment_count DESC"
	}
	q = q.Order(order)

	// 分页
	limit := opts.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	q = q.Limit(limit).Offset(offset)

	var pos []HotVideoPO
	if err := q.Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]entity.HotVideo, 0, len(pos))
	for _, p := range pos {
		out = append(out, hotVideoFromPO(p))
	}
	return out, int(total), nil
}

func hotVideoToPO(v entity.HotVideo) HotVideoPO {
	var pubTime *time.Time
	if v.PublishTime != "" {
		if t, err := time.Parse(time.RFC3339, v.PublishTime); err == nil {
			pubTime = &t
		}
	}
	return HotVideoPO{
		TenantID: v.TenantID, BrandID: v.BrandID,
		Title: v.Title, URL: v.URL, Platform: v.Platform,
		HotPoint: v.HotPoint, Topic: v.Topic, CoverURL: v.CoverURL,
		Author: v.Author, PlayCount: v.PlayCount, DiggCount: v.DiggCount,
		CommentCount: v.CommentCount, PublishTime: pubTime, Source: v.Source,
	}
}

func hotVideoFromPO(p HotVideoPO) entity.HotVideo {
	pubTime := ""
	if p.PublishTime != nil {
		pubTime = p.PublishTime.Format(time.RFC3339)
	}
	return entity.HotVideo{
		TenantID: p.TenantID, BrandID: p.BrandID,
		Title: p.Title, URL: p.URL, Platform: p.Platform,
		HotPoint: p.HotPoint, Topic: p.Topic, CoverURL: p.CoverURL,
		Author: p.Author, PlayCount: p.PlayCount, DiggCount: p.DiggCount,
		CommentCount: p.CommentCount, PublishTime: pubTime, Source: p.Source,
		CreatedAt: p.CreatedAt.Format(time.RFC3339),
	}
}

var _ = (*GormHotVideoRepository)(nil)
