//go:build e2e

// 端到端验证：图编排生成 trpc-agent-go 面试题。
// 用真实 LLM，验证 scout→generator→validator 流程真能跑通。
//
// 运行（需配 LLM_API_KEY）：
//   go test -tags e2e ./cmd/e2e/ -run TestOrchestrateE2E -v -timeout 300s
package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"

	agentadapter "webreaper/internal/adapter/agent"
	"webreaper/internal/adapter/ai"
	"webreaper/internal/adapter/mock"
	zaplogger "webreaper/internal/adapter/logger"
	"webreaper/internal/config"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// TestOrchestrateE2E 真实跑一次图编排：为 trpc-agent-go 生成面试题。
func TestOrchestrateE2E(t *testing.T) {
	// go test 的工作目录是测试包目录，config.Load 的相对路径 configs/.env 找不到。
	// 显式从项目根加载 .env（向上找两级）。
	_ = godotenv.Load("../../configs/.env")
	// 环境变量优先（CI 注入）
	if os.Getenv("LLM_API_KEY") == "" {
		t.Skip("未配置 LLM_API_KEY，跳过 e2e（需在 configs/.env 或环境变量配置）")
	}
	cfg := config.Load()
	if !cfg.LLM.IsConfigured() {
		t.Skip("LLM 未配置")
	}

	logger := zaplogger.MustNewZapLogger(cfg.Server.Env)

	// 装配真实 LLM + mock 仓储（图编排本身不落库，e2e 只验证编排逻辑）
	llmCfgRepo := mock.NewMockLLMConfigRepository()
	_ = llmCfgRepo.Save(context.Background(), entity.LLMConfig{
		Name: "default", Provider: cfg.LLM.Provider, APIKey: cfg.LLM.APIKey,
		BaseURL: cfg.LLM.BaseURL, Model: cfg.LLM.Model,
	})
	toolRegistry := port.NewToolRegistry()
	gen, err := ai.NewTrpcAgentGenerator(llmCfgRepo, toolRegistry, logger)
	if err != nil {
		t.Fatalf("LLM 初始化失败: %v", err)
	}

	// 注册 scout 探查用的爬虫工具（图编排的 scout 节点会调这些）
	// 注：e2e 这里只构造 orchestrator，工具在真实 main 里有完整注册；
	// 本测试聚焦"图编排流程本身"，scout 调 LLM 用 RunWithTools。

	orch := agentadapter.NewGraphContentOrchestrator(gen,
		[]string{"static_crawler", "search_crawler"}, logger)

	items, err := orch.Orchestrate(context.Background(), port.OrchestrateInput{
		Topic:       "trpc-agent-go 框架",
		ContentType: "interview_questions",
	}, func(msg string) {
		fmt.Printf("  [进度] %s\n", msg)
	})
	if err != nil {
		t.Fatalf("Orchestrate 失败: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("应至少生成 1 条，得到 0 条")
	}

	// 打印生成结果
	fmt.Printf("\n========== 生成 %d 条面试题 ==========\n", len(items))
	for i, it := range items {
		fmt.Printf("\n--- 第 %d 条 [模块: %s] ---\n", i+1, it.Module)
		fmt.Printf("标题: %s\n", it.Title)
		fmt.Printf("内容: %s\n", truncateStr(it.Content, 200))
		if len(it.Tags) > 0 {
			fmt.Printf("标签: %v\n", it.Tags)
		}
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
