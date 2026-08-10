package structured

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
)

// ---- InferSchemaType 推断测试 ----

func TestInferSchemaType(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    entity.SchemaType
	}{
		{"FAQ 中文标记", "问：装修工期多久？\n答：一般 60-120 天。", entity.SchemaFAQPage},
		{"FAQ 英文标记", "Q1: What is the price?\nA1: 25万起。", entity.SchemaFAQPage},
		{"Product 中文", "产品价格 25 万起，型号 M2.5 参数如下。", entity.SchemaProduct},
		{"HowTo 中文", "教程：如何选择装修公司，第一步……", entity.SchemaHowTo},
		{"Organization 中文", "关于我们：公司成立于 2010 年，团队 200 人。", entity.SchemaOrganization},
		{"默认 Article", "这是一篇普通的技术文章，讲的是优化方法论。", entity.SchemaArticle},
		{"FAQ 优先级高于 Product", "常见问题：产品价格多少？答：25 万。", entity.SchemaFAQPage},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := inferSchemaType(c.content); got != c.want {
				t.Errorf("inferSchemaType(%q) = %s, want %s", c.content, got, c.want)
			}
		})
	}
}

// ---- JSON-LD 生成测试 ----

const faqContent = `# 装修公司怎么选

问：装修工期一般多久？
答：根据项目规模 60-120 天，2026 年行业平均 85 天。

问：价格怎么算？
答：套餐价 25 万起，实测达标率 98%。`

func TestGenerateJSONLD_FAQPage(t *testing.T) {
	uc := NewStructuredDataUseCase()
	sd, err := uc.GenerateJSONLD(context.Background(), StructuredDataInput{
		Title:   "装修公司怎么选",
		Content: faqContent,
		URL:     "https://example.com/choose",
	})
	if err != nil {
		t.Fatalf("GenerateJSONLD error: %v", err)
	}
	if sd.Type != entity.SchemaFAQPage {
		t.Fatalf("type = %s, want FAQPage", sd.Type)
	}
	if sd.FAQPair != 0 {
		t.Errorf("FAQPair 字段当前未填充，应保持 0，实际 %d", sd.FAQPair)
	}
	// 校验 JSON 合法 + 关键字段
	var parsed map[string]any
	if err := json.Unmarshal([]byte(sd.JSONLD), &parsed); err != nil {
		t.Fatalf("JSONLD 不是合法 JSON: %v\n%s", err, sd.JSONLD)
	}
	if parsed["@type"] != "FAQPage" {
		t.Errorf("@type = %v, want FAQPage", parsed["@type"])
	}
	mainEntity, ok := parsed["mainEntity"].([]any)
	if !ok || len(mainEntity) != 2 {
		t.Fatalf("mainEntity 应为 2 个问答对，实际 %v", parsed["mainEntity"])
	}
	if parsed["url"] != "https://example.com/choose" {
		t.Errorf("url 未写入: %v", parsed["url"])
	}
}

func TestGenerateJSONLD_Article(t *testing.T) {
	uc := NewStructuredDataUseCase()
	sd, err := uc.GenerateJSONLD(context.Background(), StructuredDataInput{
		Title:       "GEO 优化方法论",
		Content:     "# GEO 优化方法论\n\n这是一篇讲 GEO 方法论的文章，内容覆盖优化策略与实践经验。",
		Author:      "张三",
		PublishDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("GenerateJSONLD error: %v", err)
	}
	if sd.Type != entity.SchemaArticle {
		t.Fatalf("type = %s, want Article", sd.Type)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(sd.JSONLD), &parsed); err != nil {
		t.Fatalf("JSONLD 非法: %v", err)
	}
	author, ok := parsed["author"].(map[string]any)
	if !ok || author["name"] != "张三" {
		t.Errorf("author 未正确写入: %v", parsed["author"])
	}
	if parsed["datePublished"] != "2026-08-01" {
		t.Errorf("datePublished = %v, want 2026-08-01", parsed["datePublished"])
	}
}

func TestGenerateJSONLD_FAQPageFallbackToArticle(t *testing.T) {
	uc := NewStructuredDataUseCase()
	// 类型推断命中 FAQ 但内容里没有可提取的问答结构 → 降级 Article
	sd, err := uc.GenerateJSONLD(context.Background(), StructuredDataInput{
		Title:   "常见问题汇总",
		Content: "常见问题：这里只是提到了常见问题这个词但没有问答结构。",
	})
	if err != nil {
		t.Fatalf("GenerateJSONLD error: %v", err)
	}
	if sd.Type != entity.SchemaArticle {
		t.Errorf("无问答结构时应降级 Article，实际 %s", sd.Type)
	}
}

func TestGenerateJSONLD_Validation(t *testing.T) {
	uc := NewStructuredDataUseCase()
	if _, err := uc.GenerateJSONLD(context.Background(), StructuredDataInput{Title: "", Content: "x"}); err == nil {
		t.Error("空标题应报错")
	}
	if _, err := uc.GenerateJSONLD(context.Background(), StructuredDataInput{Title: "t", Content: ""}); err == nil {
		t.Error("空内容应报错")
	}
}

// ---- 纯函数测试 ----

func TestExtractFAQPairs(t *testing.T) {
	content := `问：工期多久？
答：60-120 天。

问：价格多少？
答：25 万起。`
	pairs := extractFAQPairs(content)
	if len(pairs) != 2 {
		t.Fatalf("应提取 2 个问答对，实际 %d", len(pairs))
	}
	if pairs[0].Question != "工期多久？" || !strings.Contains(pairs[0].Answer, "60-120") {
		t.Errorf("问答对 1 解析错误: %+v", pairs[0])
	}
	// 无问答结构的内容
	if got := extractFAQPairs("普通内容没有问答。"); len(got) != 0 {
		t.Errorf("普通内容不应提取出问答对: %v", got)
	}
}

func TestExtractHowToSteps(t *testing.T) {
	content := `步骤1：确定预算
步骤2：对比三家
1. 签合同
2. 验收`
	steps := extractHowToSteps(content)
	if len(steps) != 4 {
		t.Fatalf("应提取 4 个步骤，实际 %d: %v", len(steps), steps)
	}
	if steps[0] != "确定预算" {
		t.Errorf("第一步 = %q, want 确定预算", steps[0])
	}
}

func TestExtractDescription(t *testing.T) {
	desc := extractDescription("# 标题\n\n第一段内容，长度不限。\n\n第二段不取。")
	if desc != "第一段内容，长度不限。" {
		t.Errorf("描述应取第一段: %q", desc)
	}
	// 超长截断 150 字
	long := "x" + strings.Repeat("长", 200)
	if got := extractDescription(long); len([]rune(got)) > 151 {
		t.Errorf("描述应截断到 150 字，实际 %d", len([]rune(got)))
	}
}

// ---- llms.txt 测试 ----

func TestGenerateLLMSTxt(t *testing.T) {
	uc := NewStructuredDataUseCase()
	out, err := uc.GenerateLLMSTxt(context.Background(), "装修公司官网", "提供北京装修公司推荐与对比",
		[]entity.LLMSTxtEntry{
			{URL: "https://example.com/", Title: "首页", Summary: "装修公司推荐总览"},
			{URL: "https://example.com/faq", Title: "常见问题"},
		})
	if err != nil {
		t.Fatalf("GenerateLLMSTxt error: %v", err)
	}
	if !strings.Contains(out, "# 装修公司官网") {
		t.Errorf("缺少站点标题: %s", out)
	}
	// 规范格式：Markdown 链接 [标题](URL): 描述
	if !strings.Contains(out, "[首页](https://example.com/): 装修公司推荐总览") {
		t.Errorf("条目应为 [标题](URL): 描述 格式: %s", out)
	}
	if !strings.Contains(out, "[常见问题](https://example.com/faq)") {
		t.Errorf("无摘要条目也应输出链接格式: %s", out)
	}
}
