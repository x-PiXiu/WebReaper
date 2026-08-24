package inspiration

import (
	"context"
	"log"
	"sync"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// StaggeredScheduler 分时段轮流爬取调度器。
//
// 设计（参考设计方案 5.2 节）：
//   - 品牌队列按 sort_order 排序
//   - 每 15 分钟处理一个品牌（可配置）
//   - 每处理 N 个品牌切换一次账号
//   - 失败的品牌加入重试队列
//   - 夜间（00:00-06:00）可加速
type StaggeredScheduler struct {
	uc          *UseCase
	configRepo  port.CrawlerConfigRepository
	accountRepo port.CrawlerAccountRepository

	brandQueue   []BrandJob      // 品牌任务队列
	retryQueue   []BrandJob      // 重试队列
	queueMu      sync.Mutex
	interval     time.Duration   // 基础间隔
	accountsPerN int             // 每 N 个品牌切换一次账号
	running      bool
	stopCh       chan struct{}
}

// BrandJob 品牌采集任务。
type BrandJob struct {
	Platform  string
	BrandID   string
	Keywords  []string
	RetryCount int
}

// SchedulerConfig 调度器配置。
type SchedulerConfig struct {
	Interval     time.Duration // 基础间隔（默认 15 分钟）
	AccountsPerN int           // 每 N 个品牌切换一次账号（默认 3）
}

// NewStaggeredScheduler 创建分时段调度器。
func NewStaggeredScheduler(uc *UseCase, configRepo port.CrawlerConfigRepository, accountRepo port.CrawlerAccountRepository, cfg *SchedulerConfig) *StaggeredScheduler {
	interval := 15 * time.Minute
	accountsPerN := 3
	if cfg != nil {
		if cfg.Interval > 0 {
			interval = cfg.Interval
		}
		if cfg.AccountsPerN > 0 {
			accountsPerN = cfg.AccountsPerN
		}
	}
	return &StaggeredScheduler{
		uc:           uc,
		configRepo:   configRepo,
		accountRepo:  accountRepo,
		interval:     interval,
		accountsPerN: accountsPerN,
		stopCh:       make(chan struct{}),
	}
}

// AddBrand 添加品牌到采集队列。
func (s *StaggeredScheduler) AddBrand(platform, brandID string, keywords []string) {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	s.brandQueue = append(s.brandQueue, BrandJob{
		Platform: platform,
		BrandID:  brandID,
		Keywords: keywords,
	})
}

// Start 启动调度器。
func (s *StaggeredScheduler) Start(ctx context.Context) {
	if s.running {
		return
	}
	s.running = true
	log.Printf("[scheduler] 分时段调度器启动，间隔=%v", s.interval)

	go s.run(ctx)
}

// Stop 停止调度器。
func (s *StaggeredScheduler) Stop() {
	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
	log.Printf("[scheduler] 分时段调度器已停止")
}

// run 调度器主循环。
func (s *StaggeredScheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.executeNext(ctx)
		}
	}
}

// executeNext 执行队列中的下一个品牌。
func (s *StaggeredScheduler) executeNext(ctx context.Context) {
	s.queueMu.Lock()
	// 如果主队列为空，把重试队列合并进来
	if len(s.brandQueue) == 0 && len(s.retryQueue) > 0 {
		s.brandQueue = s.retryQueue
		s.retryQueue = nil
		log.Printf("[scheduler] 重试队列合并到主队列，共 %d 个任务", len(s.brandQueue))
	}
	if len(s.brandQueue) == 0 {
		s.queueMu.Unlock()
		return
	}
	// 取出队列头部
	job := s.brandQueue[0]
	s.brandQueue = s.brandQueue[1:]
	s.queueMu.Unlock()

	log.Printf("[scheduler] 执行采集 platform=%s brand=%s keywords=%v", job.Platform, job.BrandID, job.Keywords)

	// 执行采集
	result, err := s.uc.CrawlBrand(ctx, job.Platform, job.BrandID, job.Keywords)
	if err != nil {
		log.Printf("[scheduler] 采集失败 platform=%s brand=%s: %v", job.Platform, job.BrandID, err)
		// 加入重试队列（最多重试 3 次）
		if job.RetryCount < 3 {
			job.RetryCount++
			s.queueMu.Lock()
			s.retryQueue = append(s.retryQueue, job)
			s.queueMu.Unlock()
			log.Printf("[scheduler] 加入重试队列（第 %d 次重试）", job.RetryCount)
		}
		return
	}

	log.Printf("[scheduler] 采集完成 platform=%s brand=%s found=%d new=%d duration=%dms",
		job.Platform, job.BrandID, result.VideosFound, result.VideosNew, result.DurationMs)

	// 更新配置表的最后爬取时间
	if err := s.configRepo.UpdateLastCrawled(ctx, job.Platform); err != nil {
		log.Printf("[scheduler] 更新最后爬取时间失败: %v", err)
	}
}

// LoadBrandsFromConfig 从配置表加载所有品牌到队列。
func (s *StaggeredScheduler) LoadBrandsFromConfig(ctx context.Context) error {
	configs, err := s.configRepo.ListAll(ctx)
	if err != nil {
		return err
	}

	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		keywords := append(cfg.SearchKeywords, cfg.ExtraKeywords...)
		if len(keywords) == 0 {
			continue
		}
		// 每个配置的 tenant_id 作为一个品牌
		s.AddBrand(cfg.Platform, cfg.TenantID, keywords)
	}

	log.Printf("[scheduler] 从配置加载 %d 个品牌任务", len(s.brandQueue))
	return nil
}

// QueueLen 返回队列长度（监控用）。
func (s *StaggeredScheduler) QueueLen() (main int, retry int) {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	return len(s.brandQueue), len(s.retryQueue)
}

// ---- 定时健康检查 ----

// AccountHealthChecker 平台方账号健康检查器。
type AccountHealthChecker struct {
	accountRepo port.CrawlerAccountRepository
	platforms   map[string]port.CrawlerPlatform
}

// NewAccountHealthChecker 创建健康检查器。
func NewAccountHealthChecker(accountRepo port.CrawlerAccountRepository, platforms map[string]port.CrawlerPlatform) *AccountHealthChecker {
	return &AccountHealthChecker{
		accountRepo: accountRepo,
		platforms:   platforms,
	}
}

// CheckAll 检查所有账号的健康状态。
func (h *AccountHealthChecker) CheckAll(ctx context.Context) error {
	accounts, err := h.accountRepo.ListAll(ctx)
	if err != nil {
		return err
	}

	for _, acc := range accounts {
		crawler, ok := h.platforms[acc.Platform]
		if !ok {
			continue
		}

		alive := crawler.IsAlive(ctx)
		result := entity.HealthHealthy
		if !alive {
			result = entity.HealthUnhealthy
		}

		if err := h.accountRepo.UpdateHealth(ctx, acc.ID, result); err != nil {
			log.Printf("[health] 更新健康状态失败 account=%d: %v", acc.ID, err)
		}
	}

	return nil
}

// ---- 每日用量重置 ----

// DailyUsageResetter 每日用量重置器。
type DailyUsageResetter struct {
	accountRepo port.CrawlerAccountRepository
}

// NewDailyUsageResetter 创建每日用量重置器。
func NewDailyUsageResetter(accountRepo port.CrawlerAccountRepository) *DailyUsageResetter {
	return &DailyUsageResetter{accountRepo: accountRepo}
}

// Reset 重置所有账号的每日使用次数。
func (r *DailyUsageResetter) Reset(ctx context.Context) error {
	return r.accountRepo.ResetDailyUsage(ctx)
}
