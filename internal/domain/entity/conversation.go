package entity

import "time"

// Conversation 是聊天会话聚合根。
//
// 设计要点：
//   - 持久化到后端，按 UserID 隔离（小团队场景：业务数据共享，但会话归属个人）。
//   - Title 在首条消息发送后锁定（取前 30 字符），支持 UpdateTitle 重命名。
//   - 一个会话包含多条 Message（一对多）。
type Conversation struct {
	ID        string    // 唯一标识（前端生成 conv{timestamp}，便于前端乐观更新）
	Title     string    // 会话标题
	AgentName string    // 使用的 Agent 名（空表示默认）
	UserID    string    // 归属用户 ID（隔离用）
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsValid 领域规则：会话必须有 ID 和 UserID。
func (c Conversation) IsValid() bool {
	return c.ID != "" && c.UserID != ""
}

// Message 是会话内的一条消息。
//
// 字段映射约定（与前端 RichMessage 对应）：
//   - Content：纯文本内容；assistant 消息可能含 <think>...</think> 原文，
//     前端加载历史时用 parseBlocks 重新拆分为 text/think 块。
//   - ToolCallsJSON：工具调用块（tool-call + tool-result）的 JSON 序列化，
//     对应前端 RichMessage.tools。前端加载时反序列化为 ToolRecord[]。
//   - 这样前端流式渲染逻辑零改动，只是数据源从 localStorage 换成 API。
type Message struct {
	ID             string    // 消息 ID（前端生成 u/a + timestamp）
	ConversationID string    // 外键，所属会话
	Role           string    // user / assistant
	Content        string    // 文本内容（assistant 含 <think> 原文）
	ToolCallsJSON  string    // 工具调用块 JSON（对应前端 tools）
	CreatedAt      time.Time
}

// IsValid 领域规则：消息必须有 ID、会话 ID 和角色。
func (m Message) IsValid() bool {
	return m.ID != "" && m.ConversationID != "" && (m.Role == "user" || m.Role == "assistant")
}
