package ai

import (
	"context"

	"webreaper/internal/usecase/port"
)

// DBConversationMemory 是 port.ConversationMemory 的数据库实现。
//
// 从 message_repo 读取会话历史消息，转成 port.ChatMessage 供 AI 生成器
// seed 进 LLM session。这样进程重启后（inmemory session 丢失），
// 旧会话的首次对话仍能从 DB 恢复上下文。
//
// 设计要点（谦卑对象模式）：
//   - 本对象只做"读 DB + 格式转换"这种无逻辑的搬运，不涉及业务规则；
//   - 真正的记忆编排逻辑在 TrpcAgentGenerator 里（决定何时 seed、seed 多少）。
type DBConversationMemory struct {
	msgRepo port.MessageRepository
}

// NewDBConversationMemory 创建基于 DB 的会话记忆。
func NewDBConversationMemory(msgRepo port.MessageRepository) *DBConversationMemory {
	return &DBConversationMemory{msgRepo: msgRepo}
}

// History 实现端口接口：从 DB 读会话消息，转 []port.ChatMessage。
// 只取 user/assistant 的文本内容（system prompt 由调用方单独管）。
// 错误降级为空历史（记忆恢复失败不应阻断对话，让会话作为新对话继续）。
func (m *DBConversationMemory) History(ctx context.Context, conversationID string) ([]port.ChatMessage, error) {
	if conversationID == "" {
		return nil, nil
	}
	msgs, err := m.msgRepo.ListByConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	out := make([]port.ChatMessage, 0, len(msgs))
	for _, msg := range msgs {
		// 跳过空内容（如流式中断产生的空 assistant 消息）
		if msg.Content == "" {
			continue
		}
		out = append(out, port.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	return out, nil
}

// 编译期断言：实现 port.ConversationMemory。
var _ port.ConversationMemory = (*DBConversationMemory)(nil)
