package port

import "context"

// ConversationMemory 管理一个会话的对话历史（边界接口）。
//
// 设计动机（整洁架构 / 依赖倒置）：
//   - 短期对话记忆是"用例需要的能力"，但记忆的"存储介质"是会变的细节
//     （内存 map / 数据库 / Redis / 向量库都可能）。按整洁架构，接口定义在
//     用例层（port），实现在适配器层，AI 生成器依赖接口而非具体实现。
//   - 这样换记忆存储介质（如从 DB 换 Redis）不动业务逻辑；也方便单测时
//     用 mock 替换真实存储。
//
// 当前用途：AI 生成器在首次创建某个会话的 runner 时，调用 History 取历史，
// 通过 trpc-agent-go 的 RunWithMessages 把历史 seed 进框架的 session，
// 实现"重启后旧会话续聊仍带上下文"。
type ConversationMemory interface {
	// History 返回指定会话的对话历史（按时间正序）。
	// 返回空切片表示无历史（新会话或首次启动）。
	// systemPrompt 由调用方单独传入，不在此方法职责内。
	History(ctx context.Context, conversationID string) ([]ChatMessage, error)
}
