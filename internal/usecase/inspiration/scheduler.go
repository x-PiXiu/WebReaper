package inspiration

import (
	"context"
	"log"
	"sync"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// StaggeredScheduler 分时段轮流爬取调度器（优化版）。
//
// 设计优化：
//   - Worker Pool 并发采集（默认 5 个 worker）
//   - 智能调度：跳过最近采集过的品牌
//   - 关键词轮换：从关键词池中取未使用过的关键词
//   - 夜间加速：00:00-06:00 间隔缩短
type StaggeredScheduler struct {
	uc          *UseCase
	configRepo  port.CrawlerConfigRepository
	accountRepo port.CrawlerAccountRepository

	taskQueue    chan BrandJob    // 任务队列（channel）
	retryQueue   []BrandJob      // 重试队列
	queueMu      sync.Mutex
	interval     time.Duration   // 基础间隔（白天）
	nightInterval time.Duration  // 夜间间隔
	workerCount  int             // 并发 worker 数量
	running      bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// BrandJob 品牌采集任务。
type BrandJob struct {
	Platform   string
	BrandID    string
	BrandName  string
	Industry   string
	Positioning string
	Keywords   []string
	RetryCount int
}

// SchedulerConfig 调度器配置。
type SchedulerConfig struct {
	Interval      time.Duration // 基础间隔（默认 15 分钟）
	NightInterval time.Duration // 夜间间隔（默认 5 分钟）
	WorkerCount   int           // 并发 worker 数量（默认 5）
}

// NewStaggeredScheduler 创建分时段调度器。
func NewStaggeredScheduler(uc *UseCase, configRepo port.CrawlerConfigRepository, accountRepo port.CrawlerAccountRepository, cfg *SchedulerConfig) *StaggeredScheduler {
	interval := 15 * time.Minute
	nightInterval := 5 * time.Minute
	workerCount := 5
	if cfg != nil {
		if cfg.Interval > 0 {
			interval = cfg.Interval
		}
		if cfg.NightInterval > 0 {
			nightInterval = cfg.NightInterval
		}
		if cfg.WorkerCount > 0 {
			workerCount = cfg.WorkerCount
		}
	}
	return &StaggeredScheduler{
		uc:            uc,
		configRepo:    configRepo,
		accountRepo:   accountRepo,
		taskQueue:     make(chan BrandJob, 100),
		interval:      interval,
		nightInterval: nightInterval,
		workerCount:   workerCount,
		stopCh:        make(chan struct{}),
	}
}

// Start 启动调度器。
func (s *StaggeredScheduler) Start(ctx context.Context) {
	if s.running {
		return
	}
	s.running = true
	log.Printf("[scheduler] 分时段调度器启动，间隔=%v，worker=%d", s.currentInterval(), s.workerCount)

	// 启动 worker pool
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i)
	}

	// 启动任务分发器
	go s.dispatcher(ctx)
}

// Stop 停止调度器。
func (s *StaggeredScheduler) Stop() {
	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
	s.wg.Wait()
	log.Printf("[scheduler] 分时段调度器已停止")
}

// dispatcher 任务分发器：定时从 DB 加载任务并分发到 worker。
func (s *StaggeredScheduler) dispatcher(ctx context.Context) {
	ticker := time.NewTicker(s.currentInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.loadAndDispatch(ctx)
			ticker.Reset(s.currentInterval())
		}
	}
}

// loadAndDispatch 从 DB 加载任务并分发到 worker。
func (s *StaggeredScheduler) loadAndDispatch(ctx context.Context) {
	configs, err := s.configRepo.ListAll(ctx)
	if err != nil {
		log.Printf("[scheduler] 加载配置失败: %v", err)
		return
	}

	now := time.Now()
	dispatched := 0
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}

		// 智能调度：跳过最近采集过的品牌
		if cfg.LastCrawledAt != nil {
			elapsed := now.Sub(*cfg.LastCrawledAt)
			if elapsed < time.Duration(cfg.CrawlIntervalMinutes)*time.Minute {
				continue
			}
		}

		// 获取关键词（从池中轮换）
		keywords := cfg.NextKeywords(3)
		if len(keywords) == 0 {
			// 关键词池为空，尝试用 LLM 生成
			if s.uc.llm != nil && cfg.BrandID != "" {
				log.Printf("[scheduler] 关键词池为空，尝试 LLM 生成 brand=%s", cfg.BrandID)
				generated, genErr := s.uc.GenerateKeywords(ctx, cfg.BrandID, cfg.BrandID, "")
				if genErr == nil && len(generated) > 0 {
					cfg.KeywordPool = generated
					cfg.LastKeywordIndex = 0
					keywords = cfg.NextKeywords(3)
					log.Printf("[scheduler] LLM 生成 %d 个关键词 brand=%s: %v", len(generated), cfg.BrandID, generated)
				}
			}
			// 仍然为空，使用基础关键词
			if len(keywords) == 0 {
				keywords = append(cfg.SearchKeywords, cfg.ExtraKeywords...)
			}
		}
		if len(keywords) == 0 {
			continue
		}

		// 更新关键词轮换指针和池
		s.configRepo.Save(ctx, cfg)

		job := BrandJob{
			Platform: cfg.Platform,
			BrandID:  cfg.BrandID,
			Keywords: keywords,
		}

		select {
		case s.taskQueue <- job:
			dispatched++
		default:
			log.Printf("[scheduler] 任务队列已满，跳过 brand=%s platform=%s", cfg.BrandID, cfg.Platform)
		}
	}

	if dispatched > 0 {
		log.Printf("[scheduler] 分发 %d 个采集任务", dispatched)
	}
}

// worker 工作协程：从任务队列取任务并执行。
func (s *StaggeredScheduler) worker(ctx context.Context, id int) {
	defer s.wg.Done()
	log.Printf("[scheduler] worker-%d 启动", id)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case job, ok := <-s.taskQueue:
			if !ok {
				return
			}
			s.executeJob(ctx, id, job)
		}
	}
}

// executeJob 执行单个采集任务。
func (s *StaggeredScheduler) executeJob(ctx context.Context, workerID int, job BrandJob) {
	log.Printf("[scheduler] worker-%d 执行采集 brand=%s platform=%s keywords=%v",
		workerID, job.BrandID, job.Platform, job.Keywords)

	result, err := s.uc.CrawlBrand(ctx, job.Platform, job.BrandID, job.Keywords)
	if err != nil {
		log.Printf("[scheduler] worker-%d 采集失败 brand=%s: %v", workerID, job.BrandID, err)
		// 加入重试队列
		if job.RetryCount < 3 {
			job.RetryCount++
			s.queueMu.Lock()
			s.retryQueue = append(s.retryQueue, job)
			s.queueMu.Unlock()
		}
		return
	}

	log.Printf("[scheduler] worker-%d 采集完成 brand=%s found=%d new=%d duration=%dms",
		workerID, job.BrandID, result.VideosFound, result.VideosNew, result.DurationMs)
}

// currentInterval 根据当前时间返回间隔。
func (s *StaggeredScheduler) currentInterval() time.Duration {
	hour := time.Now().Hour()
	if hour >= 0 && hour < 6 {
		return s.nightInterval
	}
	return s.interval
}

// QueueLen 返回队列长度（监控用）。
func (s *StaggeredScheduler) QueueLen() (int, int) {
	return len(s.taskQueue), len(s.retryQueue)
}

// LoadBrandsFromConfig 从配置表加载品牌任务。
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
		if len(keywords) == 0 && len(cfg.KeywordPool) == 0 {
			continue
		}
		s.AddBrand(cfg.Platform, cfg.BrandID, keywords)
	}

	log.Printf("[scheduler] 从配置加载任务")
	return nil
}

// AddBrand 添加品牌到采集队列。
func (s *StaggeredScheduler) AddBrand(platform, brandID string, keywords []string) {
	job := BrandJob{
		Platform: platform,
		BrandID:  brandID,
		Keywords: keywords,
	}
	select {
	case s.taskQueue <- job:
	default:
		log.Printf("[scheduler] 任务队列已满，跳过 brand=%s", brandID)
	}
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

// ---- 详情 API 定时刷新 ----

// MetricsRefresher 互动指标定时刷新器。
//
// 每 12 小时刷新旧数据的播放量/点赞/评论等指标。
// 搜索 API 不返回播放量，需要通过详情 API 补充。
type MetricsRefresher struct {
	uc         *UseCase
	videoRepo  port.InspirationVideoRepository
	refreshAge time.Duration // 多久未刷新的数据需要刷新
	batchSize  int           // 每批刷新数量
}

// NewMetricsRefresher 创建指标刷新器。
func NewMetricsRefresher(uc *UseCase, videoRepo port.InspirationVideoRepository) *MetricsRefresher {
	return &MetricsRefresher{
		uc:         uc,
		videoRepo:  videoRepo,
		refreshAge: 12 * time.Hour,
		batchSize:  20,
	}
}

// RefreshAll 刷新所有平台的旧数据指标。
func (r *MetricsRefresher) RefreshAll(ctx context.Context) error {
	// 遍历所有已注册的平台
	for _, platform := range r.uc.ListPlatforms() {
		if !r.uc.IsPlatformAlive(ctx, platform) {
			log.Printf("[metrics] 平台 %s 不可用，跳过刷新", platform)
			continue
		}

		// 获取该平台需要刷新的视频
		videos, _, err := r.videoRepo.List(ctx, "", platform, "", "created_at", 1, r.batchSize)
		if err != nil {
			log.Printf("[metrics] 获取 %s 视频列表失败: %v", platform, err)
			continue
		}

		// 过滤出需要刷新的视频（超过 refreshAge 未刷新）
		var toRefresh []string
		for _, v := range videos {
			if v.LastRefreshedAt == nil || time.Since(*v.LastRefreshedAt) > r.refreshAge {
				toRefresh = append(toRefresh, v.PlatformVideoID)
			}
		}

		if len(toRefresh) == 0 {
			log.Printf("[metrics] %s 无需刷新的视频", platform)
			continue
		}

		// 调用详情 API 刷新
		updated, err := r.uc.RefreshMetrics(ctx, platform, toRefresh)
		if err != nil {
			log.Printf("[metrics] %s 刷新失败: %v", platform, err)
			continue
		}

		log.Printf("[metrics] %s 刷新完成: %d/%d 更新", platform, updated, len(toRefresh))
	}

	return nil
}
