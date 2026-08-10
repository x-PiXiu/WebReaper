package port

import (
	"context"
	"time"
)

// ---- 通用定时任务域（调度器 + 任务契约 + 分布式锁）----
//
// 设计动机（避免"一个功能一套定时任务"）：
//   - 每个业务功能只需实现 ScheduledTask 接口并注册，调度器统一驱动：
//     防重入（长任务不重叠）、分布式锁（多实例不重复执行）、panic 恢复、错误日志。
//   - 未来新增定时功能（每日监测/收录重试/报表生成/视频任务轮询）=
//     实现一个接口 + Register 一行，零样板代码。
//   - 分布式演进：单机装配 noop 锁，多实例装配 Redis 锁，业务零改动。

// ScheduledTask 定时任务契约（业务功能实现 + 注册）。
type ScheduledTask interface {
	// Name 唯一任务名（作为分布式锁键与日志标识）。
	Name() string
	// Interval 执行间隔。
	Interval() time.Duration
	// Execute 任务逻辑（失败返回 error，调度器统一记日志，不影响后续周期）。
	Execute(ctx context.Context) error
}

// TaskLock 分布式执行锁（防多实例重复执行）。
// 单机部署用 noop 实现；多实例部署用 Redis/DB 实现，业务零改动。
type TaskLock interface {
	// TryLock 尝试获取锁（key 唯一；ttl 防死锁）。获取失败返回 false（别的实例在跑）。
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// Unlock 释放锁。
	Unlock(ctx context.Context, key string) error
}

// TaskScheduler 通用调度器接口（注册表 + 统一驱动）。
type TaskScheduler interface {
	// Register 注册任务（重复注册同名任务返回错误）。
	Register(task ScheduledTask) error
	// Start 启动全部任务（每任务独立 goroutine + ticker）。
	Start(ctx context.Context)
	// Stop 停止全部任务（等待当前执行中的任务结束）。
	Stop()
}
