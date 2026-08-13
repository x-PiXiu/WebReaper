// Package public 提供"公开内容站"的展示层实现。
//
// 整洁架构定位：本包是 adapter 层的纯展示工具——把领域内容（markdown 正文）
// 渲染为公开网页可用的 HTML。纯函数、零 IO、可单测；
// 路由与 HTTP 处理在 adapter/handler（public_handler.go）。
package public

import (
	"regexp"
	"strings"
)

// RenderMarkdown 把轻量 markdown 渲染为 HTML（纯函数，可单测）。
//
// 支持的语法（覆盖 GEO 生成内容的常用子集）：
//   - 标题：#/##/### → h2/h3/h4（**整体降一级**——h1 由页面模板的文章标题独占，
//     SEO 单 h1 约束；正文从 h2 开始保持完整层级，避免多 h1 被搜索引擎标记问题）
//   - 无序列表：- / * 开头 → <ul><li>
//   - 有序列表：数字. 开头 → <ol><li>
//   - 引用：> 开头 → <blockquote>
//   - 代码块：``` 包裹 → <pre><code>
//   - 行内代码：`x` → <code>
//   - 粗体：**x** / __x__ → <strong>
//   - 斜体：*x* → <em>
//
// 安全：先做 HTML 转义（用户/AI 内容不直接注入），再应用 markdown 标记替换。
func RenderMarkdown(md string) string {
	if strings.TrimSpace(md) == "" {
		return ""
	}

	lines := strings.Split(md, "\n")
	var sb strings.Builder
	inList := ""    // "ul" / "ol" / ""
	inCodeBlock := false
	codeBuf := strings.Builder{}

	closeList := func() {
		if inList != "" {
			sb.WriteString("</" + inList + ">\n")
			inList = ""
		}
	}

	for _, raw := range lines {
		line := raw

		// 代码块状态机
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inCodeBlock {
				sb.WriteString("<pre><code>" + escapeHTML(codeBuf.String()) + "</code></pre>\n")
				codeBuf.Reset()
				inCodeBlock = false
			} else {
				closeList()
				inCodeBlock = true
			}
			continue
		}
		if inCodeBlock {
			codeBuf.WriteString(line + "\n")
			continue
		}

		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			closeList()
			sb.WriteString("\n")
		case strings.HasPrefix(trimmed, "### "):
			closeList()
			sb.WriteString("<h4>" + inlineFormat(escapeHTML(trimmed[4:])) + "</h4>\n")
		case strings.HasPrefix(trimmed, "## "):
			closeList()
			sb.WriteString("<h3>" + inlineFormat(escapeHTML(trimmed[3:])) + "</h3>\n")
		case strings.HasPrefix(trimmed, "# "):
			// 正文一级标题降为 h2——h1 由页面模板的文章标题独占（SEO 单 h1 约束）
			closeList()
			sb.WriteString("<h2>" + inlineFormat(escapeHTML(trimmed[2:])) + "</h2>\n")
		case strings.HasPrefix(trimmed, "> "):
			closeList()
			sb.WriteString("<blockquote>" + inlineFormat(escapeHTML(trimmed[2:])) + "</blockquote>\n")
		case listItemMatch(trimmed):
			listType := "ul"
			if regexp.MustCompile(`^\d+[.、]`).MatchString(trimmed) {
				listType = "ol"
			}
			if inList != listType {
				closeList()
				sb.WriteString("<" + listType + ">\n")
				inList = listType
			}
			content := regexp.MustCompile(`^[-*+]\s+|^\d+[.、]\s+`).ReplaceAllString(trimmed, "")
			sb.WriteString("<li>" + inlineFormat(escapeHTML(content)) + "</li>\n")
		default:
			closeList()
			sb.WriteString("<p>" + inlineFormat(escapeHTML(trimmed)) + "</p>\n")
		}
	}
	closeList()
	if inCodeBlock {
		sb.WriteString("<pre><code>" + escapeHTML(codeBuf.String()) + "</code></pre>\n")
	}

	return strings.TrimSpace(sb.String())
}

// listItemMatch 判断是否为列表项（-/*/+ 或 数字.）。
func listItemMatch(s string) bool {
	if len(s) < 2 {
		return false
	}
	if s[0] == '-' || s[0] == '*' || s[0] == '+' {
		return s[1] == ' '
	}
	return regexp.MustCompile(`^\d+[.、]`).MatchString(s)
}

// escapeHTML HTML 转义（防注入：AI/用户内容先转义再渲染）。
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// inlineFormat 行内格式：粗体/斜体/行内代码/链接。
// 注意：在 HTML 转义之后执行（此时 < > 已是实体，不会破坏标签）。
func inlineFormat(s string) string {
	// 行内代码（`x`）
	s = regexp.MustCompile("`([^`]+)`").ReplaceAllString(s, "<code>$1</code>")
	// 粗体 **x** 或 __x__
	s = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(s, "<strong>$1</strong>")
	s = regexp.MustCompile(`__([^_]+)__`).ReplaceAllString(s, "<strong>$1</strong>")
	// 斜体 *x*（避免误伤粗体已生成的 <strong> 内内容）
	s = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(s, "<em>$1</em>")
	// 链接 [文字](https://...)——放最后（此时已转义、已处理其他行内格式）。
	// 安全：仅允许 http/https 协议（javascript: 等一律按纯文本输出，防注入）。
	s = markdownLinkRe.ReplaceAllStringFunc(s, func(m string) string {
		p := markdownLinkRe.FindStringSubmatch(m)
		text, href := p[1], p[2]
		if !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") {
			return text // 非 http(s) 协议：不生成链接（防 javascript: 注入）
		}
		// rel=nofollow：外部链接不传递权重（防被判定交换链接），noopener 防新页劫持
		return `<a href="` + href + `" rel="nofollow noopener">` + text + `</a>`
	})
	return s
}

// markdownLinkRe markdown 链接 [text](url)。
var markdownLinkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
