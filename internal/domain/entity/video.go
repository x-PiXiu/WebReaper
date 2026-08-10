package entity

import "time"

// ---- 视频生成域（Vidu 等视频模型 + 配音 + 合成 + 视频平台发布）----
//
// 设计动机（整洁架构）：
//   - 视频生成是"内容运营"的延伸：素材/文本 → 视频 → 配音 → 合成 → 发布视频平台。
//   - 纯 struct + 状态机规则，零框架依赖；状态机逻辑放实体层便于单测。
//   - 视频模型会变（Vidu/Sora/可灵...）——适配器层隔离，业务不感知。

// VideoTaskStatus 视频生成任务状态机。
type VideoTaskStatus string

const (
	VideoStatusPending    VideoTaskStatus = "pending"     // 已提交，待开始
	VideoStatusGenerating VideoTaskStatus = "generating"  // 视频生成中（异步轮询）
	VideoStatusDubbing    VideoTaskStatus = "dubbing"     // 配音中
	VideoStatusComposing  VideoTaskStatus = "composing"   // 合成中（ffmpeg）
	VideoStatusReady      VideoTaskStatus = "ready"       // 成片就绪（可发布）
	VideoStatusFailed     VideoTaskStatus = "failed"      // 失败（带 Error，可重试）
)

// VideoTask 视频生成任务（聚合根）。
// Mode 区分输入方式：text（随机文本生成）/ material（上传素材）。
type VideoTask struct {
	ID          string          `json:"id"`
	TenantID    string          `json:"tenant_id"`
	BrandID     string          `json:"brand_id"` // 可选：与品牌关联
	Mode        string          `json:"mode"`     // text / material
	Prompt      string          `json:"prompt"`   // 生成提示词（text 模式；material 模式可为描述文案）
	MaterialURL string          `json:"material_url"` // 素材地址（material 模式）
	Status      VideoTaskStatus `json:"status"`
	VideoURL    string          `json:"video_url"` // ① 生成结果视频
	VoiceText   string          `json:"voice_text"` // ② 配音文本
	VoiceURL    string          `json:"voice_url"` // ② 配音音频
	FinalURL    string          `json:"final_url"` // ③ 合成成片
	DurationSec int             `json:"duration_sec"`
	Error       string          `json:"error"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// IsValid 领域规则：任务必须有 ID、TenantID、模式与输入。
func (t VideoTask) IsValid() bool {
	if t.ID == "" || t.TenantID == "" || t.Mode == "" {
		return false
	}
	if t.Mode == "text" && t.Prompt == "" {
		return false
	}
	if t.Mode == "material" && t.MaterialURL == "" {
		return false
	}
	return true
}

// CanTransitionTo 状态机规则：只允许合法流转（实体层行为内聚）。
func (t VideoTask) CanTransitionTo(next VideoTaskStatus) bool {
	switch t.Status {
	case VideoStatusPending:
		return next == VideoStatusGenerating || next == VideoStatusFailed
	case VideoStatusGenerating:
		return next == VideoStatusDubbing || next == VideoStatusComposing || next == VideoStatusFailed
	case VideoStatusDubbing:
		return next == VideoStatusComposing || next == VideoStatusFailed
	case VideoStatusComposing:
		return next == VideoStatusReady || next == VideoStatusFailed
	case VideoStatusReady:
		return false // 就绪后不可再流转（发布是另一个域：VideoJob）
	case VideoStatusFailed:
		return next == VideoStatusPending // 失败可重试（重新排队）
	}
	return false
}

// StatusLabel 状态的中文标签（前端展示）。
func (t VideoTask) StatusLabel() string {
	switch t.Status {
	case VideoStatusPending:
		return "排队中"
	case VideoStatusGenerating:
		return "视频生成中"
	case VideoStatusDubbing:
		return "配音中"
	case VideoStatusComposing:
		return "合成中"
	case VideoStatusReady:
		return "成片就绪"
	case VideoStatusFailed:
		return "失败"
	}
	return string(t.Status)
}

// VideoJob 视频发布任务（抖音/快手等视频平台）。
type VideoJob struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	TaskID      string    `json:"task_id"` // 关联 VideoTask
	AccountID   string    `json:"account_id"` // 发布账号（空 = 账号池随机）
	Platform    string    `json:"platform"` // douyin / kuaishou ...
	Status      string    `json:"status"` // pending / publishing / published / failed
	ExternalURL string    `json:"external_url"`
	Error       string    `json:"error"`
	CreatedAt   time.Time `json:"created_at"`
}
