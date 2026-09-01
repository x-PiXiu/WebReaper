package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
)

// GenerationVoicePO 官方音色表映射（seed 自 Vidu 音色表——静态参考数据）。
// 白牌化（2026-09-01）：scope=vidu 仅管理端可见（克隆参考源），不暴露给用户。
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
	IsDefault    bool   `gorm:"not null;default:0"`              // 平台默认音色（scope=platform 内仅一条）
	ViduRegisteredAt *time.Time `gorm:"column:vidu_registered_at;index"` // Vidu 注册/续期时间（31号 L1；NULL=未注册）
}

func (GenerationVoicePO) TableName() string { return "generation_voices" }

// GormVoiceRepository 是 port.VoiceLibrary 的 GORM 实现。
type GormVoiceRepository struct {
	db *gorm.DB
}

func NewGormVoiceRepository(db *gorm.DB) *GormVoiceRepository {
	return &GormVoiceRepository{db: db}
}

func poToVoice(p GenerationVoicePO) entity.GenerationVoice {
	return entity.GenerationVoice{
		VoiceID: p.VoiceID, Language: p.Language, Name: p.Name, SampleURL: p.SampleURL,
		Recommend: p.Recommend, Scope: p.Scope, TenantID: p.TenantID,
		SourceTaskID: p.SourceTaskID, Status: p.Status, IsDefault: p.IsDefault,
		ViduRegisteredAt: p.ViduRegisteredAt,
	}
}

// ListForUser 用户端音色列表（白牌化）：scope=platform（active）+ 本租户 clone（active）。
// Vidu scope 全隐藏——上游内容不暴露给用户。
func (r *GormVoiceRepository) ListForUser(ctx context.Context, tenantID string) ([]entity.GenerationVoice, error) {
	q := r.db.WithContext(ctx).Model(&GenerationVoicePO{}).
		Where("status = 'active' AND (scope = 'platform' OR (scope = 'clone' AND tenant_id = ?))", tenantID).
		Order("is_default DESC, recommend DESC, language, voice_id").
		Limit(200)
	var pos []GenerationVoicePO
	if err := q.Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.GenerationVoice, 0, len(pos))
	for _, p := range pos {
		out = append(out, poToVoice(p))
	}
	return out, nil
}

// ListForAdmin 管理端全量音色（含 vidu 参考源 / platform / clone；含停用行）。
func (r *GormVoiceRepository) ListForAdmin(ctx context.Context, scope string) ([]entity.GenerationVoice, error) {
	q := r.db.WithContext(ctx).Model(&GenerationVoicePO{})
	if scope != "" {
		q = q.Where("scope = ?", scope)
	}
	var pos []GenerationVoicePO
	if err := q.Order("is_default DESC, recommend DESC, language, voice_id").Limit(1000).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.GenerationVoice, 0, len(pos))
	for _, p := range pos {
		out = append(out, poToVoice(p))
	}
	return out, nil
}

// GetDefault 获取平台默认音色（scope=platform 且 is_default=true 的首条）。
func (r *GormVoiceRepository) GetDefault(ctx context.Context) (entity.GenerationVoice, error) {
	var po GenerationVoicePO
	err := r.db.WithContext(ctx).Where("scope = 'platform' AND is_default = true AND status = 'active'").First(&po).Error
	return poToVoice(po), err
}

// SetDefault 设为平台默认音色（先清 platform scope 内所有 default，再置目标行）。
func (r *GormVoiceRepository) SetDefault(ctx context.Context, voiceID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&GenerationVoicePO{}).Where("scope = 'platform'").Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&GenerationVoicePO{}).Where("voice_id = ? AND scope = 'platform'", voiceID).Update("is_default", true).Error
	})
}

// FindByVoiceID 按音色 ID 精确查询单条（缺口C：样本合成通道定位样本音频）。
func (r *GormVoiceRepository) FindByVoiceID(ctx context.Context, voiceID string) (entity.GenerationVoice, error) {
	var po GenerationVoicePO
	if err := r.db.WithContext(ctx).Where("voice_id = ?", voiceID).First(&po).Error; err != nil {
		return entity.GenerationVoice{}, err
	}
	return poToVoice(po), nil
}

// UpdateViduRegisteredAt 记录/清除 Vidu 侧注册时间（31号 L2——窗口判定与缓存失效）。
func (r *GormVoiceRepository) UpdateViduRegisteredAt(ctx context.Context, voiceID string, t *time.Time) error {
	return r.db.WithContext(ctx).Model(&GenerationVoicePO{}).
		Where("voice_id = ?", voiceID).
		Update("vidu_registered_at", t).Error
}

// DeleteClone 删除克隆音色行（31号 U4：删除任务联动清理；scope+租户双条件防御）。
func (r *GormVoiceRepository) DeleteClone(ctx context.Context, tenantID, voiceID string) error {
	return r.db.WithContext(ctx).
		Where("voice_id = ? AND scope = ? AND tenant_id = ?", voiceID, "clone", tenantID).
		Delete(&GenerationVoicePO{}).Error
}

// Upsert 按 voice_id 主键幂等写入（26号计划——voice_clone 物化钩子调用）。
func (r *GormVoiceRepository) Upsert(ctx context.Context, voice entity.GenerationVoice) error {
	po := GenerationVoicePO{
		VoiceID: voice.VoiceID, Language: voice.Language, Name: voice.Name,
		SampleURL: voice.SampleURL, Recommend: voice.Recommend,
		Scope: voice.Scope, TenantID: voice.TenantID,
		SourceTaskID: voice.SourceTaskID, Status: voice.Status, IsDefault: voice.IsDefault,
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
