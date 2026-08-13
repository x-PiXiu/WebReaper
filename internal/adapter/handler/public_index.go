// 公开文章列表页：站点"首页/目录"——让爬虫与用户从单一入口发现全部已发布文章。
//
// 设计（整洁架构）：
//   - 与 sitemap.xml / llms.txt 同源（contentRepo.ListPublished，port 接口）——
//     零新增仓储方法，纯查询 + 纯模板渲染
//   - 服务端渲染（SEO 友好）：标题/摘要/发布时间/品牌名全部在 HTML 中
//   - ItemList JSON-LD：明确告诉搜索引擎"这是一个文章列表页"（列表页结构化信号）
//   - 无认证（公开段，router 挂在 JWT 之外）
package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"webreaper/internal/pkg"
)

// publicIndexTemplate 文章列表页模板。
// 站点结构信号：header（站点名/描述）→ 文章列表（标题链接/品牌/时间/摘要）→ footer。
// 列表页 → 文章页的完整内链：爬虫从 /public/ 一步发现全部内容。
const publicIndexTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.SiteTitle}} · 全部文章</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.CanonicalURL}}">
{{.JSONLD}}
<style>
  body{max-width:760px;margin:0 auto;padding:32px 20px 80px;font-family:-apple-system,'PingFang SC','Noto Sans SC',sans-serif;line-height:1.9;color:#1a1a2e}
  .site-header{margin-bottom:40px;padding-bottom:20px;border-bottom:1px solid #eee}
  .site-header h1{font-size:1.8em;letter-spacing:-0.02em;margin:0 0 8px}
  .site-header .site-desc{color:#8a8aa0;font-size:0.95em;margin:0}
  .post{margin-bottom:36px}
  .post h2{font-size:1.25em;margin:0 0 6px}
  .post h2 a{color:#1a1a2e;text-decoration:none}
  .post h2 a:hover{color:#6366f1;text-decoration:underline}
  .post-meta{color:#8a8aa0;font-size:0.85em;margin-bottom:8px}
  .post-excerpt{color:#555;font-size:0.95em;margin:0}
  .empty{color:#8a8aa0;text-align:center;padding:60px 0}
  .footer{margin-top:40px;padding-top:16px;border-top:1px solid #eee;color:#aaa;font-size:0.85em}
</style>
</head>
<body>
<header class="site-header">
  <h1>全部文章</h1>
  <p class="site-desc">{{.Description}}</p>
</header>
<main>
  {{if .Articles}}
    {{range .Articles}}
    <article class="post">
      <h2><a href="/public/articles/{{.ID}}">{{.Title}}</a></h2>
      <div class="post-meta">{{if .BrandName}}{{.BrandName}} · {{end}}{{.Date}}</div>
      <p class="post-excerpt">{{.Excerpt}}</p>
    </article>
    {{end}}
  {{else}}
    <p class="empty">暂无内容，敬请期待</p>
  {{end}}
</main>
<div class="footer">© WebReaper · 内容定期更新</div>
</body>
</html>`

// GetPublicIndex GET /public（及 /public/）—— 公开文章列表页。
// 数据源与 sitemap/llms.txt 一致（ListPublished），品牌名逐篇查询（失败降级为空）。
func (h *PublicHandler) GetPublicIndex(c *gin.Context) {
	items, err := h.contentRepo.ListPublished(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "list error")
		return
	}

	// 组装列表项：标题/品牌名/发布时间/摘要（摘要与 sitemap 描述同款清洗）
	type listItem struct {
		ID        string
		Title     string
		BrandName string
		Date      string
		Excerpt   string
	}
	articles := make([]listItem, 0, len(items))
	for _, it := range items {
		brandName := ""
		if h.brandRepo != nil && it.BrandID != "" {
			if b, bErr := h.brandRepo.FindPublishedByID(c.Request.Context(), it.BrandID); bErr == nil {
				brandName = b.Name
			}
		}
		articles = append(articles, listItem{
			ID:        it.ID,
			Title:     it.Title,
			BrandName: brandName,
			Date:      it.CreatedAt.Format("2006-01-02"),
			Excerpt:   truncateDescription(pkg.StripThinkTags(it.OptimizedText)),
		})
	}

	// ItemList JSON-LD：列表页结构化信号（position + url + name）
	pageURL := strings.TrimRight(h.baseURL, "/") + "/public/"
	var sb strings.Builder
	sb.WriteString(`<script type="application/ld+json">{"@context":"https://schema.org","@type":"ItemList","itemListElement":[`)
	for i, it := range articles {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"@type":"ListItem","position":%d,"url":"%s/public/articles/%s","name":%q}`,
			i+1, strings.TrimRight(h.baseURL, "/"), it.ID, it.Title)
	}
	sb.WriteString(`]}</script>`)

	desc := "AI 搜索引擎友好的内容中心——品牌原创内容持续更新"
	if len(articles) > 0 {
		desc = fmt.Sprintf("共 %d 篇文章 · AI 搜索引擎友好的内容中心", len(articles))
	}

	tpl, err := template.New("index").Parse(publicIndexTemplate)
	if err != nil {
		c.String(http.StatusInternalServerError, "render error")
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tpl.Execute(c.Writer, gin.H{
		"SiteTitle":    "WebReaper 内容中心",
		"Description":  desc,
		"CanonicalURL": pageURL,
		"JSONLD":       template.HTML(sb.String()),
		"Articles":     articles,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error")
	}
}
