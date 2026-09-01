package port

import (
	"context"
	"net/http"
	"time"

	"webreaper/internal/domain/entity"
)

// GenerationProvider 生成服务商策略接口（Vidu/可灵/即梦…）。
//
// 设计（整洁架构·依赖倒置）：接口归用例层所有，适配器层实现。
// 协议层只依赖本接口——换服务商 = 新增 provider 实现 + main 装配一行，
// 用例零改动。接口只表达"任务协议"（提交/轮询/取消/验签/积分），
// 端点差异由 EndpointAdapter 承载，模型差异由 ModelCapability 承载。
type GenerationProvider interface {
	Name() string // "vidu" / "mock" / "kling"…
	// Submit 提交生成任务（endpoint 由端点策略提供；body 为组装后的请求体）。
	// 同步端点（Vidu 语音合成/声音复刻——"该接口是同步接口"）创建响应直接
	// 携带终态与产物：SubmitResult.State=success/failed 时不进轮询，
	// Creations 已含产物（file_url/demo_audio）；State 空（queueing/无该字段）
	// 为异步语义，由 Poll 推进。
	Submit(ctx context.Context, endpoint string, body map[string]any) (SubmitResult, error)
	// Poll 轮询任务状态（state/错误码/生成物）。
	Poll(ctx context.Context, taskID string) (GenerationStatus, error)
	// Cancel 取消任务。
	Cancel(ctx context.Context, taskID string) error
	// VerifyCallback 回调验签（HMAC-SHA256 复合串 + Date 新鲜度 + nonce 防重放）。
	// requestURI 为回调请求自身的 RequestURI（path+query）——签名字符串基于创建任务时
	// 配置的 callback_url，而回调正是发到该 URL，故用请求行还原（修复：此前依赖
	// X-Vidu-Request-URI 自定义头，真实 Vidu 回调不携带该头，验签必然失败）。
	VerifyCallback(ctx context.Context, header http.Header, body []byte, requestURI string) error
	// QueryCredits 查询剩余积分（对账/创建前余额校验）。
	QueryCredits(ctx context.Context) (int, error)
	// TranslateError 错误码 → 产品级消息（可读 + 可重试分类）。
	TranslateError(code string) string
}

// SubmitResult 提交结果（同步/异步统一承载）。
type SubmitResult struct {
	TaskID    string                // 服务商任务 ID（主体 API 等资源型端点为资源 id）
	Credits   int                   // 本次消耗积分
	State     string                // 空=异步（进轮询）；entity.TaskStateSuccess/Failed=提交即终态
	Creations []entity.CreationItem // 终态时的产物（TTS file_url / 复刻 demo_audio）
}

// GenerationStatus 任务轮询结果。
type GenerationStatus struct {
	State     string                // entity.TaskState*
	ErrCode   string                // 服务商原始错误码
	Creations []entity.CreationItem // 生成物（success 时有）
}

// EndpointAdapter 端点策略（一个对象一个端点）。
//
// 设计（策略模式）：行为差异有归属——"这个端点怎么做"（参数校验规则、
// 请求体组装、图片编码、subjects 结构）写在策略代码里，而非配置伪代码。
// 新增端点 = 新增策略文件 + 注册一行（开闭原则）。
type EndpointAdapter interface {
	// Type 端点类型（text2video/img2video/…/subject）。
	Type() string
	// Category 端点所属生成类型（video/image/audio/digital_human/other）。
	Category() string
	// Endpoint 服务商端点路径（如 /ent/v2/text2video——路径归属端点策略）。
	Endpoint() string
	// Validate 参数校验——cap 由 Registry 提供（DB 驱动唯一入口，策略不持有能力表）；
	// 策略只做端点结构规则（首尾帧恰 2 张、主体模式支持等）。
	Validate(ctx context.Context, cap entity.ModelCapability, params entity.GenerationParams) error
	// BuildRequest 组装请求体——参数映射/图片引用/payload 透传。
	BuildRequest(ctx context.Context, model string, params entity.GenerationParams, payload string) (map[string]any, error)
}

// ModelAutoSelector 模型自动选择（可选能力——傻瓜式端点不暴露模型选择，
// 客户端 model 传空时由端点策略按参数挑选）。
// 典型：reference2video 按主体类型自动切换——图片主体→q3（效果最好）、
// 视频主体→q2-pro（Vidu 约束视频主体仅 q2-pro 支持）。模型差异知识归
// 端点策略，用例层零改动（开闭原则）。
type ModelAutoSelector interface {
	EndpointAdapter
	// PickModel 从该端点全部启用模型（含能力向量）中挑选默认模型；空=不选。
	PickModel(models []entity.ModelCapability, params entity.GenerationParams) string
}

// SyncSubmitter 同步端点（可选能力——EndpointAdapter 类型断言获得）：
// 提交响应即终态，没有 task_id 轮询语义。
// 典型：Vidu 主体 API（POST /ent/v2/subjects）同步返回主体对象
// （id=server_id），既无 task_id 也无 state——若按异步任务轮询
// /ent/v2/tasks/{id}/creations 必然 404，任务永远停在 queueing。
// usecase 对此类端点：提交成功 → 直接终态 success，服务商资源 ID
// 存 ProviderTaskID 且以 creations[0].id 暴露给前端引用。
type SyncSubmitter interface {
	EndpointAdapter
	// IsSync 是否同步端点（提交即结果）。
	IsSync() bool
}

// CallbackEndpoint 支持回调的异步端点（可选能力——EndpointAdapter 类型断言获得）。
//
// Vidu 约定：创建任务时传入 callback_url，任务状态变化时主动 POST 回调
// （结构同查询任务 API 返回体；HMAC-SHA256 验签 + Date/nonce 防重放）。
// 仅文档声明了 callback_url 参数的端点实现本接口（text2video/reference2video/
// text2image/multiframe）——对其余端点注入未声明参数有被拒风险。
// 未实现/未配置公网回调地址时自动退化为纯轮询（20s 周期，双通道幂等合并）。
type CallbackEndpoint interface {
	EndpointAdapter
	// SupportsCallback 是否支持 callback_url 注入。
	SupportsCallback() bool
}

// EndpointRegistry 端点策略注册表。
type EndpointRegistry interface {
	// Get 取端点策略（未注册报错——前端下拉由 Types 驱动）。
	Get(ctx context.Context, subType string) (EndpointAdapter, error)
	// Types 全部已注册端点（前端/管理后台枚举）。
	Types() []string
	// Capability 取模型能力（校验用；DB 覆盖优先，代码默认兜底）。
	Capability(ctx context.Context, subType, model string) (entity.ModelCapability, error)
	// Models 某端点可用模型列表（DB 全量 enabled + 代码默认合并）。
	Models(ctx context.Context, subType string) ([]string, error)
	// AllSpecs 全量规格视图（管理后台矩阵：DB 覆盖行 + 未覆盖的出厂默认值）。
	AllSpecs(ctx context.Context) []entity.GenerationSpec
}

// GenerationTaskRepository 生成任务仓储。
type GenerationTaskRepository interface {
	Save(ctx context.Context, t entity.GenerationTask) error
	FindByID(ctx context.Context, tenantID, id string) (entity.GenerationTask, error)
	// FindByProviderTaskID 轮询/查询对齐用（不限租户——回调按 payload 定位为主）。
	FindByProviderTaskID(ctx context.Context, providerTaskID string) (entity.GenerationTask, error)
	// FindPendingByHash 防重复提交：同租户同参数哈希的未终态任务。
	FindPendingByHash(ctx context.Context, tenantID, paramsHash string) ([]entity.GenerationTask, error)
	List(ctx context.Context, tenantID string, limit int) ([]entity.GenerationTask, error)
	// ListActive 轮询用：全部租户未终态任务（阶段 1 单机扫描）。
	ListActive(ctx context.Context, limit int) ([]entity.GenerationTask, error)
	// ListRecentSuccessAll 跨租户最近成功任务倒序（32号：管理端作品巡查流）。
	ListRecentSuccessAll(ctx context.Context, limit int) ([]entity.GenerationTask, error)
	// FindSuccessTaskByMediaURL 按产物 URL 反查成功任务（32号 P1：发布拦截的
	// media_urls → 成片 work_key 桥接；LIKE 匹配 creations_json，查不到返回 ErrNotFound）。
	FindSuccessTaskByMediaURL(ctx context.Context, mediaURL string) (entity.GenerationTask, error)
	// ListFailed 自动重试用：全部租户 failed 任务（按 updated_at 升序——
	// 最久未动者优先；是否可重试由用例层 ClassifyError 判定，仓储不掺业务）。
	ListFailed(ctx context.Context, limit int) ([]entity.GenerationTask, error)
	// DeleteTerminalOlderThan 清理早于 before 的终态任务（P3 任务清理）。
	DeleteTerminalOlderThan(ctx context.Context, before time.Time) (int64, error)
	// Delete 删除单条任务（本地产记录删除——资产库"删除数字人"等场景；
	// 上游取消/删除由用例层决定，仓储只做数据访问）。
	Delete(ctx context.Context, tenantID, taskID string) error
	// ListBySubType 按端点类型过滤查询（个人分身列表等资产聚合场景）。
	// state 非空时额外过滤状态；limit<=0 用默认值。
	ListBySubType(ctx context.Context, tenantID, subType, state string, limit int) ([]entity.GenerationTask, error)
	// ListTransferPending 转存补偿（缺口A）：success 且产物有 url 无 stored_url、
	// finished_at 不早于 since（Vidu 24h URL 窗口内可救）的任务。按 finished_at 降序。
	ListTransferPending(ctx context.Context, since time.Time, limit int) ([]entity.GenerationTask, error)
}

// GenerationSpecRepository 端点/模型规格仓储（DB 为唯一事实源——全局掌控）。
type GenerationSpecRepository interface {
	// ListAll 全量条目（sub_type+model → 能力 JSON + enabled + endpoint）。
	ListAll(ctx context.Context) ([]entity.GenerationSpec, error)
	// Find 单条（管理后台编辑回显；未找到返回错误）。
	Find(ctx context.Context, subType, model string) (entity.GenerationSpec, error)
	// Upsert 保存（新增模型 = 直接插入；修改 = 覆盖能力 JSON/开关）。
	Upsert(ctx context.Context, spec entity.GenerationSpec) error
	// Delete 删除行（= 恢复出厂默认——查询回退代码默认值）。
	Delete(ctx context.Context, subType, model string) error
	// FindDefaultModel 查找默认模型（按厂商+端点）。
	FindDefaultModel(ctx context.Context, provider, subType string) (entity.GenerationSpec, error)
	// ListByProvider 按厂商查询（管理后台用）。
	ListByProvider(ctx context.Context, provider string) ([]entity.GenerationSpec, error)
	// SetDefault 设置默认模型（取消同端点其他模型的默认标记）。
	SetDefault(ctx context.Context, provider, subType, model string) error
}

// MediaAssetStore 媒体资产存储（素材托管 + 产物转存适配器）。
type MediaAssetStore interface {
	// SaveFile 保存素材文件（上传）——返回资产（含可访问 URL）。
	SaveFile(ctx context.Context, tenantID, brandID, ownerType string, data []byte, mime, ext string) (entity.MediaAsset, error)
	// List 列出某租户资产（ownerType=material 素材 / creation 产物 / 空=全部），按创建时间倒序。
	List(ctx context.Context, tenantID, ownerType string) ([]entity.MediaAsset, error)
	// Delete 删除资产（仅限该租户自己的——tenant 校验由实现负责）。
	Delete(ctx context.Context, tenantID, assetID string) error
	// DownloadAndStore 下载外部 URL 到本地存储并返回永久 URL（转存用——Vidu 24h 产物永久化）。
	DownloadAndStore(ctx context.Context, tenantID, sourceURL string, meta map[string]string) (string, error)
	// CleanupBefore 清理过期资产（定时任务）。excludeURLs 为仍被任务引用的资产
	// 公网 URL 集合（R1：防止删掉商户还在用的产物——引用关系来自任务 creations/params）。
	CleanupBefore(ctx context.Context, before time.Time, excludeURLs map[string]bool) (int, error)
	// ReadLocal 若 URL 为本站托管素材且文件在本地磁盘（Local 模式），读取文件内容——
	// 用于把 Vidu 拉不到的 URL（localhost/内网）内联为 base64 data URI（Vidu 文档
	// 支持："支持传入 Base64 编码或图片URL（确保可访问）"；同步端点创建即拉素材，
	// 不可达 URL 直接 400 BadRequest）。ok=false：非本站托管或本地无文件
	//（OSS 模式 URL 本身公网可达，无需内联）。
	ReadLocal(ctx context.Context, url string) (data []byte, mime string, ok bool)
}

// VoiceLibrary 官方音色库（只读查询——seed 进 DB 的静态参考数据）。
// handler 直接依赖（同 MediaAssetStore 模式）：无任务语义，不需用例封装。
//
// 白牌化（用户确认 2026-09-01）：用户端只显示 scope=platform（管理后台创建）
// 和 scope=clone 且属于本租户的克隆音色。scope=vidu（上游 302 条）仅管理端可见
// ——作为克隆参考源，不暴露给用户。
type VoiceLibrary interface {
	// ListForUser 用户端音色列表：scope=platform（active）+ 本租户 clone（active）。
	// tenantID 必传——空则返回空列表（防御）。
	ListForUser(ctx context.Context, tenantID string) ([]entity.GenerationVoice, error)
	// ListForAdmin 管理端全量音色（含 vidu/platform/clone 所有 scope、含停用行）。
	// scope 非空时仅返回该 scope（vidu=克隆参考源 / platform=平台音色管理）。
	ListForAdmin(ctx context.Context, scope string) ([]entity.GenerationVoice, error)
	// SeedIfEmpty 表空时写入种子数据（返回写入条数；已非空返回 0）。
	SeedIfEmpty(ctx context.Context, voices []entity.GenerationVoice) (int, error)
	// Upsert 按 voice_id 主键幂等写入（26号计划——voice_clone 物化钩子调用）。
	Upsert(ctx context.Context, voice entity.GenerationVoice) error
	// GetDefault 获取平台默认音色（scope=platform 且 is_default=true 的首条；无则空）。
	GetDefault(ctx context.Context) (entity.GenerationVoice, error)
	// SetDefault 设为平台默认音色（同一 scope=platform 内仅一条 default=true）。
	SetDefault(ctx context.Context, voiceID string) error
	// FindByVoiceID 按音色 ID 精确查询单条（缺口C：克隆/平台音色的样本合成通道定位样本音频）。
	FindByVoiceID(ctx context.Context, voiceID string) (entity.GenerationVoice, error)
	// UpdateViduRegisteredAt 记录/清除 Vidu 侧注册时间（31号 L2——注册缓存窗口判定）。
	// t 为 nil 时清除（缓存失效：厂商侧报"音色不存在"后强制下次重建）。
	UpdateViduRegisteredAt(ctx context.Context, voiceID string, t *time.Time) error
	// DeleteClone 删除克隆音色行（31号 U4：删除任务联动清理——仅 scope=clone 且
	// 归属租户匹配才删；platform/vidu 行不受影响）。Vidu 侧注册无需清理（7 天自然过期）。
	DeleteClone(ctx context.Context, tenantID, voiceID string) error
}

// TaskNotifier 生成任务终态通知（可选注入——站内信主动唤醒）。
//
// 差距修复：异步任务（视频/图片）在轮询周期内完成，商户不留在页面上就永远
// 不知道结果；同步任务（主体/TTS）提交即终态但用户可能在后台运行。终态
// （success/failed）转换恰好发生一次（IsTerminal 幂等护栏），在此处通知不会重复。
type TaskNotifier interface {
	// NotifyTaskTerminal 任务进入终态时回调（同步执行，实现应快速失败不影响主流程）。
	NotifyTaskTerminal(ctx context.Context, task entity.GenerationTask)
}

// AudioSynthesizer 同步音频合成（文本→音频字节，无需轮询）。
//
// 与 GenerationProvider 的 TTS 端点互补：
//   - Vidu TTS：异步任务（Submit → task_id → Poll → 音频 URL），走 generation_tasks 链路
//   - 小米 MiMo TTS：同步接口（chat/completions + audio 输出），直接返回 base64 音频字节
//
// 三种模式：
//   - Synthesize：标准 TTS（文本 + 预置音色 ID → 音频）
//   - SynthesizeDesign：音色设计（自然语言描述风格 → 音频，小米 voicedesign 模型）
//   - SynthesizeClone：声音克隆（音频样本 + 文本 → 克隆音色音频，小米 voiceclone 模型）
//
// 向导第④步使用：用户选音色后直接拿音频字节，无需提交任务轮询。
type AudioSynthesizer interface {
	// Synthesize 标准 TTS（预置音色）。voiceID 为空时用默认音色。
	Synthesize(ctx context.Context, text string, voiceID string) (audio []byte, format string, err error)
	// SynthesizeDesign 音色设计（自然语言描述音色风格）。小米 voicedesign 模型专用。
	SynthesizeDesign(ctx context.Context, text string, styleDesc string) (audio []byte, format string, err error)
	// SynthesizeClone 声音克隆（传入音频样本 base64 + 合成文本）。
	SynthesizeClone(ctx context.Context, sampleBase64 string, text string) (audio []byte, format string, err error)
}

// CapabilityResolver 统一能力配置查询（能力路由模型——"我需要 ASR 用谁"的答案）。
//
// 设计动机：用例层已经是能力优先（SpeechTranscriber/AIGenerator 等 port 接口），
// 但配置层散落在多张表（provider_configs/llm_configs），adapter 各自实现 resolve
// 逻辑。本接口统一配置查询路径：adapter 注入本接口，按能力 ID 取当前生效配置。
//
// 实现层读 integration_capabilities + integration_vendors（新表），
// 同时兼容旧表（provider_configs/llm_configs）——渐进迁移，旧表最终下线。
type CapabilityResolver interface {
	// Resolve 按能力 ID 取当前生效配置（IsDefault=true + Enabled=true）。
	Resolve(ctx context.Context, capID string) (entity.ResolvedCap, error)
}

// ProviderConfigRepository 厂商配置仓储（管理后台按厂商管理）。
type ProviderConfigRepository interface {
	// List 全部厂商配置。
	List(ctx context.Context) ([]entity.ProviderConfig, error)
	// Get 取某厂商配置（不存在返回 entity.ErrNotFound 语义错误）。
	Get(ctx context.Context, provider string) (entity.ProviderConfig, error)
	// Upsert 保存（不存在插入，存在覆盖非空字段）。
	Upsert(ctx context.Context, cfg entity.ProviderConfig) error
}

// ConfigurableProvider 支持运行时更新配置的厂商（管理后台保存后热生效，无需重启）。
type ConfigurableProvider interface {
	// UpdateAPIKey 更新 API Key（原子生效，后续请求使用新 Key）。
	UpdateAPIKey(key string)
}

// CallbackNonceStore 回调 nonce 防重放存储（R2 无状态化：单机内存实现多实例失效——
// Redis 实现 SETNX+EX 原子判重）。 Seen 首次见到 nonce 返回 true（可处理）；重复/已过期返回 false。
type CallbackNonceStore interface {
	Seen(ctx context.Context, nonce string) bool
}

// ---- 主体库（25 号阶段一：官方主体即选即用）----

// SubjectInfo 服务商主体（官方/个人）。官方主体不返回 style/description（Vidu 契约）。
type SubjectInfo struct {
	ServerID string   `json:"server_id"`          // 主体 id（reference2video subjects[].server_id 直用）
	Name     string   `json:"name"`               // 主体名称
	Images   []string `json:"images,omitempty"`   // 主体图片（images[0] 作封面）
	Videos   []string `json:"videos,omitempty"`   // 主体视频
	VoiceID  string   `json:"voice_id,omitempty"` // 绑定音色（B 路径文本直生可用）
}

// SubjectListResult 主体分页列表（Vidu GET /ent/v2/subjects 透传形态）。
type SubjectListResult struct {
	Subjects      []SubjectInfo `json:"subjects"`
	NextPageToken string        `json:"next_page_token,omitempty"`
	Count         int           `json:"count"`
}

// SubjectLister 列出服务商主体（可选能力——Vidu 实现；mock 不实现，
// 演示模式下官方主体区返回"暂未开放"）。个人主体不查服务商（本地 subject 任务聚合）。
type SubjectLister interface {
	// ListSubjects ownership: "system"=官方主体 / "private"=个人（产品只用 system）。
	ListSubjects(ctx context.Context, ownership, pageToken string, count int) (SubjectListResult, error)
}
