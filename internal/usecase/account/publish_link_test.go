package account

import (
	"strings"
	"testing"
)

// appendPublicLink 测试：平台差异 + 边界。
func TestAppendPublicLink(t *testing.T) {
	baseURL := "https://content.example.com"
	content := "正文内容"

	t.Run("知乎追加链接", func(t *testing.T) {
		got := appendPublicLink(content, "zhihu", baseURL, "oc-1")
		if !strings.Contains(got, "https://content.example.com/public/articles/oc-1") {
			t.Errorf("知乎应追加公开站链接: %s", got)
		}
		if !strings.HasPrefix(got, content) {
			t.Errorf("原文应保留在开头: %s", got)
		}
	})

	t.Run("小红书不追加（防限流）", func(t *testing.T) {
		got := appendPublicLink(content, "xiaohongshu", baseURL, "oc-1")
		if got != content {
			t.Errorf("小红书不应追加链接: %s", got)
		}
	})

	t.Run("无 baseURL 不追加", func(t *testing.T) {
		if got := appendPublicLink(content, "zhihu", "", "oc-1"); got != content {
			t.Errorf("无 baseURL 不应追加: %s", got)
		}
	})

	t.Run("无 contentID 不追加", func(t *testing.T) {
		if got := appendPublicLink(content, "zhihu", baseURL, ""); got != content {
			t.Errorf("无 contentID 不应追加: %s", got)
		}
	})

	t.Run("baseURL 尾部斜杠兼容", func(t *testing.T) {
		got := appendPublicLink(content, "zhihu", "https://content.example.com/", "oc-1")
		if !strings.Contains(got, "https://content.example.com/public/articles/oc-1") {
			t.Errorf("尾部斜杠应被规整: %s", got)
		}
	})
}
