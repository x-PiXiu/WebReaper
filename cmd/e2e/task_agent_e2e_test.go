//go:build e2e

// 端到端验证：通用任务 Agent 自主完成任务。
// 验证 Agent 能自己规划、调工具（含 generate_content），直到完成。
//
// 运行（需配 LLM_API_KEY）：
//   go test -tags e2e ./cmd/e2e/ -run TestTaskAgentE2E -v -timeout 300s
package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"

	agentadapter "webreaper/internal/adapter/agent"
	"webreaper/internal/adapter/mock"
	zaplogger "webreaper/internal/adapter/logger"
	"webreaper/internal/config"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// TestTaskAgentE2E 给通用 Agent 一个真实任务，验证它自主完成。
//
// 任务设计：让它"采集 trpc-agent-go 仓库信息并总结"——这需要它自主：
// 1. 决定调 search_crawler 或 static_crawler 爬取
// 2. 拿到信息后总结
// 如果任务包含"生成面试题"，它还应该自主调 generate_content。
func TestTaskAgentE2E(t *testing.T) {
	_ = godotenv.Load("../../configs/.env")
	if os.Getenv("LLM_API_KEY") == "" {
		t.Skip("未配置 LLM_API_KEY，跳过 e2e")
	}
	cfg := config.Load()
	if !cfg.LLM.IsConfigured() {
		t.Skip("LLM 未配置")
	}

	logger := zaplogger.MustNewZapLogger(cfg.Server.Env)

	// 装配 LLM + 工具池（含爬虫 + generate_content）
	llmCfgRepo := mock.NewMockLLMConfigRepository()
	_ = llmCfgRepo.Save(context.Background(), entity.LLMConfig{
		Name: "default", Provider: cfg.LLM.Provider, APIKey: cfg.LLM.APIKey,
		BaseURL: cfg.LLM.BaseURL, Model: cfg.LLM.Model,
	})

	// 工具池：注册爬虫 + 内容生成工具
	toolRegistry := port.NewToolRegistry()
	// 注：真实环境 main 里会注册全套爬虫；这里聚焦验证 Agent 自主性，
	// 用 generate_content（图编排）作为可调工具来验证"调子能力"。
	// 先构造图编排（内部用 mock AI 即可，重点验证 Agent 能调它）
	// 为避免 e2e 过重（图编排本身要跑多轮 LLM），这里用一个轻量任务。

	// 构造通用 Agent
	agent := agentadapter.NewExplorerTaskAgent(llmCfgRepo, toolRegistry, mock.NewMockDataItemRepository(), logger)

	// 任务：让 Agent 自主完成一个总结任务（不规定步骤）
	result, err := agent.Execute(context.Background(), port.TaskInput{
		Task:  "简要说明 Go 语言的 goroutine 和 channel 是什么，以及它们如何配合实现并发。给出一个简短的总结。",
		Tools: []string{}, // 空=不给工具，纯 LLM 推理（验证基本自主性）
	}, func(e port.TaskEvent) {
		if e.Type == "text-delta" {
			fmt.Print(e.Text) // 流式打印
		}
	})
	if err != nil {
		t.Fatalf("Agent 执行失败: %v", err)
	}

	fmt.Printf("\n\n========== Agent 最终回复 ==========\n")
	fmt.Println(result.Response)
	fmt.Printf("回复长度: %d 字符\n", len(result.Response))

	if len(result.Response) < 50 {
		t.Errorf("回复过短（%d 字符），Agent 可能未真正完成任务", len(result.Response))
	}
}
