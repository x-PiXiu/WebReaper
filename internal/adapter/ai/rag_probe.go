package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"webreaper/internal/usecase/port"
)

// RagProbe 是 port.AIEngineProbe 的 RAG 模拟实现（主监测引擎）。
//
// 核心思想：模拟真实 AI 搜索引擎的"联网检索 → 综合回答"流程。
//   真实 AI 搜索：用户提问 → AI 联网检索网页 → 用检索结果综合回答
//   RagProbe：    关键词 → WebFetcher 爬全网 → 向量化检索 topK → LLM 用 topK 综合回答 → LLM 解析提及
//
// 这把 WebReaper 的爬虫+向量+LLM 三大能力串成 GEO 监测引擎，
// 是市面 GEO 工具（只调 LLM API）没有的差异化护城河。
//
// 设计要点：
//   - 临时向量隔离：每次 Probe 用唯一 id 前缀存入 VectorStore，监测完 Delete，不污染主知识库
//   - 提及解析升级：LLM 结构化提取（识别简称/指代），替换 strings.Contains
//   - 降级链：embedding 不可用 → 跳过向量检索用原文喂 LLM；LLM 解析失败 → strings.Contains
type RagProbe struct {
	aiGen      port.AIGenerator
	embedder   port.Embedder
	vectorStore port.VectorStore
	fetcher    *WebFetcher
}

// NewRagProbe 创建 RAG 监测适配器。
// embedder/vectorStore 可为 nil（降级为"跳过向量检索，原文直接喂 LLM"）。
func NewRagProbe(ai port.AIGenerator, emb port.Embedder, vs port.VectorStore, fetcher *WebFetcher) *RagProbe {
	return &RagProbe{aiGen: ai, embedder: emb, vectorStore: vs, fetcher: fetcher}
}

// Probe 执行一次 RAG 模拟监测。
func (p *RagProbe) Probe(ctx context.Context, in port.ProbeInput) (port.ProbeResult, error) {
	sampleSize := in.SampleSize
	if sampleSize <= 0 {
		sampleSize = 3 // RAG 模式每次采样开销大（要爬+嵌入），默认 3 次
	}

	// 多租户隔离：临时向量 id 带 tenant 前缀，避免不同商户并发监测时碰撞
	tenantPrefix := in.TenantID
	if tenantPrefix == "" {
		tenantPrefix = "shared"
	}
	probeID := fmt.Sprintf("geo-probe-%s-%d-%d", tenantPrefix, rand.Int63(), rand.Intn(9999))
	var storedIDs []string // 记录存入向量库的临时 id，监测完清理
	// 清理函数（defer 调用，确保不污染主知识库）
	defer func() {
		if p.vectorStore != nil && p.vectorStore.IsAvailable() {
			for _, id := range storedIDs {
				_ = p.vectorStore.Delete(ctx, id)
			}
		}
	}()

	// 所有品牌名（主品牌 + 别名）
	allBrandNames := append([]string{in.BrandName}, in.Aliases...)

	mentionCount := 0
	positionSum := 0
	sentimentPos, sentimentNeg := 0, 0
	competitorMentions := make(map[string]int)
	var rawSample string
	totalSourceCount := 0
	totalBrandAppearance := 0
	var allAnswers []string // 收集每次采样的完整回答

	for i := 0; i < sampleSize; i++ {
		// 1. 爬取全网相关内容（每次采样重新爬，模拟 AI 搜索的实时检索）
		docs := p.fetcher.FetchAndSearch(ctx, in.Keyword, 5)
		if len(docs) == 0 {
			continue // 没爬到内容，跳过这次采样
		}
		totalSourceCount += len(docs)

		// 2. 向量化存入临时索引（向量库可用时）
		var topKDocs []WebDoc
		if p.embedder != nil && p.vectorStore != nil && p.vectorStore.IsAvailable() {
			for di, doc := range docs {
				id := fmt.Sprintf("%s-%d-%d", probeID, i, di)
				vec, eErr := p.embedder.Embed(ctx, doc.Title+" "+doc.Content)
				if eErr == nil {
					_ = p.vectorStore.Store(ctx, id, vec, map[string]string{
						"title": doc.Title, "url": doc.URL, "content": doc.Content, "probe": probeID,
					})
					storedIDs = append(storedIDs, id)
				}
			}
			// 检索 topK 最相关内容
			qVec, qErr := p.embedder.Embed(ctx, in.Keyword)
			if qErr == nil {
				results, _ := p.vectorStore.Search(ctx, qVec, 5)
				for _, r := range results {
					if r.Metadata["probe"] == probeID { // 只取本次采样的
						if c := r.Metadata["content"]; c != "" {
							topKDocs = append(topKDocs, WebDoc{Title: r.Metadata["title"], Content: c, URL: r.Metadata["url"]})
						}
					}
				}
			}
		}
		// 降级：向量库不可用时，直接用爬取的原文（取前 3 篇）
		if len(topKDocs) == 0 {
			maxDocs := len(docs)
			if maxDocs > 3 {
				maxDocs = 3
			}
			topKDocs = docs[:maxDocs]
		}

		// 3. LLM 用检索到的内容综合回答（模拟 AI 搜索引擎行为）
		contextText := buildContextFromDocs(topKDocs)
		answer, aErr := p.llmSynthesize(ctx, in.Keyword, contextText, in.EngineName)
		if aErr != nil || answer == "" {
			continue
		}
		// 收集每次采样的完整回答（不截断，供用户查看完整 AI 生成内容）
		allAnswers = append(allAnswers, fmt.Sprintf("【第%d次采样】\n%s", i+1, answer))

		// 4. LLM 结构化解析：是否提到品牌、排第几、情感、竞品
		analysis := p.llmAnalyzeMention(ctx, answer, in.BrandName, allBrandNames, in.Competitors, in.EngineName)
		if analysis.Mentioned {
			mentionCount++
			if analysis.Position > 0 {
				positionSum += analysis.Position
			}
			totalBrandAppearance += analysis.SourceAppearanceCount
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
	// 拼接所有采样的完整回答（截断到 2000 字防过长）
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
		SourceCount:          totalSourceCount,
		BrandAppearanceCount: totalBrandAppearance,
	}, nil
}

// llmSynthesize 让 LLM 用检索到的内容综合回答关键词问题（模拟 AI 搜索行为）。
func (p *RagProbe) llmSynthesize(ctx context.Context, keyword, contextText, llmConfigName string) (string, error) {
	systemPrompt := "你是一个 AI 搜索引擎。根据提供的检索到的网页内容，综合回答用户的问题。" +
		"就像真实的 AI 搜索引擎（如豆包、Kimi）那样，引用检索到的信息作答。不要说'根据检索结果'之类的话，直接给出自然的回答。"
	userPrompt := fmt.Sprintf("检索到的相关内容：\n%s\n\n用户问题：%s", contextText, keyword)
	messages := []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	return p.aiGen.ChatStream(ctx, "", llmConfigName, messages, nil)
}

// mentionAnalysis LLM 解析出的提及分析结果。
type mentionAnalysis struct {
	Mentioned             bool
	Position              int
	Sentiment             string
	CompetitorMentions    map[string]int
	SourceAppearanceCount int // 品牌在检索源里出现的文章数
}

// llmAnalyzeMention 用 LLM 结构化解析回答里的品牌提及情况（比 strings.Contains 准确）。
func (p *RagProbe) llmAnalyzeMention(ctx context.Context, answer, brandName string, aliases, competitors []string, llmConfigName string) mentionAnalysis {
	brandNames := strings.Join(append([]string{brandName}, aliases...), "、")
	compNames := strings.Join(competitors, "、")
	systemPrompt := "你是品牌可见度分析专家。分析一段 AI 回答里指定品牌的提及情况。只返回 JSON，不要解释。"
	userPrompt := fmt.Sprintf(`分析以下 AI 回答，评估品牌「%s」的可见度：

1. mentioned：回答里是否提到该品牌（含简称/别名/指代，如"这个平台""该网站"若指代该品牌也算）
2. position：如果 AI 在回答里给出了推荐/列举（如"以下是几个推荐的平台/工具..."），该品牌在推荐列表里排第几？如果只有它一个被提到则排1；如果它排在其他品牌之后，按实际位置。如果未被提及则填0。
3. sentiment：回答对该品牌的整体评价倾向（positive/neutral/negative）
4. competitors：回答里提到了哪些竞品（从列表里匹配：%s）

AI 回答内容：
%s

只返回 JSON：{"mentioned":true,"position":1,"sentiment":"positive","competitors":["竞品A"],"reason":"简要说明排名依据"}`, brandNames, compNames, answer)

	messages := []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	resp, err := p.aiGen.ChatStream(ctx, "", llmConfigName, messages, nil)
	if err != nil {
		return p.fallbackStringMatch(answer, brandName, aliases, competitors)
	}
	return parseMentionJSON(resp, brandName, competitors)
}

// fallbackStringMatch LLM 解析失败时的降级：字符串匹配。
func (p *RagProbe) fallbackStringMatch(answer, brandName string, aliases, competitors []string) mentionAnalysis {
	ma := mentionAnalysis{CompetitorMentions: make(map[string]int)}
	lower := strings.ToLower(answer)
	for _, name := range append([]string{brandName}, aliases...) {
		if name != "" && strings.Contains(lower, strings.ToLower(name)) {
			ma.Mentioned = true
			ma.Position = 1
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

// buildContextFromDocs 把多篇 WebDoc 拼成 LLM 可读的检索上下文。
func buildContextFromDocs(docs []WebDoc) string {
	var b strings.Builder
	for i, d := range docs {
		fmt.Fprintf(&b, "【来源%d】%s\n%s\n\n", i+1, d.Title, d.Content)
		if i >= 4 {
			break // 最多 5 篇，避免 context 过长
		}
	}
	return b.String()
}

// strconv 轻量包装（避免 import strconv 只为 Itoa）。
func strconv(n int) string {
	return fmt.Sprintf("%d", n)
}

// parseMentionJSON 从 LLM 返回的 JSON 解析提及分析（强容错）。
// 三层容错：① 标准 JSON 解析 → ② 提取 {...} 块再解析 → ③ 字符串提取兜底
func parseMentionJSON(s, brandName string, competitors []string) mentionAnalysis {
	ma := mentionAnalysis{CompetitorMentions: make(map[string]int)}

	// ① 先尝试直接标准 JSON 解析
	if tryParseMentionJSON(s, &ma, competitors) {
		return ma
	}

	// ② 尝试提取第一个 {...} JSON 块再解析
	jsonBlock := extractJSONBlock(s)
	if jsonBlock != "" && jsonBlock != s {
		if tryParseMentionJSON(jsonBlock, &ma, competitors) {
			return ma
		}
	}

	// ③ 字符串提取兜底（最不靠谱但能兜住）
	ma.Mentioned = extractBoolValue(s, "mentioned")
	ma.Position = extractIntValue(s, "position")
	ma.Sentiment = extractStringValue(s, "sentiment")
	if ma.Sentiment == "" {
		ma.Sentiment = "neutral"
	}
	lower := strings.ToLower(s)
	for _, comp := range competitors {
		if comp != "" && strings.Contains(lower, strings.ToLower(comp)) {
			ma.CompetitorMentions[comp]++
		}
	}
	return ma
}

// tryParseMentionJSON 用标准 encoding/json 解析，成功返回 true。
func tryParseMentionJSON(s string, ma *mentionAnalysis, competitors []string) bool {
	s = strings.TrimSpace(s)
	// 去掉 markdown 代码块包裹
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			s = strings.Join(lines, "\n")
		}
	}
	s = strings.TrimSpace(s)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return false // 不是合法 JSON
	}

	// mentioned
	if v, ok := raw["mentioned"]; ok {
		var b bool
		if json.Unmarshal(v, &b) == nil {
			ma.Mentioned = b
		}
	}
	// position
	if v, ok := raw["position"]; ok {
		var n int
		if json.Unmarshal(v, &n) == nil {
			ma.Position = n
		}
	}
	// sentiment
	if v, ok := raw["sentiment"]; ok {
		var str string
		if json.Unmarshal(v, &str) == nil {
			str = strings.ToLower(strings.TrimSpace(str))
			if str == "positive" || str == "negative" || str == "neutral" {
				ma.Sentiment = str
			}
		}
	}
	if ma.Sentiment == "" {
		ma.Sentiment = "neutral"
	}
	// competitors
	if v, ok := raw["competitors"]; ok {
		var comps []string
		if json.Unmarshal(v, &comps) == nil {
			for _, c := range comps {
				c = strings.TrimSpace(c)
				if c != "" {
					ma.CompetitorMentions[c]++
				}
			}
		}
	}
	// 如果标准解析没提取到竞品，兜底用字符串匹配
	if len(ma.CompetitorMentions) == 0 {
		lower := strings.ToLower(s)
		for _, comp := range competitors {
			if comp != "" && strings.Contains(lower, strings.ToLower(comp)) {
				ma.CompetitorMentions[comp]++
			}
		}
	}
	return true
}

// extractJSONBlock 从字符串中提取第一个 {...} JSON 块。
func extractJSONBlock(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	// 从 start 开始找匹配的 }
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

// extractBoolValue 从 JSON 字符串提取布尔值。
func extractBoolValue(s, key string) bool {
	pattern := "\"" + key + "\":"
	idx := strings.Index(strings.ToLower(s), strings.ToLower(pattern))
	if idx < 0 {
		return false
	}
	rest := s[idx+len(pattern):]
	return strings.HasPrefix(strings.TrimSpace(strings.ToLower(rest)), "true")
}

// extractIntValue 从 JSON 字符串提取整数值。
func extractIntValue(s, key string) int {
	pattern := "\"" + key + "\":"
	idx := strings.Index(strings.ToLower(s), strings.ToLower(pattern))
	if idx < 0 {
		return 0
	}
	rest := s[idx+len(pattern):]
	rest = strings.TrimLeft(rest, " \t\n\r")
	var num strings.Builder
	for _, r := range rest {
		if r >= '0' && r <= '9' {
			num.WriteRune(r)
		} else {
			break
		}
	}
	var v int
	fmt.Sscanf(num.String(), "%d", &v)
	return v
}

// extractStringValue 从 JSON 字符串提取字符串值。
func extractStringValue(s, key string) string {
	pattern := "\"" + key + "\":"
	idx := strings.Index(strings.ToLower(s), strings.ToLower(pattern))
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(pattern):]
	rest = strings.TrimLeft(rest, " \t\n\r\"")
	// 取到下一个引号
	end := strings.Index(rest, "\"")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// 编译期断言：实现 port.AIEngineProbe。
var _ port.AIEngineProbe = (*RagProbe)(nil)
