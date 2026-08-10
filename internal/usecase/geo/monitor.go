package geo

import (
	"context"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ============ 监测用例（闭环起点）============

// MonitorUseCase 编排一次 AI 引擎监测。
// 对每个关键词 × 每个 AI 引擎 × 多次采样，聚合出 MonitoringResult。
type MonitorUseCase struct {
	brandRepo   port.BrandRepository
	keywordRepo port.KeywordRepository
	resultRepo  port.MonitoringResultRepository
	probe       port.AIEngineProbe // AI 引擎探测适配器
}

func NewMonitorUseCase(br port.BrandRepository, kr port.KeywordRepository, rr port.MonitoringResultRepository, probe port.AIEngineProbe) *MonitorUseCase {
	return &MonitorUseCase{brandRepo: br, keywordRepo: kr, resultRepo: rr, probe: probe}
}

// MonitorInput 监测的输入。
type MonitorInput struct {
	TenantID   string
	BrandID    string
	EngineName string // 探测哪个 AI 引擎；空则对所有配置的引擎
	SampleSize int    // 采样次数（默认 5）
}

// Monitor 执行监测：取关键词 → 逐个问 AI → 解析提及 → 存快照。
// 返回本次监测产生的全部结果。
func (uc *MonitorUseCase) Monitor(ctx context.Context, in MonitorInput) ([]entity.MonitoringResult, error) {
	if in.TenantID == "" {
		return nil, fmt.Errorf("tenant_id 不能为空")
	}
	brand, err := uc.brandRepo.FindByID(ctx, in.TenantID, in.BrandID)
	if err != nil {
		return nil, fmt.Errorf("品牌不存在: %w", err)
	}

	sampleSize := in.SampleSize
	if sampleSize <= 0 {
		sampleSize = 5
	}

	kws, err := uc.keywordRepo.ListByBrand(ctx, in.TenantID, in.BrandID)
	if err != nil {
		return nil, fmt.Errorf("取关键词失败: %w", err)
	}

	var results []entity.MonitoringResult
	for _, kw := range kws {
		// 探测这个关键词（用量计量上下文：租户 + 场景）
		probeResult, pErr := uc.probe.Probe(port.WithUsageContext(ctx, in.TenantID, "monitor"), port.ProbeInput{
			TenantID:    in.TenantID,
			Keyword:     kw.Term,
			EngineName:  in.EngineName,
			BrandName:   brand.Name,
			Competitors: brand.Competitors,
			SampleSize:  sampleSize,
		})
		if pErr != nil {
			// 单个关键词探测失败不中断整体（降级：记一个空结果）
			continue
		}

		// 竞品列表（被提及的）+ 竞品提及率（对比坐标系：付费说服力核心）
		var mentionedCompetitors []string
		competitorRates := make(map[string]float64)
		if probeResult.SampleCount > 0 {
			for name, cnt := range probeResult.Competitors {
				mentionedCompetitors = append(mentionedCompetitors, name)
				competitorRates[name] = float64(cnt) / float64(probeResult.SampleCount)
			}
		}

		result := entity.MonitoringResult{
			ID:              fmt.Sprintf("mr-%d-%d", time.Now().UnixNano(), len(results)),
			TenantID:        in.TenantID,
			BrandID:         in.BrandID,
			KeywordID:       kw.ID,
			EngineName:      in.EngineName,
			SampleCount:     probeResult.SampleCount,
			MentionCount:    probeResult.MentionCount,
			MentionRate:     probeResult.MentionRate,
			AvgPosition:     probeResult.AvgPosition,
			Sentiment:       probeResult.Sentiment,
			Competitors:     mentionedCompetitors,
			CompetitorRates: competitorRates,
			Confidence:      probeResult.Confidence,
			ProbedAt:        time.Now(),
			RawSample:       probeResult.RawSample,
		}
		_ = uc.resultRepo.Save(ctx, result)
		results = append(results, result)
	}
	return results, nil
}

// GetLatest 取某关键词在各引擎的最新监测结果。
func (uc *MonitorUseCase) GetLatest(ctx context.Context, tenantID, keywordID string) ([]entity.MonitoringResult, error) {
	return uc.resultRepo.LatestByKeyword(ctx, tenantID, keywordID)
}

// GetLatestByBrand 取某品牌下所有关键词的最新监测结果（关键词一览页用）。
func (uc *MonitorUseCase) GetLatestByBrand(ctx context.Context, tenantID, brandID string) ([]entity.MonitoringResult, error) {
	return uc.resultRepo.LatestByBrand(ctx, tenantID, brandID)
}

// GetLatestByTenant 取租户下所有关键词的最新监测结果（关键词一览页用，不依赖品牌筛选）。
func (uc *MonitorUseCase) GetLatestByTenant(ctx context.Context, tenantID string) ([]entity.MonitoringResult, error) {
	return uc.resultRepo.LatestByTenant(ctx, tenantID)
}

// MonitorKeywordInput 单关键词即时监测的输入。
type MonitorKeywordInput struct {
	TenantID   string
	KeywordID  string // 要监测的关键词 ID
	EngineName string
	SampleSize int
}

// MonitorKeyword 对单个关键词执行即时监测（关键词一览页"刷新"按钮用）。
// 直接按 ID 查关键词和品牌，不再遍历全部（修复 N+1 查询）。
func (uc *MonitorUseCase) MonitorKeyword(ctx context.Context, in MonitorKeywordInput) (entity.MonitoringResult, error) {
	if in.TenantID == "" {
		return entity.MonitoringResult{}, fmt.Errorf("tenant_id 不能为空")
	}

	// 直接按 ID 查关键词（O(1) 而非遍历所有品牌×关键词）
	kw, err := uc.keywordRepo.FindByID(ctx, in.TenantID, in.KeywordID)
	if err != nil {
		return entity.MonitoringResult{}, fmt.Errorf("关键词不存在: %w", err)
	}

	// 查关键词所属品牌
	brand, err := uc.brandRepo.FindByID(ctx, in.TenantID, kw.BrandID)
	if err != nil {
		return entity.MonitoringResult{}, fmt.Errorf("品牌不存在: %w", err)
	}

	sampleSize := in.SampleSize
	if sampleSize <= 0 {
		sampleSize = 3 // 快测默认 3 次采样：单次采样是噪声（AI 随机性），数字不稳用户不信
	}

	probeResult, pErr := uc.probe.Probe(ctx, port.ProbeInput{
		TenantID:    in.TenantID,
		Keyword:     kw.Term,
		EngineName:  in.EngineName,
		BrandName:   brand.Name,
		Competitors: brand.Competitors,
		SampleSize:  sampleSize,
	})
	if pErr != nil {
		return entity.MonitoringResult{}, pErr
	}

	var mentionedCompetitors []string
	competitorRates := make(map[string]float64)
	if probeResult.SampleCount > 0 {
		for name, cnt := range probeResult.Competitors {
			mentionedCompetitors = append(mentionedCompetitors, name)
			competitorRates[name] = float64(cnt) / float64(probeResult.SampleCount)
		}
	}
	result := entity.MonitoringResult{
		ID:              fmt.Sprintf("mr-%d", time.Now().UnixNano()),
		TenantID:        in.TenantID,
		BrandID:         brand.ID,
		KeywordID:       kw.ID,
		EngineName:      in.EngineName,
		SampleCount:     probeResult.SampleCount,
		MentionCount:    probeResult.MentionCount,
		MentionRate:     probeResult.MentionRate,
		AvgPosition:     probeResult.AvgPosition,
		Sentiment:       probeResult.Sentiment,
		Competitors:     mentionedCompetitors,
		CompetitorRates: competitorRates,
		Confidence:      probeResult.Confidence,
		ProbedAt:        time.Now(),
		RawSample:       probeResult.RawSample,
	}
	_ = uc.resultRepo.Save(ctx, result)
	return result, nil
}

// MonitorMultiEngine 对同一关键词用多个引擎批量监测（一次调用产生多条不同引擎的结果）。
// engineNames 为空时用所有配置的 LLM。
func (uc *MonitorUseCase) MonitorMultiEngine(ctx context.Context, tenantID, keywordID string, engineNames []string, sampleSize int) ([]entity.MonitoringResult, error) {
	if len(engineNames) == 0 {
		return nil, fmt.Errorf("至少指定一个引擎")
	}
	if sampleSize <= 0 {
		sampleSize = 1
	}
	kw, err := uc.keywordRepo.FindByID(ctx, tenantID, keywordID)
	if err != nil {
		return nil, fmt.Errorf("关键词不存在: %w", err)
	}
	brand, err := uc.brandRepo.FindByID(ctx, tenantID, kw.BrandID)
	if err != nil {
		return nil, fmt.Errorf("品牌不存在: %w", err)
	}

	var results []entity.MonitoringResult
	for _, engineName := range engineNames {
		probeResult, pErr := uc.probe.Probe(ctx, port.ProbeInput{
			TenantID:    tenantID,
			Keyword:     kw.Term,
			EngineName:  engineName,
			BrandName:   brand.Name,
			Competitors: brand.Competitors,
			SampleSize:  sampleSize,
		})
		if pErr != nil {
			continue // 单个引擎失败不中断
		}
		var mentionedCompetitors []string
		for name := range probeResult.Competitors {
			mentionedCompetitors = append(mentionedCompetitors, name)
		}
		result := entity.MonitoringResult{
			ID:           fmt.Sprintf("mr-%d-%s", time.Now().UnixNano(), engineName),
			TenantID:     tenantID,
			BrandID:      brand.ID,
			KeywordID:    kw.ID,
			EngineName:   engineName,
			SampleCount:  probeResult.SampleCount,
			MentionCount: probeResult.MentionCount,
			MentionRate:  probeResult.MentionRate,
			AvgPosition:  probeResult.AvgPosition,
			Sentiment:    probeResult.Sentiment,
			Competitors:  mentionedCompetitors,
			Confidence:   probeResult.Confidence,
			ProbedAt:     time.Now(),
			RawSample:    probeResult.RawSample,
		}
		_ = uc.resultRepo.Save(ctx, result)
		results = append(results, result)
	}
	return results, nil
}

// BrandRank 品牌在某关键词下的排名。
type BrandRank struct {
	Name       string  // 品牌名
	MentionRate float64 // 提及率
	AvgPosition int    // 平均排名
	Rank        int    // 行业第几（1=最靠前）
	Trend       string // up/down/stable
}

// RankUseCase 聚合监测结果，算出行业排名。
type RankUseCase struct {
	resultRepo port.MonitoringResultRepository
}

func NewRankUseCase(rr port.MonitoringResultRepository) *RankUseCase {
	return &RankUseCase{resultRepo: rr}
}

// BrandOverview 品牌的监测总览（仪表盘用）。
type BrandOverview struct {
	BrandID      string
	BrandName    string
	AvgMentionRate float64 // 平均提及率（所有关键词所有引擎）
	KeywordCount int       // 关键词数
	LastProbedAt time.Time
	Trend        []entity.MonitoringResult // 近期趋势
}

// Overview 取品牌的监测总览。
func (uc *RankUseCase) Overview(ctx context.Context, tenantID, brandID string, brandName string) (BrandOverview, error) {
	trend, err := uc.resultRepo.Trend(ctx, tenantID, brandID, 30)
	if err != nil {
		return BrandOverview{}, err
	}
	var sumRate float64
	var count int
	var lastAt time.Time
	for _, r := range trend {
		sumRate += r.MentionRate
		count++
		if r.ProbedAt.After(lastAt) {
			lastAt = r.ProbedAt
		}
	}
	avg := 0.0
	if count > 0 {
		avg = sumRate / float64(count)
	}
	return BrandOverview{
		BrandID:        brandID,
		BrandName:      brandName,
		AvgMentionRate: avg,
		KeywordCount:   count,
		LastProbedAt:   lastAt,
		Trend:          trend,
	}, nil
}

// TriggerMonitor 实现 port.MonitorTrigger 接口。
// 发布效果追踪用：发布成功后触发一次品牌监测，返回平均提及率。
func (uc *MonitorUseCase) TriggerMonitor(ctx context.Context, tenantID, brandID string) (float64, error) {
	results, err := uc.Monitor(ctx, MonitorInput{
		TenantID:   tenantID,
		BrandID:    brandID,
		SampleSize: 3, // 发布追踪用较少采样快速完成
	})
	if err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, nil
	}
	// 取平均提及率
	total := 0.0
	for _, r := range results {
		total += r.MentionRate
	}
	return total / float64(len(results)), nil
}

// BaselineRate 实现 port.MonitorTrigger 接口。
// 发布前基线：取品牌最近一次监测结果的平均提及率（不触发新监测，免费）。
// 无监测记录时返回 0——前端展示"无基线"而非误导性的 0%。
func (uc *MonitorUseCase) BaselineRate(ctx context.Context, tenantID, brandID string) (float64, error) {
	results, err := uc.resultRepo.LatestByBrand(ctx, tenantID, brandID)
	if err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, nil
	}
	total := 0.0
	for _, r := range results {
		total += r.MentionRate
	}
	return total / float64(len(results)), nil
}
