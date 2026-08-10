// Package ai 提供关键词来源策略的实现（适配器层）。
//
// 五种来源各一个策略实现，全部实现 port.KeywordSource 接口。
// 它们共享核心方法 distillWithLLM（把任意文本喂 LLM 提取关键词），
// 区别只在"蒸馏的 context 从哪来"：
//   - BrandSource：品牌信息 + 全网内容（RAG 增强）
//   - TextSource：用户粘贴的文本
//   - SeedSource：种子词拓展
//   - FileSource：文件内容（前端读出文本后走 TextSource 逻辑）
//   - WebSource：按主题爬全网 → 蒸馏
//
// 新增来源（如"从竞品分析导入"）只需加一个策略 struct，注册到工厂即可。
package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- 共享辅助 ----

// keywordDistiller 是所有策略共享的 LLM 蒸馏器。
// 把任意文本喂给 LLM，提取出关键词列表。
type keywordDistiller struct {
	aiGen port.AIGenerator
}

// distillWithLLM 把 contextText 喂给 LLM，让它提取/生成关键词。
// prompt 指导 LLM 如何蒸馏（不同来源的指导语不同）。
func (d *keywordDistiller) distillWithLLM(ctx context.Context, prompt, contextText, llmCfg string) ([]string, error) {
	systemPrompt := "你是 GEO（生成式引擎优化）关键词蒸馏专家。" +
		"从给定内容中提取/生成用户在 AI 搜索引擎里最可能搜索的关键词。每行一个，不要编号，不要解释。"
	userPrompt := fmt.Sprintf("%s\n\n内容：\n%s", prompt, contextText)
	messages := []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	convID := fmt.Sprintf("kw-distill-%d", time.Now().UnixNano())
	resp, err := d.aiGen.ChatStream(ctx, convID, llmCfg, messages, nil)
	if err != nil {
		return nil, fmt.Errorf("蒸馏 LLM 调用失败: %w", err)
	}
	return parseKeywordResponse(resp), nil
}

// parseKeywordResponse 解析 LLM 返回的关键词（去编号/markdown/说明文字）。
func parseKeywordResponse(resp string) []string {
	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "```") {
		lines := strings.Split(resp, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			resp = strings.Join(lines, "\n")
		}
	}
	var keywords []string
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "0123456789.、)-* ")
		line = strings.Trim(line, "\"'`")
		if line == "" || len([]rune(line)) < 2 {
			continue
		}
		if (strings.Contains(line, "。") || strings.Contains(line, "？")) && len([]rune(line)) > 20 {
			continue
		}
		keywords = append(keywords, line)
	}
	return keywords
}

// ---- 品牌来源策略 ----

// BrandSource 从品牌信息（定位/卖点/竞品）+ 全网内容蒸馏关键词。
type BrandSource struct {
	*keywordDistiller
	brandRepo port.BrandRepository
	webSearch port.WebSearcher
}

func NewBrandSource(ai port.AIGenerator, br port.BrandRepository, ws port.WebSearcher) *BrandSource {
	return &BrandSource{keywordDistiller: &keywordDistiller{aiGen: ai}, brandRepo: br, webSearch: ws}
}
func (s *BrandSource) SourceName() string { return "brand" }
func (s *BrandSource) Distill(ctx context.Context, in port.KeywordSourceInput) ([]string, error) {
	brand, err := s.brandRepo.FindByID(ctx, in.TenantID, in.BrandID)
	if err != nil {
		return nil, fmt.Errorf("品牌不存在: %w", err)
	}
	sellingPoints := "无"
	if len(brand.CoreSelling) > 0 {
		sellingPoints = strings.Join(brand.CoreSelling, "、")
	}
	competitors := "无"
	if len(brand.Competitors) > 0 {
		competitors = strings.Join(brand.Competitors, "、")
	}
	// RAG 增强：爬全网（可选）
	webContext := ""
	if s.webSearch != nil {
		if wc, e := s.webSearch.SearchByBrand(ctx, brand.Name, brand.Positioning, brand.Competitors); e == nil && wc != "" {
			webContext = wc
		}
	}
	contextText := fmt.Sprintf("品牌名：%s\n品牌定位：%s\n核心卖点：%s\n竞品：%s", brand.Name, brand.Positioning, sellingPoints, competitors)
	if webContext != "" {
		contextText += fmt.Sprintf("\n\n全网相关内容摘要：\n%s", truncateForGeo(webContext, 2000))
	}
	prompt := "根据以下品牌信息和全网内容，生成 20 个最合适的候选关键词（品牌词、行业热词、长尾问题词）。"
	return s.distillWithLLM(ctx, prompt, contextText, in.LLMConfig)
}

// ---- 文本来源策略 ----

// TextSource 从用户粘贴的文本蒸馏关键词。
type TextSource struct {
	*keywordDistiller
}

func NewTextSource(ai port.AIGenerator) *TextSource {
	return &TextSource{keywordDistiller: &keywordDistiller{aiGen: ai}}
}
func (s *TextSource) SourceName() string { return "text" }
func (s *TextSource) Distill(ctx context.Context, in port.KeywordSourceInput) ([]string, error) {
	if in.Text == "" {
		return nil, fmt.Errorf("文本内容不能为空")
	}
	prompt := "从以下文本中蒸馏出 15-20 个核心关键词（用户在 AI 搜索引擎里最可能搜的词，含长尾问题词）。"
	return s.distillWithLLM(ctx, prompt, truncateForGeo(in.Text, 4000), in.LLMConfig)
}

// ---- 种子词拓展策略 ----

// SeedSource 从种子词拓展相关关键词。
type SeedSource struct {
	*keywordDistiller
}

func NewSeedSource(ai port.AIGenerator) *SeedSource {
	return &SeedSource{keywordDistiller: &keywordDistiller{aiGen: ai}}
}
func (s *SeedSource) SourceName() string { return "seed" }
func (s *SeedSource) Distill(ctx context.Context, in port.KeywordSourceInput) ([]string, error) {
	if len(in.Seeds) == 0 {
		return nil, fmt.Errorf("种子词不能为空")
	}
	prompt := fmt.Sprintf("以下是种子关键词：%s。\n请拓展出 20 个相关的长尾关键词和问题词（用户在 AI 搜索引擎里可能搜的变体）。", strings.Join(in.Seeds, "、"))
	return s.distillWithLLM(ctx, prompt, strings.Join(in.Seeds, "\n"), in.LLMConfig)
}

// ---- 文件来源策略 ----

// FileSource 从文件内容蒸馏关键词。
// 注意：文件读取在前端完成（FileReader API），后端收到的是已读出的文本。
// 所以逻辑上等同于 TextSource，单独一个策略是为了语义清晰 + 后续可能加文件类型判断。
type FileSource struct {
	*keywordDistiller
}

func NewFileSource(ai port.AIGenerator) *FileSource {
	return &FileSource{keywordDistiller: &keywordDistiller{aiGen: ai}}
}
func (s *FileSource) SourceName() string { return "file" }
func (s *FileSource) Distill(ctx context.Context, in port.KeywordSourceInput) ([]string, error) {
	if in.Text == "" {
		return nil, fmt.Errorf("文件内容不能为空")
	}
	prompt := "以下是从文件中读取的内容。请蒸馏出 15-20 个核心关键词（用户在 AI 搜索引擎里最可能搜的词）。"
	return s.distillWithLLM(ctx, prompt, truncateForGeo(in.Text, 4000), in.LLMConfig)
}

// ---- 网络来源策略 ----

// WebSource 按主题爬全网，从爬到的内容蒸馏关键词。
// 复用 WebFetcher（RAG 内部组件），不新写爬虫。
type WebSource struct {
	*keywordDistiller
	fetcher *WebFetcher
}

func NewWebSource(ai port.AIGenerator, fetcher *WebFetcher) *WebSource {
	return &WebSource{keywordDistiller: &keywordDistiller{aiGen: ai}, fetcher: fetcher}
}
func (s *WebSource) SourceName() string { return "web" }
func (s *WebSource) Distill(ctx context.Context, in port.KeywordSourceInput) ([]string, error) {
	if in.Topic == "" {
		return nil, fmt.Errorf("爬取主题不能为空")
	}
	// 爬全网相关内容（5 篇）
	docs := s.fetcher.FetchAndSearch(ctx, in.Topic, 5)
	if len(docs) == 0 {
		return nil, fmt.Errorf("未爬取到相关内容，请换一个主题")
	}
	// 拼接正文作为 context
	var b strings.Builder
	for i, d := range docs {
		b.WriteString(d.Title)
		b.WriteString("：")
		b.WriteString(truncateForGeo(d.Content, 800))
		if i < len(docs)-1 {
			b.WriteString("\n---\n")
		}
	}
	prompt := fmt.Sprintf("以下是关于「%s」的全网文章摘要。请蒸馏出 20 个用户最可能搜索的关键词。", in.Topic)
	return s.distillWithLLM(ctx, prompt, b.String(), in.LLMConfig)
}

// ---- 编译期断言：确保所有策略实现 port.KeywordSource ----

var _ port.KeywordSource = (*BrandSource)(nil)
var _ port.KeywordSource = (*TextSource)(nil)
var _ port.KeywordSource = (*SeedSource)(nil)
var _ port.KeywordSource = (*FileSource)(nil)
var _ port.KeywordSource = (*WebSource)(nil)

// 确保 entity 包被引用（Keyword 相关）
var _ = entity.Keyword{}
