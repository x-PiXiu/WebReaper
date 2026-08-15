// 知识库采集任务：按行业配置持续爬取网页内容入库（素材带来源溯源）。
//
// 需求链路（Docs/Plans/04）：
//   平台按行业配置关键词定期采集 → 入库（来源 URL + 原文 + 向量）
//   → 生成时按"品牌行业+关键词"检索素材（带来源）→ 规格化 prompt → 上游 LLM。
// 未配置行业（system_settings kb_crawl_industries 为空）时任务空转（不报错）。
//
// 采集间隔动态化（2026-08-14）：Interval() 从 system_settings[kb_crawl_interval_minutes]
// 读取（TTL 缓存 30s），管理后台可改（30-1440 分钟）——配合 scheduler 的
// "每周期刷新 Interval" 能力，改间隔免重启、下个周期生效。
package scheduledtask

import (
	"context"
	"sync"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/knowledge"
	"webreaper/internal/usecase/port"
)

// crawlIntervalTTL 间隔配置缓存（避免每个调度周期都查 DB）。
const crawlIntervalTTL = 30 * time.Second

// KnowledgeCrawlTask 知识库采集（间隔由管理后台配置，默认 6 小时）。
type KnowledgeCrawlTask struct {
	uc       *knowledge.KnowledgeUseCase
	logger   port.Logger
	mu       sync.Mutex
	interval time.Duration // TTL 缓存的当前间隔
	cachedAt time.Time
}

// NewKnowledgeCrawlTask 创建知识库采集任务。
func NewKnowledgeCrawlTask(uc *knowledge.KnowledgeUseCase, logger port.Logger) *KnowledgeCrawlTask {
	return &KnowledgeCrawlTask{uc: uc, logger: logger}
}

func (t *KnowledgeCrawlTask) Name() string { return "knowledge-crawl" }

// Interval 采集周期（动态）：管理后台配置的分钟数；未配置/读取失败回退默认 6h。
// TTL 缓存 30s——scheduler 每周期结束刷新一次，改配置后下个周期生效（免重启）。
func (t *KnowledgeCrawlTask) Interval() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.interval > 0 && time.Since(t.cachedAt) < crawlIntervalTTL {
		return t.interval
	}
	minutes, err := t.uc.GetCrawlIntervalMinutes(context.Background())
	if err != nil || minutes <= 0 {
		minutes = entity.DefaultCrawlIntervalMinutes
	}
	d := time.Duration(minutes) * time.Minute
	t.interval = d
	t.cachedAt = time.Now()
	return d
}

// Execute 执行一轮采集（内部已按行业/关键词限额；失败仅记录不 panic）。
func (t *KnowledgeCrawlTask) Execute(ctx context.Context) error {
	if t.uc == nil {
		return nil // 未装配：空转
	}
	if err := t.uc.CrawlIndustries(ctx); err != nil {
		t.logger.Error("知识库采集失败", port.Err(err))
		return err
	}
	return nil
}
