package structured

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
)

// jsonldInput 是 JSON-LD 生成的输入（用例内部结构，纯数据）。
type jsonldInput struct {
	Title       string    // 页面标题
	Content     string    // 正文（用于生成 description 和 FAQ 提取）
	URL         string    // 页面 URL（可选）
	Author      string    // 作者（可选）
	BrandName   string    // 品牌/组织名（Organization/Product 用）
	PublishDate time.Time // 发布日期（可选）
}

// buildJSONLD 按类型构建 JSON-LD 文本（纯函数，可单测；不调 LLM）。
//
// 设计动机（模板法 vs LLM 生成）：
//   - LLM 生成 JSON-LD 便宜但格式不稳定（数组/必填字段随意丢）。
//   - 模板法：结构化字段用输入填充，格式由代码保证，100% 合法可测。
//   - 数据从结构化输入来（标题/内容/品牌），无需 LLM——这是"结构化"闭环
//     与"内容生成"（LLM）的分工：LLM 管内容，代码管结构。
func buildJSONLD(in jsonldInput) (entity.StructuredData, error) {
	st := inferSchemaType(in.Content)
	data := map[string]any{
		"@context": "https://schema.org",
		"@type":    string(st),
	}

	switch st {
	case entity.SchemaArticle:
		data["headline"] = in.Title
		if desc := extractDescription(in.Content); desc != "" {
			data["description"] = desc
		}
		if in.Author != "" {
			data["author"] = map[string]any{"@type": "Person", "name": in.Author}
		}
		if in.PublishDate.IsZero() == false {
			data["datePublished"] = in.PublishDate.Format("2006-01-02")
		}

	case entity.SchemaFAQPage:
		pairs := extractFAQPairs(in.Content)
		if len(pairs) == 0 {
			// 内容里没有明确的问答结构，降级为 Article（避免产出空 FAQPage）
			st = entity.SchemaArticle
			data["@type"] = string(st)
			data["headline"] = in.Title
			if desc := extractDescription(in.Content); desc != "" {
				data["description"] = desc
			}
			return entity.StructuredData{
				Type:   st,
				JSONLD: marshalIndent(data),
			}, nil
		}
		mainEntity := make([]map[string]any, 0, len(pairs))
		for _, p := range pairs {
			mainEntity = append(mainEntity, map[string]any{
				"@type":          "Question",
				"name":           p.Question,
				"acceptedAnswer": map[string]any{"@type": "Answer", "text": p.Answer},
			})
		}
		data["mainEntity"] = mainEntity

	case entity.SchemaProduct:
		data["name"] = in.Title
		if desc := extractDescription(in.Content); desc != "" {
			data["description"] = desc
		}
		if in.BrandName != "" {
			data["brand"] = map[string]any{"@type": "Brand", "name": in.BrandName}
		}

	case entity.SchemaHowTo:
		data["name"] = in.Title
		steps := extractHowToSteps(in.Content)
		howToSteps := make([]map[string]any, 0, len(steps))
		for i, s := range steps {
			howToSteps = append(howToSteps, map[string]any{
				"@type": "HowToStep",
				"name":  fmt.Sprintf("第 %d 步", i+1),
				"text":  s,
			})
		}
		data["step"] = howToSteps

	case entity.SchemaOrganization:
		name := in.BrandName
		if name == "" {
			name = in.Title
		}
		data["name"] = name
		if in.URL != "" {
			data["url"] = in.URL
		}
		if desc := extractDescription(in.Content); desc != "" {
			data["description"] = desc
		}
	}

	if in.URL != "" {
		data["url"] = in.URL
	}

	return entity.StructuredData{
		Type:   st,
		JSONLD: marshalIndent(data),
	}, nil
}

// marshalIndent 序列化为带缩进的 JSON（LLM/AI 爬虫易读，也是调试友好）。
func marshalIndent(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

// faqPair 是一个问答对。
type faqPair struct {
	Question string
	Answer   string
}

// faqPairPatterns 匹配"问：…答：…"问答结构（中英文，支持序号前缀）。
// 例：Q1: 装修工期多久？ A1: 一般 60-120 天。
//     问：价格多少？ 答：套餐价 25 万起。
var (
	faqPairPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s*(?:Q\d*|问)\s*[：:]\s*(.+?)\s*\n\s*(?:A\d*|答)\s*[：:]\s*(.+)$`),
	}
)

// extractFAQPairs 从内容中提取问答对（纯函数，可单测）。
// 只匹配明确的"问/答"标记结构，避免把普通正文误判为 FAQ。
func extractFAQPairs(content string) []faqPair {
	var pairs []faqPair
	for _, re := range faqPairPatterns {
		for _, m := range re.FindAllStringSubmatch(content, -1) {
			if len(m) == 3 {
				pairs = append(pairs, faqPair{
					Question: strings.TrimSpace(m[1]),
					Answer:   strings.TrimSpace(m[2]),
				})
			}
		}
	}
	return pairs
}

// extractDescription 从正文提取一句话描述（纯函数）：跳过 markdown 标题行，
// 剥离标记后取第一段前 150 字。
func extractDescription(content string) string {
	// 跳过 markdown 标题行（# 开头），标题不算正文描述
	var body []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		body = append(body, line)
	}
	text := stripMarkdown(strings.Join(body, "\n"))
	// 取第一个段落
	first := strings.Split(text, "\n\n")[0]
	first = strings.TrimSpace(first)
	if len([]rune(first)) > 150 {
		first = string([]rune(first)[:150])
	}
	return first
}

// markdownTokens 需要剥离的 markdown 行级标记。
var markdownTokens = []string{"#", "##", "###", "####", "```", "- ", "* ", "> "}

// stripMarkdown 剥离常见 markdown 标记（纯函数）。
func stripMarkdown(s string) string {
	for _, t := range markdownTokens {
		s = strings.ReplaceAll(s, t, "")
	}
	return strings.TrimSpace(s)
}

// extractHowToSteps 从内容提取步骤（纯函数）：匹配"步骤X："/"第X步："行，
// 或有序列表项。取前 8 步，避免超长。
func extractHowToSteps(content string) []string {
	var steps []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 步骤标记：步骤1：/ 第1步：/ Step 1: / 1. 
		matched := false
		for _, re := range []*regexp.Regexp{
			regexp.MustCompile(`^步骤\s*\d+\s*[：:]\s*(.+)$`),
			regexp.MustCompile(`^第\s*\d+\s*步\s*[：:]\s*(.+)$`),
			regexp.MustCompile(`^step\s*\d+\s*[：:]\s*(.+)$`),
			regexp.MustCompile(`^\d+\.\s*(.+)$`),
		} {
			if m := re.FindStringSubmatch(trimmed); m != nil {
				steps = append(steps, strings.TrimSpace(m[1]))
				matched = true
				break
			}
		}
		if matched && len(steps) >= 8 {
			break
		}
	}
	return steps
}
