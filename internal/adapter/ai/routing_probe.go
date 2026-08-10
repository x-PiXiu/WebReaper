package ai

import (
	"context"

	"webreaper/internal/usecase/port"
)

// RoutingProbe 是 port.AIEngineProbe 的"引擎路由"实现（组合模式）。
//
// 设计动机（真实测量 vs 模拟估算 的分层）：
//   - 用户在前端选了具体引擎（EngineName = LLMConfigName，如 doubao/kimi）且
//     该 LLMConfig 真实存在 → 用 DirectProbe 直测真实引擎（真实引用率）。
//   - EngineName 为空或配置不存在 → 用 AgentProbe 模拟引擎（估算兜底，保持原行为）。
//
// 路由依据：llmCfgRepo.FindByName——配置存在即视为"已接入的真实引擎"。
// 这样新引擎接入 = 管理后台加一条 LLMConfig，无需改代码（开闭原则）。
type RoutingProbe struct {
	agentProbe  port.AIEngineProbe      // 模拟引擎（Agent 自主搜索）
	directProbe port.AIEngineProbe      // 真实引擎直测
	llmCfgRepo  port.LLMConfigRepository // 引擎注册表（判断 EngineName 是否真实存在）
}

// NewRoutingProbe 创建引擎路由探测。
func NewRoutingProbe(agent, direct port.AIEngineProbe, repo port.LLMConfigRepository) *RoutingProbe {
	return &RoutingProbe{agentProbe: agent, directProbe: direct, llmCfgRepo: repo}
}

// Probe 按 EngineName 路由：真实配置存在 → 直测；否则 → 模拟。
func (p *RoutingProbe) Probe(ctx context.Context, in port.ProbeInput) (port.ProbeResult, error) {
	if in.EngineName != "" && p.llmCfgRepo != nil {
		if _, err := p.llmCfgRepo.FindByName(ctx, in.EngineName); err == nil {
			return p.directProbe.Probe(ctx, in)
		}
	}
	return p.agentProbe.Probe(ctx, in)
}

var _ port.AIEngineProbe = (*RoutingProbe)(nil)
