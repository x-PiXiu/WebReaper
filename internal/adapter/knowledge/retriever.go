// Package knowledge 提供"平台知识库"的检索与采集适配层。
//
// 整洁架构定位：adapter 层的"框架与驱动"——向量化（Embedder）、素材检索
// （KnowledgeMaterialRepository）等协议细节封在这里，用例层只依赖 port 接口。
// 采集编排在 usecase/knowledge（知识域用例），本包只做协议实现。
package knowledge

import (
	"context"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// defaultMinScore 业务相似度阈值：低于此余弦相似度的素材视为"不相关"，
// 不注入生成 prompt（保证只把高相关素材送 LLM，质量护栏的一环）。
const defaultMinScore = float32(0.25)

// KnowledgeRetrieverImpl 是 port.KnowledgeRetriever 的向量检索实现。
//
// 检索链路：embed(query) → repo.SearchSimilar(行业, 向量, 预取) → 阈值过滤 → topN。
// 职责划分：
//   - repo 守余弦自然边界（score > 0 才返回）
//   - 本实现守业务阈值（minScore）与数量裁剪（num）
type KnowledgeRetrieverImpl struct {
	repo     port.KnowledgeMaterialRepository
	embedder port.Embedder
	minScore float32
}

// NewKnowledgeRetriever 创建知识库检索器。
func NewKnowledgeRetriever(repo port.KnowledgeMaterialRepository, embedder port.Embedder) *KnowledgeRetrieverImpl {
	return &KnowledgeRetrieverImpl{repo: repo, embedder: embedder, minScore: defaultMinScore}
}

// Retrieve 检索知识库素材（industry 空 = 全行业）。
// 返回空列表 = 无命中（调用方降级为在线 RAG，行为与旧版一致）。
func (r *KnowledgeRetrieverImpl) Retrieve(ctx context.Context, industry, query string, num int) ([]entity.MaterialRef, error) {
	if r.repo == nil || r.embedder == nil || query == "" {
		return nil, nil
	}
	if num <= 0 {
		num = 3
	}
	vec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err // embedding 不可用：调用方降级
	}
	// 预取 num×3（阈值过滤后可能不足 num），由 repo 按余弦降序返回
	refs, err := r.repo.SearchSimilar(ctx, industry, vec, num*3)
	if err != nil {
		return nil, err
	}

	out := make([]entity.MaterialRef, 0, len(refs))
	for _, ref := range refs {
		if ref.Score < r.minScore {
			continue // 业务阈值：不相关素材不进生成 prompt
		}
		out = append(out, ref)
	}
	if len(out) > num {
		out = out[:num]
	}
	return out, nil
}
