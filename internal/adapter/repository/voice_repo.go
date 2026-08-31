package repository

import (
	"context"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
)

// GenerationVoicePO 官方音色表映射（seed 自 Vidu 音色表——静态参考数据）。
type GenerationVoicePO struct {
	VoiceID      string `gorm:"primaryKey;size:128"`
	Language     string `gorm:"size:64;index"`
	Name         string `gorm:"size:128"`
	SampleURL    string `gorm:"size:512"`
	Recommend    bool   `gorm:"not null;default:0"`
	Scope        string `gorm:"size:16;not null;default:vidu"`   // vidu(官方seed) / platform(官方复刻) / clone(用户克隆)
	TenantID     string `gorm:"size:64;not null;default:''"`     // clone行归属；官方行空
	SourceTaskID string `gorm:"size:64;not null;default:''"`     // 溯源任务ID
	Status       string `gorm:"size:16;not null;default:active"` // active / disabled
}

func (GenerationVoicePO) TableName() string { return "generation_voices" }

// GormVoiceRepository 是 port.VoiceLibrary 的 GORM 实现。
type GormVoiceRepository struct {
	db *gorm.DB
}

func NewGormVoiceRepository(db *gorm.DB) *GormVoiceRepository {
	return &GormVoiceRepository{db: db}
}

func (r *GormVoiceRepository) List(ctx context.Context, language, keyword, tenantID string) ([]entity.GenerationVoice, error) {
	q := r.db.WithContext(ctx).Model(&GenerationVoicePO{})
	if language != "" {
		q = q.Where("language = ?", language)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("voice_id LIKE ? OR name LIKE ?", like, like)
	}
	if tenantID != "" {
		// 租户隔离（04 号 §10.2#3）：克隆音色仅本租户可见；只展示启用行
		q = q.Where("(scope IS NULL OR scope <> 'clone' OR tenant_id = ?) AND status = 'active'", tenantID)
	}
	var pos []GenerationVoicePO
	// 全量 300 条级——一次取回由前端分组/搜索，不分页
	if err := q.Order("language, voice_id").Limit(1000).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.GenerationVoice, 0, len(pos))
	for _, p := range pos {
		out = append(out, entity.GenerationVoice{
			VoiceID: p.VoiceID, Language: p.Language, Name: p.Name, SampleURL: p.SampleURL,
			Recommend: p.Recommend, Scope: p.Scope, TenantID: p.TenantID,
			SourceTaskID: p.SourceTaskID, Status: p.Status,
		})
	}
	return out, nil
}

// Upsert 按 voice_id 主键幂等写入（26号计划——voice_clone 物化钩子调用）。
func (r *GormVoiceRepository) Upsert(ctx context.Context, voice entity.GenerationVoice) error {
	po := GenerationVoicePO{
		VoiceID: voice.VoiceID, Language: voice.Language, Name: voice.Name,
		SampleURL: voice.SampleURL, Recommend: voice.Recommend,
		Scope: voice.Scope, TenantID: voice.TenantID,
		SourceTaskID: voice.SourceTaskID, Status: voice.Status,
	}
	return r.db.WithContext(ctx).Save(&po).Error
}

// SeedIfEmpty 表空时批量写入（voice_id 主键幂等——Save 覆盖同键行）。
func (r *GormVoiceRepository) SeedIfEmpty(ctx context.Context, voices []entity.GenerationVoice) (int, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&GenerationVoicePO{}).Count(&n).Error; err != nil {
		return 0, err
	}
	if n > 0 || len(voices) == 0 {
		return 0, nil
	}
	pos := make([]GenerationVoicePO, 0, len(voices))
	for _, v := range voices {
		pos = append(pos, GenerationVoicePO{VoiceID: v.VoiceID, Language: v.Language, Name: v.Name, SampleURL: v.SampleURL, Recommend: v.Recommend})
	}
	// 分批插入（单条 SQL 上千占位符在部分 MySQL 配置下超限）
	const batchSize = 100
	for i := 0; i < len(pos); i += batchSize {
		end := min(i+batchSize, len(pos))
		if err := r.db.WithContext(ctx).Create(pos[i:end]).Error; err != nil {
			return 0, err
		}
	}
	return len(pos), nil
}
