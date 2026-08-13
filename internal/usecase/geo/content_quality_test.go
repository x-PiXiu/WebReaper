package geo

import (
	"strings"
	"testing"
)

// validateGeneratedContent 结构校验：合格内容应通过
func TestValidateGeneratedContent_OK(t *testing.T) {
	text := "# 标题示例文章\n\n## 第一节\n\n" + strings.Repeat("这是一段比较长的正文内容用于测试字数。", 30) + "\n\n### 第二节\n\n" + strings.Repeat("更多正文内容用于测试校验逻辑是否正常工作。", 20)
	problems := validateGeneratedContent(text, "标题示例文章", "article")
	if len(problems) != 0 {
		t.Errorf("合格内容不应有校验问题: %v", problems)
	}
}

// 短正文应被拦截（article 需 ≥400 字）
func TestValidateGeneratedContent_TooShort(t *testing.T) {
	text := "# 标题示例文章\n\n## 节\n\n很短的内容"
	problems := validateGeneratedContent(text, "标题示例文章", "article")
	found := false
	for _, p := range problems {
		if strings.Contains(p, "正文过短") {
			found = true
		}
	}
	if !found {
		t.Errorf("短正文应报'正文过短': %v", problems)
	}
}

// 缺标题应被拦截
func TestValidateGeneratedContent_NoTitle(t *testing.T) {
	problems := validateGeneratedContent("## 只有小标题的正文内容", "短", "article")
	found := false
	for _, p := range problems {
		if strings.Contains(p, "标题") {
			found = true
		}
	}
	if !found {
		t.Errorf("短标题应被拦截: %v", problems)
	}
}

// 深度格式（article）缺小标题应被拦截；轻格式（review）不要求小标题
func TestValidateGeneratedContent_NoSubheading(t *testing.T) {
	long := strings.Repeat("这是一段比较长的正文内容用于测试字数。", 30)
	// article：缺小标题 → 拦截
	problems := validateGeneratedContent("# 标题示例文章\n\n"+long, "标题示例文章", "article")
	found := false
	for _, p := range problems {
		if strings.Contains(p, "小标题") {
			found = true
		}
	}
	if !found {
		t.Errorf("article 缺小标题应被拦截: %v", problems)
	}
	// review：不要求小标题 → 通过（正文够长 + 标题合法）
	problems = validateGeneratedContent("# 标题示例文章\n\n"+long, "标题示例文章", "review")
	if len(problems) != 0 {
		t.Errorf("review 格式不应要求小标题: %v", problems)
	}
}
