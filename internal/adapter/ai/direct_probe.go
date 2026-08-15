package ai

import (
	"context"
	"fmt"
	"strings"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// DirectProbe 是 port.AIEngineProbe 的"真实 AI 引擎直测"实现。
//
// 核心区别（vs AgentProbe）：
//   - AgentProbe：Agent 自主搜索全网→综合回答→解析（模拟引擎行为，估算引用率）
//   - DirectProbe：直接调真实 AI 引擎 API 问问题→收集真实回答→解析（真实测量）
//
// DirectProbe 才是"真实引用率"——它测量的是：真实 AI 引擎（豆包/Kimi/文心/DeepSeek）
// 在被问到这个关键词时，会不会提到你的品牌。
//
// 引擎接入的关键事实：豆包/Kimi/文心/DeepSeek 均提供 OpenAI 兼容 API，
// 项目已支持在 LLMConfig 中配置任意 OpenAI 兼容端点。因此：
//   - llm_configs 表就是"引擎注册表"（EngineName = LLMConfigName）
//   - 复用 port.AIGenerator.ChatStream（按 llmConfigName 选引擎 + TTL 缓存），
//     无需为每个引擎写专属 SDK 适配器
//
// 解析复用 analyzeMention（LLM 客观列品牌 + Go 代码匹配，消除确认偏误），
// 解析用 AnalyzerName 指定的引擎（默认 default——与直测引擎分离，避免自判）。
type DirectProbe struct {
	aiGen      port.AIGenerator
	questioner *ProbeQuestioner
}

// NewDirectProbe 创建真实引擎直测适配器。
func NewDirectProbe(ai port.AIGenerator) *DirectProbe {
	return &DirectProbe{aiGen: ai, questioner: NewProbeQuestioner()}
}

// Probe 对一个关键词直接问真实 AI 引擎，采样 N 次，解析品牌提及。
func (p *DirectProbe) Probe(ctx context.Context, in port.ProbeInput) (port.ProbeResult, error) {
	sampleSize := in.SampleSize
	if sampleSize <= 0 {
		sampleSize = 3 // 真实直测比 Agent 搜索快（不爬网），可多采样
	}

	// 采样矩阵·问法维度：优先用预生成问法池（LLM 生成，引擎分片隔离防缓存）；
	// 池为空（生成失败/未注入）→ 模板问法兜底（随机打乱）
	questions := in.Questions
	if len(questions) == 0 {
		questions = p.questioner.Questions(in.Keyword, sampleSize, in.LocalContext)
	}
	// 真实性红线（采样层兜底）：问法池若仍含品牌/竞品名（上游校验遗漏路径），
	// 用模板中性问法替换——点名问法会诱导 AI 复述名字（回声提及），监测失真
	forbidden := append([]string{in.BrandName}, in.Competitors...)
	questions = replaceForbiddenQuestions(questions, forbidden, in.Keyword, sampleSize, in.LocalContext, p.questioner)

	// 所有品牌名（主品牌 + 别名）
	allBrandNames := append([]string{in.BrandName}, in.Aliases...)

	mentionCount := 0
	positionSum := 0
	sentimentPos, sentimentNeg := 0, 0
	firstPick := 0                                   // 被提及且位次=1 的采样数（首选率分子）
	degradedSamples := 0                             // 解析降级采样数（字符串匹配兜底）
	competitorMentions := make(map[string]int)
	compSentVotes := make(map[string]map[string]int) // 竞品情感跨采样投票（与自家多数投票同口径）
	var allOtherBrands []string // 竞品沉淀：跨采样收集"回答中出现的其他品牌"
	sourceSet := make(map[string]bool) // P5-01：跨采样合并来源（去重）
	var allAnswers []string
	totalSamples := 0

	for i := 0; i < sampleSize; i++ {
		question := questions[i%len(questions)] // 轮换问法

		// 直接问真实 AI 引擎（不加系统提示词伪装，让 AI 凭真实理解回答）
		messages := []port.ChatMessage{
			{Role: "user", Content: question},
		}
		answer, err := p.aiGen.ChatStream(ctx, "", in.EngineName, messages, nil)
		if err != nil || strings.TrimSpace(answer) == "" {
			continue // 单次失败降级跳过
		}
		answer = strings.TrimSpace(answer)
		// 过滤模型推理过程的 think 标签（展示与解析都不应看到思考过程）
		answer = pkg.StripThinkTags(answer)
		if strings.TrimSpace(answer) == "" {
			continue
		}
		totalSamples++
		allAnswers = append(allAnswers, fmt.Sprintf("【问：%s】\n%s", question, answer))

		// 解析提及（用 AnalyzerName 指定引擎，默认 default——与直测引擎分离，避免自判）
		analysis := analyzeMention(ctx, p.aiGen, answer, in.BrandName, allBrandNames, in.Competitors, in.AnalyzerName)
		if analysis.Mentioned {
			mentionCount++
			if analysis.Position > 0 {
				positionSum += analysis.Position
			}
			if analysis.Position == 1 {
				firstPick++ // 首选：该次回答中最先推荐的就是你
			}
		}
		if analysis.Degraded {
			degradedSamples++
		}
		if analysis.Sentiment == "positive" {
			sentimentPos++
		} else if analysis.Sentiment == "negative" {
			sentimentNeg++
		}
		for comp, cnt := range analysis.CompetitorMentions {
			competitorMentions[comp] += cnt
		}
		// 竞品情感聚合：跨采样投票（多数观点代表整体倾向，与自家口径一致）
		for comp, sent := range analysis.CompetitorSentiments {
			if compSentVotes[comp] == nil {
				compSentVotes[comp] = make(map[string]int)
			}
			compSentVotes[comp][entity.NormalizeSentiment(sent)]++
		}
		// 竞品沉淀：收集回答中自然出现的其他品牌（跨采样去重见 dedupeOtherBrands）
		allOtherBrands = append(allOtherBrands, analysis.OtherBrands...)
		// P5-01：合并来源（去重）
		for _, s := range analysis.Sources {
			sourceSet[s] = true
		}
	}

	// 聚合统计（与 AgentProbe 同口径）
	mentionRate := 0.0
	if totalSamples > 0 {
		mentionRate = float64(mentionCount) / float64(totalSamples)
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
	// P5-01：来源列表（去重保序）+ 自营站引用计数（归因）
	sources := make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}

	return port.ProbeResult{
		SampleCount:          totalSamples,
		MentionCount:         mentionCount,
		MentionRate:          mentionRate,
		AvgPosition:          avgPos,
		Sentiment:            sentiment,
		Competitors:          competitorMentions,
		CompetitorSentiments: voteCompetitorSentiments(compSentVotes),
		OtherBrands:          dedupeOtherBrands(allOtherBrands),
		RawSample:            rawSample,
		SourceCount:          totalSamples,
		BrandAppearanceCount: mentionCount,
		Confidence:           probeConfidenceDegraded(computeProbeConfidence(rawSample, totalSamples), degradedSamples),
		Sources:              sources,
		SelfSourceCount:      countSelfSources(sources, in.SelfBaseDomain),
		FirstPickCount:       firstPick,
		SemanticDegraded:     degradedSamples > 0,
	}, nil
}

// probeConfidenceDegraded 解析降级时置信度打折（语义维度缺失，结果可信度下降）。
func probeConfidenceDegraded(confidence float64, degradedSamples int) float64 {
	if degradedSamples > 0 {
		return confidence * 0.7
	}
	return confidence
}

// computeProbeConfidence 按信息量计算置信度（真实直测：回答长度 + 采样成功率）。
func computeProbeConfidence(rawSample string, sampleCount int) float64 {
	answerLen := len([]rune(rawSample))
	// 复用 entity 层算法（回答长度 + 采样成功数）
	score := 0.6
	if answerLen > 500 {
		score += 0.3
	} else if answerLen > 0 {
		score += 0.3 * float64(answerLen) / 500.0
	}
	if sampleCount > 0 {
		score += 0.1
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}
