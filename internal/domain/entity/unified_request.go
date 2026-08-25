package entity

// UnifiedGenerationRequest 统一生成请求（客户端输入格式）。
//
// 设计动机（整洁架构）：
//   - 客户端只需要上传素材、输入文本，不需要选择端点/模型
//   - 端点选择由EndpointSelector根据素材自动完成
//   - 模型选择由EndpointAdapter.BuildRequest()自动完成
//
// 使用场景：
//   - 用户上传1张品牌Logo + 输入"品牌宣传视频" → 系统自动选择img2video端点
//   - 用户上传1张图片 + 1个音频 → 系统自动选择digital_human端点
//   - 用户只输入文本 → 系统自动选择text2video端点
//   - 用户输入文本 + 指定type=audio → 系统选择tts端点
type UnifiedGenerationRequest struct {
	// 基础信息（从JWT/上下文获取，不需要客户端传）
	// TenantID 由 usecase 层注入（handler 已从 JWT 解析）——素材查询按租户隔离
	TenantID string `json:"-"`
	// UserID    string `json:"user_id"`    // 从JWT获取

	// 用户输入（必填）
	BrandID   string   `json:"brand_id"`   // 品牌ID
	Text      string   `json:"text"`       // 文本描述（prompt）
	Materials []string `json:"materials"`   // 素材ID列表（数据库主键）

	// 用户选择（可选，有默认值）
	Template  string   `json:"template"`   // 模板ID（可选，管理后台配置）
	Type      string   `json:"type"`       // 生成类型（可选：video/image/audio/voice）
	Duration  int      `json:"duration"`   // 时长秒数（可选，默认根据模板）
	Quality   string   `json:"quality"`    // 质量（可选，默认720p）

	// 高级选项（可选，有默认值）
	AspectRatio string `json:"aspect_ratio"` // 比例（可选，默认16:9）
	// Params 高级参数透传（兼容层的 seed/style/voice_setting_* 等专业模式参数——
	// selector 出口按白名单合并进 GenerationParams，用户显式值覆盖默认）
	Params map[string]any `json:"params,omitempty"`
}

// EndpointSelectResult 端点选择结果。
//
// 设计动机（整洁架构）：
//   - EndpointSelector的输出，包含端点类型和端点特定参数
//   - GenerationUseCase使用这个结果来调用EndpointAdapter
type EndpointSelectResult struct {
	SubType string           // 端点类型（自动选择）
	Params  GenerationParams // 端点特定参数
}
