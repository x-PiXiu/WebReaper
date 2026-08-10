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
	contentRepo        port.OptimizedContentRepository
	structured         *structured.StructuredDataUseCase
	baseURL            string // 公开站点根地址（如 https://example.com），生成绝对 URL
	indexNowKey        string // IndexNow 密钥（静态注入，启动时值）
	indexNowKeyProvider func(context.Context) string // 动态读取（运行时可调配置；优先于静态值）
}

// NewPublicHandler 创建公开站点处理器。
func NewPublicHandler(repo port.OptimizedContentRepository, structuredUC *structured.StructuredDataUseCase, baseURL string) *PublicHandler {
	if baseURL == "" {
		baseURL = "http://localhost:8082"
	}
	return &PublicHandler{contentRepo: repo, structured: structuredUC, baseURL: baseURL}
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
  .footer{margin-top:64px;padding-top:16px;border-top:1px solid #eee;color:#aaa;font-size:0.85em}
</style>
</head>
<body>
<article>
  <h1>{{.Title}}</h1>
  <div class="meta">{{.Meta}}</div>
  {{.ContentHTML}}
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

	// 生成 JSON-LD（Article/FAQPage 自动推断），内嵌为 <script> 标签。
	// 标题兜底后仍失败（如正文异常）不阻断页面——JSON-LD 是增强项。
	sd, _ := h.structured.GenerateJSONLD(c.Request.Context(), structured.StructuredDataInput{
		Title:   title,
		Content: content.OptimizedText,
		URL:     h.baseURL + "/public/articles/" + content.ID,
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
		"Description": truncateDescription(content.OptimizedText),
		"Meta":        fmt.Sprintf("GEO 优化内容 · 关键词可见度评分 %d", int(content.Score.Total)),
		"ContentHTML": template.HTML(public.RenderMarkdown(content.OptimizedText)),
		"JSONLD":      template.HTML(jsonldTag),
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
		c.Writer.WriteString(fmt.Sprintf("  <url><loc>%s/public/articles/%s</loc></url>\n", h.baseURL, it.ID))
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
			Summary: truncateDescription(it.OptimizedText),
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

// GetIndexNowKeyFile GET /public/indexnow-key.txt —— 托管 IndexNow 密钥文件。
// IndexNow 验证时会访问 keyLocation 并比对内容（必须与提交的 key 一致）。
// 优先读运行时配置（管理后台可改），未配置时返回 404。
func (h *PublicHandler) GetIndexNowKeyFile(c *gin.Context) {
	key := h.indexNowKey
	if h.indexNowKeyProvider != nil {
		key = h.indexNowKeyProvider(c.Request.Context())
	}
	if key == "" {
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
