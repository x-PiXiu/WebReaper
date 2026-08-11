package structured

import (
	"strings"

	"webreaper/internal/domain/entity"
)

// inferSchemaType 从内容特征推断最合适的 Schema.org 类型（纯函数，可单测）。
//
// 设计动机（借鉴 geo-optimizer 的 InferSchemaType 思路，词表扩展为业务常见词）：
//   同一份内容喂给不同 AI 引擎，FAQPage 比 Article 更容易被摘要引用；
//   类型选对是结构化闭环的第一步。用中英双语关键词表做确定性推断，
//   不烧 token、结果可测。
//
// 优先级：FAQPage > Product > HowTo > Organization > Article（默认）。
func inferSchemaType(content string) entity.SchemaType {
	lower := strings.ToLower(content)

	// FAQPage 优先：内容含问答结构，是 AI 摘要引用率最高的类型
	if containsAnyWord(lower, faqKeywords) {
		return entity.SchemaFAQPage
	}
	if containsAnyWord(lower, productKeywords) {
		return entity.SchemaProduct
	}
	if containsAnyWord(lower, howToKeywords) {
		return entity.SchemaHowTo
	}
	if containsAnyWord(lower, organizationKeywords) {
		return entity.SchemaOrganization
	}
	return entity.SchemaArticle
}

// 各类型的关键词表（中英双语，命中任一即判定）。
var (
	faqKeywords = []string{
		"faq", "常见问题", "问：", "问:", "答：", "答:", "q&a", "q1.", "q2.", "q1:", "q2:", "q:", "a:",
		"frequently asked", "questions and answers",
	}
	productKeywords = []string{
		// 收紧：去掉 GEO 内容常见的"套餐/价格/优惠"——它们在 GEO 文章里频繁出现但不代表是产品页
		"产品详情", "产品参数", "规格参数", "型号", "product details",
		"price:", "buy now", "add to cart", "specification",
	}
	howToKeywords = []string{
		// 注意：不含 "方法"——"方法论"等名词会误命中，导致普通文章被判成教程
		"步骤", "如何", "教程", "指南", "操作", "入门", "流程",
		"how to", "tutorial", "step by step", "guide", "walkthrough",
	}
	organizationKeywords = []string{
		"公司", "企业", "团队", "关于我们", "资质", "简介", "组织",
		"company", "organization", "about us", "team",
	}
)

// containsAnyWord 判断文本是否包含词表中任一词。
func containsAnyWord(text string, words []string) bool {
	for _, w := range words {
		if strings.Contains(text, w) {
			return true
		}
	}
	return false
}
