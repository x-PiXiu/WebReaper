package entity

// LLMConfig 是大语言模型的独立配置（OpenAI 兼容协议）。
//
// 设计动机（整洁架构 / 聚合边界）：
//   - LLM 配置从 AgentConfig 中分离，成为独立聚合根。
//   - 一个 LLMConfig 描述「用哪个厂商的哪个模型怎么连」，
//     多个 AgentConfig 可以引用同一个 LLMConfig（多对一关系）。
//   - 新增厂商/模型只需新增一条 LLMConfig 记录，符合开闭原则。
//
// 厂商协议统一为 OpenAI 兼容（MiniMax/DeepSeek/Qwen/Zhipu 等均兼容），
// 不引入策略模式——provider 字段仅作展示标签。
type LLMConfig struct {
	Name     string // 唯一标识，如 "default"、"minimax-m2"、"deepseek-chat"
	Provider string // 厂商标签：minimax / openai / zhipu / deepseek（仅用于展示）
	APIKey   string // API 密钥
	BaseURL  string // API 端点，如 https://api.minimaxi.com/v1
	Model    string // 模型名，如 MiniMax-M2.5
	// CostPerMTok 每百万 tokens 参考成本（分；默认 100 = ¥1/百万 tokens）。
	// 按引擎差异化（豆包 ~20、DeepSeek ~20、GPT 级 ~300）——成本分析按引擎细分
	// （P1-1：监测接入多引擎后，成本报表不能再按全局单一参考价估算）。
	CostPerMTok int
	// Usage 用途标签："" = 聊天/内容（默认）；"vision" = 视觉模型（浏览器截图分析）。
	// 两套模型独立配置互不影响：聊天模型坏了浏览器 Agent 不受影响，反之亦然。
	// 管理后台按用途筛选，视觉模型默认显示 Agnes 等支持视觉的模型。
	Usage string
	// IsDefault 是否为该用途的默认模型（管理后台切换默认时互斥——同 Usage 下只有一条 true）。
	IsDefault bool
}

// IsValid 领域规则：有效的 LLM 配置必须有名称、API Key 和模型名。
func (c LLMConfig) IsValid() bool {
	return c.Name != "" && c.APIKey != "" && c.Model != ""
}
