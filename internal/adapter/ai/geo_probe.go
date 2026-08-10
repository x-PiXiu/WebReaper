// Package ai 提供 GEO 监测适配器（真实 LLM 探测 + GEO 评分）。
//
// 设计动机（整洁架构 / 依赖倒置）：
//   - AIEngineProbe 复用 port.AIGenerator 调 LLM，把"问问题"包装成"探测品牌提及"。
//   - GEOScorer 用规则匹配 + LLM 评估混合打分。
//   - 两者都实现 port 层接口，用例层不关心具体实现。
package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// LLMEngineProbe 是 port.AIEngineProbe 的实现。
// 复用 port.AIGenerator 调 LLM，把关键词当问题问 AI，采样多次后解析品牌提及。
type LLMEngineProbe struct {
	aiGen port.AIGenerator
}

// NewLLMEngineProbe 创建真实 LLM 监测适配器。
func NewLLMEngineProbe(ai port.AIGenerator) *LLMEngineProbe {
	return &LLMEngineProbe{aiGen: ai}
}

// Probe 对一个关键词问 AI 引擎，采样 N 次，解析品牌提及情况。
func (p *LLMEngineProbe) Probe(ctx context.Context, in port.ProbeInput) (port.ProbeResult, error) {
	sampleSize := in.SampleSize
	if sampleSize <= 0 {
		sampleSize = 5
	}

	// 构造探测问题（模拟真实用户搜索）
	question := in.Keyword
	// 系统提示：让 AI 像回答真实用户提问一样作答（不加"你是助手"之类，避免影响结果）
	systemPrompt := "你是一个乐于助人的助手，请像回答真实用户提问一样作答。"

	// 所有品牌名（主品牌 + 别名）用于匹配
	allBrandNames := append([]string{in.BrandName}, in.Aliases...)

	mentionCount := 0
	positionSum := 0
	sentimentPos, sentimentNeg := 0, 0
	competitorMentions := make(map[string]int)
	var rawSample string

	for i := 0; i < sampleSize; i++ {
		messages := []port.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: question},
		}
		answer, err := p.aiGen.ChatStream(ctx, "", in.EngineName, messages, nil)
		if err != nil {
			continue // 单次失败降级跳过
		}
		answer = strings.TrimSpace(answer)
		if i == 0 {
			rawSample = truncateForGeo(answer, 500) // 留第一条作样本
		}

		// 解析品牌提及
		lowerAns := strings.ToLower(answer)
		mentioned := false
		for _, name := range allBrandNames {
			if name != "" && strings.Contains(lowerAns, strings.ToLower(name)) {
				mentioned = true
				break
			}
		}
		if mentioned {
			mentionCount++
			// 估算位置：品牌名在回答中第一次出现的位置（按字符比例粗略算排名）
			for _, name := range allBrandNames {
				if name == "" {
					continue
				}
				idx := strings.Index(lowerAns, strings.ToLower(name))
				if idx >= 0 {
					// 转化为"第几个被提及的品牌"的近似排名
					// 简化：位置越靠前排名越靠前（1=首位）
					ratio := float64(idx) / float64(len(lowerAns)+1)
					pos := int(ratio*10) + 1 // 1~10
					positionSum += pos
					break
				}
			}
		}

		// 情感分析（简化：关键词匹配）
		if containsAny(lowerAns, []string{"推荐", "不错", "好评", "专业", "靠谱", "优质", "recommend", "good"}) {
			sentimentPos++
		}
		if containsAny(lowerAns, []string{"差评", "投诉", "问题", "负面", "bad", "poor", "问题多"}) {
			sentimentNeg++
		}

		// 竞品提及统计
		for _, comp := range in.Competitors {
			if comp != "" && strings.Contains(lowerAns, strings.ToLower(comp)) {
				competitorMentions[comp]++
			}
		}
	}

	// 聚合统计
	mentionRate := 0.0
	if sampleSize > 0 {
		mentionRate = float64(mentionCount) / float64(sampleSize)
	}
	avgPos := 0
	if mentionCount > 0 {
		avgPos = positionSum / mentionCount
	}
	sentiment := "neutral"
	if sentimentPos > sentimentNeg {
		sentiment = "positive"
	} else if sentimentNeg > sentimentPos {
		sentiment = "negative"
	}

	return port.ProbeResult{
		SampleCount:  sampleSize,
		MentionCount: mentionCount,
		MentionRate:  mentionRate,
		AvgPosition:  avgPos,
		Sentiment:    sentiment,
		Competitors:  competitorMentions,
		RawSample:    rawSample,
	}, nil
}

// containsAny 检查 s 是否包含 words 中任一词。
func containsAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// truncateForGeo 截断字符串到指定长度（留证样本用）。
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

// parseGeoScoreJSON 从 LLM 返回的 JSON 解析五维评分（容错：提取 JSON 片段）。
func parseGeoScoreJSON(s string) entity.GEOScore {
	// 简化的 JSON 解析：提取数值（容错处理 LLM 可能带额外文字）
	score := entity.GEOScore{}
	score.Authority = extractScoreValue(s, "authority")
	score.Specificity = extractScoreValue(s, "specificity")
	score.Structure = extractScoreValue(s, "structure")
	score.Uniqueness = extractScoreValue(s, "uniqueness")
	score.Recency = extractScoreValue(s, "recency")
	return score
}

// extractScoreValue 从字符串中提取 "key":value 的数值（容错）。
func extractScoreValue(s, key string) float64 {
	// 查找 "key": 后面的数字
	pattern := "\"" + key + "\":"
	idx := strings.Index(s, pattern)
	if idx < 0 {
		return 50 // 默认中等分
	}
	rest := s[idx+len(pattern):]
	// 跳过空格，提取数字
	rest = strings.TrimLeft(rest, " \t\n\r")
	var num strings.Builder
	for _, r := range rest {
		if (r >= '0' && r <= '9') || r == '.' {
			num.WriteRune(r)
		} else {
			break
		}
	}
	var v float64
	fmt.Sscanf(num.String(), "%f", &v)
	if v <= 0 {
		v = 50
	}
	if v > 100 {
		v = 100
	}
	return v
}
