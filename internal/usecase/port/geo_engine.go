package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// ---- GEO 能力接口（监测/评分/内容生成）----

// ProbeInput 是探测一个关键词在某 AI 引擎的表现的输入。
type ProbeInput struct {
	TenantID    string   // 租户 ID（多租户隔离：临时向量 id 带 tenant 前缀，避免并发碰撞）
	Keyword     string   // 用户可能的搜索词（如"北京装修公司哪家好"）
	EngineName  string   // 探测的 AI 引擎（对应 LLMConfig.Name）
	BrandName   string   // 要监测的品牌名
	Aliases     []string // 品牌别名（AI 可能用简称/全称，监测时一起匹配）
	Competitors []string // 竞品名（同时监测对比）
	SampleSize  int      // 采样次数（默认 5，越多越准但越贵）
}

// ProbeResult 是一次探测的统计结果。
type ProbeResult struct {
	SampleCount  int
	MentionCount int     // 提到品牌的次数
	MentionRate  float64 // 提及率
	AvgPosition  int     // 平均排名
	Sentiment    string  // positive/neutral/negative
	Competitors  map[string]int // 竞品名 → 被提及次数
	RawSample    string  // 原始回答摘录
	SourceCount  int     // 搜索源文章数
	BrandAppearanceCount int // 品牌在检索源里出现的文章数
	Confidence   float64 // 置信度（由 Probe 实现按信息量计算，不再固定 sampleCount/5）
}

// AIEngineProbe 是 AI 引擎监测适配器的接口（边界）。
// 用例层声明，适配器层实现（复用 port.AIGenerator 调 LLM，但把"问问题"包装成"探测"）。
//
// 设计动机（依赖倒置）：
//   - 用例层不关心怎么调 LLM、怎么解析回答，只依赖此接口。
//   - 换 AI 引擎 = 换 LLMConfig，探测实现零改动。
type AIEngineProbe interface {
	// Probe 对一个关键词问 AI 引擎，采样 N 次，解析品牌提及情况。
	Probe(ctx context.Context, in ProbeInput) (ProbeResult, error)
}

// GEOScorer 是 GEO 评分器接口（边界）。
// 实现：规则匹配 + LLM 评估混合。用例层只依赖此接口。
type GEOScorer interface {
	// Score 给内容打 GEO 分。
	Score(ctx context.Context, content string, keyword string) (entity.GEOScore, error)
}
