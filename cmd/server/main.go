// Package main 是 WebReaper 的 Web 服务入口（通用数据采集平台）。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	milvusclient "github.com/milvus-io/milvus-sdk-go/v2/client"

	agent "webreaper/internal/adapter/agent"
	agentadapter "webreaper/internal/adapter/agent"
	"webreaper/internal/adapter/ai"
	"webreaper/internal/adapter/asropenai"
	"webreaper/internal/adapter/cache"
	"webreaper/internal/adapter/integration"
	"webreaper/internal/adapter/mediaav"
	"webreaper/internal/adapter/metrics"
	authadapter "webreaper/internal/adapter/auth"
	"webreaper/internal/adapter/bing"
	"webreaper/internal/adapter/crawler"
	douyincrawler "webreaper/internal/adapter/crawler/douyin"
	"webreaper/internal/adapter/crypto"
	geoadapter "webreaper/internal/adapter/geo"
	"webreaper/internal/adapter/ttsmimo"
	"webreaper/internal/adapter/handler"
	kbretriever "webreaper/internal/adapter/knowledge"
	"webreaper/internal/adapter/lock"
	zaplogger "webreaper/internal/adapter/logger"
	"webreaper/internal/adapter/mock"
	"webreaper/internal/adapter/payment"
	"webreaper/internal/adapter/provider"
	"webreaper/internal/adapter/provider/vidu"
	"webreaper/internal/adapter/provider/viduendpoint"
	"webreaper/internal/adapter/publisher"
	transport "webreaper/internal/adapter/publisher/transport"
	"webreaper/internal/adapter/douyinoauth"
	"webreaper/internal/adapter/douyinweb"
	"webreaper/internal/adapter/qrlogin"
	"webreaper/internal/adapter/repository"
	"webreaper/internal/adapter/scheduledtask"
	"webreaper/internal/adapter/urlprobe"
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
	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/inspiration"
	"webreaper/internal/usecase/geo"
	"webreaper/internal/usecase/works"
	"webreaper/internal/usecase/indexing"
	"webreaper/internal/usecase/knowledge"
	"webreaper/internal/usecase/llmconfig"
	"webreaper/internal/usecase/notification"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/providerconfig"
	"webreaper/internal/usecase/quota"
	"webreaper/internal/usecase/scheduler"
	"webreaper/internal/usecase/stats"
	"webreaper/internal/usecase/structured"
	"webreaper/internal/usecase/systemsettings"
	"webreaper/internal/usecase/videotranscript"
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
	agentConfigRepo, llmConfigRepo, userRepo, convRepo, msgRepo, settingRepo := initRepositories(cfg.DB, logger)

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

	// 业务用例装配（handler 只依赖这些 usecase，不直接持有仓储）
	var statsUC *stats.StatsUseCase
	agentCfgUC := agentconfig.NewAgentConfigUseCase(agentConfigRepo)
	conversationUC := conversation.NewConversationUseCase(convRepo, msgRepo)

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
		toolRegistry.Register(agent.NewContentGenerationTool(graphOrchestrator))
		log.Info("内容生成工具已注册（图编排模式，结果直接返回 LLM 不落库）")
	} else {
		log.Info("未配置 LLM，内容生成工具降级禁用")
	}

	// 路由 + HTTP 服务（handler 只依赖 usecase 与 port 接口，不直接持有仓储/具体 adapter struct）
	// 路由器（零参数——所有依赖通过 SetXxx 可选注入，端点按注入条件注册）
	router := handler.NewRouter()
	router.SetAuth(registerUC, loginUC, tokenParser)
	// 改密端点（F1-5：默认弱口令 admin/admin123 治理——配合前端常驻提醒）
	router.SetAuthChangePassword(auth.NewChangePasswordUseCase(userRepo, hasher))

	// R1/R2 Redis 基础设施（声明先于一切消费方）：分布式锁/三防缓存/回调 nonce 共享一个连接。
	// REDIS_HOST 配置即启用；连接失败全链路降级单机模式（NoopLock/内存 nonce/无缓存）并记日志。
	// 背景：8 个有副作用的定时任务（自动盯盘烧 LLM/定时发布 RPA/生成轮询…）在多实例下
	// 重复执行是事故级风险——RedisLock 是水平扩容的第一道前置。
	taskLock := port.TaskLock(lock.NewNoopLock())
	var redisClient *redis.Client
	var cacheStore port.CacheStore
	if cfg.Redis.IsConfigured() {
		rc := redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 3*time.Second)
		if pErr := rc.Ping(pingCtx).Err(); pErr != nil {
			log.Warn("Redis 连接失败，全链路降级单机模式（NoopLock/内存 nonce/无缓存）",
				port.String("addr", cfg.Redis.Host+":"+cfg.Redis.Port), port.Err(pErr))
			_ = rc.Close()
		} else {
			redisClient = rc
			taskLock = lock.NewRedisLock(rc)
			cacheStore = cache.NewRedisCache(rc)
			log.Info("Redis 已连接：分布式锁/三防缓存/回调 nonce 已启用",
				port.String("addr", cfg.Redis.Host+":"+cfg.Redis.Port))
		}
		pingCancel()
	}

	// R3 运营指标采集器（Redis INCR——多实例共享；未配 Redis 用 Noop 零开销）
	var metricsCollector port.MetricsCollector = metrics.NewNoopMetrics()
	if redisClient != nil {
		metricsCollector = metrics.NewRedisMetrics(redisClient)
		router.SetMetrics(metricsCollector)
		// 缓存命中率埋点（RedisCache → MetricsCollector）
		if rc, ok := cacheStore.(*cache.RedisCache); ok {
			rc.SetMetrics(metricsCollector)
		}
		// 配额拒绝埋点（response.go fail() 集中点）
		handler.SetQuotaMetric(metricsCollector)
		// LLM 调用成功率/慢调用/错误率埋点
		if tg, ok := aiGenerator.(*ai.TrpcAgentGenerator); ok {
			tg.SetMetrics(metricsCollector)
		}
	}

	router.SetAI(aiGenerator)
	router.SetAgentConfig(agentCfgUC)
	router.SetLLMConfig(llmCfgUC)
	router.SetConversation(conversationUC)
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
		// Bing URL Submission API 兜底渠道：IndexNow 之外再向 Bing 主动提交，
		// 与后台"URL 提交"共享 100 条/天配额（GetUrlSubmissionQuota 实测）。
		// 复用 BING_API_KEY / BING_SITE_URL（同一账号 key，与收录验证共用）——env 装配，重启生效；
		// 未配置则仅走 IndexNow/百度（组合模式：任一分发失败不影响其他渠道）。
		var submitter port.URLSubmitter = cachedSubmitter
		if cfg.Server.BingAPIKey != "" && cfg.Server.BingSiteURL != "" {
			if bingSub, subErr := urlsubmit.NewBingSubmitter(cfg.Server.BingAPIKey, cfg.Server.BingSiteURL); subErr == nil {
				submitter = urlsubmit.NewMultiSubmitter(cachedSubmitter, bingSub)
			} else {
				log.Warn("Bing URL Submission 渠道构建失败（已跳过）: " + subErr.Error())
			}
		}
		// key 文件端点读运行时配置（管理后台改 key 后即时生效）
		publicHandler.SetIndexNowKeyProvider(func(ctx context.Context) string {
			c, _ := loadIndexingConfig(ctx)
			return c.IndexNowKey
		})

		// 收录管理用例（管理后台：配置读写/提交日志/手动补提交）
		indexingLogRepo := repository.NewGormIndexingLogRepository(geoRepos.db)
		indexingUC = indexing.NewIndexingUseCase(settingRepo, indexingLogRepo, geoRepos.content, submitter, cfg.Server.PublicBaseURL)
		indexingUC.SetURLProbe(urlprobe.New()) // 密钥文件可达性探测（HTTP 细节在适配器）
		router.SetIndexing(indexingUC)
		// 发布/补提交等"自动触发"的收录提交套审计日志装饰器——
		// 成功/失败都进"提交日志"页（此前只有手动补提交有日志，发布提交无从排查）
		indexNowSubmitter = urlsubmit.NewLoggingSubmitter(submitter, indexingLogRepo)
	}
	var geoMonitorUCRef *geo.MonitorUseCase
	var geoContentUCRef *geo.ContentUseCase
	var knowledgeUCRef *knowledge.KnowledgeUseCase // 知识库采集用例（可选；任务注册用）
	var geoDistillUCRef *geo.KeywordDistillUseCase
	var geoNearbyUCRef *geo.NearbyUseCase     // X-01：附近同行配额注入用
	var geoDiagnoseUCRef *geo.DiagnoseUseCase // X-01：诊断配额注入用
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
		// R2 写后失效：监测/内容写入 → 清 health-report / monitor-results / industry-overview 缓存
		if cacheStore != nil {
			geoMonitorUC.SetCache(cacheStore)
		}
		geoMonitorUCRef = geoMonitorUC
		geoRankUC := geo.NewRankUseCase(geoRepos.result)
		geoRankUC.SetKeywordRepo(geoRepos.keyword) // Overview 的品牌关键词数（仪表盘排行）
		geoContentUC := geo.NewContentUseCase(aiGenerator, geoScorer, geoRepos.content)
		geoContentUCRef = geoContentUC
		// R2 写后失效：内容生成/发布/删除 → 清 health-report / industry-overview 缓存
		if cacheStore != nil {
			geoContentUC.SetCache(cacheStore)
		}
		// 免费规则评分器：优化前后对比用（不烧 token、可单测）
		geoContentUC.SetRuleScorer(geo.NewRuleScorer())
		// RAG 增强：原创生成前检索"品牌+关键词"真实信息注入 prompt（"不编造数据"变能力）
		geoContentUC.SetRAGRetriever(ai.NewWebContentRetriever(webFetcher))
		// 平台知识库（Docs/Plans/04）：按行业持续采集素材（带来源）→ 生成前向量检索素材注入 prompt。
		// 知识库优先于实时全网检索——素材带来源 URL，引用可溯源；检索无命中时自动回退在线 RAG。
		// 向量嵌入/向量库运行时配置（管理后台可改，30s 生效免重启）：
		//   system_settings[kb_embedding_config] 优先，EMBEDDING_* env 兜底。
		loadEmbeddingConfig := func(ctx context.Context) (entity.EmbeddingRuntimeConfig, error) {
			if s, sErr := settingRepo.Get(ctx, entity.SettingKeyEmbeddingConfig); sErr == nil {
				var c entity.EmbeddingRuntimeConfig
				if json.Unmarshal([]byte(s.Value), &c) == nil && c.IsConfigured() {
					return c, nil
				}
			}
			return entity.EmbeddingRuntimeConfig{
				Model: cfg.Embedding.Model, BaseURL: cfg.Embedding.BaseURL, APIKey: cfg.Embedding.APIKey,
				VectorDB: entity.VectorDBMySQL,
			}, nil
		}
		kbEmbedder := ai.NewCachedEmbedder(port.EmbeddingConfigLoaderFunc(loadEmbeddingConfig)) // 改模型 30s 生效（TTL 重建）
		kbVecStore := kbretriever.NewMySQLVectorStore(geoRepos.db)
		// Milvus 工厂（vector_db=milvus 时按运行时配置连接；连接失败明确报错，不静默降级）
		milvusFactory := func(ctx context.Context, cfg entity.EmbeddingRuntimeConfig) (port.VectorStore, error) {
			addr := net.JoinHostPort(cfg.MilvusHost, cfg.MilvusPort)
			cli, err := milvusclient.NewClient(ctx, milvusclient.Config{Address: addr})
			if err != nil {
				return nil, fmt.Errorf("milvus 连接失败（%s）: %w", addr, err)
			}
			return kbretriever.NewMilvusVectorStore(kbretriever.NewMilvusSDKClient(cli), cfg.MilvusCollection), nil
		}
		kbVecProvider := kbretriever.NewVectorStoreProvider(port.EmbeddingConfigLoaderFunc(loadEmbeddingConfig), kbVecStore, milvusFactory) // 改向量库 30s 生效
		kbRepo := repository.NewGormKnowledgeMaterialRepository(geoRepos.db, kbVecProvider)
		knowledgeUCRef = knowledge.NewKnowledgeUseCase(kbRepo, settingRepo,
			crawler.NewRateLimitCrawler(crawler.NewSearchCrawler(), crawlPolicy),
			crawler.NewRateLimitCrawler(crawler.NewStaticCrawler(), crawlPolicy),
			kbEmbedder, log)
		kbRetrieverInst := kbretriever.NewKnowledgeRetriever(kbRepo, kbEmbedder)
		knowledgeUCRef.SetRetriever(kbRetrieverInst) // 管理后台"检索验证"（与生成注入同一实例）
		geoContentUC.SetKnowledgeRetriever(kbRetrieverInst)
		router.SetKnowledge(knowledgeUCRef) // 管理后台：向量配置/行业采集配置/素材统计
		log.Info("平台知识库已装配（采集任务 + 生成检索注入 + 管理后台动态配置）")
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
		geoInputTipper := geoadapter.NewAmapInputTipper(cfg.AMap.APIKey)   // P1 地址联想
		geoMeasurer := geoadapter.NewAmapDistanceMeasurer(cfg.AMap.APIKey) // P2 驾车耗时
		if cfg.AMap.IsConfigured() {
			log.Info("本地生活位置服务已启用（高德：地理编码 + 周边 POI 搜索 v" + cfg.AMap.APIVersion + " + 地址联想 + 距离测量）")
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
		// 健康报告聚合（v3 归位：总分/五指数/竞品对标的后端单一事实源，替代前端 geoHealth 各自合成）
		geoHealthUC := geo.NewHealthUseCase(geoRepos.brand, geoRepos.result, geoRepos.content)
		geoIndustryUC := geo.NewIndustryUseCase(geoRepos.brand, geoRepos.result)
		if cacheStore != nil {
			// R2 性能：驾驶舱 N+1 扇出收敛为 60s 缓存、跨租户行业聚合 5min 缓存
			//（TTL 抖动防雪崩/singleflight 防击穿/空值标记防穿透——见 adapter/cache）
			geoHealthUC.SetCache(cacheStore)
			geoIndustryUC.SetCache(cacheStore)
		}
		router.SetGEOHealth(geoHealthUC)
		// 行业全景看板（v3 P2：跨商户聚合——行业能见度/品牌美誉度/信源域名榜）
		router.SetGEOIndustry(geoIndustryUC)

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
			ai.NewQuestionSource(aiGenerator),                              // 提问词挖掘（问题库）
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
	var socialSearcher *douyinweb.Searcher // 提升到外层：生成域提取管线复用（分享链解析）
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
			// 抖音站内搜索（MediaCrawler 协议复刻：cookie 账号 + 页面内免签 fetch）——
			// 热门同款 tab 主数据源；数据回读上线后复用 GetVideoDetail
			// 站内搜索共享实例（热门同款主数据源 + 数据回读取详情）
			socialSearcher = douyinweb.NewSearcher(accountRepos.account, vault)
			// 发布通道注册表（工厂模式，已注册知乎/小红书全自动通道——同时支持半自动+全自动）
			channelRegistry := publisher.NewChannelRegistry()
			// cookie 滚动回写（发布会话后把浏览器最新 cookie 写回账号库——绑定滚动续期）
			channelRegistry.SetAccountStore(accountRepos.account, vault)
			geoPublishUC = account.NewPublishUseCase(accountRepos.job, channelRegistry, accountRepos.account, vault)
			// 注入发布效果追踪（发布成功后自动触发监测对比提及率）
			geoPublishUC.SetMonitorTrigger(geoMonitorUCRef)
			// 注入公开站根地址（发布内容尾部带公开站链接，加速爬虫发现）
			geoPublishUC.SetPublicBaseURL(cfg.Server.PublicBaseURL)
			// 注入账号池（全自动发布时自动选最优账号——最久未使用优先）
			geoPublishUC.SetAccountPool(repository.NewGormAccountPool(accountRepos.account))
			// 互动数据回读（快照仓储 + 站内详情接口——每日任务/手动刷新）
			geoPublishUC.SetMetricsStore(accountRepos.metric, socialSearcher)
			// 通道轴装配（发布域三轴重构）：link/rpa 双通道共存 + 启动前短路降级 +
			// 管理后台 override；API 通道权限批下来后 Register 即接入（结构已就位）
			transportRegistry := port.NewTransportRegistry()
			credResolver := transport.NewVaultCredentialResolver(vault, accountRepos.account)
			transportRegistry.Register(transport.NewLinkTransport(channelRegistry))
			transportRegistry.Register(transport.NewRPATransport(channelRegistry))
			geoPublishUC.SetTransports(transportRegistry, credResolver)
			router.SetTransportRegistry(transportRegistry, settingRepo)
			// 作品库三源聚合（文章 + 多媒体产物 + 发布状态 + 互动数据）
			worksUC := works.NewWorksUseCase(geoRepos.content, repository.NewGormGenerationTaskRepository(geoRepos.db), accountRepos.job, accountRepos.metric)
			router.SetWorks(worksUC)

			// 商户主 Agent 工具集（Agent-as-Tool：获客管家对话编排；二期+增长子Agent/硬确认）
			pendingStore := agent.NewPendingPublishStore()
			router.SetPendingPublishStore(pendingStore)
			toolRegistry.Register(agent.NewQueryBrandsTool(geoRepos.brand))
			toolRegistry.Register(agent.NewListWorksTool(worksUC))
			toolRegistry.Register(agent.NewQueryAnalyticsTool(geoPublishUC))
			toolRegistry.Register(agent.NewTriggerMonitorTool(geoMonitorUCRef))
			toolRegistry.Register(agent.NewPublishWorkTool(geoPublishUC, worksUC, geoRepos.content, pendingStore))
			toolRegistry.Register(agent.NewQueryAccountsTool(geoAccountUC))
			if knowledgeUCRef != nil {
				toolRegistry.Register(agent.NewQueryKnowledgeTool(knowledgeUCRef))
			}
			// 子 Agent 示范：增长顾问（数据组合+领域方法论，主 Agent 只派发任务）
			toolRegistry.Register(agent.NewGrowthAdvisorTool(geoRepos.brand, worksUC, geoPublishUC, geoAccountUC, aiGenerator))

			// 抖音开放平台官方 OAuth 授权（API 通道——替代浏览器扫码 RPA 绑定；
			// 内部统一走官方 SDK bytedance/douyin-openapi-sdk-go）
			if cfg.Publish.DouyinClientKey != "" && vault != nil {
				stateSecret := cfg.JWT.Secret
				if stateSecret == "" {
					stateSecret = cfg.Publish.DouyinClientSecret
				}
				oauthClient, oErr := douyinoauth.NewClient(cfg.Publish.DouyinClientKey, cfg.Publish.DouyinClientSecret, cfg.Publish.DouyinOAuthCallback, cfg.Publish.DouyinOAuthScope)
				if oErr != nil {
					log.Error("抖音 OpenAPI SDK 初始化失败，官方授权不可用", port.Err(oErr))
				} else {
					geoAccountUC.SetOAuth(oauthClient, douyinoauth.NewStateCodec(stateSecret))
					log.Info("抖音官方 OAuth 授权已启用（API 通道绑定，回调地址=" + cfg.Publish.DouyinOAuthCallback + "）")
				}
			}

			router.SetAccount(geoAccountUC, geoPublishUC, cfg.Publish.FrontendBaseURL)
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
		// 支付闭环原子化：订单置 paid + 订阅开通同一事务（消除"已付款未开通"中间态）
		billingUC.SetPaymentClosureWriter(repository.NewGormPaymentClosureWriter(geoRepos.db))

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
	// 无 DB（mock 降级启动）时不注入租户级设置仓储：租户开关降级为始终开启，
	// 平台级总闸仍经 settingRepo（内存 mock）生效——与 initRepositories 的降级口径一致。
	var tenantSettingRepo port.TenantSettingRepository
	settingsUC := systemsettings.NewSystemSettingsUseCase(settingRepo)
	if geoRepos != nil {
		tenantSettingRepo = repository.NewGormTenantSettingRepository(geoRepos.db)
		settingsUC.SetTenantSettingRepo(tenantSettingRepo)
	}
	// 浏览器可见性即时生效：用例只声明"写完即生效"约束，
	// 全局内存同步是驱动细节——由 main 注入（用例不 import config）
	settingsUC.SetHeadedSyncer(runtimeHeadedSyncer{})
	// 初始化浏览器可见性（从 DB 读管理后台上次设置，默认 headless）
	if headed, hErr := settingsUC.GetBrowserHeaded(context.Background()); hErr == nil {
		config.SetBrowserHeaded(headed)
	} else {
		config.SetBrowserHeaded(cfg.Publish.QRLoginHeaded) // fallback 到环境变量
	}
	router.SetSystemSettings(settingsUC)

	// 站内通知（主动唤醒：提及率变化/自动复测/排期发布）——依赖 DB，无 DB 时不注册
	// （下游定时任务对 nil notifier 均有判空保护）
	var notifyUC *notification.NotifyUseCase
	if geoRepos != nil {
		notifyUC = notification.NewNotifyUseCase(repository.NewGormNotificationRepository(geoRepos.db))
		router.SetNotifications(notifyUC)
	}

	// 管理端装配（用户管理，仅 admin）
	router.SetAdmin(userRepo)

	// 通用定时任务调度器（统一驱动：防重入/分布式锁/panic 恢复/错误日志）。
	// 新增定时功能 = 实现 port.ScheduledTask + Register 一行，避免"一功能一套 ticker"。
	// 锁实现见顶部 Redis 装配（RedisLock/NoopLock 按可用性切换，业务零改动）。
	schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
	taskScheduler := scheduler.New(taskLock, log)

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
		// 视频互动数据回读（每日：全租户已发布作品 → 站内详情接口 → 快照时间序列）
		_ = taskScheduler.Register(scheduledtask.NewVideoMetricsTask(geoPublishUC, log))
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
		} else {
			log.Warn("BING_API_KEY 未配置——收录状态验证未启用（内容将长期显示「待收录」，配置后重启生效）")
		}
		indexTask := scheduledtask.NewIndexCheckTask(geoRepos.content, indexChecker, cfg.Server.PublicBaseURL, log)
		if notifyUC != nil {
			indexTask.SetNotifier(notifyUC) // 内容被收录时站内通知商户（付费说服力事件）
		}
		_ = taskScheduler.Register(indexTask)
		// 知识库采集：按行业配置持续爬取素材入库（每 6h；未配置行业则空转）
		if knowledgeUCRef != nil {
			_ = taskScheduler.Register(scheduledtask.NewKnowledgeCrawlTask(knowledgeUCRef, log))
			log.Info("知识库采集任务已注册（knowledge-crawl，每 6 小时）")
		}
	}

	// ③ 统一生成任务（Vidu 全量接入：视频 5+图片/音频/数字人——Docs/Plans/03 计划文档）
	// 协议层 + 端点策略（viduendpoint：能力向量校验/请求体组装）+ 回调验签；
	// 未配 key 走 MockGenerationProvider（模拟进度，前端全流程可演示）。
	// 配额（generation 场景）在 P1 随计费场景扩展注入；当前 mock 模式无真实成本。
	if geoRepos != nil {
		genRegistry := viduendpoint.NewRegistry()
		// 厂商配置 DB 优先（管理后台可设置 Vidu API Key / 启停），环境变量兜底。
		// SwitchingProvider 按调用期 Key 热切换 mock↔真实（修复：此前启动时一次性
		// 选定，后配的 Key 对运行中的 mock 无效——热更新接口只有 ViduProvider 实现，
		// 对 mock 断言静默失败，唯一出路是重启；Enabled 开关此前也无人消费）。
		providerCfgRepo := repository.NewGormProviderConfigRepository(geoRepos.db)
		resolveViduKey := func() (string, bool) {
			if dbCfg, cfgErr := providerCfgRepo.Get(context.Background(), "vidu"); cfgErr == nil && dbCfg.Provider != "" {
				return dbCfg.APIKey, dbCfg.Enabled
			}
			return cfg.Server.ViduAPIKey, true // 无 DB 行（未在后台配置）→ 环境变量，视为启用
		}
		bootKey, bootEnabled := resolveViduKey()
		vp := vidu.NewViduProvider(bootKey)
		// R2 修复多实例 Key 漂移：管理后台改 Key 后各实例 ≤30s 从 DB 对齐
		//（此前 UpdateAPIKey 只更新收到请求的那个实例——其他实例持旧 Key 直到重启）
		vp.SetKeySource(func(ctx context.Context) (string, error) {
			cfgRow, err := providerCfgRepo.Get(ctx, "vidu")
			if err != nil || cfgRow.APIKey == "" {
				return "", err
			}
			return cfgRow.APIKey, nil
		})
		var viduProvider port.GenerationProvider = provider.NewSwitchingProvider(vp, provider.NewMockGenerationProvider(), resolveViduKey)
		if bootKey != "" && bootEnabled {
			log.Info("统一生成已接入 Vidu（真实 API；后台改 Key/停用 ≤10s 热切换，无需重启）")
		} else if bootKey != "" {
			log.Info("统一生成运行在 mock 模式（Vidu 已配置 Key 但已被停用——后台启用后 ≤10s 生效）")
		} else {
			log.Info("统一生成运行在 mock 模式（未配置 VIDU_API_KEY——后台保存 Key 后 ≤10s 自动切真实，无需重启）")
		}
		router.SetProviderConfig(providerconfig.NewUseCase(providerCfgRepo))
		// 生成规格 DB 驱动（全局掌控）：首次启动 seed 出厂默认 → 管理后台全量可编辑，
		// 30s TTL 热生效（不重启）；删除行 = 恢复出厂默认
		genSpecRepo := repository.NewGormGenerationSpecRepository(geoRepos.db)
		genRegistry.SetSpecRepo(genSpecRepo)
		if seedErr := genRegistry.SeedDefaults(context.Background()); seedErr != nil {
			log.Warn("seed 生成规格默认值失败", port.Err(seedErr))
		}
		// 官方音色库（Vidu 语音合成音色表 302 条）：表空则 seed，客户端经
		// /api/v1/generation/voices 查询（TTS/主体/数字人的音色选择数据源）
		voiceRepo := repository.NewGormVoiceRepository(geoRepos.db)
		if n, seedErr := voiceRepo.SeedIfEmpty(context.Background(), viduendpoint.DefaultVoices()); seedErr != nil {
			log.Warn("seed 官方音色库失败", port.Err(seedErr))
		} else if n > 0 {
			log.Info("官方音色库已 seed", port.Int("voices", n))
		}
		router.SetGenerationVoices(voiceRepo)
		// 视频文案提取管线（08 计划 D4）：ffmpeg 字幕/音轨 + 云 ASR（动态配置，
		// provider=asr：base_url=endpoint / api_key=key / extra_json={model,response_style}）
		// + LLM 双产出。分享链解析复用账号域的抖音搜索器（RPA 详情接口拿播放直链）。
		avTool := mediaav.NewFFmpegTool(cfg.Server.FFMPEGPath)
		if avTool.Available() {
			log.Info("ffmpeg 可用（字幕轨/音轨抽取已启用）")
		} else {
			log.Warn("ffmpeg 不可用——提取走降级路径（≤25MB 视频直传 ASR）")
		}
		// 能力路由（08 架构重构）：统一配置查询路径——新表 integration_vendors +
		// integration_capabilities 优先，旧表（provider_configs/llm_configs）兜底。
		// 10s TTL 缓存，切换 is_default ≤10s 全链路生效，无需重启。
		integrationRepo := repository.NewGormIntegrationRepository(geoRepos.db)
		if n, seedErr := integrationRepo.SeedIfEmpty(context.Background(),
			integration.DefaultVendors, integration.DefaultCapabilities); seedErr != nil {
			log.Warn("seed 能力路由失败", port.Err(seedErr))
		} else if n > 0 {
			log.Info("能力路由已 seed（小米 MiMo + 硅基流动 + OpenAI）", port.Int("vendors", n))
		}
		capResolver := integration.NewResolver(integrationRepo, providerCfgRepo, llmConfigRepo)
		// LLM 配置解析也走能力路由（llmConfigName 为空时新表优先，旧表兜底）
		if gen, ok := aiGenerator.(*ai.TrpcAgentGenerator); ok {
			gen.SetCapResolver(capResolver)
		}
		var transcriptResolver port.VideoLinkResolver
		if socialSearcher != nil {
			transcriptResolver = douyinweb.NewLinkResolver(socialSearcher)
		}
		asrClient := asropenai.NewTranscriber(func() asropenai.ASRConfig {
			// 优先走 CapabilityResolver（新表+旧表统一查询）
			if cap, err := capResolver.Resolve(context.Background(), "asr"); err == nil && cap.APIKey != "" {
				ac := asropenai.ASRConfig{
					Endpoint:      cap.Endpoint,
					APIKey:        cap.APIKey,
					Model:         cap.Model,
					ResponseStyle: extractResponseStyle(cap.ExtraJSON),
					Protocol:      cap.Protocol,
					ASRLanguage:   extractExtraField(cap.ExtraJSON, "asr_options_language"),
				}
				return ac
			}
			// 兜底：直接读旧表（兼容 CapabilityResolver 未装配场景）
			if cfgRow, cfgErr := providerCfgRepo.Get(context.Background(), "asr"); cfgErr == nil && cfgRow.APIKey != "" {
				var ac asropenai.ASRConfig
				_ = json.Unmarshal([]byte(cfgRow.ExtraJSON), &ac)
				ac.APIKey = cfgRow.APIKey
				if ac.Endpoint == "" {
					ac.Endpoint = cfgRow.BaseURL
				}
				return ac
			}
			return asropenai.ASRConfig{}
		})
		if aiGenerator != nil {
			router.SetTranscript(videotranscript.NewUseCase(transcriptResolver, avTool, asrClient, aiGenerator))
		} else {
			log.Warn("AI 生成器未装配——文案双产出不可用（提取仍可用）")
			router.SetTranscript(videotranscript.NewUseCase(transcriptResolver, avTool, asrClient, nil))
		}
		// 多厂商 provider 注册
		providers := map[string]port.GenerationProvider{
			"vidu": viduProvider,
		}
		// 小米MiMo TTS provider（音频/TTS/声音克隆走小米MiMo）
		// API Key 优先从 capability resolver（管理后台配置的 integration_vendors 表）读取，
		// 未配置则降级到环境变量 MIMO_API_KEY。
		mimoKey := os.Getenv("MIMO_API_KEY")
		if mimoKey == "" && capResolver != nil {
			if cap, capErr := capResolver.Resolve(context.Background(), "tts"); capErr == nil && cap.VendorID == "xiaomi-mimo" && cap.APIKey != "" {
				mimoKey = cap.APIKey
				log.Info("小米MiMo API Key 从能力路由配置读取（管理后台配置）")
			}
		}
		if mimoKey == "" && capResolver != nil {
			if cap, capErr := capResolver.Resolve(context.Background(), "voice-clone"); capErr == nil && cap.VendorID == "xiaomi-mimo" && cap.APIKey != "" {
				mimoKey = cap.APIKey
				log.Info("小米MiMo API Key 从声音克隆能力路由配置读取")
			}
		}
		if mimoKey != "" {
			mimoTTS := ttsmimo.NewMiMoTTSProvider(mimoKey, "")
			providers["xiaomi-mimo"] = ttsmimo.NewMiMoAsGenerationProvider(mimoTTS)
			log.Info("小米MiMo TTS provider 已注册（音频/TTS/声音克隆）")
		} else {
			log.Warn("小米MiMo TTS 未启用（MIMO_API_KEY 未配置且管理后台未设置 API Key）")
		}

		genUC := generation.NewGenerationUseCase(providers, genRegistry, repository.NewGormGenerationTaskRepository(geoRepos.db))
		// 注入能力路由解析器
		if capResolver != nil {
			genUC.SetCapabilityResolver(capResolver)
		}
		// 任务终态站内通知（异步任务完成/失败主动唤醒——此前静默完成，商户不留
		// 在页面上就看不到结果）。notifyUC 未装配（DB 缺失等）则静默跳过。
		if notifyUC != nil {
			genUC.SetTaskNotifier(genTaskNotifier{notifyUC})
		}
		// 回调通道激活（双通道：回调秒级推送 + 20s 轮询兜底，幂等合并）。
		// 仅公网可达地址注入——localhost 对 Vidu 不可达，注入只会产生 3 次
		// 失败投递；本地开发自动保持纯轮询。显式 VIDU_CALLBACK_URL 优先。
		callbackURL := os.Getenv("VIDU_CALLBACK_URL")
		if callbackURL == "" {
			base := strings.TrimRight(cfg.Server.PublicBaseURL, "/")
			if base != "" && !strings.Contains(base, "localhost") && !strings.Contains(base, "127.0.0.1") {
				callbackURL = base + cfg.Server.APIPrefix + "/api/v1/generation/callback"
			}
		}
		if callbackURL != "" {
			genUC.SetCallbackURL(callbackURL)
			log.Info("生成任务回调已启用（文档声明 callback_url 的端点秒级推送，其余纯轮询）", port.String("callback", callbackURL))
		} else {
			log.Info("生成任务回调未启用（PUBLIC_BASE_URL 非公网且未设 VIDU_CALLBACK_URL）——全端点纯轮询（20s）")
		}
		// 生成域计费接线（F-fix：quotaGate/usageRec 端口此前声明未装配——
		// generation 场景按次限额（超限 402）+ usages 计量（成本分析/配额核对数据源）。
		// Gate/Recorder 均为无状态计算，与 billing 块各持实例无副作用）
		genUsageRec := repository.NewGormUsageRecorder(geoRepos.db)
		genUC.SetUsageRecorder(genUsageRec)
		genUC.SetQuotaGate(quota.NewGate(
			repository.NewGormPlanRepository(geoRepos.db),
			repository.NewGormSubscriptionRepository(geoRepos.db),
			genUsageRec,
		))
		// R2：回调 nonce 判重 Redis 化（多实例安全；未配置 Redis 保持内存实现）
		if redisClient != nil {
			genUC.SetNonceStore(cache.NewRedisNonceStore(redisClient))
		}
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
		// 注入端点选择器（统一提交API需要）
		endpointSelector := generation.NewEndpointSelector(mediaStore, nil)
		genUC.SetEndpointSelector(endpointSelector)
		// 模板管理用例（管理后台可动态配置生成模板）
		templateRepo := repository.NewGormTemplateRepository(geoRepos.db)
		templateUC := generation.NewTemplateUseCase(templateRepo)
		router.SetTemplate(templateUC)
		router.SetGeneration(genUC, viduProvider, genRegistry, genSpecRepo)
		router.SetIntegrationRepo(integrationRepo) // 能力路由新表（集成中心 vendor/capability 管理）

		// ---- 灵感广场（热门视频采集+展示）----
		{
			inspirationVideoRepo := repository.NewGormInspirationVideoRepository(geoRepos.db)
			brandInspirationRepo := repository.NewGormBrandInspirationRepository(geoRepos.db)
			crawlerConfigRepo := repository.NewGormCrawlerConfigRepository(geoRepos.db)
			crawlerAccountRepo := repository.NewGormCrawlerAccountRepository(geoRepos.db)
			crawlerTaskLogRepo := repository.NewGormCrawlerTaskLogRepository(geoRepos.db)

			// 注入爬虫账号仓储到账号管理用例（支持 crawler 场景的扫码登录）
			if geoAccountUC != nil {
				geoAccountUC.SetCrawlerAccountRepo(crawlerAccountRepo)
			}

			// 注入爬虫账号仓储到抖音搜索器（从 crawler_accounts 表读取 Cookie）
			if socialSearcher != nil {
				socialSearcher.SetCrawlerAccountRepo(crawlerAccountRepo)
			}

			inspirationUC := inspiration.NewUseCase(
				inspirationVideoRepo,
				brandInspirationRepo,
				crawlerConfigRepo,
				crawlerAccountRepo,
			)

			// 注册抖音爬虫（复用现有 douyinweb.Searcher）
			if socialSearcher != nil {
				douyinCrawler := douyincrawler.NewDouyinCrawler(socialSearcher, nil)
				inspirationUC.RegisterPlatform("douyin", douyinCrawler)
				log.Info("灵感广场：抖音爬虫已注册")
			}

			router.SetInspiration(inspirationUC, inspirationVideoRepo, geoRepos.brand)
			router.SetCrawlerAdmin(crawlerConfigRepo, crawlerAccountRepo, crawlerTaskLogRepo)
			log.Info("灵感广场已启用")
		}

		// 品牌发布配置（P06：多平台发布模块）
		if geoRepos != nil {
			publishConfigRepo := repository.NewGormBrandPublishConfigRepository(geoRepos.db)
			publishBindingRepo := repository.NewGormAccountBrandBindingRepository(geoRepos.db)
			publishUsageRepo := repository.NewGormPublishUsageRepository(geoRepos.db)
			router.SetPublishConfig(publishConfigRepo, publishBindingRepo, publishUsageRepo)
			log.Info("品牌发布配置已启用")
		}
		// 并发节流（P3）：限制同时提交到 Vidu 的请求数，防瞬时高峰触发 QuotaExceeded/429
		genUC.SetConcurrency(5)
		// 轮询驱动：20s 周期扫描未终态任务（回调到达后幂等跳过；双通道合并）
		_ = taskScheduler.Register(scheduledtask.NewGenerationPollTask(genUC, log))
		// 任务清理（P3）：每日清理 30 天前终态任务 + 过期素材文件
		_ = taskScheduler.Register(scheduledtask.NewGenerationCleanupTask(genUC, log))

		// 注册智能体工具（获客智能体专用）
		// 注意：这些工具不实现CrawlerTool接口，而是独立的工具
		// 智能体通过AgentOrchestrator调用这些工具
		log.Info("智能体工具已注册（query_material, generate_content）")
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // RPA 发布可能需要较长时间完成
	defer cancel()
	_ = server.Shutdown(ctx)
	log.Info("服务已停止")
}

func initRepositories(dbCfg config.DBConfig, logger port.Logger) (
	port.AgentConfigRepository,
	port.LLMConfigRepository, port.UserRepository,
	port.ConversationRepository, port.MessageRepository, port.SystemSettingRepository,
) {
	log := logger.With(port.String("component", "repository"))
	if !dbCfg.IsConfigured() {
		log.Info("未配置数据库，使用内存 mock")
		return mock.NewMockAgentConfigRepository(), mock.NewMockLLMConfigRepository(),
			mock.NewMockUserRepository(),
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
		repository.NewGormUserRepository(db),
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

// genTaskNotifier 生成任务终态 → 站内通知（port.TaskNotifier 适配 NotifyUseCase）。
type genTaskNotifier struct{ uc *notification.NotifyUseCase }

// genSubTypeLabels 端点中文名（通知标题用；未列出的直接展示 sub_type）。
var genSubTypeLabels = map[string]string{
	"text2video": "文生视频", "img2video": "图生视频", "start_end2video": "首尾帧视频",
	"reference2video": "参考生视频", "multiframe": "智能多帧", "digital_human": "数字人口播",
	"subject": "数字分身", "text2image": "图片生成", "text2audio": "文生音频",
	"sound_effect": "音效", "tts": "语音合成", "voice_clone": "声音克隆",
}

func (n genTaskNotifier) NotifyTaskTerminal(ctx context.Context, t entity.GenerationTask) {
	label := genSubTypeLabels[t.SubType]
	if label == "" {
		label = t.SubType
	}
	var title, content, link string
	switch t.State {
	case entity.TaskStateSuccess:
		title = label + "已完成"
		content = "生成成功，点击查看产物"
	default:
		title = label + "生成失败"
		msg := t.ErrMsg
		if msg == "" {
			msg = "请检查参数后重新生成"
		}
		content = msg
	}
	// 结果入口：主体在资产库；其余产物在创作工作台
	if t.SubType == "subject" {
		link = "/m/assets"
	} else {
		link = "/m/compose/tools?tab=media"
	}
	_ = n.uc.Push(ctx, t.TenantID, "generation", title, content, link)
}

// extractResponseStyle 从 extra_json 提取 response_style 字段（小米 MiMo ASR 用 "chat"）。
func extractResponseStyle(extraJSON string) string {
	if extraJSON == "" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(extraJSON), &m) == nil {
		if v, ok := m["response_style"].(string); ok {
			return v
		}
	}
	return ""
}

// extractExtraField 从 extra_json 提取指定字段的字符串值。
func extractExtraField(extraJSON, key string) string {
	if extraJSON == "" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(extraJSON), &m) == nil {
		if v, ok := m[key].(string); ok {
			return v
		}
	}
	return ""
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

// runtimeHeadedSyncer 把浏览器可见性同步到运行时全局内存（RPA allocOpts 即时读新值）。
// systemsettings.HeadedSyncer 的装配实现——main 是唯一允许同时知道"用例"与"config 细节"的地方。
type runtimeHeadedSyncer struct{}

func (runtimeHeadedSyncer) SyncBrowserHeaded(headed bool) { config.SetBrowserHeaded(headed) }

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
	metric  port.VideoMetricRepository // 视频互动数据快照（数据回读）
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
		metric:  repository.NewGormVideoMetricRepository(db),
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
