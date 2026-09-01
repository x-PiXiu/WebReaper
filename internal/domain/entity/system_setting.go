package entity

import "time"

// SystemSetting 系统级配置项（key-value 形式，全局单例语义）。
//
// 设计动机（整洁架构）：
//   - 爬虫速率、robots 开关等运行时可调的配置，存数据库而非 .env，
//     让 UI 能动态修改、无需重启。
//   - 用通用 key-value 表承载，避免每加一个配置就建一张表。
//   - CrawlPolicy 等结构化配置序列化为 JSON 存 Value 字段。
type SystemSetting struct {
	Key       string    // 配置键，如 "crawl_policy"
	Value     string    // 配置值（结构化配置存 JSON）
	UpdatedAt time.Time
}

// 配置键常量
const (
	SettingKeyCrawlPolicy    = "crawl_policy"        // 爬虫限流策略（JSON 序列化的 CrawlPolicy）
	SettingKeyKnowledgeCrawl = "kb_crawl_industries" // 行业采集配置（JSON：[]IndustryCrawlConfig）

	// ---- 生成域默认值（27 号硬编码治理——管理后台可调，免重启）----

	SettingKeyGenDefaultVoiceID     = "gen_default_voice_id"      // 默认音色 ID（如 "female-shaonv"）
	SettingKeyGenDefaultResolution  = "gen_default_resolution"    // 默认分辨率（如 "1080p"）
	SettingKeyGenDefaultAspectRatio = "gen_default_aspect_ratio"  // 默认画面比例（如 "16:9"）
	SettingKeyGenDefaultDuration    = "gen_default_duration"      // 默认时长（秒，如 5）
	SettingKeyGenDefaultAvatarPrompt = "gen_default_avatar_prompt" // 默认形象视频 prompt（28号计划）

	// ---- 31号：音色物化架构（样本为源、注册为缓存）----

	SettingKeyGenViduVoiceWindow    = "gen_vidu_voice_window"     // Vidu 注册缓存窗口（小时；默认144=6天，上游7天过期留1天余量）
	SettingKeyGenVoiceMaterializer  = "gen_voice_materializer_enabled" // 音色物化层总开关（false=回退旧行为直传 ID；灰度用）
	SettingKeyGenVoiceAuditionText  = "gen_voice_audition_text"   // 注册/续期时的固定试听文案（Vidu audio-clone text 必填 ≤1000）
	SettingKeyGenLipsyncTwoStep     = "gen_lipsync_two_step"      // 备胎道开关（lip_sync 注册失败自动降级：tts样本合成→音频驱动；零厂商ID依赖）
)
