package geo

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// RuleScorer 是 port.GEOScorer 的"免费规则评分"实现。
//
// 设计动机（与 LLMGEOScorer 的职责分工）：
//   - LLMGEOScorer（adapter/ai）：五维深度评审，烧 token，适合"内容最终定级"。
//   - RuleScorer（本文件）：纯正则 + 关键词计数，零 LLM 调用，免费、确定、可单测。
//     适合"优化前后对比"、"内容快筛"这类高频调用场景。
//
// 分层校验思想（与图编排 coverage_validator/quality_validator 同构）：
//   规则快筛（免费，可测）→ 不达标才值得烧 token 做 LLM 深度评审。
//
// 维度与 LLMGEOScorer 完全对齐（Authority/Specificity/Structure/Uniqueness/Recency），
// 保证两套评分器可以互换、可比。各维度独立封顶 100，Total 取平均。
type RuleScorer struct{}

// 编译期断言：实现 port.GEOScorer。
var _ port.GEOScorer = (*RuleScorer)(nil)

// 预编译正则（性能：评分可能高频调用，避免每次编译）。
var (
	// authorityDataPatterns 数据/证据模式（权威性：具体数据支撑）
	authorityDataPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\d+%`),
		regexp.MustCompile(`\d+(\.\d+)?\s*(万|千|亿|元|人|家|年)`),
		regexp.MustCompile(`\d{4}\s*年`),
		regexp.MustCompile(`据\s*\S+(\s*统计|\s*数据|\s*报道|\s*介绍)`),
		regexp.MustCompile(`来源[：:]`),
		regexp.MustCompile(`数据显示|研究报告|调查报告|白皮书`),
	}
	// authoritySourcePatterns 引用来源模式（注意：\S+ 而非 \w+——中文名称不是 ASCII \w）
	authoritySourcePatterns = []*regexp.Regexp{
		regexp.MustCompile(`来源[：:]\s*\S+`),
		regexp.MustCompile(`\[\d+\]`),
		regexp.MustCompile(`\([^)]*\d{4}\)`),
	}
	// authorityEvidencePatterns 证据/案例模式（逐词独立——每个词命中各计一次）
	authorityEvidencePatterns = []*regexp.Regexp{
		regexp.MustCompile(`例如`),
		regexp.MustCompile(`比如`),
		regexp.MustCompile(`案例`),
		regexp.MustCompile(`举例`),
		regexp.MustCompile(`实例`),
		regexp.MustCompile(`实测`),
		regexp.MustCompile(`(?i)for\s+example|for\s+instance|case\s+study`),
	}
	// specificityPatterns 具体性模式（数字/细节/可验证信息）
	specificityPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\d+`),                  // 任何数字
		regexp.MustCompile(`[A-Za-z]{2,}[-–]?\d+`), // 型号/版本号（如 M2.5、ISO9001）
		regexp.MustCompile(`\d{4}-\d{2}-\d{2}`),    // 日期
		regexp.MustCompile(`(?i)(版本|型号|规格|参数|配置)\s*[：:]?\s*\S+`),
	}
	// recencyPatterns 时效性模式
	recencyPatterns = []*regexp.Regexp{
		regexp.MustCompile(`20(2[3-9]|3\d)\s*年`), // 2023-2039 年
		regexp.MustCompile(`(最新|今年|本月|本季度|刚刚|近期|当前)`),
		regexp.MustCompile(`(?i)(202[3-9]|203\d)`),
	}
	// uniquenessPatterns 独特性模式（逐词独立正则——每个词命中各计一次，避免合并后只算一次）
	uniquenessPatterns = []*regexp.Regexp{
		regexp.MustCompile(`独特`),
		regexp.MustCompile(`创新`),
		regexp.MustCompile(`首创|首家|独家`),
		regexp.MustCompile(`差异化|区别于|不同于`),
		regexp.MustCompile(`相比|对比|竞品`),
		regexp.MustCompile(`(?i)unique|innovative`),
		regexp.MustCompile(`(?i)first`),
		regexp.MustCompile(`(?i)unlike|compared\s+to|differentiated`),
	}
	// professionalWords 专业术语（权威性加分）
	professionalWords = []string{
		`优化`, `策略`, `分析`, `评估`, `方案`, `方法论`, `体系`, `流程`,
		`optimize`, `strategy`, `analysis`, `framework`, `methodology`,
	}
	// connectorWords 逻辑连接词（清晰度，并入 Structure 的段落质量判定）
	connectorWords = []string{
		`因此`, `所以`, `但是`, `然而`, `此外`, `另外`, `首先`, `其次`, `最后`,
		`therefore`, `however`, `moreover`, `furthermore`, `in addition`,
	}
)

// NewRuleScorer 创建免费规则评分器。
func NewRuleScorer() *RuleScorer { return &RuleScorer{} }

// Score 实现 port.GEOScorer：规则评分（不调 LLM，keyword 参数暂未参与规则，
// 保留以匹配接口签名——关键词相关维度（匹配度）由 LLM 深评负责）。
func (s *RuleScorer) Score(ctx context.Context, content string, keyword string) (entity.GEOScore, error) {
	sc := entity.GEOScore{
		Authority:   s.scoreAuthority(content),
		Specificity: s.scoreSpecificity(content),
		Structure:   s.scoreStructure(content),
		Uniqueness:  s.scoreUniqueness(content),
		Recency:     s.scoreRecency(content),
	}
	sc.Total = (sc.Authority + sc.Specificity + sc.Structure + sc.Uniqueness + sc.Recency) / 5
	return sc, nil
}

// scoreAuthority 权威性：数据支撑(封顶30) + 引用来源(封顶30) + 专业词(封顶20) + 证据案例(封顶20)。
func (s *RuleScorer) scoreAuthority(content string) float64 {
	score := 0.0

	for _, re := range authorityDataPatterns {
		if re.MatchString(content) {
			score += 5.0
		}
	}
	if score > 30 {
		score = 30
	}

	sourceCount := 0
	for _, re := range authoritySourcePatterns {
		sourceCount += len(re.FindAllString(content, -1))
	}
	score += float64(sourceCount) * 5.0
	if score > 30 {
		score = 30
	}

	termCount := 0
	for _, w := range professionalWords {
		if strings.Contains(content, w) {
			termCount++
		}
	}
	termScore := float64(termCount) * 4.0
	if termScore > 20 {
		termScore = 20
	}
	score += termScore

	evidenceScore := 0.0
	for _, re := range authorityEvidencePatterns {
		if re.MatchString(content) {
			evidenceScore += 5.0
		}
	}
	if evidenceScore > 20 {
		evidenceScore = 20
	}
	score += evidenceScore

	if score > 100 {
		score = 100
	}
	return score
}

// scoreSpecificity 具体性：数字密度 + 型号/参数 + 日期。按"命中模式种数"计分，
// 避免大段数字重复刷分（去重后按种数 ×20）。
func (s *RuleScorer) scoreSpecificity(content string) float64 {
	if strings.TrimSpace(content) == "" {
		return 0
	}
	score := 0.0

	// 数字密度：数字字符占比（0-5% 线性映射到 0-40 分）
	digitCount := 0
	totalRunes := utf8.RuneCountInString(content)
	for _, r := range content {
		if r >= '0' && r <= '9' {
			digitCount++
		}
	}
	if totalRunes > 0 {
		ratio := float64(digitCount) / float64(totalRunes)
		if ratio > 0.05 {
			ratio = 0.05
		}
		score += (ratio / 0.05) * 40.0
	}

	// 型号/规格/日期：每种模式 +20（封顶 60）
	extra := 0
	for _, re := range specificityPatterns[1:] {
		if re.MatchString(content) {
			extra += 20
		}
	}
	score += float64(extra)
	if score > 60 {
		score = 60
	}
	if score > 100 {
		score = 100
	}
	return score
}

// scoreStructure 结构化：标题(封顶30) + 列表(封顶20) + 段落比例(封顶25) + 连接词(封顶25)。
func (s *RuleScorer) scoreStructure(content string) float64 {
	if strings.TrimSpace(content) == "" {
		return 0
	}
	score := 0.0

	headings := strings.Count(content, "#")
	if headings > 0 {
		score += 15.0
		if headings >= 3 {
			score += 10.0
		}
		if headings >= 5 {
			score += 5.0
		}
	}
	if score > 30 {
		score = 30
	}

	if strings.Contains(content, "- ") || strings.Contains(content, "* ") {
		score += 10.0
	}
	if strings.Contains(content, "1. ") || strings.Contains(content, "1、") {
		score += 10.0
	}

	paragraphs := strings.Split(content, "\n\n")
	wellStructured := 0
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" && utf8.RuneCountInString(p) <= 500 {
			wellStructured++
		}
	}
	if len(paragraphs) > 0 {
		score += (float64(wellStructured) / float64(len(paragraphs))) * 15.0
	}

	connectorCount := 0
	for _, w := range connectorWords {
		if strings.Contains(content, w) {
			connectorCount++
		}
	}
	connectorScore := float64(connectorCount) * 5.0
	if connectorScore > 25 {
		connectorScore = 25
	}
	score += connectorScore

	if score > 100 {
		score = 100
	}
	return score
}

// scoreUniqueness 独特性：差异化词逐个计数（每个词 +20，封顶 100）。
func (s *RuleScorer) scoreUniqueness(content string) float64 {
	score := 0.0
	for _, re := range uniquenessPatterns {
		if re.MatchString(content) {
			score += 20.0
		}
	}
	if score > 100 {
		score = 100
	}
	return score
}

// scoreRecency 时效性：近期年份(封顶60) + 时效词(封顶40)。
func (s *RuleScorer) scoreRecency(content string) float64 {
	score := 0.0
	if recencyPatterns[0].MatchString(content) {
		score += 30.0
	}
	if recencyPatterns[2].MatchString(content) {
		score += 30.0
	}
	if recencyPatterns[1].MatchString(content) {
		score += 20.0
	}
	if score > 100 {
		score = 100
	}
	return score
}
