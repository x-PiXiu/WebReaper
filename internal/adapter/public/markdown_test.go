package public

import (
	"strings"
	"testing"
)

func TestRenderMarkdown_Headings(t *testing.T) {
	md := "# 大标题\n\n## 小标题\n\n### 三级"
	html := RenderMarkdown(md)
	if !strings.Contains(html, "<h1>大标题</h1>") {
		t.Errorf("缺少 h1: %s", html)
	}
	if !strings.Contains(html, "<h2>小标题</h2>") {
		t.Errorf("缺少 h2: %s", html)
	}
	if !strings.Contains(html, "<h3>三级</h3>") {
		t.Errorf("缺少 h3: %s", html)
	}
}

func TestRenderMarkdown_Lists(t *testing.T) {
	md := "- 甲\n- 乙\n\n1. 一\n2. 二"
	html := RenderMarkdown(md)
	if !strings.Contains(html, "<ul>") || !strings.Contains(html, "<li>甲</li>") {
		t.Errorf("无序列表渲染错误: %s", html)
	}
	if !strings.Contains(html, "<ol>") || !strings.Contains(html, "<li>一</li>") {
		t.Errorf("有序列表渲染错误: %s", html)
	}
}

func TestRenderMarkdown_CodeBlock(t *testing.T) {
	md := "```go\nfunc main() {}\n```"
	html := RenderMarkdown(md)
	if !strings.Contains(html, "<pre><code>") || !strings.Contains(html, "func main() {}") {
		t.Errorf("代码块渲染错误: %s", html)
	}
	// 代码内容不应被转义成可见实体
	if strings.Contains(html, "&lt;") {
		t.Errorf("代码块内容不应转义: %s", html)
	}
}

func TestRenderMarkdown_InlineFormat(t *testing.T) {
	html := RenderMarkdown("这是 **重点** 和 `code` 内容")
	if !strings.Contains(html, "<strong>重点</strong>") {
		t.Errorf("粗体渲染错误: %s", html)
	}
	if !strings.Contains(html, "<code>code</code>") {
		t.Errorf("行内代码渲染错误: %s", html)
	}
}

func TestRenderMarkdown_EscapeHTML(t *testing.T) {
	// 注入脚本不应原样输出
	html := RenderMarkdown("<script>alert(1)</script>")
	if strings.Contains(html, "<script>") {
		t.Errorf("HTML 未转义（注入风险）: %s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Errorf("应输出转义后的实体: %s", html)
	}
}

func TestRenderMarkdown_Empty(t *testing.T) {
	if RenderMarkdown("") != "" {
		t.Error("空内容应返回空")
	}
	if RenderMarkdown("   \n  ") != "" {
		t.Error("空白内容应返回空")
	}
}

func TestRenderMarkdown_Blockquote(t *testing.T) {
	html := RenderMarkdown("> 引用内容")
	if !strings.Contains(html, "<blockquote>引用内容</blockquote>") {
		t.Errorf("引用渲染错误: %s", html)
	}
}
