// Package main 是 WebReaper 的 Web 服务入口（通用数据采集平台）。
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	authadapter "webreaper/internal/adapter/auth"
	agentadapter "webreaper/internal/adapter/agent"
	"webreaper/internal/adapter/ai"
	"webreaper/internal/adapter/crawler"
	"webreaper/internal/adapter/embedding"
	zaplogger "webreaper/internal/adapter/logger"
	"webreaper/internal/adapter/handler"
	"webreaper/internal/adapter/mock"
	"webreaper/internal/adapter/repository"
	"webreaper/internal/adapter/vectorstore"
	"webreaper/internal/adapter/telemetry"
	"webreaper/internal/config"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/agentconfig"
	"webreaper/internal/usecase/auth"
	"webreaper/internal/usecase/conversation"
	"webreaper/internal/usecase/crawlconfig"
	"webreaper/internal/usecase/dataitem"
	"webreaper/internal/usecase/llmconfig"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/publish"
	"webreaper/internal/usecase/orchestrate"
	"webreaper/internal/usecase/taskagent"
	taskquery "webreaper/internal/usecase/taskquery"
	taskuc "webreaper/internal/usecase/task"
	"webreaper/internal/usecase/process"
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
	dataItemRepo, collectionRepo, agentConfigRepo, llmConfigRepo, taskRepo, userRepo, convRepo, msgRepo, settingRepo, extSysRepo, pubRecRepo := initRepositories(cfg.DB, logger)

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
	// 3 种装饰器爬虫（包装基础爬虫，增加能力）
	// 注意：装饰器需要指定被包装的基础爬虫，这里用 static_crawler 作为默认基础
	staticCrawler := crawler.NewStaticCrawler()
	registerLimited(crawler.NewFocusedCrawler(staticCrawler, []string{})) // 关键词由 Agent 动态指定
	registerLimited(crawler.NewIncrementalCrawler(staticCrawler))
	registerLimited(crawler.NewDeepCrawler(staticCrawler))

	// AI 生成器（注入 LLMConfigRepository 用于运行时按配置选 LLM，注入 ToolRegistry 让 LLM 调用全部爬虫）
	aiGenerator := initAIGenerator(cfg.LLM, llmConfigRepo, toolRegistry, logger)

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
	if cfg.Milvus.Host != "" {
		vs, err := vectorstore.NewMilvusVectorStore(cfg.Milvus.Host, cfg.Milvus.Port)
		if err != nil {
			log.Warn("Milvus 连接失败，使用内存向量存储", port.Err(err))
			vectorStore = vectorstore.NewMemoryVectorStore()
		} else {
			vectorStore = vs
		}
	} else {
		vectorStore = vectorstore.NewMemoryVectorStore()
		log.Info("未配置 Milvus，使用内存向量存储")
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
	if cfg.JWT.Secret != "" { tokenParser = tokenGen.(*authadapter.JWTGenerator) }
	registerUC := auth.NewRegisterUseCase(userRepo, hasher)
	loginUC := auth.NewLoginUseCase(userRepo, hasher, tokenGen)

	// Agent 执行器（注入 LLMConfigRepository 用于按 Agent 选 LLM，
	// 注入 DataItemRepo 用于工具采集结果落库，注入 Logger 用于工具落库日志）
	agentRunner := agentadapter.NewTrpcAgentRunner(llmConfigRepo, toolRegistry, dataItemRepo, logger)

	// 业务用例装配（handler 只依赖这些 usecase，不直接持有仓储）
	taskQueryUC := taskquery.NewTaskQueryUseCase(taskRepo)
	dataItemUC := dataitem.NewDataItemUseCase(dataItemRepo, collectionRepo, itemProcessor, logger)
	agentCfgUC := agentconfig.NewAgentConfigUseCase(agentConfigRepo)
	conversationUC := conversation.NewConversationUseCase(convRepo, msgRepo)
	crawlCfgUC := crawlconfig.NewCrawlConfigUseCase(settingRepo)
	// 首次启动 seed 默认采集策略
	_ = crawlCfgUC.EnsureDefault(context.Background())

	// 外部系统推送用例（字段映射 + HTTP 推送 + 推送记录）
	publishUC := publish.NewPublishUseCase(extSysRepo, pubRecRepo, dataItemRepo, logger)
	publishUC.SetTracer(tracer)
	publishUC.SetMaxRetries(cfg.Publish.MaxRetries)
	sysCfgUC := publish.NewSystemConfigUseCase(extSysRepo)

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

	// 通用任务 Agent（任意任务 → LLM 自主规划 → 调工具/子能力 → 直到完成）。
	// 仅配了 LLM 时启用；否则降级为 nil，该端点不注册。
	var taskAgentUC *taskagent.TaskAgentUseCase
	if cfg.LLM.IsConfigured() {
		taskAgent := agentadapter.NewExplorerTaskAgent(llmConfigRepo, toolRegistry, dataItemRepo, logger)
		taskAgentUC = taskagent.NewTaskAgentUseCase(taskAgent, logger)
		log.Info("通用任务 Agent 已启用（Explorer ReAct 模式）")
	}

	// 注册推送工具为 Agent 可调用工具（装配层做适配，依赖方向合法）
	// PublisherAdapter 把 publish.PublishUseCase 适配为 crawler.Publisher 接口
	publisherAdapter := &publisherAdapterImpl{uc: publishUC, sysRepo: extSysRepo}
	toolRegistry.Register(crawler.NewPublishTool(publisherAdapter))
	toolRegistry.Register(crawler.NewListExternalSystemsTool(publisherAdapter))

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
		publishUC, sysCfgUC, toolRegistry, knowledgeSearch, orchestrateUC, taskAgentUC)
	server := &http.Server{Addr: ":" + cfg.Server.Port, Handler: router.Engine()}

	go func() {
		log.Info("HTTP 服务已启动", port.String("port", cfg.Server.Port))
		if cfg.JWT.Secret == "" { log.Warn("JWT_SECRET 未配置，API 认证已禁用") }
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("服务启动失败", port.Err(err)); os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("正在关闭服务...")
	workerCancel()
	_ = taskQueue.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	log.Info("服务已停止")
}

func initRepositories(dbCfg config.DBConfig, logger port.Logger) (
	port.DataItemRepository, port.CollectionRepository, port.AgentConfigRepository,
	port.LLMConfigRepository, port.TaskRepository, port.UserRepository,
	port.ConversationRepository, port.MessageRepository, port.SystemSettingRepository,
	port.ExternalSystemRepository, port.PublishRecordRepository,
) {
	log := logger.With(port.String("component", "repository"))
	if !dbCfg.IsConfigured() {
		log.Info("未配置数据库，使用内存 mock")
		return mock.NewMockDataItemRepository(), mock.NewMockCollectionRepository(),
			mock.NewMockAgentConfigRepository(), mock.NewMockLLMConfigRepository(),
			mock.NewMockTaskRepository(), mock.NewMockUserRepository(),
			mock.NewMockConversationRepository(), mock.NewMockMessageRepository(),
			mock.NewMockSystemSettingRepository(),
			mock.NewMockExternalSystemRepository(), mock.NewMockPublishRecordRepository()
	}
	log.Info("连接 MySQL", port.String("host", dbCfg.Host), port.String("db", dbCfg.Name))
	db, err := repository.NewMySQLDBFromConfig(dbCfg)
	if err != nil { log.Error("连接数据库失败", port.Err(err)); os.Exit(1) }
	log.Info("MySQL 连接成功")
	return repository.NewGormDataItemRepository(db), repository.NewGormCollectionRepository(db),
		repository.NewGormAgentConfigRepository(db), repository.NewGormLLMConfigRepository(db),
		repository.NewGormTaskRepository(db), repository.NewGormUserRepository(db),
		repository.NewGormConversationRepository(db), repository.NewGormMessageRepository(db),
		repository.NewGormSystemSettingRepository(db),
		repository.NewGormExternalSystemRepository(db), repository.NewGormPublishRecordRepository(db)
}

func initAIGenerator(llmCfg config.LLMConfig, llmCfgRepo port.LLMConfigRepository, toolRegistry *port.ToolRegistry, logger port.Logger) port.AIGenerator {
	log := logger.With(port.String("component", "ai"))
	if !llmCfg.IsConfigured() {
		log.Info("未配置 LLM_API_KEY，使用 mock AI")
		return mock.NewMockAIGenerator()
	}
	gen, err := ai.NewTrpcAgentGenerator(llmCfgRepo, toolRegistry, logger)
	if err != nil {
		log.Error("LLM 初始化失败，降级 mock", port.Err(err))
		return mock.NewMockAIGenerator()
	}
	log.Info("trpc-agent-go LLM 就绪（按 LLMConfig 动态选择）")
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

// publisherAdapterImpl 把 usecase/publish.PublishUseCase 适配为 crawler.Publisher 接口。
//
// 设计动机（依赖倒置）：crawler.Publisher 接口定义在 adapter/crawler 包，
// publish 用例不能反向依赖 crawler 包（违反依赖方向）。
// 所以在装配层（main）写适配器，让 publish 用例通过它被 crawler 工具调用，
// 依赖方向：crawler(工具) → Publisher(接口，crawler包定义) ← adapter(main) → publish(用例)。
type publisherAdapterImpl struct {
	uc      *publish.PublishUseCase
	sysRepo port.ExternalSystemRepository
}

func (a *publisherAdapterImpl) PublishTo(ctx context.Context, dataItemID, systemName string) (crawler.PublishResult, error) {
	out, err := a.uc.Publish(ctx, publish.PublishInput{DataItemID: dataItemID, SystemName: systemName})
	if err != nil {
		return crawler.PublishResult{}, err
	}
	return crawler.PublishResult{
		Success: out.Success, ExternalID: out.ExternalID, Error: out.ErrorMsg,
	}, nil
}

func (a *publisherAdapterImpl) ListSystems(ctx context.Context) ([]crawler.SystemInfo, error) {
	list, err := a.sysRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]crawler.SystemInfo, 0, len(list))
	for _, s := range list {
		if !s.Enabled {
			continue
		}
		result = append(result, crawler.SystemInfo{
			Name: s.Name, Description: s.Description,
			ContentType: s.ContentType, Endpoint: s.Endpoint,
		})
	}
	return result, nil
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
