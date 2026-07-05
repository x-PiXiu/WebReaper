package entity

import "time"

// ExternalSystem 外部目标系统配置（聚合根）。
//
// 设计动机（整洁架构 + DIP + 配置驱动）：
//   - 推送目标系统的 API 路径、字段映射、认证方式都不同且会变，不能硬编码。
//   - 把每个外部系统抽象为一份配置：Endpoint + Headers + FieldMapping，
//     推送时数据驱动地转换+发送，新增系统只需新增一条配置（OCP）。
//   - 字段映射用 JSON：{"本系统字段":"目标字段"}，如 {"title":"title","content":"stem"}。
//   - 认证用自定义请求头列表 JSON：{"X-API-Key":"xxx","Content-Type":"application/json"}。
//
// 示例（对接 AgentCore 面试题入库）：
//   Name=agentcore-question
//   Endpoint=https://agentcore.xxx/api/v1/ingest/question
//   Headers={"X-API-Key":"ac-xxx","Content-Type":"application/json"}
//
// 两种推送模式（Mode 字段控制）：
//   raw     —— 原样转发：DataItem.Content 已是目标系统请求体 JSON，直接 POST（推荐）。
//             适合"Agent 提示词里已定义目标格式"的场景，无需字段映射。
//   mapping —— 字段映射：按 FieldMapping 把本系统字段映射为目标字段（旧模式，字段有限制）。
type ExternalSystem struct {
	Name         string    // 唯一标识，如 "agentcore-question"
	Description  string    // 描述，如 "AgentCore 面试题入库"
	Endpoint     string    // 完整 API 地址，如 https://xxx/api/v1/ingest/question
	Method       string    // HTTP 方法，默认 POST
	Headers      string    // 请求头 JSON，如 {"X-API-Key":"xxx"}
	Mode         string    // 推送模式：raw（原样转发，默认）/ mapping（字段映射）
	FieldMapping string    // mapping 模式的字段映射 JSON，如 {"title":"title","content":"stem"}
	BodyTemplate string    // raw 模式的请求体示例（可选，用于校验/提示，实际推送用 DataItem 内容）
	ContentType  string    // 接收的数据类型标记，如 "question"/"article"（可选，用于 UI 分组提示）
	Enabled      bool      // 是否启用
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// 推送模式常量
const (
	PublishModeRaw    = "raw"    // 原样转发：DataItem.Content 作为请求体直接 POST
	PublishModeMapping = "mapping" // 字段映射：按 FieldMapping 转换
)

// IsValid 领域规则：外部系统必须有名称和端点。
// raw 模式不要求 FieldMapping；mapping 模式要求 FieldMapping。
func (e ExternalSystem) IsValid() bool {
	if e.Name == "" || e.Endpoint == "" {
		return false
	}
	if e.Mode == PublishModeMapping && e.FieldMapping == "" {
		return false
	}
	return true
}

// PublishRecord 推送结果记录（对应 001_init.sql 的 publish_records 表）。
//
// 记录每次推送的结果，用于：
//   - 去重（同一内容+系统不重复推）
//   - 审计（谁在什么时候推了什么）
//   - 失败排查（记录错误信息）
type PublishRecord struct {
	ID          string     // 记录 ID
	ContentID   string     // 被推送的 DataItem ID
	ContentType string     // 内容类型，如 "data_item"
	SystemName  string     // 目标系统名（对应 ExternalSystem.Name）
	Success     bool       // 是否成功
	ExternalID  string     // 外部系统返回的 ID（成功时）
	ErrorMsg    string     // 失败时的错误信息
	ResultAt    time.Time  // 推送结果时间
	CreatedAt   time.Time
}
