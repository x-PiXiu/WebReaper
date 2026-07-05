package task

import (
	"context"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/domain/valueobject"
	"webreaper/internal/usecase/port"
)

// Worker 是后台任务消费者。
//
// 它在一个独立 goroutine 中循环：Dequeue 任务 → 更新为 running →
// 调 DispatchUseCase 执行 → 更新为 succeeded/failed → Ack。
//
// Worker 是用例层的一部分，只依赖 port 接口，不依赖 HTTP/队列框架。
// context 取消时优雅退出。
type Worker struct {
	queue     port.TaskQueue
	repo      port.TaskRepository
	dispatch  *DispatchUseCase
	logger    port.Logger
	maxRetries int // 最大重试次数（默认 3，测试可设 1 跳过重试）
}

// NewWorker 创建后台任务消费者。
func NewWorker(queue port.TaskQueue, repo port.TaskRepository, dispatch *DispatchUseCase, logger port.Logger) *Worker {
	if logger == nil {
		logger = port.NopLogger{}
	}
	return &Worker{
		queue: queue, repo: repo, dispatch: dispatch,
		logger: logger.With(port.String("component", "worker")),
		maxRetries: 3,
	}
}

// WithMaxRetries 设置最大重试次数（链式，供测试用 1 跳过重试）。
func (w *Worker) WithMaxRetries(n int) *Worker { w.maxRetries = n; return w }

// Start 启动后台消费循环，阻塞直到 ctx 取消或队列关闭。
// 应在独立 goroutine 中调用：go worker.Start(ctx)。
func (w *Worker) Start(ctx context.Context) {
	w.logger.Info("任务 Worker 已启动")
	defer w.logger.Info("任务 Worker 已停止")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Dequeue 会阻塞直到有任务或 ctx 取消
		task, err := w.queue.Dequeue(ctx)
		if err != nil {
			// ctx 取消导致的 Dequeue 错误，正常退出
			if ctx.Err() != nil {
				return
			}
			w.logger.Error("出队失败", port.Err(err))
			time.Sleep(time.Second) // 避免错误时疯狂循环
			continue
		}

		w.processTask(ctx, task)
	}
}

// processTask 处理单个任务：状态流转 running → succeeded/failed，带指数退避重试。
func (w *Worker) processTask(ctx context.Context, task entity.Task) {
	// 标记为 running
	if err := w.repo.UpdateStatus(ctx, task.ID, valueobject.TaskStatusRunning, ""); err != nil {
		w.logger.Error("更新任务状态失败(running)", port.String("task_id", task.ID), port.Err(err))
	}
	// 写入初始进度（让前端能立刻看到"已开始"而非空白）
	if err := w.repo.UpdateProgress(ctx, task.ID, "任务已开始执行..."); err != nil {
		w.logger.Warn("更新任务进度失败", port.String("task_id", task.ID), port.Err(err))
	}

	// 执行（带重试，指数退避：1s → 2s → 4s）
	maxRetries := w.maxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}
	var output string
	var execErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			backoff := time.Duration(1<<(attempt-2)) * time.Second // 1s, 2s, 4s
			progressMsg := fmt.Sprintf("第 %d/%d 次重试中（%v 后）...", attempt, maxRetries, backoff)
			_ = w.repo.UpdateProgress(ctx, task.ID, progressMsg)
			w.logger.Info("任务重试", port.String("task_id", task.ID),
				port.String("attempt", fmt.Sprintf("%d/%d", attempt, maxRetries)))
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				// ctx 取消：放弃重试，以最后一次错误终止
				if execErr == nil {
					execErr = ctx.Err()
				}
				goto done
			}
		}
		output, execErr = w.dispatch.Execute(ctx, task)
		if execErr == nil {
			break // 成功则跳出重试循环
		}
	}
done:

	// 根据结果更新状态
	if execErr != nil {
		if err := w.repo.UpdateStatus(ctx, task.ID, valueobject.TaskStatusFailed, execErr.Error()); err != nil {
			w.logger.Error("更新任务状态失败(failed)", port.String("task_id", task.ID), port.Err(err))
		}
	} else {
		// 存储输出（Agent 的回复/采集结果）
		if output != "" {
			if err := w.repo.UpdateOutput(ctx, task.ID, output); err != nil {
				w.logger.Warn("存储任务输出失败", port.String("task_id", task.ID), port.Err(err))
			}
		}
		// 清空进度（已完成）
		_ = w.repo.UpdateProgress(ctx, task.ID, "")
		if err := w.repo.UpdateStatus(ctx, task.ID, valueobject.TaskStatusSucceeded, ""); err != nil {
			w.logger.Error("更新任务状态失败(succeeded)", port.String("task_id", task.ID), port.Err(err))
		}
	}

	// 确认消费（无论成败）
	errMsg := ""
	if execErr != nil {
		errMsg = execErr.Error()
	}
	if err := w.queue.Ack(ctx, task.ID, errMsg); err != nil {
		w.logger.Warn("Ack 任务失败", port.String("task_id", task.ID), port.Err(err))
	}
}
