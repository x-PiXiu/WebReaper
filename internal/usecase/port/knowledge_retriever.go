package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// KnowledgeRetriever 知识库素材检索器（生成前注入点，adapter 实现）。
//
// 设计动机（素材溯源 + 本地优先）：
//   - 用户生成内容前按"品牌行业 + 关键词"检索平台知识库素材，
//     返回的 MaterialRef 自带来源 URL——LLM 引用有据可查。
//   - 本地知识库优先于实时全网检索（省钱、稳定、可溯源）；返回空列表 =
//     无命中，调用方降级为在线 RAG（行为与旧版一致）。
type KnowledgeRetriever interface {
	// Retrieve 检索知识库素材（industry 为空 = 全行业）。
	// num 为期望数量；实现可返回更少（阈值过滤后）。nil 列表 = 无命中。
	Retrieve(ctx context.Context, industry, query string, num int) ([]entity.MaterialRef, error)
}
