// geo.go GEO 域初始化（从 main.go 迁移——27号优化 main.go 瘦身）。
package bootstrap

import (
	"context"
	"fmt"
	"net"

	milvusclient "github.com/milvus-io/milvus-sdk-go/v2/client"

	"webreaper/internal/adapter/ai"
	"webreaper/internal/adapter/crawler"
	geoadapter "webreaper/internal/adapter/geo"
	"webreaper/internal/adapter/handler"
	kbretriever "webreaper/internal/adapter/knowledge"
	"webreaper/internal/adapter/repository"
	"webreaper/internal/adapter/urlprobe"
	"webreaper/internal/adapter/urlsubmit"
	"webreaper/internal/config"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/geo"
	"webreaper/internal/usecase/indexing"
	"webreaper/internal/usecase/knowledge"
	"webreaper/internal/usecase/port"
)

// GEOUseCases GEO 域用例集合（InitGEO 返回）。
type GEOUseCases struct {
	MonitorUC   *geo.MonitorUseCase
	ContentUC   *geo.ContentUseCase
	KnowledgeUC *knowledge.KnowledgeUseCase
	DistillUC   *geo.KeywordDistillUseCase
	NearbyUC    *geo.NearbyUseCase
	DiagnoseUC  *geo.DiagnoseUseCase
	IndexingUC  *indexing.IndexingUseCase
	IndexSubmitter port.URLSubmitter
}

// InitGEO 初始化 GEO 域（品牌/监测/内容/知识库/收录/门店/附近同行/诊断）。
//
// 依赖：DB + LLM + GeoRepos + AI 生成器 + 路由器 + 配置。
// 返回 nil 表示 GEO 未启用（缺 DB 或 LLM）。
func InitGEO(
	geoRepos *GeoRepos,
	cfg *config.Config,
	aiGenerator port.AIGenerator,
	llmCfgRepo port.LLMConfigRepository,
	settingRepo port.SystemSettingRepository,
	toolRegistry *port.ToolRegistry,
	cacheStore port.CacheStore,
	log port.Logger,
	router *handler.Router,
) *GEOUseCases {
	if geoRepos == nil || !cfg.LLM.IsConfigured() {
		log.Info("GEO 业务未启用（需配置 DB + LLM_API_KEY）")
		return nil
	}

	uc := &GEOUseCases{}
	db := geoRepos.DB

	// LLM 用量计量
	if gen, ok := aiGenerator.(*ai.TrpcAgentGenerator); ok {
		gen.SetUsageRecorder(repository.NewGormUsageRecorder(db))
		log.Info("LLM 用量计量已启用（usages 表，按租户/场景记录 token）")
	}

	// 收录通知
	uc.IndexSubmitter = initIndexing(geoRepos, cfg, settingRepo, router, log, uc)

	// 监测引擎
	geoProbe := ai.NewRoutingProbe(
		ai.NewAgentProbe(aiGenerator),
		ai.NewDirectProbe(aiGenerator),
		llmCfgRepo,
	)
	log.Info("GEO 监测引擎：RoutingProbe（真实引擎直测 + Agent 模拟兜底）")

	geoScorer := ai.NewLLMGEOScorer(aiGenerator)
	geoBrandUC := geo.NewBrandUseCase(geoRepos.Brand, geoRepos.Keyword)
	geoBrandUC.SetAIGenerator(aiGenerator)
	geoBrandUC.SetStoreRepo(geoRepos.Store)

	webFetcher := ai.NewWebFetcher()
	geoBrandUC.SetWebSearcher(ai.NewBrandWebSearcher(webFetcher))

	// 监测用例
	uc.MonitorUC = geo.NewMonitorUseCase(geoRepos.Brand, geoRepos.Keyword, geoRepos.Result, geoProbe)
	uc.MonitorUC.SetQuestionGenerator(ai.NewLLMQuestionGenerator(aiGenerator))
	uc.MonitorUC.SetSelfBaseDomain(cfg.Server.PublicBaseURL)
	uc.MonitorUC.SetStoreRepo(geoRepos.Store)
	if cacheStore != nil {
		uc.MonitorUC.SetCache(cacheStore)
	}

	// 内容优化用例
	uc.ContentUC = geo.NewContentUseCase(aiGenerator, geoScorer, geoRepos.Content)
	if cacheStore != nil {
		uc.ContentUC.SetCache(cacheStore)
	}
	uc.ContentUC.SetRuleScorer(geo.NewRuleScorer())
	uc.ContentUC.SetRAGRetriever(ai.NewWebContentRetriever(webFetcher))
	uc.ContentUC.SetStoreRepo(geoRepos.Store)
	uc.ContentUC.SetBrandRepo(geoRepos.Brand)
	uc.ContentUC.SetLogger(log)

	// 提示词模板
	promptTemplateRepo := repository.NewGormPromptTemplateRepository(db)
	if seedErr := SeedPromptTemplates(promptTemplateRepo); seedErr != nil {
		log.Warn("seed 提示词模板失败（将使用内置默认）", port.Err(seedErr))
	}
	uc.ContentUC.SetPromptTemplateRepo(promptTemplateRepo)
	router.SetPromptTemplates(promptTemplateRepo)

	// 知识库
	uc.KnowledgeUC = initKnowledge(geoRepos, cfg, settingRepo, aiGenerator, toolRegistry, log, router, uc.ContentUC)

	// 门店/附近同行/诊断
	initLocalLife(geoRepos, cfg, aiGenerator, llmCfgRepo, settingRepo, cacheStore, log, router, uc, geoBrandUC)

	// 收录通知注入
	if uc.IndexSubmitter != nil {
		uc.ContentUC.SetPublicBaseURL(cfg.Server.PublicBaseURL)
		uc.ContentUC.SetURLSubmitter(uc.IndexSubmitter)
	}

	// 诊断→优化闭环
	if uc.DiagnoseUC != nil {
		uc.ContentUC.SetDiagnoseUC(uc.DiagnoseUC)
	}

	// 关键词蒸馏
	brandWebSearcher := ai.NewBrandWebSearcher(webFetcher)
	uc.DistillUC = geo.NewKeywordDistillUseCase(
		ai.NewBrandSource(aiGenerator, geoRepos.Brand, brandWebSearcher),
		ai.NewTextSource(aiGenerator),
		ai.NewSeedSource(aiGenerator),
		ai.NewFileSource(aiGenerator),
		ai.NewWebSource(aiGenerator, webFetcher),
		ai.NewQuestionSource(aiGenerator),
	)
	router.SetKeywordDistill(uc.DistillUC)

	// GEO 路由
	geoRankUC := geo.NewRankUseCase(geoRepos.Result)
	geoRankUC.SetKeywordRepo(geoRepos.Keyword)
	router.SetGEO(geoBrandUC, uc.MonitorUC, geoRankUC, uc.ContentUC, uc.DiagnoseUC)

	log.Info("GEO 业务已启用（品牌监测/排行榜/内容优化/关键词生成/诊断/关键词蒸馏引擎）")
	return uc
}

// initIndexing 初始化收录通知。
func initIndexing(
	geoRepos *GeoRepos,
	cfg *config.Config,
	settingRepo port.SystemSettingRepository,
	router *handler.Router,
	log port.Logger,
	uc *GEOUseCases,
) port.URLSubmitter {
	loadIndexingConfig := func(ctx context.Context) (entity.IndexingConfig, error) {
		return LoadIndexingConfig(ctx, settingRepo, cfg), nil
	}

	cachedSubmitter := urlsubmit.NewCachedProvider(loadIndexingConfig, cfg.Server.PublicBaseURL)
	var submitter port.URLSubmitter = cachedSubmitter

	if cfg.Server.BingAPIKey != "" && cfg.Server.BingSiteURL != "" {
		if bingSub, subErr := urlsubmit.NewBingSubmitter(cfg.Server.BingAPIKey, cfg.Server.BingSiteURL); subErr == nil {
			submitter = urlsubmit.NewMultiSubmitter(cachedSubmitter, bingSub)
		} else {
			log.Warn("Bing URL Submission 渠道构建失败（已跳过）: " + subErr.Error())
		}
	}

	indexingLogRepo := repository.NewGormIndexingLogRepository(geoRepos.DB)
	uc.IndexingUC = indexing.NewIndexingUseCase(settingRepo, indexingLogRepo, geoRepos.Content, submitter, cfg.Server.PublicBaseURL)
	uc.IndexingUC.SetURLProbe(urlprobe.New())
	router.SetIndexing(uc.IndexingUC)

	return urlsubmit.NewLoggingSubmitter(submitter, indexingLogRepo)
}

// initKnowledge 初始化知识库。
func initKnowledge(
	geoRepos *GeoRepos,
	cfg *config.Config,
	settingRepo port.SystemSettingRepository,
	aiGenerator port.AIGenerator,
	toolRegistry *port.ToolRegistry,
	log port.Logger,
	router *handler.Router,
	contentUC *geo.ContentUseCase,
) *knowledge.KnowledgeUseCase {
	loadEmbeddingConfig := func(ctx context.Context) (entity.EmbeddingRuntimeConfig, error) {
		return LoadEmbeddingConfig(ctx, settingRepo, cfg), nil
	}

	kbEmbedder := ai.NewCachedEmbedder(port.EmbeddingConfigLoaderFunc(loadEmbeddingConfig))
	kbVecStore := kbretriever.NewMySQLVectorStore(geoRepos.DB)

	milvusFactory := func(ctx context.Context, cfg entity.EmbeddingRuntimeConfig) (port.VectorStore, error) {
		addr := net.JoinHostPort(cfg.MilvusHost, cfg.MilvusPort)
		dialCtx, dialCancel := context.WithTimeout(ctx, 10*1000*1000*1000) // 10s
		defer dialCancel()
		cli, err := milvusclient.NewClient(dialCtx, milvusclient.Config{Address: addr})
		if err != nil {
			return nil, fmt.Errorf("milvus 连接失败（%s）: %w", addr, err)
		}
		return kbretriever.NewMilvusVectorStore(kbretriever.NewMilvusSDKClient(cli), cfg.MilvusCollection), nil
	}
	kbVecProvider := kbretriever.NewVectorStoreProvider(port.EmbeddingConfigLoaderFunc(loadEmbeddingConfig), kbVecStore, milvusFactory)
	kbRepo := repository.NewGormKnowledgeMaterialRepository(geoRepos.DB, kbVecProvider)

	crawlPolicy := cfg.Crawler.ToPolicy()
	knowledgeUC := knowledge.NewKnowledgeUseCase(kbRepo, settingRepo,
		crawler.NewRateLimitCrawler(crawler.NewSearchCrawler(), crawlPolicy),
		crawler.NewRateLimitCrawler(crawler.NewStaticCrawler(), crawlPolicy),
		kbEmbedder, log)

	kbRetriever := kbretriever.NewKnowledgeRetriever(kbRepo, kbEmbedder)
	knowledgeUC.SetRetriever(kbRetriever)
	contentUC.SetKnowledgeRetriever(kbRetriever)
	router.SetKnowledge(knowledgeUC)
	log.Info("平台知识库已装配（采集任务 + 生成检索注入 + 管理后台动态配置）")

	return knowledgeUC
}

// initLocalLife 初始化本地生活（门店/附近同行/诊断）。
func initLocalLife(
	geoRepos *GeoRepos,
	cfg *config.Config,
	aiGenerator port.AIGenerator,
	llmCfgRepo port.LLMConfigRepository,
	settingRepo port.SystemSettingRepository,
	cacheStore port.CacheStore,
	log port.Logger,
	router *handler.Router,
	uc *GEOUseCases,
	geoBrandUC *geo.BrandUseCase,
) {
	geoLocator := geoadapter.NewAmapGeoCoder(cfg.AMap.APIKey)
	geoPOISearcher := geoadapter.NewAmapPOISearcher(cfg.AMap.APIKey, cfg.AMap.APIVersion)
	geoInputTipper := geoadapter.NewAmapInputTipper(cfg.AMap.APIKey)
	geoMeasurer := geoadapter.NewAmapDistanceMeasurer(cfg.AMap.APIKey)

	if cfg.AMap.IsConfigured() {
		log.Info("本地生活位置服务已启用（高德：地理编码 + 周边 POI 搜索 v" + cfg.AMap.APIVersion + " + 地址联想 + 距离测量）")
	} else {
		log.Info("本地生活位置服务未配置 AMAP_API_KEY（门店暂不编码，附近同行仅 AI 榜）")
	}

	router.SetInputTipper(geoInputTipper)
	geoStoreUC := geo.NewStoreLocationUseCase(geoRepos.Store, geoRepos.Brand)
	geoStoreUC.SetLocator(geoLocator)

	uc.ContentUC.SetStoreRepo(geoRepos.Store)
	uc.ContentUC.SetBrandRepo(geoRepos.Brand)

	geoNearbyUC := geo.NewNearbyUseCase(geoRepos.Brand, geoRepos.Store, geoRepos.Result)
	geoNearbyUC.SetPOISearcher(geoPOISearcher)
	geoNearbyUC.SetDistanceMeasurer(geoMeasurer)
	uc.NearbyUC = geoNearbyUC

	geoAirProbeUC := geo.NewAIRankProbeUseCase(
		ai.NewRoutingProbe(ai.NewAgentProbe(aiGenerator), ai.NewDirectProbe(aiGenerator), llmCfgRepo),
		geoRepos.Brand, geoRepos.Store, geoRepos.Keyword,
		repository.NewGormAIRankProbeRepository(geoRepos.DB), geoPOISearcher,
	)
	geoNearbyUC.SetAIRankProbeRepo(repository.NewGormAIRankProbeRepository(geoRepos.DB))
	router.SetGeoLocal(geoStoreUC, geoNearbyUC)
	router.SetAIRankProbe(geoAirProbeUC)

	router.SetAdvice(geo.NewAdviceUseCase(geoRepos.Brand, geoRepos.Store, geoRepos.Result, geoRepos.Content))
	router.SetCitation(geo.NewCitationUseCase(geoRepos.Result))

	geoHealthUC := geo.NewHealthUseCase(geoRepos.Brand, geoRepos.Result, geoRepos.Content)
	geoIndustryUC := geo.NewIndustryUseCase(geoRepos.Brand, geoRepos.Result)
	if cacheStore != nil {
		geoHealthUC.SetCache(cacheStore)
		geoIndustryUC.SetCache(cacheStore)
	}
	router.SetGEOHealth(geoHealthUC)
	router.SetGEOIndustry(geoIndustryUC)

	uc.DiagnoseUC = geo.NewDiagnoseUseCase(geoRepos.Brand, geoRepos.Result, aiGenerator)
	router.SetGEOHealth(geoHealthUC)
}
