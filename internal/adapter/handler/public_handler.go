// Package handler 的公开站点端点：让"自有平台被 AI 引擎引用"成立。
//
// 设计动机（GEO 引用闭环的必要环节）：
//   模型/搜索爬虫只能引用"公开可访问"的内容。本组端点把已发布（status=published）
//   的内容以 SEO/AI 友好形态暴露到公网：
//     - 文章页：服务端渲染 HTML + 内嵌 JSON-LD（结构化信号）
//     - sitemap.xml：让搜索引擎发现全部公开文章
//     - llms.txt：让 AI 爬虫快速了解站点结构（llmstxt.org）
//
// 这些路由挂在 JWT 认证之外（router 的公开段）。
package handler

import (
	"context"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/public"
	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/structured"
)

// PublicHandler 公开站点处理器。
// 只依赖 port 仓储 + 结构化用例（纯查询 + 纯生成，无写操作、无认证）。
type PublicHandler struct {
	contentRepo         port.OptimizedContentRepository
	brandRepo           port.BrandRepository             // 品牌信息（公开文章页作者署名用）
	structured          *structured.StructuredDataUseCase
	baseURL             string                       // 公开站点根地址（如 https://example.com），生成绝对 URL
	indexNowKey         string                       // IndexNow 密钥（静态注入，启动时值）
	indexNowKeyProvider func(context.Context) string // 动态读取（运行时可调配置；优先于静态值）
}

// NewPublicHandler 创建公开站点处理器。
func NewPublicHandler(repo port.OptimizedContentRepository, brandRepo port.BrandRepository, structuredUC *structured.StructuredDataUseCase, baseURL string) *PublicHandler {
	if baseURL == "" {
		baseURL = "http://localhost:8082"
	}
	return &PublicHandler{contentRepo: repo, brandRepo: brandRepo, structured: structuredUC, baseURL: baseURL}
}

// SetIndexNowKey 注入 IndexNow 密钥（启用 /public/indexnow-key.txt 托管端点）。
func (h *PublicHandler) SetIndexNowKey(key string) {
	h.indexNowKey = key
}

// SetIndexNowKeyProvider 注入运行时 key 读取函数（管理后台改配置后 key 文件即时生效）。
func (h *PublicHandler) SetIndexNowKeyProvider(fn func(context.Context) string) {
	h.indexNowKeyProvider = fn
}

// articlePageTemplate 文章页模板（自包含内联样式，无外部依赖）。
const articlePageTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
{{.JSONLD}}
<style>
  body{max-width:760px;margin:0 auto;padding:32px 20px 80px;font-family:-apple-system,'PingFang SC','Noto Sans SC',sans-serif;line-height:1.9;color:#1a1a2e}
  h1{font-size:1.8em;letter-spacing:-0.02em;line-height:1.4}
  h2{font-size:1.3em;margin-top:2em}
  h3{font-size:1.1em}
  pre{background:#f5f5f8;padding:16px;border-radius:8px;overflow-x:auto}
  code{background:#f5f5f8;padding:2px 6px;border-radius:4px;font-size:0.92em}
  blockquote{border-left:4px solid #6366f1;margin-left:0;padding-left:16px;color:#555}
  a{color:#6366f1}
  .meta{color:#8a8aa0;font-size:0.9em;margin-bottom:32px}
  .author-box{margin-top:48px;padding:20px;background:#f8f8fc;border-radius:12px}
  .author-box .author-name{font-weight:600;font-size:1.05em;color:#1a1a2e}
  .author-box .author-desc{color:#666;font-size:0.9em;margin-top:4px;line-height:1.7}
  .footer{margin-top:24px;padding-top:16px;border-top:1px solid #eee;color:#aaa;font-size:0.85em}
</style>
</head>
<body>
<article>
  <h1>{{.Title}}</h1>
  <div class="meta">{{.Meta}}</div>
  {{.ContentHTML}}
  {{if .BrandName}}
  <div class="author-box">
    <div class="author-name">本文由 {{.BrandName}} 提供</div>
    {{if .BrandDesc}}<div class="author-desc">{{.BrandDesc}}</div>{{end}}
  </div>
  {{end}}
</article>
<div class="footer">本文由 WebReaper GEO 引擎生成并发布</div>
</body>
</html>`

// GetArticleHTML GET /public/articles/:id —— 服务端渲染单篇文章（含 JSON-LD）。
func (h *PublicHandler) GetArticleHTML(c *gin.Context) {
	id := c.Param("id")
	content, err := h.contentRepo.FindPublishedByID(c.Request.Context(), id)
	if err != nil {
		c.String(http.StatusNotFound, "404 Not Found")
		return
	}

	// 标题兜底：历史内容可能没有 Title 字段（迁移前生成），从正文提取
	title := content.Title
	if title == "" {
		title = pkg.ExtractTitle(content.OptimizedText)
	}

	// 防御性清洗：存量数据（StripThinkTags 上线前生成）正文可能残留 <think> 块，
	// 公开渲染（正文/描述/JSON-LD/llms.txt）必须保证零泄漏——统一在此清洗一次。
	cleanText := pkg.StripThinkTags(content.OptimizedText)

	// 查品牌信息（作者署名 + JSON-LD author/publisher——增强 E-E-A-T 中的 Expertise/Authority）
	brandName, brandDesc := "", ""
	if h.brandRepo != nil && content.BrandID != "" {
		if brand, bErr := h.brandRepo.FindPublishedByID(c.Request.Context(), content.BrandID); bErr == nil {
			brandName = brand.Name
			brandDesc = brand.Positioning // 品牌定位作为简介
		}
	}

	// 生成 JSON-LD：公开文章页固定 Article 类型（避免"套餐/价格"等 GEO 常见词误判 Product），
	// 但保留 FAQ 结构检测（有问答的内容用 FAQPage 更利于 AI 摘要引用）。
	// 注入作者署名（品牌名）——让搜索引擎识别内容的权威来源（E-E-A-T Expertise/Authority）。
	sd, _ := h.structured.GenerateJSONLD(c.Request.Context(), structured.StructuredDataInput{
		Title:        title,
		Content:      cleanText,
		URL:          h.baseURL + "/public/articles/" + content.ID,
		Author:       brandName,
		BrandName:    brandName,
		ForceArticle: true, // 公开文章页固定 Article（GEO 内容就是文章）
	})
	jsonldTag := ""
	if sd.JSONLD != "" {
		jsonldTag = `<script type="application/ld+json">` + sd.JSONLD + `</script>`
	}

	tpl, err := template.New("article").Parse(articlePageTemplate)
	if err != nil {
		c.String(http.StatusInternalServerError, "render error")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tpl.Execute(c.Writer, gin.H{
		"Title":       title,
		"Description": truncateDescription(cleanText),
		"Meta":        fmt.Sprintf("GEO 优化内容 · 关键词可见度评分 %d", int(content.Score.Total)),
		"ContentHTML": template.HTML(public.RenderMarkdown(cleanText)),
		"JSONLD":      template.HTML(jsonldTag),
		"BrandName":   brandName,
		"BrandDesc":   brandDesc,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error")
	}
}

// GetSitemapXML GET /public/sitemap.xml —— 站点地图（让搜索引擎发现全部公开文章）。
func (h *PublicHandler) GetSitemapXML(c *gin.Context) {
	items, err := h.contentRepo.ListPublished(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "sitemap error")
		return
	}
	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(http.StatusOK, `<?xml version="1.0" encoding="UTF-8"?>`+"\n"+
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`+"\n")
	for _, it := range items {
		// lastmod 按 IndexNow FAQ：sitemap（带 lastmod）补充全量/历史变更发现
		c.Writer.WriteString(fmt.Sprintf("  <url><loc>%s/public/articles/%s</loc><lastmod>%s</lastmod></url>\n",
			h.baseURL, it.ID, it.CreatedAt.Format("2006-01-02")))
	}
	c.Writer.WriteString(`</urlset>`)
}

// GetLLMSTxt GET /public/llms.txt —— AI 爬虫站点索引（llmstxt.org 规范）。
func (h *PublicHandler) GetLLMSTxt(c *gin.Context) {
	items, err := h.contentRepo.ListPublished(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "llms.txt error")
		return
	}
	entries := make([]entity.LLMSTxtEntry, 0, len(items))
	for _, it := range items {
		entries = append(entries, entity.LLMSTxtEntry{
			URL:     h.baseURL + "/public/articles/" + it.ID,
			Title:   it.Title,
			Summary: truncateDescription(pkg.StripThinkTags(it.OptimizedText)),
		})
	}
	txt, err := h.structured.GenerateLLMSTxt(c.Request.Context(), "WebReaper 内容平台", "AI 搜索引擎友好的结构化内容", entries)
	if err != nil {
		c.String(http.StatusInternalServerError, "llms.txt error")
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, txt)
}

// GetIndexNowKeyFile 托管 IndexNow 密钥文件（协议关键端点）。
//
// IndexNow 协议要求（官方 FAQ）：搜索引擎验证域名所有权时访问
//   https://<domain>/{key}.txt   （文件名 = 密钥本身，放网站根目录）
// 内容必须与密钥一致。因此本端点支持两种路径：
//   - /{key}.txt            ← 协议要求的根目录位置（Gin 动态路由 /:key.txt 进入）
//   - /public/indexnow-key.txt ← 兼容旧路径
// 未配置密钥或文件名与当前密钥不符时返回 404（避免暴露错误文件）。
func (h *PublicHandler) GetIndexNowKeyFile(c *gin.Context) {
	key := h.indexNowKey
	if h.indexNowKeyProvider != nil {
		key = h.indexNowKeyProvider(c.Request.Context())
	}
	if key == "" {
		c.String(http.StatusNotFound, "not found")
		return
	}
	// 动态路由进入时校验文件名 = 当前密钥（IndexNow 只认 {key}.txt）。
	// Gin 的 :key.txt 参数值含 .txt 后缀（param 名=key.txt），兼容两种取值。
	param := c.Param("key.txt")
	if param == "" {
		param = c.Param("key")
	}
	if param != "" && param != key && param != key+".txt" {
		c.String(http.StatusNotFound, "not found")
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, key)
}

// truncateDescription 截取前 150 字作描述（纯文本，去 markdown 标记）。
func truncateDescription(md string) string {
	plain := pkg.MarkdownToPlainText(md)
	runes := []rune(plain)
	if len(runes) > 150 {
		return string(runes[:150]) + "..."
	}
	return plain
}
