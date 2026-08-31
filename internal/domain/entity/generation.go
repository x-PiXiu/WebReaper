package entity

import (
	"fmt"
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

// validTransitions 合法状态转换表（27 号：实体层贫血治理——状态规则下沉到 entity）。
// key=当前状态，value=可转换到的目标状态集合。
var validTransitions = map[string]map[string]bool{
	TaskStateCreated:    {TaskStateQueueing: true},
	TaskStateQueueing:   {TaskStateProcessing: true, TaskStateSuccess: true, TaskStateFailed: true, TaskStateCancelled: true},
	TaskStateProcessing: {TaskStateSuccess: true, TaskStateFailed: true, TaskStateCancelled: true},
	// 终态不可转换
	TaskStateSuccess:   {},
	TaskStateFailed:    {},
	TaskStateCancelled: {},
}

// IsTerminalM 方法版终态判断。
func (t GenerationTask) IsTerminalM() bool { return IsTerminal(t.State) }

// CanTransitionTo 检查是否可转换到目标状态。
func (t GenerationTask) CanTransitionTo(target string) bool {
	targets, ok := validTransitions[t.State]
	if !ok {
		return false
	}
	return targets[target]
}

// TransitionTo 执行状态转换（27 号：状态规则下沉——usecase 不再直接赋值 State）。
//
// 合法转换：created→queueing, queueing→processing/success/failed/cancelled,
// processing→success/failed/cancelled。终态不可转换。
// 成功/失败/取消自动设置 FinishedAt。
// 返回 error 而非 panic——调用方可选择忽略（向后兼容）或处理。
func (t *GenerationTask) TransitionTo(target string, errMsg string) error {
	if !t.CanTransitionTo(target) {
		return fmt.Errorf("非法状态转换: %s → %s", t.State, target)
	}
	t.State = target
	t.UpdatedAt = time.Now()
	if IsTerminal(target) {
		now := time.Now()
		t.FinishedAt = &now
	}
	if target == TaskStateFailed && errMsg != "" {
		t.ErrMsg = errMsg
	}
	return nil
}

// MarkSuccess 标记成功（含产物 JSON 和积分）。
func (t *GenerationTask) MarkSuccess(creationsJSON string, credits int) error {
	if err := t.TransitionTo(TaskStateSuccess, ""); err != nil {
		return err
	}
	t.CreationsJSON = creationsJSON
	t.Credits = credits
	return nil
}

// MarkFailed 标记失败。
func (t *GenerationTask) MarkFailed(errMsg string) error {
	return t.TransitionTo(TaskStateFailed, errMsg)
}

// MarkCancelled 标记取消。
func (t *GenerationTask) MarkCancelled() error {
	return t.TransitionTo(TaskStateCancelled, "")
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
	TimelineJSON     string // B-Roll 台词时间轴（静音检测定位产物 JSON；空=未定位——22 计划）
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
//
// 设计动机（按厂商区分）：
//   - Provider 字段区分不同厂商（vidu/kling/...）
//   - IsDefault 字段标记每个端点的默认模型
//   - 管理后台可以为每个厂商的每个端点配置默认模型
//   - CostCredits 字段标记每次调用消耗积分（27 号：模型差异化计费）
type GenerationSpec struct {
	SubType          string `json:"sub_type"`           // 端点类型（text2video/…）
	Model            string `json:"model"`              // 模型名
	Provider         string `json:"provider"`           // 厂商名称（vidu/kling/...）
	Endpoint         string `json:"endpoint"`           // 服务商端点路径
	Enabled          bool   `json:"enabled"`            // 启用开关（false=停用）
	IsDefault        bool   `json:"is_default"`         // 是否为默认模型
	SortOrder        int    `json:"sort_order"`         // 排序
	CapabilitiesJSON string `json:"capabilities_json"`  // 能力向量 JSON（管理后台可编辑）
	CostCredits      int    `json:"cost_credits"`       // 每次调用消耗积分（0=使用服务商返回值；>0=覆盖）
	UpdatedAt        time.Time `json:"updated_at"`
}

// MediaAsset 媒体资产（素材上传 + 产物转存）。
//
// 两类：material（用户上传的图片/音频素材——供 Vidu 引用，避开 20MB body 限制）
//       creation（生成物转存——24h URL 下载到本地/OSS 永久化）。
//
// 素材类型（Type 字段）：
//   - image：图片素材（png/jpeg/jpg/webp）
//   - video：视频素材（mp4/avi/mov）
//   - audio：音频素材（mp3/wav/m4a/aac）
//
// 设计动机（整洁架构）：
//   - Type/Name/Width/Height/Duration 字段用于端点自动选择（EndpointSelector）
//   - 端点选择器根据素材类型自动选择合适的端点（如1张图片→img2video）
//   - 零框架依赖：纯 struct + 领域规则，不 import gorm/llm。
const (
	AssetTypeMaterial = "material" // 用户上传素材
	AssetTypeCreation = "creation" // 生成物转存
)

// 素材类型常量（用于端点自动选择）。
const (
	MaterialTypeImage = "image" // 图片素材
	MaterialTypeVideo = "video" // 视频素材
	MaterialTypeAudio = "audio" // 音频素材
)

type MediaAsset struct {
	ID        string
	TenantID  string
	BrandID   string
	OwnerType string // material/creation
	Type      string // 素材类型：image/video/audio（端点自动选择用）
	Name      string // 素材名称（用户可见，如"品牌Logo"）
	SourceURL string
	StoredURL string
	Mime      string
	SizeBytes int64
	Width     int       // 图片/视频宽度（像素）
	Height    int       // 图片/视频高度（像素）
	Duration  float64   // 音频/视频时长（秒）
	MetaJSON  string    // 音色 voice_id / 主体 server_id 等关联
	CreatedAt time.Time
	ExpiresAt *time.Time
}

// IsImage 判断是否为图片素材。
func (m MediaAsset) IsImage() bool {
	return m.Type == MaterialTypeImage
}

// IsVideo 判断是否为视频素材。
func (m MediaAsset) IsVideo() bool {
	return m.Type == MaterialTypeVideo
}

// IsAudio 判断是否为音频素材。
func (m MediaAsset) IsAudio() bool {
	return m.Type == MaterialTypeAudio
}

// InferTypeFromMime 根据MIME类型推断素材类型。
func InferTypeFromMime(mime string) string {
	switch {
	case len(mime) >= 6 && mime[:6] == "image/":
		return MaterialTypeImage
	case len(mime) >= 6 && mime[:6] == "video/":
		return MaterialTypeVideo
	case len(mime) >= 6 && mime[:6] == "audio/":
		return MaterialTypeAudio
	default:
		return ""
	}
}

// GenerationVoice 官方音色（Vidu 语音合成音色表——静态参考数据）。
// 启动 seed 进 generation_voices 表，客户端查询/筛选（TTS 的
// voice_setting_voice_id、主体的 voice_id、数字人 voice_id 均可引用）。
type GenerationVoice struct {
	VoiceID   string `json:"voice_id"`   // 音色 ID（提交参数的取值）
	Language  string `json:"language"`   // 语言（中文 (普通话)/英文/日文…分组用）
	Name      string `json:"name"`       // 音色名称（展示用）
	SampleURL string `json:"sample_url"` // 试听示例音频 URL
	Recommend bool   `json:"recommend"`  // 精选推荐（口播常用音色——服务端标记，替代前端截断）
}

// PromptRef 提示词 @引用（客户端从素材库选择，提交给服务端统一翻译）。
//
// 设计：客户端只表达"引用了哪个素材 + 什么类型"，服务端提示词翻译层
// （translateRefs）按 端点×能力向量 把引用翻译成上游需要的参数格式
// （images/audio_url/start_image/videos…）——新增端点/类型只改翻译规则，
// 客户端零改动。
const (
	RefKindImage = "image" // 图片引用
	RefKindAudio = "audio" // 音频引用
	RefKindVideo = "video" // 视频引用（仅 reference2video q2-pro）
)

type PromptRef struct {
	ID   string `json:"id"`   // 素材 ID（文件名，用于校验归属）
	Name string `json:"name"` // 素材名（prompt 中 @名称 标记）
	URL  string `json:"url"`  // 素材可访问 URL（最终引用值）
	Kind string `json:"kind"` // RefKindImage/Audio/Video
}

// ProviderConfig 厂商配置（管理后台按厂商管理：Vidu API Key 等）。
// DB 为事实源：装配优先 DB，环境变量兜底；保存后对已装配厂商热生效。
type ProviderConfig struct {
	Provider  string
	APIKey    string // 明文（后台管理场景）；API 层只返回掩码
	BaseURL   string
	Enabled   bool
	ExtraJSON string // 扩展字段（签名密钥等，预留）
	UpdatedAt time.Time
}
