package repository

import "time"

// VideoTaskPO 视频生成任务。
type VideoTaskPO struct {
	ID          string    `gorm:"primaryKey;size:64"`
	TenantID    string    `gorm:"size:64;index"`
	BrandID     string    `gorm:"size:64"`
	Mode        string    `gorm:"size:16"` // text / material
	Prompt      string    `gorm:"type:text"`
	MaterialURL string    `gorm:"type:text"`
	Status      string    `gorm:"size:16;index"`
	VideoURL    string    `gorm:"type:text"` // ① 生成结果视频
	VoiceText   string    `gorm:"type:text"` // ② 配音文本
	VoiceURL    string    `gorm:"type:text"` // ② 配音音频
	FinalURL    string    `gorm:"type:text"` // ③ 合成成片
	DurationSec int
	Error       string    `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (VideoTaskPO) TableName() string { return "video_tasks" }

// VideoJobPO 视频发布任务。
type VideoJobPO struct {
	ID          string    `gorm:"primaryKey;size:64"`
	TenantID    string    `gorm:"size:64;index"`
	TaskID      string    `gorm:"size:64;index"`
	AccountID   string    `gorm:"size:64"`
	Platform    string    `gorm:"size:32"`
	Status      string    `gorm:"size:16"`
	ExternalURL string    `gorm:"type:text"`
	Error       string    `gorm:"type:text"`
	CreatedAt   time.Time
}

func (VideoJobPO) TableName() string { return "video_jobs" }
