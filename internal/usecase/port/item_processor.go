package port

import "context"

// ItemProcessor 数据项结构化处理能力接口（用例层声明，适配器/其他用例实现）。
//
// 设计动机（DIP + ISP）：
//   - usecase/dataitem.Approve 需要在审核通过后触发"结构化+向量化"，
//     但不应反向依赖 usecase/process 包（那会形成 usecase 之间的横向耦合）。
//   - 把 ProcessItem 抽象为 port 接口，由 usecase/process.ProcessUseCase 实现，
//     在 main 装配时注入到 dataitem usecase。
//   - 与 port.KnowledgeSearcher 分离：一个管"处理"、一个管"检索"，
//     遵循接口隔离原则，调用方只看到自己需要的方法。
type ItemProcessor interface {
	// ProcessItem 对单条数据项执行结构化提取与向量化。
	ProcessItem(ctx context.Context, itemID string) error
}
