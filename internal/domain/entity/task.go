package entity

import (
	"time"

	"webreaper/internal/domain/valueobject"
)

// TaskType 表示任务类型。
//
// 架构演进说明：项目早期设计了「采集/生成面试题/总结知识点」三类独立任务，
// 后续整体转向「单一 Agent + N 工具」模型——采集、加工都由 Agent 通过工具调用完成，
// 原有的三类任务无对应 Handler 且不再使用，已移除。当前仅保留 Agent 异步执行任务。
type TaskType string

const (
	TaskTypeAgentRun TaskType = "agent_run"
)

// Task 表示一个异步执行的任务。
// Agent 执行是耗时操作，统一抽象为 Task 进入队列异步执行，
// 不阻塞 Web 请求。
type Task struct {
	ID        string
	Type      TaskType
	Input     string    // 任务输入（JSON 序列化后的参数，具体结构由用例解析）
	Output    string    // 任务输出（JSON 序列化后的结果）
	Progress  string    // 运行中进度描述（如"正在采集..."），供前端实时展示
	Status    valueobject.TaskStatus
	Error     string    // 失败时的错误信息
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CanStart 判断任务是否可以开始执行（必须处于 pending 态）。
func (t Task) CanStart() bool {
	return t.Status == valueobject.TaskStatusPending
}

// IsDone 判断任务是否已结束（含成功/失败/取消）。
func (t Task) IsDone() bool {
	return t.Status.IsTerminal()
}

// TransitionTo 安全地迁移任务状态，违反状态机规则则返回 false。
func (t *Task) TransitionTo(target valueobject.TaskStatus) bool {
	if !t.Status.CanTransitionTo(target) {
		return false
	}
	t.Status = target
	t.UpdatedAt = time.Now()
	return true
}
