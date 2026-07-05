package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// TaskQueue 是任务队列的抽象接口（边界）。
//
// 用例层通过此接口投递/消费任务，不关心底层是 Go channel（进程内）
// 还是 Redis/Kafka（分布式）。骨架阶段用内存 channel 实现，
// 未来升级到 Redis 时用例层零修改。
type TaskQueue interface {
	// Enqueue 投递一个任务到队列。
	Enqueue(ctx context.Context, task entity.Task) error

	// Dequeue 阻塞地获取一个待执行任务。
	Dequeue(ctx context.Context) (entity.Task, error)

	// Ack 确认任务处理完成（或失败）。具体语义由实现决定。
	Ack(ctx context.Context, taskID string, errMsg string) error

	// Close 关闭队列，释放资源。
	Close() error
}
