// Package scheduler 实现通用定时任务调度器。
//
// 整洁架构定位：
//   - 业务功能实现 port.ScheduledTask 接口后注册，本包统一驱动。
//   - 职责单一：周期触发 + 防重入 + 分布式锁 + panic 恢复 + 错误日志。
//   - 业务不感知调度细节；调度器不感知业务内容（接口隔离）。
//
// 分布式演进：lock 传 nil = 单机直跑（无锁）；多实例部署传 Redis 锁实现，
// 任务间通过锁键互斥，保证同一时刻只有一个实例执行同一任务。
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"webreaper/internal/usecase/port"
)

// Scheduler 通用定时任务调度器（注册表 + 每任务独立 goroutine 驱动）。
type Scheduler struct {
	mu     sync.Mutex
	tasks  map[string]port.ScheduledTask
	lock   port.TaskLock // 分布式锁（nil = 单机直跑）
	logger port.Logger
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New 创建调度器。
// lock 为 nil 时单机直跑（无分布式互斥）；多实例部署传入 Redis/DB 锁实现。
func New(lock port.TaskLock, logger port.Logger) *Scheduler {
	if logger == nil {
		logger = port.NopLogger{}
	}
	return &Scheduler{
		tasks:  make(map[string]port.ScheduledTask),
		lock:   lock,
		logger: logger,
	}
}

// Register 注册任务（同名重复注册报错）。
func (s *Scheduler) Register(task port.ScheduledTask) error {
	if task == nil || task.Name() == "" {
		return fmt.Errorf("任务名为空")
	}
	if task.Interval() <= 0 {
		return fmt.Errorf("任务 %s 间隔必须 > 0", task.Name())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tasks[task.Name()]; exists {
		return fmt.Errorf("任务 %s 已注册", task.Name())
	}
	s.tasks[task.Name()] = task
	return nil
}

// Start 启动全部任务（每任务独立 goroutine）。
func (s *Scheduler) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	s.mu.Lock()
	tasks := make([]port.ScheduledTask, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	s.mu.Unlock()

	for _, task := range tasks {
		s.wg.Add(1)
		go s.run(ctx, task)
		s.logger.Info("定时任务已启动",
			port.String("task", task.Name()),
			port.String("interval", task.Interval().String()),
		)
	}
}

// Stop 停止全部任务（等待当前执行中的任务结束）。
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

// run 单个任务驱动循环：ticker 触发 → 防重入 → 分布式锁 → 执行 → 恢复。
// 动态间隔（2026-08-14）：每个周期执行后重新读取 task.Interval()——任务可从
// 运行时配置（如 system_settings）读周期，管理后台改间隔免重启。现有任务
// Interval() 是常量，刷新后不变，行为与旧版完全一致（向后兼容）。
func (s *Scheduler) run(ctx context.Context, task port.ScheduledTask) {
	defer s.wg.Done()
	interval := task.Interval()
	if interval <= 0 {
		interval = time.Hour // 防御：非法间隔兜底（Register 已校验，此处双保险）
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	running := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if running {
				s.logger.Warn("定时任务上次执行未完成，跳过本次周期",
					port.String("task", task.Name()))
				continue
			}
			running = true
			s.executeOnce(ctx, task)
			running = false
			// 动态间隔：周期结束刷新 ticker（任务可改配置调整周期，免重启）
			if d := task.Interval(); d > 0 && d != interval {
				interval = d
				ticker.Reset(interval)
			}
		}
	}
}

// executeOnce 单次执行：分布式锁（可选）→ Execute → panic 恢复 + 错误日志。
func (s *Scheduler) executeOnce(ctx context.Context, task port.ScheduledTask) {
	taskCtx := ctx
	lockKey := "scheduler:" + task.Name()

	// 分布式锁（nil = 单机直跑）
	if s.lock != nil {
		ok, err := s.lock.TryLock(ctx, lockKey, 2*task.Interval())
		if err != nil {
			s.logger.Error("定时任务加锁失败",
				port.Err(err), port.String("task", task.Name()))
			return
		}
		if !ok {
			// 其他实例正在执行本任务——跳过（分布式互斥）
			return
		}
		defer func() { _ = s.lock.Unlock(context.Background(), lockKey) }()
	}

	// panic 恢复：单个任务崩溃不影响调度器与其他任务
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("定时任务 panic",
				port.String("task", task.Name()),
				port.String("panic", fmt.Sprintf("%v", r)))
		}
	}()

	start := time.Now()
	if err := task.Execute(taskCtx); err != nil {
		s.logger.Error("定时任务执行失败",
			port.Err(err), port.String("task", task.Name()))
		return
	}
	s.logger.Info("定时任务执行完成",
		port.String("task", task.Name()),
		port.String("duration", time.Since(start).Round(time.Millisecond).String()))
}
