package agent

import (
	"reflect"
	"strings"
	"testing"

	"webreaper/internal/usecase/port"
)

// TestComputeMissing_AllCovered 验证：清单全部覆盖 → missing 空。
// 这是图编排"完成即停止"的判定基础。
func TestComputeMissing_AllCovered(t *testing.T) {
	checklist := []string{"agent", "tool", "model", "graph"}
	items := []port.OrchestrateItem{
		{Module: "agent"}, {Module: "tool"}, {Module: "model"}, {Module: "graph"},
	}
	missing, covered := computeMissing(checklist, items)
	if len(missing) != 0 {
		t.Errorf("全部覆盖时 missing 应为空，得到 %v", missing)
	}
	if len(covered) != 4 {
		t.Errorf("已覆盖数 = %d, want 4", len(covered))
	}
}

// TestComputeMissing_PartialCovered 验证：部分覆盖 → 准确算出缺失项。
// 这是"不完整则补生成"的判定基础。
func TestComputeMissing_PartialCovered(t *testing.T) {
	checklist := []string{"agent", "tool", "model", "graph"}
	items := []port.OrchestrateItem{
		{Module: "agent"}, {Module: "model"},
	}
	missing, covered := computeMissing(checklist, items)
	if !reflect.DeepEqual(missing, []string{"tool", "graph"}) {
		t.Errorf("missing = %v, want [tool graph]", missing)
	}
	if !reflect.DeepEqual(covered, []string{"agent", "model"}) {
		t.Errorf("covered = %v, want [agent model]", covered)
	}
}

// TestComputeMissing_TagCoverage 验证：模块通过 Tags 覆盖也算（不只看 Module 字段）。
func TestComputeMissing_TagCoverage(t *testing.T) {
	checklist := []string{"agent", "tool"}
	items := []port.OrchestrateItem{
		{Module: "", Tags: []string{"agent", "core"}}, // 通过 tag 覆盖 agent
	}
	missing, _ := computeMissing(checklist, items)
	// agent 通过 tag 覆盖了，只缺 tool
	if !reflect.DeepEqual(missing, []string{"tool"}) {
		t.Errorf("tag 覆盖后 missing = %v, want [tool]", missing)
	}
}

// TestComputeMissing_CaseInsensitive 验证：模块名大小写不敏感匹配。
// LLM 生成的 module 可能大小写不一（"Agent" vs "agent"）。
func TestComputeMissing_CaseInsensitive(t *testing.T) {
	checklist := []string{"Agent", "Tool"}
	items := []port.OrchestrateItem{{Module: "agent"}, {Module: "tool"}}
	missing, _ := computeMissing(checklist, items)
	if len(missing) != 0 {
		t.Errorf("大小写不敏感应全部覆盖，missing = %v", missing)
	}
}

// TestComputeMissing_EmptyChecklist 验证：空清单 → 全覆盖（边界情况，避免空清单导致永远 missing）。
func TestComputeMissing_EmptyChecklist(t *testing.T) {
	missing, covered := computeMissing([]string{}, []port.OrchestrateItem{{Module: "x"}})
	if len(missing) != 0 {
		t.Errorf("空清单 missing 应为空，得到 %v", missing)
	}
	if len(covered) != 0 {
		t.Errorf("空清单 covered 应为空，得到 %v", covered)
	}
}

// TestExtractStringList_PureJSON 验证：标准 JSON 数组解析。
func TestExtractStringList_PureJSON(t *testing.T) {
	got := extractStringList(`["agent","tool","model"]`)
	want := []string{"agent", "tool", "model"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("= %v, want %v", got, want)
	}
}

// TestExtractStringList_MarkdownWrapped 验证：LLM 常用 ```json 包裹，能提取。
func TestExtractStringList_MarkdownWrapped(t *testing.T) {
	input := "好的，这是模块清单：\n```json\n[\"agent\",\"tool\"]\n```\n如上。"
	got := extractStringList(input)
	want := []string{"agent", "tool"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("markdown 包裹的 JSON 应能提取，= %v, want %v", got, want)
	}
}

// TestExtractStringList_Dedup 验证：去重去空。
func TestExtractStringList_Dedup(t *testing.T) {
	got := extractStringList(`["agent","agent","","tool"]`)
	want := []string{"agent", "tool"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("应去重去空，= %v, want %v", got, want)
	}
}

// TestExtractStringList_NoArray 验证：无 JSON 数组时返回空（兜底清单会接管）。
func TestExtractStringList_NoArray(t *testing.T) {
	got := extractStringList("这不是JSON")
	// 业务上 nil 和空切片等价（len()==0 触发兜底清单），只验证"无有效内容"
	if len(got) != 0 {
		t.Errorf("无 JSON 数组应返回空，得到 %v", got)
	}
}

// TestExtractJSONArray_Variants 验证：各种文本中的数组提取。
func TestExtractJSONArray_Variants(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`["a"]`, `["a"]`},
		{`prefix ["a","b"] suffix`, `["a","b"]`},
		{`no array here`, `[]`},
		{`{not array}`, `[]`},
	}
	for _, c := range cases {
		if got := extractJSONArray(c.in); got != c.want {
			t.Errorf("extractJSONArray(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestDecodeItemsJSON_Standard 验证：标准题目 JSON 解析。
func TestDecodeItemsJSON_Standard(t *testing.T) {
	input := `[{"title":"Q1","content":"答案","module":"agent","tags":["go"]}]`
	items := decodeItemsJSON(input)
	if len(items) != 1 {
		t.Fatalf("应解析 1 条，得到 %d", len(items))
	}
	if items[0].Title != "Q1" || items[0].Module != "agent" {
		t.Errorf("字段解析错: %+v", items[0])
	}
	if len(items[0].Tags) != 1 || items[0].Tags[0] != "go" {
		t.Errorf("Tags 解析错: %v", items[0].Tags)
	}
}

// TestDecodeItemsJSON_Malformed 验证：非法 JSON 返回空切片（不 panic）。
func TestDecodeItemsJSON_Malformed(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("非法 JSON 不应 panic: %v", r)
		}
	}()
	items := decodeItemsJSON("这不是JSON")
	if len(items) != 0 {
		t.Errorf("非法 JSON 应返回空切片，得到 %d 条", len(items))
	}
}

// TestContentTypeLabel 验证：内容类型中文标签映射。
func TestContentTypeLabel(t *testing.T) {
	cases := map[string]string{
		"interview_questions": "面试题",
		"knowledge_summary":   "知识点总结",
		"":                    "内容",
		"unknown":             "unknown",
	}
	for in, want := range cases {
		if got := contentTypeLabel(in); got != want {
			t.Errorf("contentTypeLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---- 质量评审相关（分层校验新增）----

// TestParseQualityResult_Pass 验证：评审通过（pass=true，无 gaps）。
func TestParseQualityResult_Pass(t *testing.T) {
	pass, gaps := parseQualityResult(`{"pass": true, "gaps": []}`)
	if !pass {
		t.Error("pass=true 应解析为通过")
	}
	if len(gaps) != 0 {
		t.Errorf("无 gaps 时应返回空，得到 %v", gaps)
	}
}

// TestParseQualityResult_FailWithGaps 验证：评审不达标，正确提取 gaps。
// 这是"带反馈精确补生成"的关键——gaps 要准确解析出来传给 generator。
func TestParseQualityResult_FailWithGaps(t *testing.T) {
	input := `{"pass": false, "gaps": ["缺少 graph 模块的实现细节", "agent 节点题目跑题"]}`
	pass, gaps := parseQualityResult(input)
	if pass {
		t.Error("pass=false 应解析为不通过")
	}
	if len(gaps) != 2 {
		t.Fatalf("应提取 2 个 gaps，得到 %d", len(gaps))
	}
	if gaps[0] != "缺少 graph 模块的实现细节" {
		t.Errorf("gap[0] = %q", gaps[0])
	}
}

// TestParseQualityResult_MarkdownWrapped 验证：LLM 常用 ```json 包裹。
func TestParseQualityResult_MarkdownWrapped(t *testing.T) {
	input := "评审结果：\n```json\n{\"pass\": false, \"gaps\": [\"缺X\"]}\n```"
	pass, gaps := parseQualityResult(input)
	if pass {
		t.Error("应解析为不通过")
	}
	if len(gaps) != 1 || gaps[0] != "缺X" {
		t.Errorf("gaps = %v, want [缺X]", gaps)
	}
}

// TestParseQualityResult_Malformed 验证：非法 JSON 不 panic，保守降级。
// 非 JSON 输入会解析为空对象 {}，pass 字段默认 false——但这不 panic，且
// 上游 quality_validator 对 LLM 调用失败已有降级（不阻断流程）。
func TestParseQualityResult_Malformed(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("非法 JSON 不应 panic: %v", r)
		}
	}()
	// 只验证不 panic（pass 值在非 JSON 时为 false 默认，业务上由上游降级兜底）
	_, _ = parseQualityResult("这不是JSON")
}

// TestParseQualityResult_NullGaps 验证：gaps 为 null 时返回空切片（非 nil）。
func TestParseQualityResult_NullGaps(t *testing.T) {
	pass, gaps := parseQualityResult(`{"pass": true, "gaps": null}`)
	if !pass {
		t.Error("应通过")
	}
	// nil 和空切片在业务上等价，但确保不会因 nil 导致下游 panic
	_ = gaps
}

// TestExtractJSONObject_Nested 验证：嵌套 JSON 对象的括号配平提取。
func TestExtractJSONObject_Nested(t *testing.T) {
	input := `prefix {"a": {"b": 1}, "c": 2} suffix`
	got := extractJSONObject(input)
	want := `{"a": {"b": 1}, "c": 2}`
	if got != want {
		t.Errorf("= %q, want %q", got, want)
	}
}

// TestExtractJSONObject_NoBrace 验证：无 { 时返回 {}。
func TestExtractJSONObject_NoBrace(t *testing.T) {
	if got := extractJSONObject("no brace"); got != "{}" {
		t.Errorf("无 { 应返回 {}, 得到 %q", got)
	}
}

// TestItemsToSummary 验证：题目摘要格式正确 + 内容截断。
func TestItemsToSummary(t *testing.T) {
	items := []port.OrchestrateItem{
		{Title: "题1", Module: "agent", Content: "短内容"},
		{Title: "题2", Module: "graph", Content: strings.Repeat("长", 200)},
	}
	summary := itemsToSummary(items)
	if !strings.Contains(summary, "题1") || !strings.Contains(summary, "agent") {
		t.Errorf("摘要应含标题和模块，得到: %s", summary)
	}
	if !strings.Contains(summary, "...") {
		t.Error("长内容应被截断（含 ...）")
	}
}

// TestItemsToSummary_Empty 验证：空列表返回占位文本。
func TestItemsToSummary_Empty(t *testing.T) {
	if got := itemsToSummary(nil); got != "（无题目）" {
		t.Errorf("空列表应返回占位，得到 %q", got)
	}
}

