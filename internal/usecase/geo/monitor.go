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
	quotaGate   port.QuotaStore   // 配额检查门（可选；nil=不检查）
	// selfBaseDomain 自营公开站域名（归因 P5-01）：注入后探测统计
	// "AI 回答引用的来源里包含自营站内容的次数"——回答内容 GEO 是否真实生效。
	selfBaseDomain string
	// storeRepo 门店档案（可选；本地生活 P0 补全）：
	// 注入后监测自动携带门店位置（LocalContext）——问"望京附近有什么川菜馆"，
	// 测的是本地生意而非泛化品牌声量。
	storeRepo port.StoreLocationRepository
	// questionGen 问法池生成器（可选；采样矩阵·问法维度 v2）：
	// 注入后监测前按品牌/卖点/竞品/地址 LLM 生成问法池，多引擎分片隔离防缓存；
	// 未注入/生成失败 → probe 内部模板问法兜底（零失败风险）。
	questionGen port.ProbeQuestionGenerator
}

func NewMonitorUseCase(br port.BrandRepository, kr port.KeywordRepository, rr port.MonitoringResultRepository, probe port.AIEngineProbe) *MonitorUseCase {
	return &MonitorUseCase{brandRepo: br, keywordRepo: kr, resultRepo: rr, probe: probe}
}

// SetQuestionGenerator 注入问法池生成器（可选；v2 去缓存：LLM 生成真实问法，引擎分片）。
func (uc *MonitorUseCase) SetQuestionGenerator(g port.ProbeQuestionGenerator) {
	if g != nil {
		uc.questionGen = g
	}
}

// SetSelfBaseDomain 注入自营公开站域名（可选；归因 P5-01）。
// 传参为域名（如 content.example.com）或完整 URL（内部提取域名）。
func (uc *MonitorUseCase) SetSelfBaseDomain(publicBaseURL string) {
	if publicBaseURL == "" {
		return
	}
	uc.selfBaseDomain = publicBaseURL
}

// SetStoreRepo 注入门店档案仓储（可选；本地生活 P0 补全）。
// 注入后监测自动携带门店位置上下文（LocalContext）——位置型问法测本地决策场景。
func (uc *MonitorUseCase) SetStoreRepo(r port.StoreLocationRepository) {
	if r != nil {
		uc.storeRepo = r
	}
}

// buildLocalContext 取品牌主门店并格式化为位置上下文（纯文本，零失败风险）。
// 位置优先级：商圈 > 区 > 城市（P1 商圈补全后，问法从"朝阳区有什么川菜馆"
// 精确到"望京有什么川菜馆"——商圈级最贴近真实本地决策）。
func (uc *MonitorUseCase) buildLocalContext(ctx context.Context, brandID string) string {
	if uc.storeRepo == nil || brandID == "" {
		return ""
	}
	store, err := uc.storeRepo.FindPrimaryByBrand(ctx, brandID)
	if err != nil {
		return ""
	}
	return localContextFromStore(store)
}

// SetQuotaGate 注入配额检查门（可选；未注入时不检查配额——向后兼容）。
func (uc *MonitorUseCase) SetQuotaGate(g port.QuotaStore) {
	if g != nil {
		uc.quotaGate = g
	}
}

// buildQuestions 生成问法池（v2：LLM 按品牌上下文生成真实问法；一次生成多引擎分片）。
// 失败/未注入返回 nil——probe 内部模板问法兜底，监测永不因问法生成中断（零失败风险）。
func (uc *MonitorUseCase) buildQuestions(ctx context.Context, brand entity.Brand, keyword string, localCtx string) []string {
	if uc.questionGen == nil {
		return nil
	}
	qs, err := uc.questionGen.Generate(ctx, port.QuestionGenInput{
		BrandName:    brand.Name,
		Positioning:  brand.Positioning,
		CoreSelling:  brand.CoreSelling,
		Competitors:  brand.Competitors,
		Keyword:      keyword,
		LocalContext: localCtx,
		Count:        8, // 问法池 8 个：覆盖常见采样数（3/5）且多引擎分片错位
	})
	if err != nil || len(qs) == 0 {
		return nil // 降级：模板问法
	}
	return qs
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

	// 配额检查（计费周期内 monitor 次数；超限返回 ErrQuotaExceeded → HTTP 402）
	if uc.quotaGate != nil {
		if err := uc.quotaGate.Check(ctx, in.TenantID, "monitor"); err != nil {
			return nil, err
		}
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
	// 本地位置上下文（P0 补全）：一次监测查一次门店，注入位置型问法
	localCtx := uc.buildLocalContext(ctx, in.BrandID)
	for _, kw := range kws {
		// 问法池（v2）：每关键词 LLM 生成一次真实问法（品牌/卖点/竞品/地址融入），
		// 单引擎取池子 0 号分片（错位起点）
		pool := uc.buildQuestions(ctx, brand, kw.Term, localCtx)
		// 探测这个关键词（用量计量上下文：租户 + 场景）
		probeResult, pErr := uc.probe.Probe(port.WithUsageContext(ctx, in.TenantID, "monitor"), port.ProbeInput{
			TenantID:       in.TenantID,
			Keyword:        kw.Term,
			EngineName:     in.EngineName,
			BrandName:      brand.Name,
			Competitors:    brand.Competitors,
			SampleSize:     sampleSize,
			SelfBaseDomain: uc.selfBaseDomain, // 归因 P5-01：统计自营站被引用次数
			LocalContext:   localCtx,          // 本地生活 P0：位置型问法
			Questions:      port.ShardQuestions(pool, 0, sampleSize), // v2 问法池分片
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
		if err := uc.resultRepo.Save(ctx, result); err != nil {
			return nil, fmt.Errorf("监测结果保存失败（%s/%s）: %w", kw.Term, in.EngineName, err)
		}
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

	// 配额检查（X-01 补齐：单关键词刷新同样烧 LLM——与全量 Monitor 同口径计费）
	if uc.quotaGate != nil {
		if err := uc.quotaGate.Check(ctx, in.TenantID, "monitor"); err != nil {
			return entity.MonitoringResult{}, err
		}
	}
	// 用量计量上下文：probe 内部的 LLM 调用（提问/解析）落 monitor scene——
	// usages 行数即配额计数（每次 LLM 调用 1 行），进成本报表
	ctx = port.WithUsageContext(ctx, in.TenantID, "monitor")

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

	// 问法池（v2）：LLM 生成一次真实问法（品牌上下文），单引擎取 0 号分片
	pool := uc.buildQuestions(ctx, brand, kw.Term, uc.buildLocalContext(ctx, brand.ID))
	probeResult, pErr := uc.probe.Probe(ctx, port.ProbeInput{
		TenantID:       in.TenantID,
		Keyword:        kw.Term,
		EngineName:     in.EngineName,
		BrandName:      brand.Name,
		Competitors:    brand.Competitors,
		SampleSize:     sampleSize,
		SelfBaseDomain: uc.selfBaseDomain,            // 归因 P5-01：统计自营站被引用次数
		LocalContext:   uc.buildLocalContext(ctx, brand.ID), // 本地生活 P0：位置型问法
		Questions:      port.ShardQuestions(pool, 0, sampleSize), // v2 问法池分片
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
		Sources:         probeResult.Sources,          // 引用来源（归因 P5-01）
		SelfSourceCount: probeResult.SelfSourceCount,  // 自营站被引用次数
	}
	if err := uc.resultRepo.Save(ctx, result); err != nil {
		return entity.MonitoringResult{}, fmt.Errorf("监测结果保存失败: %w", err)
	}
	return result, nil
}

// MonitorMultiEngine 对同一关键词用多个引擎批量监测（采样矩阵：一次调用=每引擎独立采样）。
// engineNames 为空时用所有配置的 LLM。
func (uc *MonitorUseCase) MonitorMultiEngine(ctx context.Context, tenantID, keywordID string, engineNames []string, sampleSize int) ([]entity.MonitoringResult, error) {
	if len(engineNames) == 0 {
		return nil, fmt.Errorf("至少指定一个引擎")
	}
	// 配额检查（X-01 补齐：多引擎监测 = 引擎数 × 采样数 LLM 调用，按次计费）
	if uc.quotaGate != nil {
		if err := uc.quotaGate.Check(ctx, tenantID, "monitor"); err != nil {
			return nil, err
		}
	}
	// 计量上下文：probe 内部 LLM 调用落 monitor scene（usages 行数即配额计数）
	ctx = port.WithUsageContext(ctx, tenantID, "monitor")
	if sampleSize <= 0 {
		sampleSize = 3 // 采样矩阵：每引擎默认 3 次采样（有统计意义且成本可控）
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
	// 本地位置上下文（P0 补全）：一次查门店，所有引擎注入位置型问法
	localCtx := uc.buildLocalContext(ctx, brand.ID)
	// 问法池（v2 去缓存核心）：LLM 按品牌上下文生成一次真实问法，
	// 每引擎取不同分片（ShardQuestions 错位起点）——引擎间问法隔离，缓存互不命中
	pool := uc.buildQuestions(ctx, brand, kw.Term, localCtx)
	for i, engineName := range engineNames {
		probeResult, pErr := uc.probe.Probe(ctx, port.ProbeInput{
			TenantID:       tenantID,
			EngineName:     engineName,
			BrandName:      brand.Name,
			Competitors:    brand.Competitors,
			SampleSize:     sampleSize,
			SelfBaseDomain: uc.selfBaseDomain, // 归因 P5-01
			LocalContext:   localCtx,          // 本地生活 P0：位置型问法
			Questions:      port.ShardQuestions(pool, i, sampleSize), // 引擎 i 分片
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
		if err := uc.resultRepo.Save(ctx, result); err != nil {
			return nil, fmt.Errorf("监测结果保存失败（%s/%s）: %w", kw.Term, engineName, err)
		}
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
	resultRepo  port.MonitoringResultRepository
	keywordRepo port.KeywordRepository // 关键词计数（Overview.KeywordCount 用）
}

func NewRankUseCase(rr port.MonitoringResultRepository) *RankUseCase {
	return &RankUseCase{resultRepo: rr}
}

// SetKeywordRepo 注入关键词仓储（可选；Overview 的品牌关键词数展示用）。
func (uc *RankUseCase) SetKeywordRepo(kr port.KeywordRepository) {
	if kr != nil {
		uc.keywordRepo = kr
	}
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
	// 真实关键词数（修复：原实现误用 trend 结果条数——"0 个关键词"误导）
	keywordCount := 0
	if uc.keywordRepo != nil {
		if kws, kErr := uc.keywordRepo.ListByBrand(ctx, tenantID, brandID); kErr == nil {
			keywordCount = len(kws)
		}
	}
	return BrandOverview{
		BrandID:        brandID,
		BrandName:      brandName,
		AvgMentionRate: avg,
		KeywordCount:   keywordCount,
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
