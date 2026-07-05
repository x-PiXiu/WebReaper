package entity

import "time"

// CollectionStatus 采集集合的状态。
type CollectionStatus string

const (
	CollectionStatusCollecting CollectionStatus = "collecting" // 采集中
	CollectionStatusCompleted  CollectionStatus = "completed"  // 采集完成
	CollectionStatusFailed     CollectionStatus = "failed"     // 采集失败
)

// Collection 是采集集合——一次 Agent 任务产生的数据集。
//
// 用户视角的分组："Go技术文章合集"、"竞品分析数据"等。
// 一个 Collection 包含多个 DataItem，关联一个 Agent。
type Collection struct {
	ID        string
	Name      string            // 用户起的名字
	AgentName string            // 用哪个 Agent 采的
	TaskID    string            // 关联的异步任务
	Status    CollectionStatus
	ItemCount int               // 数据项数量
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsCompleted 判断采集是否完成。
func (c Collection) IsCompleted() bool {
	return c.Status == CollectionStatusCompleted
}
