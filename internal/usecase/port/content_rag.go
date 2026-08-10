package port

import "context"

// ContentRAGRetriever 内容生成 RAG 检索器（可选注入，nil=不启用）。
//
// 设计动机（"不编造数据"从口号变能力）：
//   - 内容生成前检索"品牌 + 关键词"真实信息注入 prompt，LLM 引用真实资料创作，
//     显著提升内容被 AI 引擎引用的概率（有数据支撑 = 权威性维度得分高）。
//   - 可选注入：未配置时行为不变（纯 LLM 推断），配置后自动增强。
type ContentRAGRetriever interface {
	// RetrieveContent 按 query 检索全网并返回内容摘要（供 LLM 参考）。
	// num 为期望返回的文档数；实现可截断/降级，返回空串表示无可用内容。
	RetrieveContent(ctx context.Context, query string, num int) (string, error)
}
