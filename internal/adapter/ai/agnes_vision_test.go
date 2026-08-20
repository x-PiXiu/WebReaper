package ai

import (
	"testing"
)

// TestExtractJSONFromContent 测试从 LLM 回答中提取 JSON 的各种情况。
func TestExtractJSONFromContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "纯 JSON",
			content:  `{"tool": "browser_click", "args": {"selector": "#btn"}}`,
			expected: `{"tool": "browser_click", "args": {"selector": "#btn"}}`,
		},
		{
			name:     "markdown json 代码块",
			content:  "这是我的分析：\n```json\n{\"tool\": \"browser_done\", \"done\": true}\n```\n以上是决策。",
			expected: `{"tool": "browser_done", "done": true}`,
		},
		{
			name:     "markdown 无语言标记代码块",
			content:  "分析结果：\n```\n{\"tool\": \"browser_wait\"}\n```",
			expected: `{"tool": "browser_wait"}`,
		},
		{
			name:     "花括号提取",
			content:  "根据截图，我建议 {'tool': 'browser_scroll'} 这个操作。",
			expected: `{'tool': 'browser_scroll'}`,
		},
		{
			name:     "无 JSON",
			content:  "我看不到任何问题，页面正常。",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONFromContent(tt.content)
			if got != tt.expected {
				t.Errorf("extractJSONFromContent() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestAgnesVisionLLM_Configured 配置状态测试。
func TestAgnesVisionLLM_Configured(t *testing.T) {
	// 未配置
	v1 := NewAgnesVisionLLM("", "", "", nil)
	if v1.IsConfigured() {
		t.Error("空 API Key 应返回 false")
	}

	// 已配置
	v2 := NewAgnesVisionLLM("test-key", "", "", nil)
	if !v2.IsConfigured() {
		t.Error("非空 API Key 应返回 true")
	}

	// 默认值检查
	if v2.baseURL != "https://apihub.agnes-ai.com/v1" {
		t.Errorf("默认 baseURL = %s", v2.baseURL)
	}
	if v2.model != "agnes-2.5-flash" {
		t.Errorf("默认 model = %s", v2.model)
	}
}
