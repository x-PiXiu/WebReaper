package entity

// SchemaType 是 JSON-LD Schema.org 结构化数据类型的枚举。
//
// 设计动机（GEO 结构化闭环）：
//   让 AI 搜索引擎高效抓取语义块的前提是"机器可读的结构化标记"。
//   这些类型是 Schema.org 开放标准（稳定、与具体平台无关），属于实体层规则。
type SchemaType string

// Schema.org 类型常量（与官方 vocabulary 对齐）。
const (
	SchemaArticle      SchemaType = "Article"      // 文章（默认）
	SchemaFAQPage      SchemaType = "FAQPage"      // 常见问题页（AI 摘要引用率最高的类型）
	SchemaProduct      SchemaType = "Product"      // 产品页
	SchemaHowTo        SchemaType = "HowTo"        // 教程/步骤
	SchemaOrganization SchemaType = "Organization" // 组织/企业页
)

// StructuredData 是一次 JSON-LD 结构化数据生成的产物。
type StructuredData struct {
	Type    SchemaType // Schema.org 类型
	JSONLD  string     // 可直接嵌入 <script type="application/ld+json"> 的 JSON-LD 文本
	FAQPair int        // FAQPage 模式下提取到的问答对数（0 表示非 FAQ 模式）
}

// IsValid 领域规则：必须包含 JSON-LD 文本。
func (s StructuredData) IsValid() bool {
	return s.JSONLD != ""
}

// LLMSTxtEntry 是 llms.txt 中的一条内容索引。
// llms.txt 是给 AI 爬虫看的站点地图（详见 llmstxt.org 规范）。
type LLMSTxtEntry struct {
	URL     string // 页面 URL
	Title   string // 页面标题
	Summary string // 一句话摘要（供 AI 快速判断相关性）
}

// LLMSTxt 是站点的 llms.txt 全文。
type LLMSTxt struct {
	Title   string
	Summary string // 站点一句话介绍
	Entries []LLMSTxtEntry
}
