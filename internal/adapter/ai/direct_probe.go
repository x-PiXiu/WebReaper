package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/usecase/port"
)

// DirectProbe 是 port.AIEngineProbe 的"真实 AI 直测"实现。
//
// 核心区别（vs RagProbe）：
//   - RagProbe：自己爬全网→自建RAG→自己综合回答→解析（自问自答，模拟性质）
//   - DirectProbe：直接调真实 AI 引擎问问题→收集真实回答→解析（真实测量）
//
// DirectProbe 才是真正的 GEO 监测——它测量的是"真实 AI 引擎（豆包/Kimi/文心/DeepSeek）
// 在被问到这个关键词时，会不会提到你的品牌"。
//
// 流程极简（无爬虫无RAG）：
//   循环采样 N 次 → 直接问 AI 引擎 → 收集回答 → LLM 解析每条回答的提及情况 → 聚合
//
// 采样问法多样化：不只用原关键词，还用同义改写，避免 AI 因缓存返回相同答案。
type DirectProbe struct {
	aiGen port.AIGenerator
}

func NewDirectProbe(ai port.AIGenerator) *DirectProbe {
	return &DirectProbe{aiGen: ai}
}

// Probe 对一个关键词直接问真实 AI 引擎，采样 N 次，解析品牌提及。
func (p *DirectProbe) Probe(ctx context.Context, in port.ProbeInput) (port.ProbeResult, error) {
	sampleSize := in.SampleSize
	if sampleSize <= 0 {
		sampleSize = 5 // 真实直测比 RAG 快（不爬网），可以多采样
	}

	// 多样化问法（避免 AI 缓存返回相同答案，提高统计有效性）
	questions := p.generateQuestions(in.Keyword, sampleSize)

	// 所有品牌名（主品牌 + 别名）
	allBrandNames := append([]string{in.BrandName}, in.Aliases...)

	mentionCount := 0
	positionSum := 0
	sentimentPos, sentimentNeg := 0, 0
	competitorMentions := make(map[string]int)
	var allAnswers []string

	for i := 0; i < sampleSize; i++ {
		question := questions[i%len(questions)] // 轮换问法

		// 直接问真实 AI 引擎（不加系统提示词伪装，让 AI 凭真实理解回答）
		messages := []port.ChatMessage{
			{Role: "user", Content: question},
		}
		answer, err := p.aiGen.ChatStream(ctx, "", in.EngineName, messages, nil)
		if err != nil || strings.TrimSpace(answer) == "" {
			continue
		}
		allAnswers = append(allAnswers, fmt.Sprintf("【问：%s】\n%s", question, answer))

		// LLM 解析这条回答里的品牌提及
		analysis := p.llmAnalyzeMention(ctx, answer, in.BrandName, allBrandNames, in.Competitors, in.EngineName)
		if analysis.Mentioned {
			mentionCount++
			if analysis.Position > 0 {
				positionSum += analysis.Position
			}
		}
		if analysis.Sentiment == "positive" {
			sentimentPos++
		} else if analysis.Sentiment == "negative" {
			sentimentNeg++
		}
		for comp, cnt := range analysis.CompetitorMentions {
			competitorMentions[comp] += cnt
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
	rawSample := ""
	if len(allAnswers) > 0 {
		rawSample = strings.Join(allAnswers, "\n\n---\n\n")
		rawSample = truncateForGeo(rawSample, 2000)
	}

	return port.ProbeResult{
		SampleCount:          sampleSize,
		MentionCount:         mentionCount,
		MentionRate:          mentionRate,
		AvgPosition:          avgPos,
		Sentiment:            sentiment,
		Competitors:          competitorMentions,
		RawSample:            rawSample,
		SourceCount:          0, // 真实直测不爬网，无检索源
		BrandAppearanceCount: mentionCount,
	}, nil
}

// generateQuestions 生成多样化的问法（避免 AI 因缓存返回相同答案）。
// 用不同角度问同一个意图，让 AI 展现真实理解能力。
func (p *DirectProbe) generateQuestions(keyword string, count int) []string {
	// 基础问法 + 多角度变体
	templates := []string{
		keyword,                                   // 原始关键词直接问
		fmt.Sprintf("推荐%s", keyword),             // 推荐角度
		fmt.Sprintf("%s哪个好？", keyword),          // 比较角度
		fmt.Sprintf("我想了解%s，有什么推荐？", keyword), // 咨询角度
		fmt.Sprintf("%s相关的平台或工具有哪些？", keyword), // 列举角度
	}
	// 如果要的次数比模板多，循环用
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		result = append(result, templates[i%len(templates)])
	}
	return result
}

// llmAnalyzeMention 用 LLM 解析回答里的品牌提及情况（复用 RagProbe 的同款逻辑）。
// 为了 DirectProbe 自包含，这里独立实现一份（与 RagProbe 的解析逻辑一致）。
func (p *DirectProbe) llmAnalyzeMention(ctx context.Context, answer, brandName string, aliases, competitors []string, llmConfigName string) mentionAnalysis {
	brandNames := strings.Join(append([]string{brandName}, aliases...), "、")
	compNames := strings.Join(competitors, "、")
	systemPrompt := "你是品牌可见度分析专家。分析一段 AI 回答里指定品牌的提及情况。只返回 JSON，不要解释。"
	userPrompt := fmt.Sprintf(`分析以下 AI 回答，评估品牌「%s」的可见度：

1. mentioned：回答里是否提到该品牌（含简称/别名/指代）
2. position：如果 AI 给出了推荐/列举，该品牌排第几？未被提及则填0。
3. sentiment：回答对该品牌的整体评价倾向（positive/neutral/negative）
4. competitors：回答里提到了哪些竞品（从列表里匹配：%s）

AI 回答内容：
%s

只返回 JSON：{"mentioned":true,"position":1,"sentiment":"positive","competitors":["竞品A"]}`, brandNames, compNames, answer)

	messages := []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	convID := fmt.Sprintf("direct-analyze-%d", time.Now().UnixNano())
	resp, err := p.aiGen.ChatStream(ctx, convID, llmConfigName, messages, nil)
	if err != nil {
		return p.fallbackStringMatch(answer, brandName, aliases, competitors)
	}
	return parseMentionJSON(resp, brandName, competitors)
}

// fallbackStringMatch LLM 解析失败时的降级：字符串匹配。
func (p *DirectProbe) fallbackStringMatch(answer, brandName string, aliases, competitors []string) mentionAnalysis {
	ma := mentionAnalysis{CompetitorMentions: make(map[string]int)}
	lower := strings.ToLower(answer)
	for _, name := range append([]string{brandName}, aliases...) {
		if name != "" && strings.Contains(lower, strings.ToLower(name)) {
			ma.Mentioned = true
			break
		}
	}
	for _, comp := range competitors {
		if comp != "" && strings.Contains(lower, strings.ToLower(comp)) {
			ma.CompetitorMentions[comp]++
		}
	}
	return ma
}

// 编译期断言：实现 port.AIEngineProbe。
var _ port.AIEngineProbe = (*DirectProbe)(nil)
