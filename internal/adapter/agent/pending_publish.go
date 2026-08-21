package agent

import (
	"sync"
	"time"

	"webreaper/internal/usecase/account"
)

// PendingPublishStore 发布计划暂存（硬确认机制的 pending 层）。
//
// 流程：publish_work 未确认调用 → 组装完整 PublishInput 存入（plan_id 索引）→
// 返回 plan_id 给前端 → 前端确认卡片点「确认发布」→ REST 端点按 plan_id 取出执行。
// 模型全程只能拿到 plan_id，无法伪造确认（确认动作来自用户点击的 REST 调用，
// 与 Agent 对话链路完全分离）——这就是"UI 级强制"，软确认之上的硬闸。
//
// scheduled_at 复用：确认时可带定时时间 → 走 PublishInput.ScheduledAt 排期通道，
// pending 层同时服务"立即确认"和"定时发布"两个场景。
//
// 注：内存版（10 分钟过期自动清理）。多实例部署时确认请求需路由到同一实例——
// 上多实例前改为 Redis 存储（SetGet 带 TTL），接口不变。
type PendingPublishStore struct {
	mu    sync.Mutex
	plans map[string]pendingPlan
}

type pendingPlan struct {
	Input     account.PublishInput // 完整发布输入（确认后直接执行）
	Title     string               // 展示用作品名
	ExpiresAt time.Time
}

const pendingPlanTTL = 10 * time.Minute

func NewPendingPublishStore() *PendingPublishStore {
	return &PendingPublishStore{plans: make(map[string]pendingPlan)}
}

// Save 存入发布计划，返回 plan_id。
func (s *PendingPublishStore) Save(input account.PublishInput, title string) string {
	id := "plan-" + input.Platform + "-" + time.Now().Format("20060102150405.000000000")
	s.mu.Lock()
	defer s.mu.Unlock()
	// 顺带清理过期项（懒清理足够——TTL 短、量级小）
	now := time.Now()
	for k, p := range s.plans {
		if now.After(p.ExpiresAt) {
			delete(s.plans, k)
		}
	}
	s.plans[id] = pendingPlan{Input: input, Title: title, ExpiresAt: now.Add(pendingPlanTTL)}
	return id
}

// Take 取出并删除（确认执行/取消都消费掉——一次性）。
func (s *PendingPublishStore) Take(planID string) (account.PublishInput, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.plans[planID]
	if !ok || time.Now().After(p.ExpiresAt) {
		delete(s.plans, planID)
		return account.PublishInput{}, "", false
	}
	delete(s.plans, planID)
	return p.Input, p.Title, true
}
