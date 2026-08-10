// Package main 是 WebReaper 真实闭环联调脚本（一次性验证工具）。
//
// 目标：端到端验证「采集 → AI 加工 → 推送」核心价值链真能跑通。
// 不做自动化测试框架，只做"跑一次看结果"的联调工具。
//
// 流程：
//  1. 采集：用 api_crawler 调开放招聘 API（arbeitnow，无需 auth），拿真实职位 JSON
//  2. 落库：构造 DataItem 存入 mock repo（拿到 ID）
//  3. AI 加工：用真实 LLM（配了 MiniMax 时）做结构化提取；无 key 则降级跳过
//  4. 推送：起本地 HTTP 接收服务，用 publish.PublishUseCase 真实推送一条
//
// 设计：每步独立可验证，失败打 WARN 不阻断后续，最终汇总结果。
//
// 启动：go run ./cmd/e2e
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"webreaper/internal/adapter/crawler"
	"webreaper/internal/adapter/mock"
	zaplogger "webreaper/internal/adapter/logger"
	"webreaper/internal/adapter/ai"
	"webreaper/internal/config"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/publish"
)

// arbeitnow 开放招聘 API（无需 auth，JSON 格式）。
// 备选：RemoteOK https://remoteok.com/api
const arbeitnowAPI = "https://www.arbeitnow.com/api/job-board-api"

func main() {
	ctx := context.Background()
	cfg := config.Load()
	logger := zaplogger.MustNewZapLogger(cfg.Server.Env)
	defer logger.Sync()
	log := logger.With(port.String("component", "e2e"))

	fmt.Println("========== WebReaper 真实闭环联调 ==========")
	fmt.Printf("环境: %s | LLM: %v | 时间: %s\n\n",
		cfg.Server.Env, cfg.LLM.IsConfigured(), time.Now().Format(time.RFC3339))

	// ── 步骤 1：真实采集（api_crawler 调 arbeitnow）──
	fmt.Println("【步骤 1】采集 arbeitnow 开放招聘 API ...")
	item, err := step1Collect(ctx)
	if err != nil {
		fmt.Printf("  ❌ 采集失败: %v\n", err)
		fmt.Println("  （可能是网络问题或 API 不可用，联调中止）")
		os.Exit(1)
	}
	fmt.Printf("  ✅ 采集成功：title=%q, content 长度=%d 字符\n", truncate(item.Title, 60), len(item.Content))
	fmt.Printf("     原始 JSON 预览: %s\n\n", truncate(item.RawContent, 200))

	// ── 步骤 2：落库（mock repo，拿到 ID 供后续步骤引用）──
	fmt.Println("【步骤 2】存入 mock 仓储 ...")
	dataRepo := mock.NewMockDataItemRepository()
	if err := dataRepo.Save(ctx, item); err != nil {
		fmt.Printf("  ❌ 落库失败: %v\n", err)
	} else {
		fmt.Printf("  ✅ 落库成功：item_id=%s\n\n", item.ID)
	}

	// ── 步骤 3：AI 加工（真实 LLM 或降级跳过）──
	fmt.Println("【步骤 3】AI 结构化加工 ...")
	step3AIProcess(ctx, cfg, log, item)

	// ── 步骤 4：真实推送（本地 HTTP 接收服务 + publish 用例）──
	fmt.Println("\n【步骤 4】真实推送（本地 HTTP 接收）...")
	step4Publish(ctx, log, item.ID)

	fmt.Println("\n========== 联调完成 ==========")
	fmt.Println("详见上方各步骤结果。架构是否真能用，看每步 ✅/❌。")
}

// step1Collect 用 api_crawler 真实采集 arbeitnow。
func step1Collect(ctx context.Context) (entity.DataItem, error) {
	c := crawler.NewAPICrawler()
	argsJSON := fmt.Sprintf(`{"url":%q,"method":"GET"}`, arbeitnowAPI)
	item, err := c.Execute(ctx, argsJSON)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("api_crawler 执行失败: %w", err)
	}
	// 解析 JSON 看是否拿到真实数据（arbeitnow 返回 {data:[{title,company,...}]}）
	var resp struct {
		Data []struct {
			Title       string `json:"title"`
			Company     string `json:"company_name"`
			Description string `json:"description"`
			Location    string `json:"location"`
		} `json:"data"`
	}
	if jerr := json.Unmarshal([]byte(item.RawContent), &resp); jerr != nil {
		return entity.DataItem{}, fmt.Errorf("解析 arbeitnow 响应失败（非 JSON？）: %w", jerr)
	}
	if len(resp.Data) == 0 {
		return entity.DataItem{}, fmt.Errorf("arbeitnow 返回 0 条职位（数据源异常或被限流）")
	}
	// 用第一条职位构造更清晰的 DataItem
	job := resp.Data[0]
	return entity.DataItem{
		ID:         fmt.Sprintf("e2e-%d", time.Now().UnixNano()),
		Title:      job.Title,
		Content:    fmt.Sprintf("%s @ %s\n\n%s", job.Title, job.Company, job.Description),
		Summary:    "",
		SourceURL:  arbeitnowAPI,
		RawContent: item.RawContent, // 保留完整原始 JSON
		Status:     entity.ItemStatusPendingReview,
		Tags:       []string{},
		Metadata:   map[string]string{"crawler_type": "e2e", "company": job.Company, "location": job.Location},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

// step3AIProcess 用真实 LLM 做结构化提取；无 key 降级跳过。
func step3AIProcess(ctx context.Context, cfg config.Config, log port.Logger, item entity.DataItem) {
	if !cfg.LLM.IsConfigured() {
		fmt.Println("  ⚠️ 未配置 LLM_API_KEY，AI 加工降级跳过（配置后可验证完整 LLM 链路）")
		fmt.Printf("     提示：cp configs/.env.example configs/.env 并填 LLM_API_KEY\n")
		return
	}
	// 用真实 trpc-agent-go（注入 mock LLMConfigRepo，seed 一条 default 配置）
	llmCfgRepo := mock.NewMockLLMConfigRepository()
	_ = llmCfgRepo.Save(ctx, entity.LLMConfig{
		Name: "default", Provider: cfg.LLM.Provider, APIKey: cfg.LLM.APIKey,
		BaseURL: cfg.LLM.BaseURL, Model: cfg.LLM.Model,
	})
	toolRegistry := port.NewToolRegistry()
	gen, err := ai.NewTrpcAgentGenerator(llmCfgRepo, toolRegistry, nil, log) // e2e 不启用历史恢复
	if err != nil {
		fmt.Printf("  ❌ LLM 初始化失败: %v\n", err)
		return
	}
	prompt := fmt.Sprintf(`分析以下招聘信息，提取结构化字段。返回 JSON：
{"title":"简洁标题","summary":"一句话摘要","tags":["技能1","技能2"]}

内容：
%s`, truncate(item.Content, 2000))
	resp, err := gen.ChatStream(ctx, "", "", []port.ChatMessage{
		{Role: "system", Content: "你是招聘信息结构化助手。只返回 JSON。"},
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		fmt.Printf("  ❌ LLM 调用失败: %v\n", err)
		return
	}
	fmt.Printf("  ✅ AI 加工成功（token 消耗见日志）\n")
	fmt.Printf("     LLM 返回: %s\n", truncate(resp, 300))
}

// step4Publish 起本地 HTTP 接收服务，用 publish 用例真实推送。
func step4Publish(ctx context.Context, log port.Logger, itemID string) {
	// 起一个本地 HTTP 服务模拟外部系统（接收推送）
	received := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/ingest", func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 4096)
		n, _ := r.Body.Read(body)
		received <- string(body[:n])
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"ext-123"}`))
	})
	go func() { _ = http.ListenAndServe(":8099", mux) }()
	time.Sleep(200 * time.Millisecond) // 等服务起来

	// 构造外部系统配置 + 用例
	extSysRepo := mock.NewMockExternalSystemRepository()
	_ = extSysRepo.Save(ctx, entity.ExternalSystem{
		Name: "e2e-sink", Endpoint: "http://localhost:8099/ingest",
		Method: "POST", Mode: entity.PublishModeRaw, Enabled: true,
	})
	dataRepo := mock.NewMockDataItemRepository()
	// 注意：publish 会查 dataRepo.FindByID，需先把 item 存进去
	_ = dataRepo.Save(ctx, entity.DataItem{
		ID: itemID, Title: "e2e-item", Content: `{"hello":"world"}`,
		RawContent: `{"hello":"world","job":true}`, Status: entity.ItemStatusApproved,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	recRepo := mock.NewMockPublishRecordRepository()
	uc := publish.NewPublishUseCase(extSysRepo, recRepo, dataRepo, log)

	out, err := uc.Publish(ctx, publish.PublishInput{DataItemID: itemID, SystemName: "e2e-sink"})
	if err != nil {
		fmt.Printf("  ❌ 推送失败: %v\n", err)
		return
	}
	// 验证接收端真的收到了数据
	select {
	case body := <-received:
		fmt.Printf("  ✅ 推送成功：external_id=%q, success=%v\n", out.ExternalID, out.Success)
		fmt.Printf("     接收端收到的 body: %s\n", truncate(body, 200))
	case <-time.After(3 * time.Second):
		fmt.Println("  ❌ 推送调用成功但接收端 3 秒未收到数据（异常）")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
