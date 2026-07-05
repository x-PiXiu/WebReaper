package port

import "context"

// KnowledgeSearcher 知识检索能力接口（用例层声明，适配器/工具实现）。
//
// 设计动机（DIP + 消除重复定义）：
//   - 原先 adapter/crawler/knowledge_search.go 和 adapter/handler/data_handler.go
//     各自定义了一份同构的 ProcessUseCaseInterface，破坏"单一接口定义点"。
//   - 现统一上移到 port 层，由 usecase/process.ProcessUseCase 实现，
//     adapter/crawler 和 adapter/handler 都依赖此接口。
//
// 这个接口只暴露"知识检索"能力，不暴露 ProcessItem（那是 ReviewUseCase 的职责），
// 遵循接口隔离原则（ISP）——调用方只看到它需要的方法。
type KnowledgeSearcher interface {
	// SearchKnowledge 在已采集并向量化的知识库中语义检索。
	SearchKnowledge(ctx context.Context, query string, topK int) ([]VectorSearchResult, error)
}
