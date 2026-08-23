package entity

import "time"

// ---- 提示词模板仓库（内容生成/优化的系统提示词可管理）----
//
// 设计动机（整洁架构）：
//   - 此前内容生成/优化的系统提示词硬编码在 usecase 里——改提示词要改代码+发版。
//   - 模板入库后：版本化（Version 递增）、可管理（admin 后台编辑）、
//     可热更新（内容生成时按 Key 读取，无记录回退内置默认）。
//   - 模板内容遵循既有硬约束（think 禁令/标题格式/引擎偏好），
//     管理后台编辑时由 usecase 层追加引擎偏好指令，业务规则不被绕过。

// PromptTemplate 提示词模板。
type PromptTemplate struct {
	Key       string    `json:"key"`       // 模板键（content-generate / content-optimize）
	Version   int       `json:"version"`   // 版本号（每次更新 +1）
	Content   string    `json:"content"`   // 模板内容（系统提示词）
	UpdatedAt time.Time `json:"updated_at"`
}

// 内置模板键。
const (
	PromptKeyContentGenerate = "content-generate" // 内容原创生成系统提示词
	PromptKeyContentOptimize = "content-optimize" // 内容优化（GEO 改写）系统提示词
	// 格式模板键前缀（geo_format_<key>——如 geo_format_xiaohongshu）。
	// admin 可在管理后台编辑各格式的输出指令（字数/风格/结构），热更新无需发版。
	PromptKeyFormatPrefix = "geo_format_"

	// 智能体系统提示词键（获客智能体专用）。
	// 管理后台可动态配置，热更新无需发版。
	PromptKeyAgentPrefix = "agent-prefix"  // 智能体前缀（在硬编码之前）
	PromptKeyAgentSuffix = "agent-suffix"  // 智能体后缀（在硬编码之后）
	PromptKeyAgentRules  = "agent-rules"   // 智能体补充规则
)
