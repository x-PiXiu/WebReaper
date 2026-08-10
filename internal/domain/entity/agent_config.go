package entity

// 默认系统提示词与默认工具列表（业务默认值，统一在领域层定义，避免散落重复）。
//
// 设计动机（DRY）：原先 handler/agent_handler.go 和 adapter/agent/trpc_agent_runner.go
// 各硬编码了一份相同的默认 prompt，违反 DRY。下沉到领域层后，所有调用方共享同一份。
const (
	DefaultSystemPrompt = "你是一个智能数据采集助手。你可以调用爬虫工具采集数据，" +
		"并对采集到的内容进行结构化总结。执行任务时，先用工具采集数据，再总结要点。"

	// DefaultMaxIterations 默认最大工具调用次数（防 Agent 死循环）。
	DefaultMaxIterations = 10
)

// DefaultAgentTools 工具全局可用后，异步任务路径的默认工具集（历史兼容）。
var DefaultAgentTools = []string{"api_crawler", "static_crawler"}

// AgentConfig 是 Agent 的配置（可存数据库，运行时动态加载）。
//
// 每个 Agent 有自己的系统提示词（定义角色和目标），并引用一个 LLMConfig
// 决定使用哪个厂商/模型。工具现已全局可用（运行时所有爬虫对 Agent 开放），
// Tools 字段保留用于历史兼容与异步任务路径。
//
// 设计要点（依赖倒置 + 聚合引用）：
//   - AgentConfig 不内嵌 LLM 连接细节（apiKey/baseURL），只引用 LLMConfig.Name。
//   - 切换厂商/模型 = 换一个 llm_config_name，无需改 Agent 本身。
type AgentConfig struct {
	Name          string   // Agent 名称，唯一标识
	SystemPrompt  string   // 系统提示词（定义 Agent 的角色、目标、约束）
	Tools         []string // 历史兼容字段；工具现已全局可用，新建 Agent 时不再配置
	LLMConfigName string   // 引用的 LLMConfig.Name（留空用 "default"）
	MaxIterations int      // 最大工具调用次数（防死循环，默认 10）
}

// IsValid 领域规则：有效的 Agent 配置必须有名称和系统提示词。
func (a AgentConfig) IsValid() bool {
	return a.Name != "" && a.SystemPrompt != ""
}

// HasTool 判断本 Agent 是否允许使用指定工具。
// 注：工具全局化后此方法仅用于历史兼容判断。
func (a AgentConfig) HasTool(toolName string) bool {
	for _, t := range a.Tools {
		if t == toolName {
			return true
		}
	}
	return false
}

// FillDefaults 填充零值为业务默认值（MaxIterations<=0 → 10；Tools 空 → 默认工具集）。
// 返回填充后的副本（不修改原对象）。
func (a AgentConfig) FillDefaults() AgentConfig {
	out := a
	if out.MaxIterations <= 0 {
		out.MaxIterations = DefaultMaxIterations
	}
	if len(out.Tools) == 0 {
		out.Tools = DefaultAgentTools
	}
	return out
}
