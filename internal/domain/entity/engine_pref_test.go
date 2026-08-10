package entity

import (
	"strings"
	"testing"
)

// BuildEnginePrefInstruction 测试：预设表 + 兜底。
func TestBuildEnginePrefInstruction(t *testing.T) {
	// 已收录引擎：返回带引擎偏好内容的指令
	pref := BuildEnginePrefInstruction("perplexity")
	if !strings.Contains(pref, "Perplexity") {
		t.Errorf("Perplexity 指令应包含引擎名: %s", pref)
	}
	if !strings.Contains(pref, "conclusion") {
		t.Errorf("Perplexity 指令应包含结论前置提示: %s", pref)
	}

	// 大小写不敏感
	if got := BuildEnginePrefInstruction("ChatGPT"); got != BuildEnginePrefInstruction("chatgpt") {
		t.Errorf("引擎名应大小写不敏感")
	}

	// 中文引擎
	if got := BuildEnginePrefInstruction("doubao"); !strings.Contains(got, "口语化") {
		t.Errorf("豆包指令应包含中文风格: %s", got)
	}

	// 未收录 → 通用兜底
	generic := BuildEnginePrefInstruction("generic")
	for _, unknown := range []string{"", "unknown-engine", "Gemini"} {
		if got := BuildEnginePrefInstruction(unknown); got != generic {
			t.Errorf("未收录引擎 %q 应返回通用指令", unknown)
		}
	}

	// 所有预设都非空
	for name := range EnginePrefs {
		if got := BuildEnginePrefInstruction(name); got == "" {
			t.Errorf("预设 %q 指令不应为空", name)
		}
	}
}

// EnginePrefLabel 测试。
func TestEnginePrefLabel(t *testing.T) {
	if EnginePrefLabel("kimi") != "Kimi" {
		t.Errorf("kimi 应显示 Kimi")
	}
	if EnginePrefLabel("doubao") != "豆包" {
		t.Errorf("doubao 应显示 豆包")
	}
	if EnginePrefLabel("unknown") != "通用" {
		t.Errorf("未收录应显示 通用")
	}
}
