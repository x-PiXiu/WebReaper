// Package ai 提供 GEO 适配器（LLM 深度评分 + 通用工具函数）。
//
// 设计动机（整洁架构 / 依赖倒置）：
//   - LLMGEOScorer 复用 port.AIGenerator 调 LLM 做五维深度评分（内容定级用）。
//   - 免费快筛评分由 usecase/geo.RuleScorer 提供（纯函数，优化前后对比用）。
//   - 监测探测实现为 agent_probe.go（AgentProbe，main 装配的唯一探测引擎）。
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// truncateForGeo 截断字符串到指定长度（留证样本用；跨文件共用，勿删）。
func truncateForGeo(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// ---- GEO 评分器 ----

// LLMGEOScorer 是 port.GEOScorer 的实现。
// 用 LLM 评估内容的五个维度（权威性/具体性/结构化/独特性/时效性）。
type LLMGEOScorer struct {
	aiGen port.AIGenerator
}

// NewLLMGEOScorer 创建 GEO 评分器。
func NewLLMGEOScorer(ai port.AIGenerator) *LLMGEOScorer {
	return &LLMGEOScorer{aiGen: ai}
}

// Score 用 LLM 给内容打 GEO 分。
// LLM 返回 JSON 格式的五维评分，解析后加权算总分。
func (s *LLMGEOScorer) Score(ctx context.Context, content string, keyword string) (entity.GEOScore, error) {
	if content == "" {
		return entity.GEOScore{}, fmt.Errorf("内容为空")
	}
	systemPrompt := `你是 GEO（生成式引擎优化）评分专家。评估给定内容被 AI 搜索引擎引用的可能性。
从五个维度评分（每项 0-100）：
1. authority：权威性（有无数据/案例/资质支撑）
2. specificity：具体性（数字、细节、可验证信息）
3. structure：结构化（标题层级、列表、FAQ）
4. uniqueness：独特性（与全网内容的差异化）
5. recency：时效性（信息是否最新）

只返回 JSON，格式：{"authority":85,"specificity":70,"structure":80,"uniqueness":65,"recency":75}`

	userPrompt := fmt.Sprintf("目标关键词：%s\n\n待评内容：\n%s", keyword, content)

	messages := []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	convID := fmt.Sprintf("geo-score-%d", time.Now().UnixNano())
	resp, err := s.aiGen.ChatStream(ctx, convID, "", messages, nil)
	if err != nil {
		return entity.GEOScore{}, fmt.Errorf("评分 LLM 调用失败: %w", err)
	}

	score := parseGeoScoreJSON(resp)
	score.Total = (score.Authority + score.Specificity + score.Structure + score.Uniqueness + score.Recency) / 5
	return score, nil
}

// geoScoreJSON 是 LLM 评分输出的结构化定义。
type geoScoreJSON struct {
	Authority   float64 `json:"authority"`
	Specificity float64 `json:"specificity"`
	Structure   float64 `json:"structure"`
	Uniqueness  float64 `json:"uniqueness"`
	Recency     float64 `json:"recency"`
}

// parseGeoScoreJSON 从 LLM 返回的文本解析五维评分。
// 容错：LLM 可能包裹 markdown 或多余文字——先提取 JSON 对象再标准解析；
// 解析失败返回全 50 分（保守中等分，不阻断流程）。
func parseGeoScoreJSON(s string) entity.GEOScore {
	jsonStr := extractJSONBlock(s)
	if jsonStr == "" {
		jsonStr = s
	}
	var raw geoScoreJSON
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return entity.GEOScore{Authority: 50, Specificity: 50, Structure: 50, Uniqueness: 50, Recency: 50}
	}
	clamp := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		if v > 100 {
			return 100
		}
		return v
	}
	return entity.GEOScore{
		Authority:   clamp(raw.Authority),
		Specificity: clamp(raw.Specificity),
		Structure:   clamp(raw.Structure),
		Uniqueness:  clamp(raw.Uniqueness),
		Recency:     clamp(raw.Recency),
	}
}
