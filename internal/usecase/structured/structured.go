// Package structured 提供"GEO 结构化闭环"用例：把内容加工成 AI 引擎可直接消费的
// 机器可读资产（JSON-LD 结构化数据 + llms.txt 站点索引）。
//
// 整洁架构定位：
//   - 本包全部是纯逻辑（模板 + 正则），零框架依赖，不定义 port（无需 IO）。
//   - 输入输出用纯数据结构（StructuredDataInput/entity.StructuredData）。
//   - 与内容生成（LLM）的分工：LLM 管"内容好不好"，本包管"结构 AI 认不认"。
//   - 将来若要换成 LLM 生成 JSON-LD，只需在 adapter 层加一个实现，用例层不变。
package structured

import (
	"context"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
)

// StructuredDataInput 是 JSON-LD 生成的输入。
type StructuredDataInput struct {
	Title       string    // 页面标题（必填）
	Content     string    // 正文（必填；用于类型推断、描述提取、FAQ 提取）
	URL         string    // 页面 URL（可选）
	Author      string    // 作者（可选）
	BrandName   string    // 品牌/组织名（可选；Organization/Product 用）
	PublishDate time.Time // 发布日期（可选）
	ForceArticle bool     // 固定为 Article 类型（公开文章页用——避免"套餐/价格"等词误判 Product）
	Store       *entity.StoreLocation // 门店信息（可选；本地生活 P0：提供时输出 @graph 双节点
	                                  // [Article/FAQPage, LocalBusiness]——地址/电话/营业时间/坐标
	                                  // 是本地搜索的核心结构化信号）
}

// StructuredDataUseCase 是结构化数据生成用例。
type StructuredDataUseCase struct{}

// NewStructuredDataUseCase 创建结构化用例（无状态，可安全共享）。
func NewStructuredDataUseCase() *StructuredDataUseCase {
	return &StructuredDataUseCase{}
}

// GenerateJSONLD 生成 JSON-LD 结构化数据。
// 类型由内容特征自动推断（FAQPage > Product > HowTo > Organization > Article）。
func (uc *StructuredDataUseCase) GenerateJSONLD(ctx context.Context, in StructuredDataInput) (entity.StructuredData, error) {
	if strings.TrimSpace(in.Title) == "" {
		return entity.StructuredData{}, fmt.Errorf("title is required")
	}
	if strings.TrimSpace(in.Content) == "" {
		return entity.StructuredData{}, fmt.Errorf("content is required")
	}
	return buildJSONLD(jsonldInput{
		Title:        strings.TrimSpace(in.Title),
		Content:      in.Content,
		URL:          strings.TrimSpace(in.URL),
		Author:       strings.TrimSpace(in.Author),
		BrandName:    strings.TrimSpace(in.BrandName),
		PublishDate:  in.PublishDate,
		ForceArticle: in.ForceArticle,
		Store:        in.Store,
	})
}

// InferSchemaType 暴露类型推断（供 handler/外部直接使用）。
func (uc *StructuredDataUseCase) InferSchemaType(content string) entity.SchemaType {
	return inferSchemaType(content)
}

// GenerateLLMSTxt 生成 llms.txt 全文（llmstxt.org 规范）。
//
// 格式（对齐 llmstxt.org）：
//   # 站点名（H1，唯一必需）
//   > 一句话摘要（blockquote）
//   [标题](URL): 一句话描述    ← 条目必须是 Markdown 链接格式
func (uc *StructuredDataUseCase) GenerateLLMSTxt(ctx context.Context, siteTitle, siteSummary string, entries []entity.LLMSTxtEntry) (string, error) {
	var sb strings.Builder
	sb.WriteString("# " + siteTitle + "\n\n")
	if siteSummary != "" {
		sb.WriteString("> " + siteSummary + "\n\n")
	}
	for _, e := range entries {
		title := e.Title
		if title == "" {
			title = e.URL
		}
		line := "[" + title + "](" + e.URL + ")"
		if e.Summary != "" {
			line += ": " + e.Summary
		}
		sb.WriteString(line + "\n")
	}
	return sb.String(), nil
}
