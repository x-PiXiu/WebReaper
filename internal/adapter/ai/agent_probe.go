package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// AgentProbe 是 port.AIEngineProbe 的"Agent 自主搜索"实现。
//
// 优化版设计：
//   - 默认 sampleSize=5（对齐 Monitor 默认；每次采样用不同问法——采样矩阵的"问法维度"）
//   - 问法由 ProbeQuestioner 生成并随机打乱（修复"5 次搜索词全一样"的同质化问题）
//   - 解析提及用 default 引擎（固定）；搜索同样用 default 引擎——本实现只在
//     "引擎未接入真实 LLMConfig"时被 RoutingProbe 选中（模拟估算路径），
//     in.EngineName 仅作模拟标签与问法分片种子，不能当真实配置传给 LLM 层
//     （否则 resolveLLM 找不到配置，采样全部静默失败，结果恒为 0）
type AgentProbe struct {
	aiGen      port.AIGenerator
	questioner *ProbeQuestioner
}

func NewAgentProbe(ai port.AIGenerator) *AgentProbe {
	return &AgentProbe{aiGen: ai, questioner: NewProbeQuestioner()}
}

// Probe 让 Agent 自主搜索关键词并综合回答，解析品牌提及。
func (p *AgentProbe) Probe(ctx context.Context, in port.ProbeInput) (port.ProbeResult, error) {
	sampleSize := in.SampleSize
	if sampleSize <= 0 {
		sampleSize = 5 // 默认 5 次采样（每次一问法，多样化覆盖）
	}
	// 采样矩阵·问法维度：优先用预生成问法池（LLM 生成，引擎分片隔离防缓存）；
	// 池为空（生成失败/未注入）→ 模板问法兜底（随机打乱，兼容旧行为）
	questions := in.Questions
	if len(questions) == 0 {
		questions = p.questioner.Questions(in.Keyword, sampleSize, in.LocalContext)
	}

	allBrandNames := append([]string{in.BrandName}, in.Aliases...)
	mentionCount := 0
	positionSum := 0
	sentimentPos, sentimentNeg := 0, 0
	competitorMentions := make(map[string]int)
	sourceSet := make(map[string]bool) // P5-01：跨采样合并来源（去重）
	var allAnswers []string
	totalSamples := 0 // 实际成功采样数（用于置信度）

	for i := 0; i < sampleSize; i++ {
		question := questions[i%len(questions)]
		// 构造丰富的搜索任务——不是只问一个词，而是模拟"不同用户的不同问法"
		task := p.buildSearchTask(question)

		// systemPrompt：让 Agent 知道它是 AI 搜索引擎，要联网搜索
		systemPrompt := "你是一个 AI 搜索引擎。用户提问时，你应该先调用搜索工具搜索相关信息，" +
			"然后基于搜索到的真实内容综合回答。就像豆包、Kimi 的搜索功能一样——先联网检索，再回答。" +
			"请搜索多个角度的信息后给出全面的回答。"

		// 收集 Agent 的回答
		var answerBuilder strings.Builder
		var agentErr error
		searchTools := []string{"search_crawler", "tavily_search"} // GetByNames 自动过滤未启用的
		// 模拟路径固定用 default LLM（""）：引擎未接入真实配置时，传 in.EngineName 会让 resolveLLM 失败
		err := p.aiGen.RunWithTools(ctx, "", "", task, systemPrompt,
			searchTools,
			func(evt port.ToolEvent) {
				if evt.Type == "text-delta" && evt.Text != "" {
					answerBuilder.WriteString(evt.Text)
				}
				if evt.Type == "error" && evt.Error != "" {
					agentErr = fmt.Errorf("%s", evt.Error)
				}
			},
		)
		if (err != nil || agentErr != nil || strings.TrimSpace(answerBuilder.String()) == "") && i < sampleSize-1 {
			// 搜索工具失败率高（网络/工具抖动）——同采样重试 1 次，稳定实际采样数
			answerBuilder.Reset()
			agentErr = nil
			retryErr := p.aiGen.RunWithTools(ctx, "", "", task, systemPrompt,
				searchTools,
				func(evt port.ToolEvent) {
					if evt.Type == "text-delta" && evt.Text != "" {
						answerBuilder.WriteString(evt.Text)
					}
					if evt.Type == "error" && evt.Error != "" {
						agentErr = fmt.Errorf("%s", evt.Error)
					}
				},
			)
			if retryErr != nil || agentErr != nil {
				continue
			}
		} else if err != nil || agentErr != nil {
			continue
		}
		answer := strings.TrimSpace(answerBuilder.String())
		if answer == "" {
			continue
		}
		// 过滤模型推理过程的 think 标签（展示与解析都不应看到思考过程）
		answer = pkg.StripThinkTags(answer)
		totalSamples++
		// 前缀标注实际问法（报告里"提问 + 回答"一一对应）
		allAnswers = append(allAnswers, fmt.Sprintf("【问：%s】\n%s", question, answer))

		// 解析用 AnalyzerName（默认 default 引擎），与搜索引擎分离——避免自判
		analysis := analyzeMention(ctx, p.aiGen, answer, in.BrandName, allBrandNames, in.Competitors, in.AnalyzerName)
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
		// P5-01：合并来源（去重）
		for _, s := range analysis.Sources {
			sourceSet[s] = true
		}
	}

	// 聚合统计
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
	// 置信度：基于回答长度+采样成功+搜索源数（而非固定 sampleCount/5）
	answerLen := len([]rune(rawSample))
	confidence := entity.ComputeConfidenceEx(answerLen, totalSamples, totalSamples)
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
		RawSample:            rawSample,
		SourceCount:          totalSamples,
		BrandAppearanceCount: mentionCount,
		Confidence:           confidence, // 基于信息量的置信度
		Sources:              sources,
		SelfSourceCount:      countSelfSources(sources, in.SelfBaseDomain),
	}, nil
}

// buildSearchTask 把问法包装成搜索任务——中立的搜索指令，不引导"推荐"，
// 让 Agent 自然地搜索和回答（问法本身已含本地化信息）。
func (p *AgentProbe) buildSearchTask(question string) string {
	return fmt.Sprintf("用户问：%s\n请先搜索了解这个话题，然后根据搜索到的信息给出全面回答。", question)
}

// analyzeMention 分析回答里的品牌提及——消除"确认偏误"（包级函数：AgentProbe/DirectProbe 共用）。
//
// 关键设计（修复提及率100%问题）：
//   不告诉解析 LLM "找品牌X"（那会导致确认偏误——它知道你要找什么就偏向说找到了）。
//   而是让 LLM 客观列出"回答里提到了哪些品牌/产品/平台"，然后在 Go 代码里检查目标品牌是否在列表中。
//
// 解析 LLM 用 default 引擎（P2-③：与搜索引擎分离，避免自判）。
// P5-01 扩展：同时让 LLM 列出回答中出现的来源（链接/网站名）——这是客观事实（回答里
// 确实写了哪些链接），无确认偏误风险；正则 extractURLs 做兜底。
func analyzeMention(ctx context.Context, aiGen port.AIGenerator, answer, brandName string, aliases, competitors []string, llmConfigName string) mentionAnalysis {
	systemPrompt := "你是内容分析专家。阅读一段文字，客观列出其中提到的所有品牌名、产品名、平台名、网站名。只返回 JSON。"
	userPrompt := fmt.Sprintf(`阅读以下内容，列出其中提到的所有品牌/产品/平台/工具的名称（不要遗漏，不要添加未提及的）。

返回 JSON 格式：
{"brands":[{"name":"品牌名","position":1,"sentiment":"positive"}],"sources":["来源链接或网站名"]}

position：该品牌在文中的推荐优先级排名。排在最前面/最被推荐的=1，其次=2，以此类推。
  判断依据：是否被详细展开介绍、是否被放在标题/开头、是否被推荐为重点。
  如果只是简单列举没展开，所有列举的给相同的中等排名（如都填3）。
  如果未被提及不要列入。
sentiment：文中对该品牌的评价倾向（positive/neutral/negative）。如果只是列举没有评价则为 neutral。
sources：文中明确提到的来源——链接（http/https）、网站名、平台名、文献名。未提到任何来源则返回空数组。不要推断、不要添加。

内容：
%s`, answer)

	messages := []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	convID := fmt.Sprintf("agent-analyze-%d", time.Now().UnixNano())
	resp, err := aiGen.ChatStream(ctx, convID, llmConfigName, messages, nil)
	if err != nil {
		ma := fallbackStringMatch(answer, brandName, aliases, competitors)
		ma.Sources = extractURLs(answer)
		return ma
	}

	// 在 Go 代码里检查目标品牌是否在 LLM 列出的品牌列表中（而非让 LLM 判断"是否提到X"）
	ma := matchBrandFromList(resp, brandName, aliases, competitors)
	// P5-01 兜底：LLM 没返回 sources 时用正则提取 URL（双保险）
	if len(ma.Sources) == 0 {
		ma.Sources = extractURLs(answer)
	}
	return ma
}

// matchBrandFromList 从 LLM 返回的品牌列表 JSON 中，检查目标品牌是否被提及。
// 这是消除确认偏误的关键——LLM 不知道你在找哪个品牌，它只是客观列出所有提到的名字。
func matchBrandFromList(resp, brandName string, aliases, competitors []string) mentionAnalysis {
	ma := mentionAnalysis{CompetitorMentions: make(map[string]int)}

	// 解析 LLM 返回的品牌列表
	type brandEntry struct {
		Name      string `json:"name"`
		Position  int    `json:"position"`
		Sentiment string `json:"sentiment"`
	}

	// 尝试标准 JSON 解析
	jsonBlock := extractJSONBlock(resp)
	if jsonBlock == "" {
		jsonBlock = resp
	}
	// 去掉 markdown 包裹
	jsonBlock = strings.TrimSpace(jsonBlock)
	if strings.HasPrefix(jsonBlock, "```") {
		lines := strings.Split(jsonBlock, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			jsonBlock = strings.Join(lines, "\n")
		}
	}

	var parsed struct {
		Brands  []brandEntry `json:"brands"`
		Sources []string     `json:"sources"` // P5-01：回答中提到的来源链接/网站名
	}
	if err := json.Unmarshal([]byte(jsonBlock), &parsed); err != nil {
		// JSON 解析失败，降级到字符串匹配
		ma := fallbackStringMatch(resp, brandName, aliases, competitors)
		ma.Sources = extractURLs(resp)
		return ma
	}

	// P5-01 来源解析：LLM 提取（客观事实，无偏）+ URL 正则兜底，去重
	sourceSet := make(map[string]bool)
	for _, s := range parsed.Sources {
		if s = strings.TrimSpace(s); s != "" {
			sourceSet[s] = true
		}
	}
	for _, u := range extractURLs(resp) {
		sourceSet[u] = true
	}
	for s := range sourceSet {
		ma.Sources = append(ma.Sources, s)
	}

	// 在品牌列表中查找目标品牌（模糊匹配：包含即可，不区分大小写）
	allTargetNames := append([]string{brandName}, aliases...)
	for _, entry := range parsed.Brands {
		entryName := strings.ToLower(strings.TrimSpace(entry.Name))
		// 检查是否是目标品牌
		for _, target := range allTargetNames {
			if target == "" {
				continue
			}
			targetLower := strings.ToLower(target)
			// 双向包含匹配（品牌名可能不完全一致）
			if strings.Contains(entryName, targetLower) || strings.Contains(targetLower, entryName) {
				ma.Mentioned = true
				if entry.Position > 0 {
					ma.Position = entry.Position
				}
				if entry.Sentiment != "" {
					ma.Sentiment = strings.ToLower(entry.Sentiment)
				}
				break
			}
		}
		// 检查是否是竞品
		for _, comp := range competitors {
			if comp == "" {
				continue
			}
			compLower := strings.ToLower(comp)
			if strings.Contains(entryName, compLower) || strings.Contains(compLower, entryName) {
				ma.CompetitorMentions[comp]++
			}
		}
	}

	if ma.Sentiment == "" {
		ma.Sentiment = "neutral"
	}
	return ma
}

func fallbackStringMatch(answer, brandName string, aliases, competitors []string) mentionAnalysis {
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

var _ port.AIEngineProbe = (*AgentProbe)(nil)

// mentionAnalysis LLM 解析出的提及分析结果。
// （定义在本文件：llmAnalyzeMention/matchBrandFromList 与 probe 共用。）
type mentionAnalysis struct {
	Mentioned             bool
	Position              int
	Sentiment             string
	CompetitorMentions    map[string]int
	SourceAppearanceCount int // 品牌在检索源里出现的文章数
	Sources               []string // P5-01：回答中提到的来源（链接/网站名，去重）
}

// extractURLs 从文本正则提取 http/https 链接（去重，保序）——P5-01 兜底。
// LLM 可能漏列来源，URL 是客观可正则提取的（回答里出现的链接不会"看错"）。
func extractURLs(s string) []string {
	re := regexp.MustCompile(`https?://[^\s\)\]\}，。；、""''<>]+`)
	seen := make(map[string]bool)
	var out []string
	for _, m := range re.FindAllString(s, -1) {
		u := strings.TrimRight(m, ".,;:!?")
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

// countSelfSources 统计来源中包含自营公开站域名的次数（P5-01 归因）。
// 域名匹配用"包含"（子域/路径前缀均视为自营站内容）。
func countSelfSources(sources []string, selfBaseDomain string) int {
	if selfBaseDomain == "" {
		return 0
	}
	domain := strings.ToLower(strings.TrimSpace(selfBaseDomain))
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	count := 0
	for _, s := range sources {
		if strings.Contains(strings.ToLower(s), domain) {
			count++
		}
	}
	return count
}

// extractJSONBlock 从字符串中提取第一个 {...} JSON 块（括号配平，处理嵌套）。
// （通用解析辅助：同包 geo_probe.go 的 parseGeoScoreJSON 也复用。）
func extractJSONBlock(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '{' {
			depth++
		}
		if s[i] == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:] // 不完整的 JSON，尽力返回
}
