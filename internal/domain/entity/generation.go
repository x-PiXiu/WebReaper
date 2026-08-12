package entity

import (
	"time"
)

// 生成类型（GenerationTask.Type）。
const (
	GenerationTypeVideo        = "video"
	GenerationTypeImage        = "image"
	GenerationTypeAudio        = "audio"
	GenerationTypeDigitalHuman = "digital_human"
	GenerationTypeOther        = "other"
)

// 生成任务状态（统一状态机：created → queueing → processing → success/failed/cancelled）。
const (
	TaskStateCreated    = "created"
	TaskStateQueueing   = "queueing"
	TaskStateProcessing = "processing"
	TaskStateSuccess    = "success"
	TaskStateFailed     = "failed"
	TaskStateCancelled  = "cancelled"
)

// IsTerminal 终态判断（终态后回调/轮询重复到达直接忽略——双通道幂等）。
func IsTerminal(state string) bool {
	return state == TaskStateSuccess || state == TaskStateFailed || state == TaskStateCancelled
}

// GenerationTask 统一生成任务（Vidu 全量接入的核心资产）。
//
// 设计（整洁架构·实体层）：所有端点共享同一任务模型——"提交 → task_id →
// 轮询/回调 → creations"。17 个端点的差异全部收敛到 SubType + Model + ParamsJSON
// 三个字段，协议层（状态机/轮询/回调/重试）只写一次。
type GenerationTask struct {
	ID             string
	TenantID       string
	BrandID        string
	Type           string // video/image/audio/digital_human/other
	SubType        string // 17 端点枚举（text2video/img2video/…/subject）
	Model          string // viduq3-pro/audio1.0/…
	Provider       string // vidu/mock
	ProviderTaskID string // 服务商侧任务 ID
	State          string // TaskState*
	ErrCode        string // 服务商错误码（原始）
	ErrMsg         string // 翻译后的产品级消息
	ParamsJSON     string // 完整提交参数（幂等重放/防重哈希）
	Payload        string // 透传给服务商的本地关联键（回调免查表）
	CreationsJSON  string // [{id,url,cover_url,watermarked_url,stored_url,stored_at}]
	Credits        int    // 本次消耗积分（对账用）
	OffPeak        bool   // 错峰模式（积分更低，48h 内完成）
	Watermark      bool   // 是否加水印
	CallbackReceived bool // 回调是否已到达（轮询可提前停）
	CallbackAt     *time.Time
	RetryCount     int    // 自动重试次数（失败分类后）
	ParamsHash     string // 提交参数哈希（防重复提交）
	CreatedAt      time.Time
	UpdatedAt      time.Time
	FinishedAt     *time.Time
}

// GenerationParams 生成参数（端点策略校验/组装用；map 以适配端点差异）。
type GenerationParams map[string]any

// CreationItem 生成物（Vidu creations[].url 24h 过期——转存后 stored_url 永久化）。
type CreationItem struct {
	ID             string `json:"id"`
	URL            string `json:"url"`
	CoverURL       string `json:"cover_url"`
	WatermarkedURL string `json:"watermarked_url"`
	StoredURL      string `json:"stored_url"`
	StoredAt       string `json:"stored_at"`
}

// ModelCapability 模型能力向量（类型化约束——替代裸 JSON schema）。
//
// 设计（对 Vidu 文档 parameter_schema_json 方案的改进）：参数约束按"模型能力"
// 组织而非"参数平铺"——校验是直查（duration ∈ [Durations]）而非解释执行配置。
// **数据库驱动**：generation_specs 表为唯一事实源（首次启动 seed 代码默认值，
// 管理后台全量可编辑；删除行 = 恢复出厂默认）。JSON tags 用于表存储与管理后台编辑。
type ModelCapability struct {
	Model            string   `json:"model"`             // 模型名（viduq3-pro/audio1.0…）
	Family           string   `json:"family"`            // 系列（q3/q2/q1/vidu2.0/audio1.0）
	Endpoint         string   `json:"endpoint,omitempty"` // 适用端点（同模型多端点时分别注册）
	Durations        [2]int   `json:"durations"`          // 时长范围 [min,max]（0 表示不支持自定义）
	Resolutions      []string `json:"resolutions,omitempty"` // 分辨率枚举
	AspectRatios     []string `json:"aspect_ratios,omitempty"` // 比例枚举
	AudioDefault     bool     `json:"audio_default"`      // audio 默认值
	AudioTypes       []string `json:"audio_types,omitempty"` // audio_type 枚举
	ImageSlots       int      `json:"image_slots"`        // 图片槽位：0=不需要 1=单图 2=双图 -1=动态(1-7)
	VideoSlots       int      `json:"video_slots"`        // 视频参考槽位（仅 q2-pro 参考生视频）
	SupportsBGM      bool     `json:"supports_bgm"`
	SupportsSubjects bool     `json:"supports_subjects"`  // 参考生视频主体模式
	SupportsMovement bool     `json:"supports_movement"`  // movement_amplitude（q1/2.0 生效）
	MaxPromptLen     int      `json:"max_prompt_len"`     // prompt 上限（默认 5000）
}

// GenerationSpec 端点/模型注册表条目（generation_specs 表——唯一事实源）。
// 管理后台全量可编辑：能力 JSON（capabilities_json）覆盖 + 启用开关 + 端点路径。
// Enabled=false 时该模型在端点下拉隐藏、提交被拒（全局掌控：停用/限流单模型）。
type GenerationSpec struct {
	SubType          string `json:"sub_type"`           // 端点类型（text2video/…）
	Model            string `json:"model"`              // 模型名
	Endpoint         string `json:"endpoint"`           // 服务商端点路径
	Enabled          bool   `json:"enabled"`            // 启用开关（false=停用）
	CapabilitiesJSON string `json:"capabilities_json"`  // 能力向量 JSON（管理后台可编辑）
	UpdatedAt        time.Time `json:"updated_at"`
}

// MediaAsset 媒体资产（素材上传 + 产物转存）。
//
// 两类：material（用户上传的图片/音频素材——供 Vidu 引用，避开 20MB body 限制）
//       creation（生成物转存——24h URL 下载到本地/OSS 永久化）。
const (
	AssetTypeMaterial = "material" // 用户上传素材
	AssetTypeCreation = "creation" // 生成物转存
)

type MediaAsset struct {
	ID        string
	TenantID  string
	BrandID   string
	OwnerType string // material/creation
	SourceURL string
	StoredURL string
	Mime      string
	SizeBytes int64
	MetaJSON  string // 音色 voice_id / 主体 server_id 等关联
	CreatedAt time.Time
	ExpiresAt *time.Time
}
