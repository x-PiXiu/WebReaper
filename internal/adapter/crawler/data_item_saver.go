package crawler

import (
	"context"
)

// DataItemSaver 把内容保存为 DataItem 的能力（供 Agent 工具调用）。
//
// 设计动机（DIP）：crawler 包的 SaveDataItem 工具需要保存数据，
// 但不能反向依赖 usecase/dataitem 包（违反依赖方向）。
// 由 main 装配时把 DataItemUseCase 适配为此接口注入。
type DataItemSaver interface {
	// SaveFromContent 把原始内容（通常是 LLM 生成的结构化 JSON）保存为 DataItem。
	// fieldMapping 是字段映射 JSON（可空，空则整体作为 content）。
	// 返回保存后的 DataItem ID 和标题。
	SaveFromContent(ctx context.Context, content, fieldMapping, sourceURL string) (id, title string, err error)
}
