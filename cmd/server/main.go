// Package main 是 WebReaper 的 Web 服务入口（通用数据采集平台）。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/gorm"

	agentadapter "webreaper/internal/adapter/agent"
	"webreaper/internal/adapter/ai"
	authadapter "webreaper/internal/adapter/auth"
	"webreaper/internal/adapter/bing"
	"webreaper/internal/adapter/crawler"
	"webreaper/internal/adapter/crypto"
	geoadapter "webreaper/internal/adapter/geo"
	"webreaper/internal/adapter/handler"
	"webreaper/internal/adapter/lock"
	zaplogger "webreaper/internal/adapter/logger"
	"webreaper/internal/adapter/mock"
	"webreaper/internal/adapter/provider"
	"webreaper/internal/adapter/provider/vidu"
	"webreaper/internal/adapter/provider/viduendpoint"
	"webreaper/internal/adapter/publisher"
	"webreaper/internal/adapter/payment"
	"webreaper/internal/adapter/qrlogin"
	"webreaper/internal/adapter/repository"
	"webreaper/internal/adapter/scheduledtask"
	"webreaper/internal/adapter/storage"
	"webreaper/internal/adapter/telemetry"
	"webreaper/internal/adapter/urlsubmit"
	"webreaper/internal/config"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/account"
	"webreaper/internal/usecase/agentconfig"
	"webreaper/internal/usecase/auth"
	"webreaper/internal/usecase/billing"
	"webreaper/internal/usecase/conversation"
	"webreaper/internal/usecase/crawlconfig"
	"webreaper/internal/usecase/geo"
	"webreaper/internal/usecase/indexing"
	"webreaper/internal/usecase/llmconfig"
	"webreaper/internal/usecase/notification"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/quota"
	"webreaper/internal/usecase/scheduler"
	"webreaper/internal/usecase/stats"
	"webreaper/internal/usecase/structured"
	"webreaper/internal/usecase/systemsettings"
	taskuc "webreaper/internal/usecase/task"
	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/providerconfig"
)

func main() {
	cfg := config.Load()
	logger := zaplogger.MustNewZapLogger(cfg.Server.Env)
	defer logger.Sync()

	// 生产环境配置校验（fail-fast——缺失必填项直接退出）
	if vErr := cfg.Validate(); vErr != nil {
		fmt.Println(vErr)
		os.Exit(1)
	}

	traceShutdown, _, err := telemetry.Init(telemetry.Config{
		Enabled:      cfg.Telemetry.Enabled,
		Exporter:     telemetry.ExporterKind(cfg.Telemetry.Exporter),
		OTLPEndpoint: cfg.Telemetry.OTLPEndpoint,
		ServiceName:  "webreaper",
	})
	if err != nil {
		// trace 初始化失败不阻断启动——降级为 no-op，业务照常运行
		log := zaplogger.MustNewZapLogger(cfg.Server.Env)
		log.Warn("trace 初始化失败，降级为 no-op", port.Err(err))
	}
	defer func() { _ = traceShutdown(context.Background()) }()

	log := logger.With(port.String("component", "main"))
	if cfg.Telemetry.Enabled {
		log.Info("链路追踪已启用",
			port.String("exporter", cfg.Telemetry.Exporter),
			port.String("otlp_endpoint", cfg.Telemetry.OTLPEndpoint))
	}
	log.Info("WebReaper 启动中", port.String("env", cfg.Server.Env))

	// 初始化仓储（降级 mock）
	agentConfigRepo, llmConfigRepo, taskRepo, userRepo, convRepo, msgRepo, settingRepo := initRepositories(cfg.DB, logger)

	// 爬虫限流策略（从 config 读取，可经 .env 的 CRAWLER_REQUEST_INTERVAL_MS 调配）
	crawlPolicy := cfg.Crawler.ToPolicy()
	// 同步合规开关到爬虫全局状态（User-Agent 可识别化 + robots 遵守开关）
	crawler.SetGlobalUserAgent(cfg.Crawler.UserAgent)
	crawler.SetRespectRobots(crawlPolicy.RespectRobots)
	log.Info("爬虫限流策略",
		port.String("interval_ms", fmt.Sprintf("%d", crawlPolicy.RequestIntervalMs)),
		port.String("timeout_ms", fmt.Sprintf("%d", crawlPolicy.RequestTimeoutMs)),
		port.String("respect_robots", fmt.Sprintf("%v", crawlPolicy.RespectRobots)),
		port.String("user_agent", cfg.Crawler.UserAgent))

	// 爬虫工具注册（7种：4基础+3装饰器，全部用 RateLimitCrawler 包装限流）
	// 用装饰器模式：RateLimitCrawler 在最外层，统一节流，不侵入各爬虫实现。
	toolRegistry := port.NewToolRegistry()
	registerLimited := func(c port.CrawlerTool) {
		toolRegistry.Register(crawler.NewRateLimitCrawler(c, crawlPolicy))
	}
	// 4 种基础爬虫（各自独立可用）
	registerLimited(crawler.NewAPICrawler())
	registerLimited(crawler.NewStaticCrawler())
	registerLimited(crawler.NewDynamicCrawler())
	registerLimited(crawler.NewSearchCrawler())
	// Tavily 搜索（专为 AI 设计的高质量搜索源）
	// 无论有没有 Key 都注册实例（管理后台可运行时配 Key），Key 空时禁用
	tavilyCrawler := crawler.NewTavilyCrawler(cfg.Tavily.APIKey)
	registerLimited(tavilyCrawler)
	if cfg.Tavily.IsConfigured() {
		log.Info("Tavily 搜索工具已就绪（tavily_search）")
	} else {
		toolRegistry.SetEnabled("tavily_search", false) // 无 Key 时禁用
		log.Info("Tavily 搜索工具已注册但未启用（需在管理后台配置 API Key）")
	}
	// 3 种装饰器爬虫（包装基础爬虫，增加能力）
	// 注意：装饰器需要指定被包装的基础爬虫，这里用 static_crawler 作为默认基础
	staticCrawler := crawler.NewStaticCrawler()
	registerLimited(crawler.NewFocusedCrawler(staticCrawler, []string{})) // 关键词由 Agent 动态指定
	registerLimited(crawler.NewIncrementalCrawler(staticCrawler))
	registerLimited(crawler.NewDeepCrawler(staticCrawler))

	// AI 生成器（注入 LLMConfigRepository 用于运行时按配置选 LLM，注入 ToolRegistry 让 LLM 调用全部爬虫）
	aiGenerator := initAIGenerator(cfg.LLM, llmConfigRepo, toolRegistry, msgRepo, logger)

	// LLM 配置管理用例（封装 LLMConfig CRUD + seed default）
	llmCfgUC := llmconfig.NewLLMConfigUseCase(llmConfigRepo)
	// 首次启动 seed 默认 LLM 配置（移到 usecase 内）
	if cfg.LLM.IsConfigured() {
		_ = llmCfgUC.EnsureDefault(context.Background(), entity.LLMConfig{
			Name:     "default",
			Provider: cfg.LLM.Provider,
			APIKey:   cfg.LLM.APIKey,
			BaseURL:  cfg.LLM.BaseURL,
			Model:    cfg.LLM.Model,
		})
	}


	// 认证
	hasher := authadapter.NewBcryptHasher()
	var tokenGen port.TokenGenerator = authadapter.NewJWTGenerator(cfg.JWT.Secret, cfg.JWT.Expiration)
	var tokenParser *authadapter.JWTGenerator
	if cfg.JWT.Secret != "" {
		tokenParser = tokenGen.(*authadapter.JWTGenerator)
	}
	registerUC := auth.NewRegisterUseCase(userRepo, hasher)
	loginUC := auth.NewLoginUseCase(userRepo, hasher, tokenGen)

	// 首次启动 seed 默认管理员账号（多租户改造后需要至少一个 admin）
	// 用固定用户名 admin / admin123，已存在则忽略。生产环境部署后请立即改密。
	_, seedErr := registerUC.Execute(context.Background(), auth.RegisterInput{
		Username: "admin", Password: "admin123", Role: entity.RoleAdmin,
	})
	if seedErr == nil {
		log.Info("已 seed 默认管理员账号 admin/admin123（请立即修改密码）")
	}

	// Agent 执行器（解耦后不依赖 DataItemRepository——工具结果不自动落库）
	agentRunner := agentadapter.NewTrpcAgentRunner(llmConfigRepo, toolRegistry, logger)

	// 业务用例装配（handler 只依赖这些 usecase，不直接持有仓储）
	var statsUC *stats.StatsUseCase
	agentCfgUC := agentconfig.NewAgentConfigUseCase(agentConfigRepo)
	conversationUC := conversation.NewConversationUseCase(convRepo, msgRepo)
	crawlCfgUC := crawlconfig.NewCrawlConfigUseCase(settingRepo)
	// 首次启动 seed 默认采集策略
	_ = crawlCfgUC.EnsureDefault(context.Background())

	// 框架内容编排工具（图编排：探查→生成→校验→补生成）。
	// 仅配了 LLM 时启用；包装成 generate_content 工具注册进工具池。
	// 注意：原 orchestrate usecase 已删除（它把结果落库到 dataitem，与 GEO 无关）。
	// ContentGenerationTool 直接返回生成内容给 LLM，不落库。
	if cfg.LLM.IsConfigured() {
		graphOrchestrator := agentadapter.NewGraphContentOrchestrator(
			aiGenerator,
			[]string{"static_crawler", "search_crawler", "dynamic_crawler"}, // scout 探查文档用的爬虫
			logger,
		)
		toolRegistry.Register(agentadapter.NewContentGenerationTool(graphOrchestrator))
		log.Info("内容生成工具已注册（图编排模式，结果直接返回 LLM 不落库）")
	} else {
		log.Info("未配置 LLM，内容生成工具降级禁用")
	}

	// 异步任务
	taskQueue := mock.NewMockTaskQueue(100)
	enqueueUC := taskuc.NewEnqueueUseCase(taskQueue, taskRepo)

	// 注册 Agent 异步处理器
	handlerRegistry := taskuc.NewHandlerRegistry()
	handlerRegistry.Register(taskuc.NewAgentHandler(agentRunner))
	dispatchUC := taskuc.NewDispatchUseCase(handlerRegistry, logger)
	worker := taskuc.NewWorker(taskQueue, taskRepo, dispatchUC, logger)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	go worker.Start(workerCtx)

	// mocksite
	if os.Getenv("MOCKSITE_ENABLED") == "true" {
		go startMockSite(log)
	}

	// 路由 + HTTP 服务（handler 只依赖 usecase 与 port 接口，不直接持有仓储/具体 adapter struct）
	// 路由器（零参数——所有依赖通过 SetXxx 可选注入，端点按注入条件注册）
	router := handler.NewRouter()
	router.SetAuth(registerUC, loginUC, tokenParser)
	router.SetAI(aiGenerator)
	router.SetTask(enqueueUC, agentRunner)
	router.SetAgentConfig(agentCfgUC)
	router.SetLLMConfig(llmCfgUC)
	router.SetConversation(conversationUC)
	router.SetCrawlConfig(crawlCfgUC)
	router.SetToolRegistry(toolRegistry)

	// GEO 业务装配（商户端核心）。需要 DB + LLM 才启用。
	geoRepos := initGEORpositories(cfg.DB)

	// LLM 用量计量（经济系统基础）：所有 LLM 调用 token 落库（usages 表），
	// 支撑后续套餐额度/账单/用量报表——租户与场景由调用方经 ctx 注入。
	if geoRepos != nil {
		if gen, ok := aiGenerator.(*ai.TrpcAgentGenerator); ok {
			gen.SetUsageRecorder(repository.NewGormUsageRecorder(geoRepos.db))
			log.Info("LLM 用量计量已启用（usages 表，按租户/场景记录 token）")
		}
	}
	// 结构化数据用例（JSON-LD/llms.txt 生成）——纯逻辑零依赖，无 DB/LLM 也可用
	structuredUC := structured.NewStructuredDataUseCase()
	router.SetStructured(structuredUC)
	// 公开内容站（让 AI 引擎/搜索引擎可爬取已发布内容）——需要 DB（内容仓储）
	// 收录通知（多渠道并行、失败互不影响；配置运行时动态可调）：
	//   - 配置来源：system_settings（管理后台可改）优先，.env 兜底
	//   - CachedProvider 按 30s TTL 重建 submitter——改配置免重启（参照 LLMConfig 模式）
	//   - 内容发布为 published 时自动触发；未配置任何渠道则空转（不阻断业务）
	var indexNowSubmitter port.URLSubmitter
	var indexingUC *indexing.IndexingUseCase
	if geoRepos != nil {
		publicHandler := handler.NewPublicHandler(geoRepos.content, geoRepos.brand, structuredUC, cfg.Server.PublicBaseURL)
		// 门店档案注入公开站（本地生活 P0：文章页 NAP 信息块 + JSON-LD 门店节点）
		publicHandler.SetStoreRepo(geoRepos.store)
		// 门店位置静态图（P2）：302 到高德静态地图，Key 不暴露给浏览器
		publicHandler.SetStaticMapKey(cfg.AMap.APIKey)
		router.SetPublic(publicHandler)

		// 收录配置加载器：DB（system_settings）优先，env 兜底（main 装配层职责）
		loadIndexingConfig := func(ctx context.Context) (entity.IndexingConfig, error) {
			if s, sErr := settingRepo.Get(ctx, entity.SettingKeyIndexingConfig); sErr == nil {
				var c entity.IndexingConfig
				if json.Unmarshal([]byte(s.Value), &c) == nil {
					return c, nil
				}
			}
			return entity.IndexingConfig{
				IndexNowKey: cfg.Server.IndexNowKey,
				BaiduSite:   cfg.Baidu.Site,
				BaiduToken:  cfg.Baidu.Token,
			}, nil
		}

		cachedSubmitter := urlsubmit.NewCachedProvider(loadIndexingConfig, cfg.Server.PublicBaseURL)
		// key 文件端点读运行时配置（管理后台改 key 后即时生效）
		publicHandler.SetIndexNowKeyProvider(func(ctx context.Context) string {
			c, _ := loadIndexingConfig(ctx)
			return c.IndexNowKey
		})

		// 收录管理用例（管理后台：配置读写/提交日志/手动补提交）
		indexingLogRepo := repository.NewGormIndexingLogRepository(geoRepos.db)
		indexingUC = indexing.NewIndexingUseCase(settingRepo, indexingLogRepo, geoRepos.content, cachedSubmitter, cfg.Server.PublicBaseURL)
		router.SetIndexing(indexingUC)
		indexNowSubmitter = cachedSubmitter
	}
	var geoMonitorUCRef *geo.MonitorUseCase
	var geoContentUCRef *geo.ContentUseCase
	var geoDistillUCRef *geo.KeywordDistillUseCase
	var geoNearbyUCRef *geo.NearbyUseCase      // X-01：附近同行配额注入用
	var geoDiagnoseUCRef *geo.DiagnoseUseCase  // X-01：诊断配额注入用
	if geoRepos != nil && cfg.LLM.IsConfigured() {
		geoScorer := ai.NewLLMGEOScorer(aiGenerator)
		geoBrandUC := geo.NewBrandUseCase(geoRepos.brand, geoRepos.keyword)
		geoBrandUC.SetAIGenerator(aiGenerator) // 关键词生成用
		// 本地意图关键词（P0 补全）：品牌有门店时生成"望京 川菜馆"类本地搜索词
		geoBrandUC.SetStoreRepo(geoRepos.store)

		// WebFetcher 供 RAG 监测 + 关键词发现的 RAG 增强共用
		webFetcher := ai.NewWebFetcher()
		// 关键词生成 RAG 增强：结合全网内容生成更准的关键词
		geoBrandUC.SetWebSearcher(ai.NewBrandWebSearcher(webFetcher))

		// 监测引擎：AgentProbe 为首选（Agent 自主搜索——把搜索工具交给 LLM Agent，让它自主搜索+综合回答）。
		// 这是最接近真实 AI 搜索引擎的监测方式：
		//   真实引擎（豆包/Kimi）= Agent 自主搜索 + LLM 综合
		//   AgentProbe           = Agent 自主调 search_crawler + LLM 综合
		// 小众品牌只要网上有内容，搜索工具就能爬到，Agent 综合回答时就会提及。
		// 监测引擎：RoutingProbe 按 EngineName 路由——
		//   选了真实引擎（LLMConfig 存在，如豆包/Kimi）→ DirectProbe 真实直测
		//   未选/配置不存在 → AgentProbe 模拟引擎（Agent 自主搜索兜底）
		geoProbe := ai.NewRoutingProbe(
			ai.NewAgentProbe(aiGenerator),
			ai.NewDirectProbe(aiGenerator),
			llmConfigRepo,
		)
		log.Info("GEO 监测引擎：RoutingProbe（真实引擎直测 + Agent 模拟兜底）")

		geoMonitorUC := geo.NewMonitorUseCase(geoRepos.brand, geoRepos.keyword, geoRepos.result, geoProbe)
		// 采样矩阵·问法维度 v2（去缓存）：LLM 按品牌/卖点/竞品/门店地址生成真实问法池，
		// 多引擎分片隔离（相同 prompt 不跨引擎命中缓存）；生成失败时 probe 内部模板兜底
		geoMonitorUC.SetQuestionGenerator(ai.NewLLMQuestionGenerator(aiGenerator))
		// 归因生命线（P5-01）：注入自营公开站域名——探测统计"AI 回答引用的来源里
		// 包含自营站内容的次数"，回答"我们做的内容到底有没有被 AI 引用"。
		geoMonitorUC.SetSelfBaseDomain(cfg.Server.PublicBaseURL)
		// 本地监测问法（P0 补全）：有门店时问"望京附近有什么川菜馆"——测本地生意
		geoMonitorUC.SetStoreRepo(geoRepos.store)
		geoMonitorUCRef = geoMonitorUC
		geoRankUC := geo.NewRankUseCase(geoRepos.result)
		geoRankUC.SetKeywordRepo(geoRepos.keyword) // Overview 的品牌关键词数（仪表盘排行）
		geoContentUC := geo.NewContentUseCase(aiGenerator, geoScorer, geoRepos.content)
		geoContentUCRef = geoContentUC
		// 免费规则评分器：优化前后对比用（不烧 token、可单测）
		geoContentUC.SetRuleScorer(geo.NewRuleScorer())
		// RAG 增强：原创生成前检索"品牌+关键词"真实信息注入 prompt（"不编造数据"变能力）
		geoContentUC.SetRAGRetriever(ai.NewWebContentRetriever(webFetcher))
		geoContentUC.SetLogger(log)
		// 提示词模板仓库：内容生成/优化系统提示词可管理、可热更新（seed 内置默认模板）
		promptTemplateRepo := repository.NewGormPromptTemplateRepository(geoRepos.db)
		if seedErr := seedPromptTemplates(promptTemplateRepo); seedErr != nil {
			log.Warn("seed 提示词模板失败（将使用内置默认）", port.Err(seedErr))
		}
		geoContentUC.SetPromptTemplateRepo(promptTemplateRepo)
		router.SetPromptTemplates(promptTemplateRepo) // admin 管理端点（列表/热更新）

		// 本地生活改造（P0/P1/P2）：门店档案 + 高德位置服务 + 附近同行双榜。
		// 策略模式 + 双实现降级：配置 AMAP_API_KEY 走高德真实 API，
		// 否则 mock 降级（门店照常创建 geo_status=pending，附近同行只显示 AI 榜）。
		geoLocator := geoadapter.NewAmapGeoCoder(cfg.AMap.APIKey)
		geoPOISearcher := geoadapter.NewAmapPOISearcher(cfg.AMap.APIKey, cfg.AMap.APIVersion)
		geoInputTipper := geoadapter.NewAmapInputTipper(cfg.AMap.APIKey)          // P1 地址联想
		geoMeasurer := geoadapter.NewAmapDistanceMeasurer(cfg.AMap.APIKey)        // P2 驾车耗时
		if cfg.AMap.IsConfigured() {
			log.Info("本地生活位置服务已启用（高德：地理编码 + 周边 POI 搜索 v"+cfg.AMap.APIVersion+" + 地址联想 + 距离测量）")
		} else {
			log.Info("本地生活位置服务未配置 AMAP_API_KEY（门店暂不编码，附近同行仅 AI 榜）")
		}
		router.SetInputTipper(geoInputTipper)
		geoStoreUC := geo.NewStoreLocationUseCase(geoRepos.store, geoRepos.brand)
		geoStoreUC.SetLocator(geoLocator)
		// 内容生成注入门店 NAP（地址/营业时间/电话——本地信任信号，P0-04）
		geoContentUC.SetStoreRepo(geoRepos.store)
		geoContentUC.SetBrandRepo(geoRepos.brand) // BizType 分流（online 跳过线下 NAP）
		geoNearbyUC := geo.NewNearbyUseCase(geoRepos.brand, geoRepos.store, geoRepos.result)
		geoNearbyUC.SetPOISearcher(geoPOISearcher)
		geoNearbyUC.SetDistanceMeasurer(geoMeasurer) // P2 驾车耗时（未配置自动降级）
		geoNearbyUCRef = geoNearbyUC
		// AI 榜单探查（v2：AI 榜真实数据源——中性问法真实搜索 + 附近名单归因匹配，缓存 24h）
		geoAirProbeUC := geo.NewAIRankProbeUseCase(
			ai.NewRoutingProbe(ai.NewAgentProbe(aiGenerator), ai.NewDirectProbe(aiGenerator), llmConfigRepo),
			geoRepos.brand, geoRepos.store, geoRepos.keyword,
			repository.NewGormAIRankProbeRepository(geoRepos.db), geoPOISearcher,
		)
		geoNearbyUC.SetAIRankProbeRepo(repository.NewGormAIRankProbeRepository(geoRepos.db))
		router.SetGeoLocal(geoStoreUC, geoNearbyUC)
		router.SetAIRankProbe(geoAirProbeUC)
		// 行动建议（P5-05：规则引擎，给老板"下一步做什么"）
		router.SetAdvice(geo.NewAdviceUseCase(geoRepos.brand, geoRepos.store, geoRepos.result, geoRepos.content))
		// 内容引用统计（P5-02：每篇被 AI 引用几次——评分校准数据源）
		router.SetCitation(geo.NewCitationUseCase(geoRepos.result))

		// 收录通知（IndexNow）：发布为 published 时自动通知搜索引擎
		if indexNowSubmitter != nil {
			geoContentUC.SetPublicBaseURL(cfg.Server.PublicBaseURL)
			geoContentUC.SetURLSubmitter(indexNowSubmitter)
		}
		geoDiagnoseUC := geo.NewDiagnoseUseCase(geoRepos.brand, geoRepos.result, aiGenerator)
		geoDiagnoseUCRef = geoDiagnoseUC
		router.SetGEO(geoBrandUC, geoMonitorUC, geoRankUC, geoContentUC, geoDiagnoseUC)
		// 诊断→优化闭环（P5-03）：生成内容时可选择"先诊断再对症下药"
		geoContentUC.SetDiagnoseUC(geoDiagnoseUC)

		// 关键词蒸馏引擎：五种来源策略（策略模式 + 工厂）
		brandWebSearcher := ai.NewBrandWebSearcher(webFetcher)
		geoDistillUC := geo.NewKeywordDistillUseCase(
			ai.NewBrandSource(aiGenerator, geoRepos.brand, brandWebSearcher), // 品牌信息+全网
			ai.NewTextSource(aiGenerator),                                    // 用户文本
			ai.NewSeedSource(aiGenerator),                                    // 种子词拓展
			ai.NewFileSource(aiGenerator),                                    // 文件内容
			ai.NewWebSource(aiGenerator, webFetcher),                         // 网络爬取
		)
		geoDistillUCRef = geoDistillUC
		router.SetKeywordDistill(geoDistillUC)
		log.Info("GEO 业务已启用（品牌监测/排行榜/内容优化/关键词生成/诊断/关键词蒸馏引擎）")
	} else {
		log.Info("GEO 业务未启用（需配置 DB + LLM_API_KEY）")
	}

	// 多平台发布账号域装配（扫码绑定 + 半自动/全自动发布）。需要 DB 才启用。
	var geoAccountUC *account.AccountUseCase
	var geoPublishUC *account.PublishUseCase
	var accountRepos *accountRepos // 提升到外层：平台总览统计需要发布任务计数
	if geoRepos != nil {
		accountRepos = initAccountRepositories(cfg.DB)
		if accountRepos != nil {
			// cookie 加密保险库（需要 PUBLISH_COOKIE_SECRET）
			var vault port.CookieVault
			if cfg.Publish.CookieSecret != "" {
				v, err := crypto.NewAESCookieVault(cfg.Publish.CookieSecret)
				if err != nil {
					log.Error("cookie 加密保险库初始化失败，扫码登录将不可用", port.Err(err))
				} else {
					vault = v
				}
			} else {
				log.Warn("PUBLISH_COOKIE_SECRET 未配置，扫码登录不可用（cookie 无法加密存储）")
			}

			// 扫码登录（浏览器自动化，需要 vault 才有意义）
			// QR_LOGIN_HEADED=true 时显示浏览器窗口（调试用），默认 false 走灰盒 headless
			var qrSession port.QRLoginSession
			if vault != nil {
				qrSession = qrlogin.NewChromedpQRLogin(cfg.Publish.QRLoginHeaded)
				if cfg.Publish.QRLoginHeaded {
					log.Info("扫码登录运行在「显示模式」（QR_LOGIN_HEADED=true，浏览器窗口可见，仅供调试）")
				} else {
					log.Info("扫码登录运行在「灰盒模式」（headless 无头，生产默认）")
				}
			}

			geoAccountUC = account.NewAccountUseCase(accountRepos.account, qrSession, vault)
			// 发布通道注册表（工厂模式，已注册知乎/小红书全自动通道——同时支持半自动+全自动）
			channelRegistry := publisher.NewChannelRegistry()
			geoPublishUC = account.NewPublishUseCase(accountRepos.job, channelRegistry, accountRepos.account, vault)
			// 注入发布效果追踪（发布成功后自动触发监测对比提及率）
			geoPublishUC.SetMonitorTrigger(geoMonitorUCRef)
			// 注入公开站根地址（发布内容尾部带公开站链接，加速爬虫发现）
			geoPublishUC.SetPublicBaseURL(cfg.Server.PublicBaseURL)
			// 注入账号池（全自动发布时自动选最优账号——最久未使用优先）
			geoPublishUC.SetAccountPool(repository.NewGormAccountPool(accountRepos.account))

			router.SetAccount(geoAccountUC, geoPublishUC)
			log.Info("多平台发布已启用（账号绑定 + 半自动/全自动发布：知乎/小红书）")
		}
	} else {
		log.Info("多平台发布未启用（需配置 DB）")
	}

	// 平台总览统计（SaaS 级聚合，一次返回全量指标）：
	// GEO/发布仓储未装配（无 DB）时注入 nil → StatsUseCase 内判 nil，对应指标为 0。
	if geoRepos != nil && accountRepos != nil {
		statsUC = stats.NewStatsUseCase(userRepo,
			geoRepos.brand, geoRepos.keyword, geoRepos.result, geoRepos.content,
			accountRepos.job)
	} else {
		statsUC = stats.NewStatsUseCase(userRepo, nil, nil, nil, nil, nil)
	}
	router.SetStats(statsUC)

	// 经济系统（套餐/订阅/订单）：seed 内置默认套餐，admin 可管理
	if geoRepos != nil {
		planRepo := repository.NewGormPlanRepository(geoRepos.db)
		subRepo := repository.NewGormSubscriptionRepository(geoRepos.db)
		orderRepo := repository.NewGormOrderRepository(geoRepos.db)
		if seedErr := billing.SeedPlans(context.Background(), planRepo); seedErr != nil {
			log.Warn("seed 默认套餐失败（将无在售套餐）", port.Err(seedErr))
		}
		billingUC := billing.NewBillingUseCase(planRepo, subRepo, orderRepo)
		billingUC.SetSettingRepo(settingRepo)

		// 支付网关策略选择：根据 system_settings 的 payment_config 决定用 mock 还是 zpay
		// 未配置或配置不完整 → mock（开发演示）；配置完整 → zpay（真实收款）
		var paymentGW port.PaymentGateway = payment.NewMockPaymentGateway(cfg.Server.PublicBaseURL)
		if payCfg, gwErr := settingRepo.Get(context.Background(), entity.SettingKeyPaymentConfig); gwErr == nil {
			var pc map[string]string
			if json.Unmarshal([]byte(payCfg.Value), &pc) == nil && pc["gateway"] == "zpay" && pc["pid"] != "" && pc["key"] != "" {
				paymentGW = payment.NewZPayGateway(payment.ZPayConfig{
					PID: pc["pid"], Key: pc["key"],
					NotifyURL: pc["notify_url"], ReturnURL: pc["return_url"],
				})
				log.Info("支付网关已接入 ZPAY（真实收款模式）")
			}
		}
		if _, ok := paymentGW.(*payment.ZPayGateway); !ok {
			log.Info("支付网关运行在 mock 模式（未配置 ZPAY 或配置不完整，admin 后台可设置）")
		}
		billingUC.SetPaymentGateway(paymentGW)
		router.SetBilling(billingUC)

		// 配额检查门（计数派生型：plan 配额 vs usages 表当月用量）
		// 注入到烧 token 的 usecase——超限返回 ErrQuotaExceeded → HTTP 402
		usageRecorder := repository.NewGormUsageRecorder(geoRepos.db)
		// X-01 商业闭环成本侧：成本分析（admin 报表）——参考单价来自 LLM_COST_PER_MToken
		billingUC.SetUsageStats(usageRecorder)
		billingUC.SetReferencePricePerMToken(cfg.LLM.CostPerMTokenCents)
		// P1-1：按引擎单价成本分析（llm_configs.cost_per_mtok——豆包 vs GPT 级差异化）
		billingUC.SetLLMConfigRepo(llmConfigRepo)
		quotaGate := quota.NewGate(planRepo, subRepo, usageRecorder)
		router.SetQuotaGate(quotaGate) // ChatHandler 等无独立 usecase 的端点用
		if geoContentUCRef != nil {
			geoContentUCRef.SetQuotaGate(quotaGate)
		}
		if geoMonitorUCRef != nil {
			geoMonitorUCRef.SetQuotaGate(quotaGate)
		}
		if geoDistillUCRef != nil {
			geoDistillUCRef.SetQuotaGate(quotaGate)
		}
		// X-01 新场景计量：附近同行（nearby）+ 诊断（diagnose）配额
		if geoNearbyUCRef != nil {
			geoNearbyUCRef.SetQuotaGate(quotaGate)
			geoNearbyUCRef.SetUsageRecorder(usageRecorder)
		}
		if geoDiagnoseUCRef != nil {
			geoDiagnoseUCRef.SetQuotaGate(quotaGate)
		}
		log.Info("配额检查已启用（content-opt/content-gen/monitor/keyword-distill/nearby/diagnose 超限返回 402）")
	}

	// 平台系统设置（运行时开关：自动盯盘等）——管理后台可切换，调度器即时生效
	tenantSettingRepo := repository.NewGormTenantSettingRepository(geoRepos.db)
	settingsUC := systemsettings.NewSystemSettingsUseCase(settingRepo)
	settingsUC.SetTenantSettingRepo(tenantSettingRepo)
	router.SetSystemSettings(settingsUC)

	// 站内通知（主动唤醒：提及率变化/自动复测/排期发布）
	notifyUC := notification.NewNotifyUseCase(repository.NewGormNotificationRepository(geoRepos.db))
	router.SetNotifications(notifyUC)

	// 管理端装配（用户管理，仅 admin）
	router.SetAdmin(userRepo)

	// 通用定时任务调度器（统一驱动：防重入/分布式锁/panic 恢复/错误日志）。
	// 新增定时功能 = 实现 port.ScheduledTask + Register 一行，避免"一功能一套 ticker"。
	// 分布式演进：多实例部署时把 NoopLock 换成 lock.RedisLock，业务零改动。
	schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
	taskScheduler := scheduler.New(lock.NewNoopLock(), log)

	// ① 账号健康度定时检查：每 10 分钟检查所有账号的 cookie 过期状态
	//（原裸 goroutine + ticker 改造为注册任务——统一调度语义）
	if geoAccountUC != nil {
		accUC := geoAccountUC
		_ = taskScheduler.Register(scheduledtask.NewAccountHealthTask(accUC, log))
	}

	// ② 每日自动监测（自动盯盘：让趋势图自动生长，无需用户手动点监测）
	// 需要 DB + LLM + 开启开关（AUTO_MONITOR_ENABLED=true）
	if geoMonitorUCRef != nil && geoRepos != nil && cfg.LLM.IsConfigured() && cfg.Server.AutoMonitorEnabled {
		monUC := geoMonitorUCRef
		dailyTask := scheduledtask.NewDailyMonitorTask(monUC, geoRepos.brand, settingRepo, tenantSettingRepo, cfg.Server.AutoMonitorEnabled, log)
		// 套餐能力位门禁：auto-monitor 是付费能力（free 无）——免费用户不参与自动盯盘
		dailyTask.SetPlanGate(
			repository.NewGormPlanRepository(geoRepos.db),
			repository.NewGormSubscriptionRepository(geoRepos.db),
		)
		if notifyUC != nil {
			dailyTask.SetNotifier(scheduledtask.NewMonitorNotifier(geoRepos.result, notifyUC, log))
		}
		_ = taskScheduler.Register(dailyTask)
		log.Info("每日自动监测任务已注册（AUTO_MONITOR_ENABLED=true）")
	}

	// 排期发布（定时发送）：每 5 分钟扫描到期任务
	if geoPublishUC != nil && accountRepos != nil {
		_ = taskScheduler.Register(scheduledtask.NewScheduledPublishTask(accountRepos.job, geoPublishUC, notifyUC, log))
	}
	// 自动复测：发布 7 天后自动复测提及率并通知（效果追踪闭环）
	if geoPublishUC != nil && accountRepos != nil {
		_ = taskScheduler.Register(scheduledtask.NewAutoRecheckTask(accountRepos.job, geoPublishUC, notifyUC, settingRepo, log))
	}
	// 收录状态验证：每日查询已发布内容是否被 Bing 真正收录（IndexNow 提交 ≠ 收录）
	if geoRepos != nil {
		var indexChecker port.IndexStatusChecker
		if cfg.Server.BingAPIKey != "" {
			indexChecker = bing.NewChecker(cfg.Server.BingAPIKey, cfg.Server.BingSiteURL)
		}
		_ = taskScheduler.Register(scheduledtask.NewIndexCheckTask(geoRepos.content, indexChecker, cfg.Server.PublicBaseURL, log))
	}

	// ③ 统一生成任务（Vidu 全量接入：视频 5+图片/音频/数字人——Docs/Plans/03 计划文档）
	// 协议层 + 端点策略（viduendpoint：能力向量校验/请求体组装）+ 回调验签；
	// 未配 key 走 MockGenerationProvider（模拟进度，前端全流程可演示）。
	// 配额（generation 场景）在 P1 随计费场景扩展注入；当前 mock 模式无真实成本。
	if geoRepos != nil {
		genRegistry := viduendpoint.NewRegistry()
		// 厂商配置 DB 优先（管理后台可设置 Vidu API Key），环境变量兜底
		providerCfgRepo := repository.NewGormProviderConfigRepository(geoRepos.db)
		viduAPIKey := cfg.Server.ViduAPIKey
		if dbCfg, cfgErr := providerCfgRepo.Get(context.Background(), "vidu"); cfgErr == nil && dbCfg.APIKey != "" {
			viduAPIKey = dbCfg.APIKey
			log.Info("统一生成使用厂商配置（管理后台 DB 设置 Vidu API Key）")
		}
		var genProvider port.GenerationProvider = provider.NewMockGenerationProvider()
		if viduAPIKey != "" {
			genProvider = vidu.NewViduProvider(viduAPIKey)
			log.Info("统一生成已接入 Vidu（真实 API）")
		} else {
			log.Info("统一生成运行在 mock 模式（未配置 VIDU_API_KEY）")
		}
		router.SetProviderConfig(providerconfig.NewUseCase(providerCfgRepo))
		// 生成规格 DB 驱动（全局掌控）：首次启动 seed 出厂默认 → 管理后台全量可编辑，
		// 30s TTL 热生效（不重启）；删除行 = 恢复出厂默认
		genSpecRepo := repository.NewGormGenerationSpecRepository(geoRepos.db)
		genRegistry.SetSpecRepo(genSpecRepo)
		if seedErr := genRegistry.SeedDefaults(context.Background()); seedErr != nil {
			log.Warn("seed 生成规格默认值失败", port.Err(seedErr))
		}
		genUC := generation.NewGenerationUseCase(genProvider, genRegistry, repository.NewGormGenerationTaskRepository(geoRepos.db))
		// 媒体资产存储——双模式切换（STORAGE_TYPE 环境变量控制）
		// local（默认/本地开发）：LocalMediaStore（./data/media + /media 静态托管）
		// oss（云端部署）：OSSMediaStore（阿里云 OSS，素材+产物持久化到云端）
		var mediaStore port.MediaAssetStore
		var mediaDir string
		switch cfg.Storage.Type {
		case "oss":
			// 云服务器用内网 endpoint（免流量费），本地开发用公网
			ep := cfg.Storage.Endpoint
			if cfg.Storage.InternalEndpoint != "" {
				ep = cfg.Storage.InternalEndpoint
			}
			ms, ossErr := storage.NewOSSMediaStore(ep, cfg.Storage.Bucket, cfg.Storage.AccessKey, cfg.Storage.SecretKey, cfg.Storage.PublicDomain)
			if ossErr != nil {
				log.Warn("OSS 初始化失败，降级本地存储", port.Err(ossErr))
			} else {
				mediaStore = ms
				log.Info("媒体存储已启用（阿里云 OSS）")
			}
		default:
			ms, lErr := storage.NewLocalMediaStore("./data/media", cfg.Server.PublicBaseURL)
			if lErr == nil {
				mediaStore = ms
				mediaDir = "./data/media"
				log.Info("媒体存储已启用（本地目录 ./data/media）")
			} else {
				log.Warn("媒体存储初始化失败", port.Err(lErr))
			}
		}
		if mediaStore != nil {
			genUC.SetAssetStore(mediaStore)
			router.SetMedia(mediaStore, mediaDir)
		}
		router.SetGeneration(genUC, genProvider, genRegistry, genSpecRepo)
		// 并发节流（P3）：限制同时提交到 Vidu 的请求数，防瞬时高峰触发 QuotaExceeded/429
		genUC.SetConcurrency(5)
		// 轮询驱动：20s 周期扫描未终态任务（回调到达后幂等跳过；双通道合并）
		_ = taskScheduler.Register(scheduledtask.NewGenerationPollTask(genUC, log))
		// 任务清理（P3）：每日清理 30 天前终态任务 + 过期素材文件
		_ = taskScheduler.Register(scheduledtask.NewGenerationCleanupTask(genUC, log))
	}
	taskScheduler.Start(schedulerCtx)

	// 路由统一前缀（生产部署在 nginx 后面分流用，如 /webreaper；空=无前缀）
	router.SetAPIPrefix(cfg.Server.APIPrefix)
	// 健康检查（healthz 端点检查 DB 连通性——Docker healthcheck 用）
	if geoRepos != nil {
		router.SetHealthCheck(func() error {
			sqlDB, _ := geoRepos.db.DB()
			return sqlDB.Ping()
		})
	}

	server := &http.Server{Addr: ":" + cfg.Server.Port, Handler: router.Engine()}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("HTTP 服务已启动", port.String("port", cfg.Server.Port))
		if cfg.JWT.Secret == "" {
			log.Warn("JWT_SECRET 未配置，API 认证已禁用")
		}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("服务启动失败", port.Err(err))
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("正在关闭服务...")
	schedulerCancel()
	taskScheduler.Stop()
	workerCancel()
	_ = taskQueue.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // RPA 发布可能需要较长时间完成
	defer cancel()
	_ = server.Shutdown(ctx)
	log.Info("服务已停止")
}

func initRepositories(dbCfg config.DBConfig, logger port.Logger) (
	port.AgentConfigRepository,
	port.LLMConfigRepository, port.TaskRepository, port.UserRepository,
	port.ConversationRepository, port.MessageRepository, port.SystemSettingRepository,
) {
	log := logger.With(port.String("component", "repository"))
	if !dbCfg.IsConfigured() {
		log.Info("未配置数据库，使用内存 mock")
		return mock.NewMockAgentConfigRepository(), mock.NewMockLLMConfigRepository(),
			mock.NewMockTaskRepository(), mock.NewMockUserRepository(),
			mock.NewMockConversationRepository(), mock.NewMockMessageRepository(),
			mock.NewMockSystemSettingRepository()
	}
	log.Info("连接 MySQL", port.String("host", dbCfg.Host), port.String("db", dbCfg.Name))
	db, err := repository.NewMySQLDBFromConfig(dbCfg)
	if err != nil {
		log.Error("连接数据库失败", port.Err(err))
		os.Exit(1)
	}
	log.Info("MySQL 连接成功")
	return repository.NewGormAgentConfigRepository(db), repository.NewGormLLMConfigRepository(db),
		repository.NewGormTaskRepository(db), repository.NewGormUserRepository(db),
		repository.NewGormConversationRepository(db), repository.NewGormMessageRepository(db),
		repository.NewGormSystemSettingRepository(db)
}

// geoRepos 打包 GEO 仓储，避免 initRepositories 返回值过多。
type geoRepos struct {
	db      *gorm.DB // 复用的 DB 连接（收录日志等 GEO 附属仓储共用）
	brand   port.BrandRepository
	keyword port.KeywordRepository
	result  port.MonitoringResultRepository
	content port.OptimizedContentRepository
	store   port.StoreLocationRepository // 门店档案（本地生活 GEO 地基）
}

// seedPromptTemplates 首次启动写入内置默认提示词模板（已存在则跳过，保留运营修改）。
func seedPromptTemplates(repo *repository.GormPromptTemplateRepository) error {
	ctx := context.Background()
	for _, t := range geo.DefaultPromptTemplates() {
		if _, err := repo.Get(ctx, t.Key); err == nil {
			continue // 已存在（可能被管理后台改过）——不覆盖
		}
		if err := repo.Save(ctx, t); err != nil {
			return fmt.Errorf("seed 提示词模板 %s: %w", t.Key, err)
		}
	}
	return nil
}

// initGEORpositories 初始化 GEO 仓储（需要数据库；未配置 DB 时返回 nil，GEO 功能降级禁用）。
func initGEORpositories(dbCfg config.DBConfig) *geoRepos {
	if !dbCfg.IsConfigured() {
		return nil // 无 DB，GEO 端点不注册
	}
	db, err := repository.NewMySQLDBFromConfig(dbCfg)
	if err != nil {
		return nil
	}
	return &geoRepos{
		db:      db,
		brand:   repository.NewGormBrandRepository(db),
		keyword: repository.NewGormKeywordRepository(db),
		result:  repository.NewGormMonitoringResultRepository(db),
		content: repository.NewGormOptimizedContentRepository(db),
		store:   repository.NewGormStoreLocationRepository(db),
	}
}

// accountRepos 打包发布账号域仓储。
type accountRepos struct {
	account port.AccountRepository
	job     port.PublishJobRepository
}

// initAccountRepositories 初始化发布账号域仓储（需要数据库；未配置 DB 时返回 nil）。
func initAccountRepositories(dbCfg config.DBConfig) *accountRepos {
	if !dbCfg.IsConfigured() {
		return nil
	}
	db, err := repository.NewMySQLDBFromConfig(dbCfg)
	if err != nil {
		return nil
	}
	return &accountRepos{
		account: repository.NewGormAccountRepository(db),
		job:     repository.NewGormPublishJobRepository(db),
	}
}

func initAIGenerator(llmCfg config.LLMConfig, llmCfgRepo port.LLMConfigRepository, toolRegistry *port.ToolRegistry, msgRepo port.MessageRepository, logger port.Logger) port.AIGenerator {
	log := logger.With(port.String("component", "ai"))
	if !llmCfg.IsConfigured() {
		log.Info("未配置 LLM_API_KEY，使用 mock AI")
		return mock.NewMockAIGenerator()
	}
	// 会话记忆：从 DB 读历史，重启后旧会话续聊仍带上下文
	memory := ai.NewDBConversationMemory(msgRepo)
	gen, err := ai.NewTrpcAgentGenerator(llmCfgRepo, toolRegistry, memory, logger)
	if err != nil {
		log.Error("LLM 初始化失败，降级 mock", port.Err(err))
		return mock.NewMockAIGenerator()
	}
	log.Info("trpc-agent-go LLM 就绪（按 LLMConfig 动态选择 + 会话历史恢复）")
	return gen
}

const mockSiteHTML = `<!DOCTYPE html><html><body><div class="job-list">
<div class="job-item"><h3 class="position">Go 后端工程师</h3><span class="company">字节跳动</span><span class="salary">25-40K</span><ul class="requirements"><li>3年以上Go经验</li><li>熟悉Gin/GORM</li></ul></div>
<div class="job-item"><h3 class="position">前端工程师</h3><span class="company">腾讯</span><span class="salary">20-35K</span><ul class="requirements"><li>精通React/Vue</li><li>熟悉TypeScript</li></ul></div>
</div></body></html>`

func startMockSite(log port.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(mockSiteHTML))
	})
	log.Info("示例招聘站已启动", port.String("url", "http://localhost:8088/jobs"))
	_ = http.ListenAndServe(":8088", mux)
}

