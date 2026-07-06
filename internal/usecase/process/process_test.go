package process

import (
	"strings"
	"testing"
)

// TestStripHTML_BasicTags 验证：常见 HTML 标签被去除，保留纯文本。
// 对应联调发现的 arbeitnow description 含 <p>/<strong> 等标签。
func TestStripHTML_BasicTags(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "段落+加粗",
			input: "<p>We're looking for a <strong>Senior Engineer</strong></p>",
			want:  "We're looking for a Senior Engineer",
		},
		{
			name:  "列表",
			input: "<ul><li>Go</li><li>MySQL</li></ul>",
			want:  "Go MySQL", // goquery 把 li 文本用空格连
		},
		{
			name:  "换行标签",
			input: "line1<br>line2",
			want:  "line1 line2",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripHTML(c.input)
			// goquery 对空白处理可能不完全一致，用"包含核心文本"断言更稳
			if !strings.Contains(got, c.want) && normalize(got) != normalize(c.want) {
				t.Errorf("stripHTML(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

// TestStripHTML_Entities 验证：HTML 实体（&amp; 等）被正确解码。
func TestStripHTML_Entities(t *testing.T) {
	got := stripHTML("<p>Tom &amp; Jerry &lt;test&gt;</p>")
	// &amp; → &, &lt; → <
	if !strings.Contains(got, "Tom & Jerry") {
		t.Errorf("应解码 &amp;，得到: %q", got)
	}
}

// TestStripHTML_ScriptStyleRemoved 验证：script/style 内容被移除（不当正文）。
// 重要：避免把 JS 代码当文本喂给 LLM，既浪费 token 又干扰理解。
func TestStripHTML_ScriptStyleRemoved(t *testing.T) {
	input := `<p>正文</p><script>alert('hack')</script><style>.x{color:red}</style>`
	got := stripHTML(input)
	if strings.Contains(got, "alert") {
		t.Errorf("script 内容应被移除，得到: %q", got)
	}
	if strings.Contains(got, "color") {
		t.Errorf("style 内容应被移除，得到: %q", got)
	}
	if !strings.Contains(got, "正文") {
		t.Errorf("正文应保留，得到: %q", got)
	}
}

// TestStripHTML_NoTagsPassthrough 验证：纯文本（无标签）原样返回。
func TestStripHTML_NoTagsPassthrough(t *testing.T) {
	input := "这是一段纯文本，没有 HTML 标签。"
	got := stripHTML(input)
	if got != input {
		t.Errorf("纯文本应原样返回，got=%q want=%q", got, input)
	}
}

// TestStripHTML_Empty 验证：空串不 panic。
func TestStripHTML_Empty(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("空串不应 panic: %v", r)
		}
	}()
	if got := stripHTML(""); got != "" {
		t.Errorf("空串应返回空，got=%q", got)
	}
}

// TestStripHTML_MalformedHTML 验证：残缺 HTML 不崩溃（保守返回解析结果）。
func TestStripHTML_MalformedHTML(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("残缺 HTML 不应 panic: %v", r)
		}
	}()
	// 未闭合标签
	got := stripHTML("<p>未闭合的内容")
	if !strings.Contains(got, "未闭合的内容") {
		t.Errorf("残缺 HTML 应保守提取文本，得到: %q", got)
	}
}

// normalize 压缩多余空白，便于跨 goquery 版本断言。
func normalize(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
