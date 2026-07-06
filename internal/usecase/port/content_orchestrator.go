package port

import "context"

// ContentOrchestrator 编排"框架内容生产"流程（用例层声明，适配器实现）。
//
// 设计动机（DIP + 整洁架构）：
//   - "按主题生成结构化内容并校验完整性"是应用级业务流程，归用例层编排。
//   - 但流程内部的具体实现（图编排 / 单 Agent / 别的框架）是易变的技术细节，
//     通过本接口隔离——用例只依赖接口，adapter 用 graphagent 等实现。
//   - 这样换实现（如换 LangGraph 风格的别的库）时用例零改动。
//
// 典型实现：adapter/agent.GraphContentOrchestrator（trpc-agent-go 的 graphagent）。
type ContentOrchestrator interface {
	// Orchestrate 按主题编排内容生产，返回生成的结构化条目。
	//
	// 流程（由实现决定，典型为图编排）：
	//   探查主题范围 → 逐项生成 → 校验完整性 → 不完整则补生成 → 完成才返回。
	//
	// onProgress 回调用于实时上报进度（如"正在分析模块 X..."），可为 nil。
	Orchestrate(ctx context.Context, in OrchestrateInput, onProgress func(msg string)) ([]OrchestrateItem, error)
}

// OrchestrateInput 编排输入。
type OrchestrateInput struct {
	Topic       string // 主题，如 "trpc-agent-go 框架"、"Gin Web 框架"
	ContentType string // 内容类型，如 "interview_questions"（面试题）、"knowledge_summary"（知识点）
}

// OrchestrateItem 编排产出的单条结构化内容。
//
// 这是 port 层 DTO（不依赖 graphagent 等框架类型），
// 用例层把它转成 DataItem 落库。
type OrchestrateItem struct {
	Title   string   // 条目标题（如题目标题）
	Content string   // 条目正文（如题目+答案+解析）
	Tags    []string // 标签（如关联的技术领域）
	Module  string   // 所属模块（用于完整性校验，如 "agent"、"graph"）
}
