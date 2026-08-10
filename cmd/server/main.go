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
	"webreaper/internal/adapter/crawler"
	"webreaper/internal/adapter/crypto"
	"webreaper/internal/adapter/embedding"
	"webreaper/internal/adapter/handler"
	"webreaper/internal/adapter/lock"
	zaplogger "webreaper/internal/adapter/logger"
	"webreaper/internal/adapter/mock"
	"webreaper/internal/adapter/publisher"
	"webreaper/internal/adapter/qrlogin"
	"webreaper/internal/adapter/repository"
	"webreaper/internal/adapter/scheduledtask"
	"webreaper/internal/adapter/telemetry"
	"webreaper/internal/adapter/urlsubmit"
	"webreaper/internal/adapter/vectorstore"
	"webreaper/internal/config"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/account"
	"webreaper/internal/usecase/agentconfig"
	"webreaper/internal/usecase/auth"
	"webreaper/internal/usecase/conversation"
	"webreaper/internal/usecase/crawlconfig"
	"webreaper/internal/usecase/dataitem"
	"webreaper/internal/usecase/geo"
	"webreaper/internal/usecase/indexing"
	"webreaper/internal/usecase/llmconfig"
	"webreaper/internal/usecase/orchestrate"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/process"
	"webreaper/internal/usecase/scheduler"
	"webreaper/internal/usecase/stats"
	"webreaper/internal/usecase/structured"
	taskuc "webreaper/internal/usecase/task"
	taskquery "webreaper/internal/usecase/taskquery"
)

func main() {
	cfg := config.Load()
	logger := zaplogger.MustNewZapLogger(cfg.Server.Env)
	defer logger.Sync()

	traceShutdown, tracer, err := telemetry.Init(telemetry.Config{
		Enabled:      cfg.Telemetry.Enabled,
		Exporter:     telemetry.ExporterKind(cfg.Telemetry.Exporter),
		OTLPEndpoint: cfg.Telemetry.OTLPEndpoint,
		ServiceName:  "webreaper",
	})
	if err != nil {
		// trace 初始化失败不阻断启动——降级为 no-op tracer，业务照常运行
		log := zaplogger.MustNewZapLogger(cfg.Server.Env)
		log.Warn("trace 初始化失败，降级为 no-op", port.Err(err))
		tracer = port.NewNopTracer()
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
	dataItemRepo, agentConfigRepo, llmConfigRepo, taskRepo, userRepo, convRepo, msgRepo, settingRepo := initRepositories(cfg.DB, logger)

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

	// Embedding + 向量存储（数据闭环：结构化→向量化→检索）
	var embedder port.Embedder
	var vectorStore port.VectorStore
	if cfg.Embedding.APIKey != "" {
		embedder = embedding.NewOpenAIEmbedder(cfg.Embedding)
		log.Info("Embedding 已配置", port.String("model", cfg.Embedding.Model))
	}
	if cfg.Milvus.IsConfigured() {
		vs, err := vectorstore.NewMilvusVectorStore(cfg.Milvus.Addr(), cfg.Milvus.CollectionName)
		if err != nil {
			log.Warn("Milvus 连接失败，使用内存向量存储", port.Err(err))
			vectorStore = vectorstore.NewMemoryVectorStore()
		} else {
			vectorStore = vs
			log.Info("Milvus 向量库已连接", port.String("addr", cfg.Milvus.Addr()), port.String("collection", cfg.Milvus.CollectionName))
		}
	} else {
		vectorStore = vectorstore.NewMemoryVectorStore()
		log.Info("未配置 MILVUS_HOST，使用内存向量存储（重启即丢，仅供开发）")
	}

	// 结构化处理用例（审核通过后：LLM提取→向量化→存向量库）
	// 同时作为 port.ItemProcessor（供 dataitem usecase 审核后触发）和
	// port.KnowledgeSearcher（供知识检索工具和搜索 API）。
	var processUC *process.ProcessUseCase
	var itemProcessor port.ItemProcessor
	var knowledgeSearch port.KnowledgeSearcher
	if embedder != nil && vectorStore != nil {
		processUC = process.NewProcessUseCase(dataItemRepo, aiGenerator, embedder, vectorStore)
		processUC.SetTracer(tracer)
		itemProcessor = processUC
		knowledgeSearch = processUC
		// 注册知识检索工具
		toolRegistry.Register(crawler.NewKnowledgeSearcher(knowledgeSearch))
		log.Info("数据闭环就绪：审核通过→结构化→向量化→检索")
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

	// Agent 执行器（注入 LLMConfigRepository 用于按 Agent 选 LLM，
	// 注入 DataItemRepo 用于工具采集结果落库，注入 Logger 用于工具落库日志）
	agentRunner := agentadapter.NewTrpcAgentRunner(llmConfigRepo, toolRegistry, dataItemRepo, logger)

	// 业务用例装配（handler 只依赖这些 usecase，不直接持有仓储）
	taskQueryUC := taskquery.NewTaskQueryUseCase(taskRepo)
	dataItemUC := dataitem.NewDataItemUseCase(dataItemRepo, itemProcessor, logger)
	// 平台总览统计：GEO/发布仓储在后续装配，此处先声明（nil），
	// 待 geoRepos/accountRepos 就绪后经 router.SetStats 重新注入完整统计。
	var statsUC *stats.StatsUseCase
	agentCfgUC := agentconfig.NewAgentConfigUseCase(agentConfigRepo)
	conversationUC := conversation.NewConversationUseCase(convRepo, msgRepo)
	crawlCfgUC := crawlconfig.NewCrawlConfigUseCase(settingRepo)
	// 首次启动 seed 默认采集策略
	_ = crawlCfgUC.EnsureDefault(context.Background())

	// 框架内容编排用例（图编排：探查→生成→校验→补生成，落库不推送）。
	// 仅配了 LLM 时启用（scout/generator 依赖 LLM）；否则降级为 nil，编排端点不注册。
	var orchestrateUC *orchestrate.OrchestratorUseCase
	var graphOrchestrator port.ContentOrchestrator
	if cfg.LLM.IsConfigured() {
		graphOrchestrator = agentadapter.NewGraphContentOrchestrator(
			aiGenerator,
			[]string{"static_crawler", "search_crawler", "dynamic_crawler"}, // scout 探查文档用的爬虫
			logger,
		)
		orchestrateUC = orchestrate.NewOrchestratorUseCase(graphOrchestrator, dataItemRepo, logger)
		log.Info("框架内容编排已启用（图编排模式）")
		// 把图编排包装成 generate_content 工具，注册进工具池——
		// 让通用 Agent 能把它当作"子能力"自主调用。
		toolRegistry.Register(agentadapter.NewContentGenerationTool(graphOrchestrator))
	} else {
		log.Info("未配置 LLM，框架内容编排降级禁用")
	}

	// 注册 save_data_item 工具：让 LLM 自主保存结构化内容为 DataItem
	// （LLM 生成 JSON → 调此工具保存 → 收到"已保存" → 回复友好总结，不显示 JSON 原文）
	saverAdapter := &dataItemSaverAdapter{uc: dataItemUC}
	toolRegistry.Register(crawler.NewSaveDataItemTool(saverAdapter))

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
	router := handler.NewRouter(registerUC, loginUC, tokenParser, aiGenerator, enqueueUC,
		agentRunner, taskQueryUC, dataItemUC, agentCfgUC, llmCfgUC, conversationUC, crawlCfgUC,
		toolRegistry, knowledgeSearch, orchestrateUC, statsUC)

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
		publicHandler := handler.NewPublicHandler(geoRepos.content, structuredUC, cfg.Server.PublicBaseURL)
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
	if geoRepos != nil && cfg.LLM.IsConfigured() {
		geoScorer := ai.NewLLMGEOScorer(aiGenerator)
		geoBrandUC := geo.NewBrandUseCase(geoRepos.brand, geoRepos.keyword)
		geoBrandUC.SetAIGenerator(aiGenerator) // 关键词生成用

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
		geoMonitorUCRef = geoMonitorUC
		geoRankUC := geo.NewRankUseCase(geoRepos.result)
		geoContentUC := geo.NewContentUseCase(aiGenerator, geoScorer, geoRepos.content)
		// 免费规则评分器：优化前后对比用（不烧 token、可单测）
		geoContentUC.SetRuleScorer(geo.NewRuleScorer())
		// 收录通知（IndexNow）：发布为 published 时自动通知搜索引擎
		if indexNowSubmitter != nil {
			geoContentUC.SetPublicBaseURL(cfg.Server.PublicBaseURL)
			geoContentUC.SetURLSubmitter(indexNowSubmitter)
		}
		geoDiagnoseUC := geo.NewDiagnoseUseCase(geoRepos.brand, geoRepos.result, aiGenerator)
		router.SetGEO(geoBrandUC, geoMonitorUC, geoRankUC, geoContentUC, geoDiagnoseUC)

		// 关键词蒸馏引擎：五种来源策略（策略模式 + 工厂）
		brandWebSearcher := ai.NewBrandWebSearcher(webFetcher)
		geoDistillUC := geo.NewKeywordDistillUseCase(
			ai.NewBrandSource(aiGenerator, geoRepos.brand, brandWebSearcher), // 品牌信息+全网
			ai.NewTextSource(aiGenerator),                                    // 用户文本
			ai.NewSeedSource(aiGenerator),                                    // 种子词拓展
			ai.NewFileSource(aiGenerator),                                    // 文件内容
			ai.NewWebSource(aiGenerator, webFetcher),                         // 网络爬取
		)
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
		statsUC = stats.NewStatsUseCase(dataItemRepo, userRepo,
			geoRepos.brand, geoRepos.keyword, geoRepos.result, geoRepos.content,
			accountRepos.job)
	} else {
		statsUC = stats.NewStatsUseCase(dataItemRepo, userRepo, nil, nil, nil, nil, nil)
	}
	router.SetStats(statsUC)

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
		_ = taskScheduler.Register(scheduledtask.NewDailyMonitorTask(monUC, geoRepos.brand, log))
		log.Info("每日自动监测任务已注册（AUTO_MONITOR_ENABLED=true）")
	}

	taskScheduler.Start(schedulerCtx)

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	log.Info("服务已停止")
}

func initRepositories(dbCfg config.DBConfig, logger port.Logger) (
	port.DataItemRepository, port.AgentConfigRepository,
	port.LLMConfigRepository, port.TaskRepository, port.UserRepository,
	port.ConversationRepository, port.MessageRepository, port.SystemSettingRepository,
) {
	log := logger.With(port.String("component", "repository"))
	if !dbCfg.IsConfigured() {
		log.Info("未配置数据库，使用内存 mock")
		return mock.NewMockDataItemRepository(),
			mock.NewMockAgentConfigRepository(), mock.NewMockLLMConfigRepository(),
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
	return repository.NewGormDataItemRepository(db),
		repository.NewGormAgentConfigRepository(db), repository.NewGormLLMConfigRepository(db),
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

// dataItemSaverAdapter 把 dataitem.DataItemUseCase 适配为 crawler.DataItemSaver 接口。
// 让 save_data_item 工具能通过 usecase 落库，依赖方向合法。
type dataItemSaverAdapter struct {
	uc *dataitem.DataItemUseCase
}

func (a *dataItemSaverAdapter) SaveFromContent(ctx context.Context, content, fieldMapping, sourceURL string) (string, string, error) {
	item, err := a.uc.CreateFromContent(ctx, dataitem.CreateFromContentInput{
		Content:      content,
		FieldMapping: fieldMapping,
		SourceURL:    sourceURL,
	})
	if err != nil {
		return "", "", err
	}
	return item.ID, item.Title, nil
}
