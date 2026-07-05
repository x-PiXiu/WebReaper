// Package llm 提供 LLM 客户端的创建工厂（适配器层）。
//
// 设计动机（DRY + 工厂方法模式）：
//   - 原先 openai.New 的调用散落在 trpc_agent.go 和 trpc_agent_runner.go 两处，
//     且都从全局 config.LLMConfig 读取参数。
//   - 现将「按 LLMConfig 创建 OpenAI 兼容客户端」的逻辑收敛到本工厂，
//     上层只需传入 entity.LLMConfig，不关心 openai SDK 的细节。
//
// 厂商协议统一为 OpenAI 兼容（MiniMax/DeepSeek/Zhipu 等均兼容），
// 故不引入策略模式——provider 字段仅作日志展示。
package llm

import (
	"trpc.group/trpc-go/trpc-agent-go/model/openai"

	"webreaper/internal/domain/entity"
)

// Build 按 LLMConfig 创建一个 OpenAI 兼容的 *openai.Model 客户端。
//
// 传入的 cfg 应已通过领域校验（Name/APIKey/Model 非空）。
// 每次 Build 返回新实例，由调用方按需复用或丢弃。
func Build(cfg entity.LLMConfig) *openai.Model {
	opts := []openai.Option{openai.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
	}
	return openai.New(cfg.Model, opts...)
}
