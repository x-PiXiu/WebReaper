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
	Submit(ctx context.Context, endpoint string, body map[string]any) (taskID string, credits int, err error)
	// Poll 轮询任务状态（state/错误码/生成物）。
	Poll(ctx context.Context, taskID string) (GenerationStatus, error)
	// Cancel 取消任务。
	Cancel(ctx context.Context, taskID string) error
	// VerifyCallback 回调验签（HMAC-SHA256 复合串 + Date 新鲜度 + nonce 防重放）。
	VerifyCallback(ctx context.Context, header http.Header, body []byte) error
	// QueryCredits 查询剩余积分（对账/创建前余额校验）。
	QueryCredits(ctx context.Context) (int, error)
	// TranslateError 错误码 → 产品级消息（可读 + 可重试分类）。
	TranslateError(code string) string
}

// GenerationStatus 任务轮询结果。
type GenerationStatus struct {
	State     string               // entity.TaskState*
	ErrCode   string               // 服务商原始错误码
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
	// CleanupBefore 清理过期资产（定时任务）。
	CleanupBefore(ctx context.Context, before time.Time) (int, error)
}
