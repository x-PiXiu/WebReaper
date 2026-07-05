package valueobject

// TaskStatus 表示任务的生命周期状态。
// 这是一个值对象：不可变、无唯一标识，只描述领域中的"状态"概念。
//
// 状态机：
//   Pending -> Running -> Succeeded
//                      \-> Failed
//                      \-> Cancelled
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusSucceeded TaskStatus = "succeeded"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// CanTransitionTo 判断是否允许从当前状态迁移到目标状态。
// 领域规则：终态（Succeeded/Failed/Cancelled）不可再迁移。
func (s TaskStatus) CanTransitionTo(target TaskStatus) bool {
	if s.IsTerminal() {
		return false
	}
	transitions := map[TaskStatus][]TaskStatus{
		TaskStatusPending: {TaskStatusRunning, TaskStatusCancelled},
		TaskStatusRunning: {TaskStatusSucceeded, TaskStatusFailed, TaskStatusCancelled},
	}
	allowed, ok := transitions[s]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == target {
			return true
		}
	}
	return false
}

// IsTerminal 判断是否为终态（不会再变化）。
func (s TaskStatus) IsTerminal() bool {
	switch s {
	case TaskStatusSucceeded, TaskStatusFailed, TaskStatusCancelled:
		return true
	}
	return false
}
