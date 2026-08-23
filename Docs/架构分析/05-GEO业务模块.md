# GEO（生成式引擎优化）业务模块分析

> 帮助商户在 AI 搜索引擎中提升品牌可见度的核心业务

## 1. 模块概述

GEO（Generative Engine Optimization）模块是 WebReaper 的**核心业务模块**，帮助商户在 AI 搜索引擎（豆包/Kimi/ChatGPT）中被提及和引用。

**业务目标**：
- 监测品牌在 AI 引擎中的可见度
- 优化内容以提高被 AI 引用的概率
- 追踪发布效果和竞品对比
- 提供本地生活场景的门店优化

## 2. 核心业务流程

```
品牌配置 → 关键词管理 → AI 引擎监测 → 内容优化 → 多平台发布 → 效果追踪
    ↓           ↓           ↓           ↓           ↓           ↓
  Brand     Keyword    MonitorResult  Content    PublishJob   Health
```

### 2.1 业务闭环

1. **品牌配置**：商户创建品牌，设置定位、卖点、竞品
2. **关键词管理**：添加要监测的搜索词（如"北京装修公司哪家好"）
3. **AI 引擎监测**：对每个关键词 × 每个 AI 引擎进行多次采样
4. **内容优化**：基于监测结果，优化内容以提高被引用概率
5. **多平台发布**：将优化后的内容发布到知乎、小红书等平台
6. **效果追踪**：监测发布后提及率变化，形成闭环

## 3. 核心用例分析

### 3.1 BrandUseCase（品牌管理用例）

**位置**：`internal/usecase/geo/geo.go`

**核心职责**：
```go
type BrandUseCase struct {
    brandRepo   port.BrandRepository
    keywordRepo port.KeywordRepository
    aiGen       port.AIGenerator        // 关键词生成用
    storeRepo   port.StoreLocationRepository  // 本地意图关键词
    webSearcher port.BrandWebSearcher   // RAG 增强
}
```

**核心业务流程**：

**创建品牌**：
```go
func (uc *BrandUseCase) Create(ctx context.Context, input CreateBrandInput) (entity.Brand, error) {
    // 1. 参数校验
    if input.Name == "" {
        return entity.Brand{}, fmt.Errorf("品牌名不能为空")
    }
    
    // 2. 创建品牌实体
    brand := entity.Brand{
        ID:          generateID(),
        TenantID:    input.TenantID,
        Name:        input.Name,
        Positioning: input.Positioning,
        CoreSelling: input.CoreSelling,
        Competitors: input.Competitors,
        BizType:     input.BizType,
        Industry:    input.Industry,
        WebsiteURL:  input.WebsiteURL,
        CreatedAt:   time.Now(),
    }
    
    // 3. 领域验证
    if !brand.IsValid() {
        return entity.Brand{}, fmt.Errorf("品牌信息不完整")
    }
    
    // 4. 持久化
    if err := uc.brandRepo.Save(ctx, brand); err != nil {
        return entity.Brand{}, err
    }
    
    return brand, nil
}
```

**生成关键词**：
```go
func (uc *BrandUseCase) GenerateKeywords(ctx context.Context, tenantID, brandID string) ([]string, error) {
    // 1. 加载品牌信息
    brand, err := uc.brandRepo.FindByID(ctx, tenantID, brandID)
    if err != nil {
        return nil, err
    }
    
    // 2. 构建提示词
    prompt := fmt.Sprintf(`
品牌名：%s
品牌定位：%s
核心卖点：%s
竞品：%s

请为这个品牌生成 10 个可能被用户搜索的关键词，用于 AI 搜索引擎优化。
关键词应该包含：
1. 品牌相关词（如"XX品牌怎么样"）
2. 行业通用词（如"XX行业哪家好"）
3. 本地意图词（如"XX城市XX行业"）
4. 竞品对比词（如"XX品牌 vs 竞品"）
`, brand.Name, brand.Positioning, strings.Join(brand.CoreSelling, "、"), strings.Join(brand.Competitors, "、"))
    
    // 3. 调用 LLM 生成
    resp, err := uc.aiGen.Generate(ctx, port.AIRequest{Prompt: prompt})
    if err != nil {
        return nil, err
    }
    
    // 4. 解析关键词列表
    keywords := parseKeywords(resp.Content)
    
    return keywords, nil
}
```

### 3.2 MonitorUseCase（监测用例）

**位置**：`internal/usecase/geo/monitor.go`

**核心职责**：
```go
type MonitorUseCase struct {
    brandRepo      port.BrandRepository
    keywordRepo    port.KeywordRepository
    resultRepo     port.MonitoringResultRepository
    probe          port.AIEngineProbe        // AI 引擎探测适配器
    quotaGate      port.QuotaStore           // 配额检查门
    selfBaseDomain string                    // 自营公开站域名
    storeRepo      port.StoreLocationRepository  // 门店档案
    questionGen    port.ProbeQuestionGenerator   // 问法池生成器
    cache          port.CacheStore           // 写后缓存失效
}
```

**核心业务流程**：

**单次监测**：
```go
func (uc *MonitorUseCase) Monitor(ctx context.Context, input MonitorInput) ([]entity.MonitoringResult, error) {
    // 1. 加载品牌和关键词
    brand, err := uc.brandRepo.FindByID(ctx, input.TenantID, input.BrandID)
    if err != nil {
        return nil, err
    }
    
    keywords, err := uc.keywordRepo.ListByBrand(ctx, input.BrandID)
    if err != nil {
        return nil, err
    }
    
    // 2. 构建本地上下文（本地生活场景）
    localCtx := uc.buildLocalContext(ctx, input.BrandID)
    
    // 3. 生成问法池
    questions := uc.buildQuestions(ctx, brand, keywords[0].Term, localCtx)
    
    // 4. 配额检查
    if uc.quotaGate != nil {
        if err := uc.quotaGate.Check(ctx, input.TenantID, "monitor"); err != nil {
            return nil, err
        }
    }
    
    // 5. 对每个关键词 × 每个引擎 × 多次采样
    var results []entity.MonitoringResult
    for _, keyword := range keywords {
        for _, engine := range getEngines(input.EngineName) {
            result := uc.probeKeyword(ctx, brand, keyword, engine, input.SampleSize, questions, localCtx)
            results = append(results, result)
        }
    }
    
    // 6. 结果落库
    for _, result := range results {
        _ = uc.resultRepo.Save(ctx, result)
    }
    
    // 7. 缓存失效
    uc.invalidateAfterWrite(ctx, input.TenantID)
    
    return results, nil
}
```

**单关键词探测**：
```go
func (uc *MonitorUseCase) probeKeyword(ctx context.Context, brand entity.Brand, keyword entity.Keyword, engine string, sampleSize int, questions []string, localCtx string) entity.MonitoringResult {
    var mentionCount int
    var avgPosition int
    var competitors []string
    var sources []string
    var selfSourceCount int
    var firstPickCount int
    
    // 多次采样
    for i := 0; i < sampleSize; i++ {
        // 选择问法（轮询问法池）
        question := selectQuestion(questions, i, keyword.Term, localCtx)
        
        // 调用探测器
        probeResult, err := uc.probe.Probe(ctx, question, engine)
        if err != nil {
            continue
        }
        
        // 统计提及
        if probeResult.Mentioned {
            mentionCount++
            if probeResult.Position == 1 {
                firstPickCount++
            }
        }
        
        // 统计竞品
        competitors = append(competitors, probeResult.Competitors...)
        
        // 统计来源
        sources = append(sources, probeResult.Sources...)
        if probeResult.SelfSourceMentioned {
            selfSourceCount++
        }
    }
    
    // 计算提及率
    mentionRate := float64(mentionCount) / float64(sampleSize)
    
    // 计算平均排名
    if mentionCount > 0 {
        avgPosition = totalPosition / mentionCount
    }
    
    // 构建监测结果
    return entity.MonitoringResult{
        ID:               generateID(),
        TenantID:         brand.TenantID,
        BrandID:          brand.ID,
        KeywordID:        keyword.ID,
        EngineName:       engine,
        SampleCount:      sampleSize,
        MentionCount:     mentionCount,
        MentionRate:      mentionRate,
        AvgPosition:      avgPosition,
        Competitors:      unique(competitors),
        Sources:          unique(sources),
        SelfSourceCount:  selfSourceCount,
        FirstPickCount:   firstPickCount,
        ProbedAt:         time.Now(),
    }
}
```

### 3.3 ContentUseCase（内容优化用例）

**位置**：`internal/usecase/geo/content.go`

**核心职责**：
```go
type ContentUseCase struct {
    aiGen              port.AIGenerator
    scorer             port.GEOScorer
    ruleScorer         port.GEOScorer
    contentRepo        port.OptimizedContentRepository
    urlSubmitter       port.URLSubmitter
    publicBaseURL      string
    logger             port.Logger
    ragRetriever       port.ContentRAGRetriever
    knowledgeRetriever port.KnowledgeRetriever
    templateRepo       port.PromptTemplateRepository
    quotaGate          port.QuotaStore
    storeRepo          port.StoreLocationRepository
    brandRepo          port.BrandRepository
    diagnoseUC         *DiagnoseUseCase
    cache              port.CacheStore
}
```

**核心业务流程**：

**内容优化**：
```go
func (uc *ContentUseCase) Optimize(ctx context.Context, input OptimizeInput) (OptimizeOutput, error) {
    // 1. 配额检查
    if uc.quotaGate != nil {
        if err := uc.quotaGate.Check(ctx, input.TenantID, "content-opt"); err != nil {
            return OptimizeOutput{}, err
        }
    }
    
    // 2. 构建系统提示词
    systemPrompt := uc.systemPrompt(ctx, entity.PromptKeyContentOptimize, defaultOptimizePrompt, input.TargetEngine, input.Format)
    
    // 3. 注入门店 NAP 信号
    napContext := uc.buildNAPContext(ctx, input.BrandID)
    
    // 4. 注入知识库素材
    knowledgeContext := uc.buildKnowledgeContext(ctx, input.BrandID, input.Keywords)
    
    // 5. 注入 RAG 检索结果
    ragContext := uc.buildRAGContext(ctx, input.BrandID, input.Keywords)
    
    // 6. 构建用户提示词
    userPrompt := fmt.Sprintf(`
原始内容：
%s

关键词：%s
%s%s%s

请优化以上内容，使其更容易被 AI 搜索引擎引用。
`, input.Content, strings.Join(input.Keywords, "、"), napContext, knowledgeContext, ragContext)
    
    // 7. 调用 LLM 生成优化内容
    resp, err := uc.aiGen.Generate(ctx, port.AIRequest{
        System: systemPrompt,
        Prompt: userPrompt,
    })
    if err != nil {
        return OptimizeOutput{}, err
    }
    
    // 8. 规则评分（优化前后对比）
    ruleScore, _ := uc.ruleScorer.Score(ctx, resp.Content, input.Keywords)
    
    // 9. LLM 深度评分（落库用）
    llmScore, _ := uc.scorer.Score(ctx, resp.Content, input.Keywords)
    
    // 10. 内容落库
    content := entity.OptimizedContent{
        ID:          generateID(),
        TenantID:    input.TenantID,
        BrandID:     input.BrandID,
        Title:       extractTitle(resp.Content),
        Content:     resp.Content,
        Keywords:    input.Keywords,
        Score:       llmScore.Score,
        RuleScore:   ruleScore.Score,
        Format:      input.Format,
        Status:      "draft",
        CreatedAt:   time.Now(),
    }
    
    if err := uc.contentRepo.Save(ctx, content); err != nil {
        return OptimizeOutput{}, err
    }
    
    // 11. 缓存失效
    uc.invalidateAfterWrite(ctx, input.TenantID)
    
    return OptimizeOutput{
        Content:    content,
        RuleScore:  ruleScore,
        LLMScore:   llmScore,
    }, nil
}
```

**内容原创生成**：
```go
func (uc *ContentUseCase) Generate(ctx context.Context, input GenerateInput) (GenerateOutput, error) {
    // 1. 配额检查
    if uc.quotaGate != nil {
        if err := uc.quotaGate.Check(ctx, input.TenantID, "content-gen"); err != nil {
            return GenerateOutput{}, err
        }
    }
    
    // 2. 构建系统提示词
    systemPrompt := uc.systemPrompt(ctx, entity.PromptKeyContentGenerate, defaultGeneratePrompt, input.TargetEngine, input.Format)
    
    // 3. 注入品牌信息
    brand, _ := uc.brandRepo.FindByID(ctx, input.TenantID, input.BrandID)
    brandContext := fmt.Sprintf("品牌名：%s\n品牌定位：%s\n核心卖点：%s", brand.Name, brand.Positioning, strings.Join(brand.CoreSelling, "、"))
    
    // 4. 注入门店 NAP 信号
    napContext := uc.buildNAPContext(ctx, input.BrandID)
    
    // 5. 注入知识库素材
    knowledgeContext := uc.buildKnowledgeContext(ctx, input.BrandID, input.Keywords)
    
    // 6. 注入 RAG 检索结果
    ragContext := uc.buildRAGContext(ctx, input.BrandID, input.Keywords)
    
    // 7. 构建用户提示词
    userPrompt := fmt.Sprintf(`
品牌信息：
%s

关键词：%s
%s%s%s

请根据以上信息创作一篇 800-1500 字的高质量文章。
`, brandContext, strings.Join(input.Keywords, "、"), napContext, knowledgeContext, ragContext)
    
    // 8. 调用 LLM 生成
    resp, err := uc.aiGen.Generate(ctx, port.AIRequest{
        System: systemPrompt,
        Prompt: userPrompt,
    })
    if err != nil {
        return GenerateOutput{}, err
    }
    
    // 9. LLM 深度评分
    llmScore, _ := uc.scorer.Score(ctx, resp.Content, input.Keywords)
    
    // 10. 内容落库
    content := entity.OptimizedContent{
        ID:          generateID(),
        TenantID:    input.TenantID,
        BrandID:     input.BrandID,
        Title:       extractTitle(resp.Content),
        Content:     resp.Content,
        Keywords:    input.Keywords,
        Score:       llmScore.Score,
        Format:      input.Format,
        Status:      "draft",
        CreatedAt:   time.Now(),
    }
    
    if err := uc.contentRepo.Save(ctx, content); err != nil {
        return GenerateOutput{}, err
    }
    
    // 11. 缓存失效
    uc.invalidateAfterWrite(ctx, input.TenantID)
    
    return GenerateOutput{
        Content:  content,
        LLMScore: llmScore,
    }, nil
}
```

### 3.4 HealthUseCase（健康报告用例）

**位置**：`internal/usecase/geo/health.go`

**核心职责**：
```go
type HealthUseCase struct {
    brandRepo   port.BrandRepository
    resultRepo  port.MonitoringResultRepository
    contentRepo port.OptimizedContentRepository
    cache       port.CacheStore
}
```

**核心业务流程**：

**健康报告聚合**：
```go
func (uc *HealthUseCase) GetHealthReport(ctx context.Context, tenantID string) (HealthReport, error) {
    // 1. 缓存读取
    if uc.cache != nil {
        cached, err := uc.cache.Get(ctx, HealthReportCacheKey(tenantID))
        if err == nil && cached != "" {
            var report HealthReport
            if json.Unmarshal([]byte(cached), &report) == nil {
                return report, nil
            }
        }
    }
    
    // 2. 加载品牌列表
    brands, err := uc.brandRepo.ListByTenant(ctx, tenantID)
    if err != nil {
        return HealthReport{}, err
    }
    
    // 3. 加载监测结果
    results, err := uc.resultRepo.LatestByTenant(ctx, tenantID)
    if err != nil {
        return HealthReport{}, err
    }
    
    // 4. 加载内容统计
    contents, err := uc.contentRepo.ListByTenant(ctx, tenantID)
    if err != nil {
        return HealthReport{}, err
    }
    
    // 5. 计算五指数
    indicators := uc.calculateIndicators(brands, results, contents)
    
    // 6. 计算总分
    totalScore := uc.calculateTotalScore(indicators)
    
    // 7. 竞品对标
    competitors := uc.analyzeCompetitors(results)
    
    // 8. 构建报告
    report := HealthReport{
        TenantID:      tenantID,
        TotalScore:    totalScore,
        Indicators:    indicators,
        Competitors:   competitors,
        BrandCount:    len(brands),
        KeywordCount:  countKeywords(brands),
        ContentCount:  len(contents),
        LastUpdatedAt: time.Now(),
    }
    
    // 9. 缓存写入
    if uc.cache != nil {
        data, _ := json.Marshal(report)
        _ = uc.cache.Set(ctx, HealthReportCacheKey(tenantID), string(data), healthCacheTTL)
    }
    
    return report, nil
}
```

**五指数计算**：
```go
func (uc *HealthUseCase) calculateIndicators(brands []entity.Brand, results []entity.MonitoringResult, contents []entity.OptimizedContent) HealthIndicators {
    var totalMentionRate float64
    var totalSentimentScore float64
    var totalFirstPickRate float64
    var publishedCount int
    var selfSourceCount int
    
    // 遍历监测结果
    for _, result := range results {
        totalMentionRate += result.MentionRate
        totalFirstPickRate += float64(result.FirstPickCount) / float64(result.SampleCount)
        
        if result.Sentiment == "positive" {
            totalSentimentScore += 1
        } else if result.Sentiment == "negative" {
            totalSentimentScore -= 1
        }
        
        if result.SelfSourceCount > 0 {
            selfSourceCount++
        }
    }
    
    // 遍历内容
    for _, content := range contents {
        if content.Status == "published" {
            publishedCount++
        }
    }
    
    // 计算指数（0-100）
    resultCount := len(results)
    if resultCount == 0 {
        resultCount = 1
    }
    
    return HealthIndicators{
        MentionCoverage: (totalMentionRate / float64(resultCount)) * 100,
        SentimentScore:  ((totalSentimentScore/float64(resultCount) + 1) / 2) * 100,
        FirstPickRate:   (totalFirstPickRate / float64(resultCount)) * 100,
        ContentAsset:    math.Min(float64(publishedCount*15+len(contents)*5), 100),
        SourceIntegrity: (float64(selfSourceCount) / float64(resultCount)) * 100,
    }
}
```

### 3.5 NearbyUseCase（附近同行业用例）

**位置**：`internal/usecase/geo/nearby.go`

**核心职责**：
```go
type NearbyUseCase struct {
    brandRepo   port.BrandRepository
    storeRepo   port.StoreLocationRepository
    resultRepo  port.MonitoringResultRepository
    probeRepo   port.AIRankProbeRepository
    searcher    port.POISearcher
    measurer    port.DistanceMeasurer
    quotaGate   port.QuotaStore
    usageRec    port.UsageRecorder
}
```

**核心业务流程**：

**附近同行双榜**：
```go
func (uc *NearbyUseCase) GetRanking(ctx context.Context, tenantID, brandID string) (NearbyRanking, error) {
    // 1. 加载品牌和门店
    brand, err := uc.brandRepo.FindByID(ctx, tenantID, brandID)
    if err != nil {
        return NearbyRanking{}, err
    }
    
    store, err := uc.storeRepo.FindPrimaryByBrand(ctx, brandID)
    if err != nil {
        return NearbyRanking{}, err
    }
    
    // 2. 配额检查
    if uc.quotaGate != nil {
        if err := uc.quotaGate.Check(ctx, tenantID, "nearby"); err != nil {
            return NearbyRanking{}, err
        }
    }
    
    // 3. 地图榜：周边搜索
    var mapRanking []MapRankEntry
    if uc.searcher != nil && store.HasGeo() {
        // 搜索周边同行业门店
        pois, err := uc.searcher.SearchAround(ctx, port.SearchAroundInput{
            Latitude:  store.Lat,
            Longitude: store.Lng,
            Radius:    3000,  // 3km 半径
            Category:  brand.Industry,
            Limit:     20,
        })
        if err == nil {
            for _, poi := range pois {
                entry := MapRankEntry{
                    Name:      poi.Name,
                    Address:   poi.Address,
                    DistanceM: poi.Distance,
                    Rating:    poi.Rating,
                    Category:  poi.Category,
                    Lat:       poi.Lat,
                    Lng:       poi.Lng,
                }
                mapRanking = append(mapRanking, entry)
            }
        }
    }
    
    // 4. AI 榜：探查结果
    var aiRanking []AIRankEntry
    var aiRankFromProbe bool
    if uc.probeRepo != nil {
        // 优先使用探查结果
        probeResults, err := uc.probeRepo.ListByBrand(ctx, brandID)
        if err == nil && len(probeResults) > 0 {
            aiRankFromProbe = true
            for _, probe := range probeResults {
                entry := AIRankEntry{
                    Name:        probe.StoreName,
                    MentionRate: probe.MentionRate,
                    Mentioned:   probe.Mentioned,
                }
                aiRanking = append(aiRanking, entry)
            }
        }
    }
    
    // 5. 降级：监测竞品提及率
    if len(aiRanking) == 0 {
        results, _ := uc.resultRepo.LatestByBrand(ctx, brandID)
        for _, result := range results {
            for _, competitor := range result.Competitors {
                entry := AIRankEntry{
                    Name:        competitor,
                    MentionRate: result.CompetitorRates[competitor],
                    Mentioned:   true,
                }
                aiRanking = append(aiRanking, entry)
            }
        }
    }
    
    // 6. 计算自己的提及率
    ownRate := -1.0
    results, _ := uc.resultRepo.LatestByBrand(ctx, brandID)
    if len(results) > 0 {
        var totalRate float64
        for _, result := range results {
            totalRate += result.MentionRate
        }
        ownRate = totalRate / float64(len(results))
    }
    
    // 7. 记录用量
    if uc.usageRec != nil {
        _ = uc.usageRec.RecordUsage(ctx, entity.UsageRecord{
            TenantID: tenantID,
            Scene:    "nearby",
            LLMCalls: 1,
        })
    }
    
    return NearbyRanking{
        Store:           store,
        MapRanking:      mapRanking,
        AIRanking:       aiRanking,
        OwnRate:         ownRate,
        MapAvailable:    uc.searcher != nil && store.HasGeo(),
        AIRankFromProbe: aiRankFromProbe,
    }, nil
}
```

### 3.6 DiagnoseUseCase（诊断用例）

**位置**：`internal/usecase/geo/diagnose.go`

**核心职责**：
```go
type DiagnoseUseCase struct {
    brandRepo  port.BrandRepository
    resultRepo port.MonitoringResultRepository
    aiGen      port.AIGenerator
}
```

**核心业务流程**：

**GEO 诊断**：
```go
func (uc *DiagnoseUseCase) Diagnose(ctx context.Context, tenantID, brandID string) (DiagnoseResult, error) {
    // 1. 加载品牌信息
    brand, err := uc.brandRepo.FindByID(ctx, tenantID, brandID)
    if err != nil {
        return DiagnoseResult{}, err
    }
    
    // 2. 加载监测结果
    results, err := uc.resultRepo.ListByBrand(ctx, brandID, 100)
    if err != nil {
        return DiagnoseResult{}, err
    }
    
    // 3. 分析提及率趋势
    mentionTrend := analyzeMentionTrend(results)
    
    // 4. 分析竞品对比
    competitorAnalysis := analyzeCompetitors(results)
    
    // 5. 分析来源分布
    sourceAnalysis := analyzeSources(results)
    
    // 6. LLM 生成诊断报告
    prompt := fmt.Sprintf(`
品牌信息：
- 品牌名：%s
- 品牌定位：%s
- 核心卖点：%s

监测数据：
- 提及率趋势：%s
- 竞品对比：%s
- 来源分布：%s

请分析这个品牌的 GEO 表现，给出：
1. 优势和劣势
2. 改进建议
3. 行动计划
`, brand.Name, brand.Positioning, strings.Join(brand.CoreSelling, "、"),
   mentionTrend, competitorAnalysis, sourceAnalysis)
    
    resp, err := uc.aiGen.Generate(ctx, port.AIRequest{Prompt: prompt})
    if err != nil {
        return DiagnoseResult{}, err
    }
    
    // 7. 构建诊断结果
    return DiagnoseResult{
        BrandID:           brandID,
        MentionTrend:      mentionTrend,
        CompetitorAnalysis: competitorAnalysis,
        SourceAnalysis:    sourceAnalysis,
        Diagnosis:         resp.Content,
        GeneratedAt:       time.Now(),
    }, nil
}
```

### 3.7 KeywordDistillUseCase（关键词蒸馏用例）

**位置**：`internal/usecase/geo/keyword_distill.go`

**核心职责**：
```go
type KeywordDistillUseCase struct {
    brandSource    port.KeywordSource  // 品牌信息+全网
    textSource     port.KeywordSource  // 用户文本
    seedSource     port.KeywordSource  // 种子词拓展
    fileSource     port.KeywordSource  // 文件内容
    webSource      port.KeywordSource  // 网络爬取
    questionSource port.KeywordSource  // 提问词挖掘
}
```

**核心业务流程**：

**关键词蒸馏**：
```go
func (uc *KeywordDistillUseCase) Distill(ctx context.Context, input DistillInput) (DistillOutput, error) {
    var keywords []string
    var err error
    
    // 1. 根据输入类型选择来源策略
    switch input.SourceType {
    case "brand":
        keywords, err = uc.brandSource.Generate(ctx, port.KeywordSourceInput{
            BrandID: input.BrandID,
            Text:    input.Text,
        })
    case "text":
        keywords, err = uc.textSource.Generate(ctx, port.KeywordSourceInput{
            Text: input.Text,
        })
    case "seed":
        keywords, err = uc.seedSource.Generate(ctx, port.KeywordSourceInput{
            Seeds: input.Seeds,
        })
    case "file":
        keywords, err = uc.fileSource.Generate(ctx, port.KeywordSourceInput{
            FileContent: input.FileContent,
        })
    case "web":
        keywords, err = uc.webSource.Generate(ctx, port.KeywordSourceInput{
            URL: input.URL,
        })
    case "question":
        keywords, err = uc.questionSource.Generate(ctx, port.KeywordSourceInput{
            BrandID: input.BrandID,
        })
    default:
        return DistillOutput{}, fmt.Errorf("不支持的来源类型: %s", input.SourceType)
    }
    
    if err != nil {
        return DistillOutput{}, err
    }
    
    // 2. LLM 蒸馏（去重、分类、排序）
    distilled, err := uc.distillKeywords(ctx, keywords, input.BrandID)
    if err != nil {
        return DistillOutput{}, err
    }
    
    return DistillOutput{
        Keywords:    distilled,
        SourceType:  input.SourceType,
        RawCount:    len(keywords),
        DistillCount: len(distilled),
    }, nil
}
```

## 4. 核心设计模式

### 4.1 策略模式（Strategy Pattern）

**探测引擎策略**：
```go
type AIEngineProbe interface {
    Probe(ctx context.Context, question string, engineName string) (ProbeResult, error)
}

// 不同探测策略
type AgentProbe struct{}   // Agent 模拟探测
type DirectProbe struct{}  // 真实引擎直测
type RoutingProbe struct{} // 路由探测器
```

**关键词来源策略**：
```go
type KeywordSource interface {
    Generate(ctx context.Context, input KeywordSourceInput) ([]string, error)
}

// 不同来源策略
type BrandSource struct{}    // 品牌信息+全网
type TextSource struct{}     // 用户文本
type SeedSource struct{}     // 种子词拓展
type FileSource struct{}     // 文件内容
type WebSource struct{}      // 网络爬取
type QuestionSource struct{} // 提问词挖掘
```

### 4.2 装饰器模式（Decorator Pattern）

**缓存装饰器**：
```go
func (uc *HealthUseCase) CachedLatestByTenant(ctx context.Context, tenantID string) ([]entity.MonitoringResult, error) {
    if uc.cache == nil {
        return uc.resultRepo.LatestByTenant(ctx, tenantID)
    }
    
    cached, err := uc.cache.GetOrCompute(ctx, MonitorResultsCacheKey(tenantID), monitorResultsCacheTTL, func(ctx context.Context) (string, error) {
        rs, err := uc.resultRepo.LatestByTenant(ctx, tenantID)
        if err != nil {
            return "", err
        }
        b, _ := json.Marshal(rs)
        return string(b), nil
    })
    
    // ...
}
```

### 4.3 模板方法模式（Template Method）

**内容生成流程骨架**：
```
1. 配额检查
2. 构建系统提示词
3. 注入上下文（品牌/门店/知识库/RAG）
4. 调用 LLM 生成
5. 评分
6. 内容落库
7. 缓存失效
```

## 5. 数据模型

### 5.1 核心实体

**Brand（品牌）**：
```go
type Brand struct {
    ID           string
    TenantID     string
    Name         string
    Positioning  string
    CoreSelling  []string
    Competitors  []string
    BizType      string   // local/online
    Industry     string
    WebsiteURL   string
    CreatedAt    time.Time
}
```

**Keyword（关键词）**：
```go
type Keyword struct {
    ID        string
    TenantID  string
    BrandID   string
    Term      string
    Intent    string   // informational/transactional/local
    CreatedAt time.Time
}
```

**MonitoringResult（监测结果）**：
```go
type MonitoringResult struct {
    ID                 string
    TenantID           string
    BrandID            string
    KeywordID          string
    EngineName         string
    SampleCount        int
    MentionCount       int
    MentionRate        float64
    AvgPosition        int
    Sentiment          string
    Competitors        []string
    CompetitorRates    map[string]float64
    Sources            []string
    SelfSourceCount    int
    FirstPickCount     int
    ProbedAt           time.Time
}
```

### 5.2 聚合关系

```
Brand (聚合根)
  ├── Keyword (1:N)
  ├── StoreLocation (1:1)
  ├── MonitoringResult (1:N)
  └── OptimizedContent (1:N)
```

## 6. 缓存策略

### 6.1 缓存 Key 设计

```go
// 健康报告缓存
func HealthReportCacheKey(tenantID string) string {
    return "health-report:" + tenantID
}

// 监测结果缓存
func MonitorResultsCacheKey(tenantID string) string {
    return "monitor-results:" + tenantID
}

// 行业全景缓存
const IndustryOverviewCacheKey = "industry-overview"
```

### 6.2 缓存失效策略

**写后主动失效**：
```go
func (uc *MonitorUseCase) invalidateAfterWrite(ctx context.Context, tenantID string) {
    if uc.cache == nil {
        return
    }
    _ = uc.cache.Del(ctx,
        HealthReportCacheKey(tenantID),
        MonitorResultsCacheKey(tenantID),
        IndustryOverviewCacheKey,
    )
}
```

### 6.3 缓存 TTL

- 健康报告：60s + 抖动
- 监测结果：60s
- 行业全景：5min

## 7. 配额管理

### 7.1 配额场景

- `monitor`：AI 引擎监测（按次限额）
- `content-opt`：内容优化（按次限额）
- `content-gen`：内容原创生成（按次限额）
- `nearby`：附近同行（按次限额）
- `diagnose`：GEO 诊断（按次限额）

### 7.2 配额检查

```go
if uc.quotaGate != nil {
    if err := uc.quotaGate.Check(ctx, tenantID, "monitor"); err != nil {
        return nil, err
    }
}
```

## 8. 总结

GEO 业务模块的设计体现了整洁架构的核心思想：

1. **业务闭环**：品牌配置 → 关键词管理 → AI 引擎监测 → 内容优化 → 多平台发布 → 效果追踪
2. **策略驱动**：探测引擎、关键词来源、评分器等通过策略模式实现
3. **缓存优化**：写后主动失效 + TTL 缓存，提升查询性能
4. **配额管理**：按场景限额，控制第三方 API 成本
5. **可扩展性**：新增探测引擎/关键词来源/评分器 = 实现接口 + 注册

---

*文档生成时间：2026-08-23*
*基于整洁架构思想分析*
