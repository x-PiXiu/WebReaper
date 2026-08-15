package handler

import (
	"reflect"
	"strings"
	"testing"

	"webreaper/internal/domain/entity"
)

// 契约守护：实体公开字段必须出现在 API 视图——防止实体演化时序列化静默断流。
// 背景（信源断流缺陷回归测试）：实体加了 Sources/SelfSourceCount 而视图没同步，
// 前端三处展示位长期空转且 TS 类型无法拦截。此测试让"实体加字段、视图忘加"
// 在 CI 阶段直接失败。
func TestMonitoringResultViewCoversEntityFields(t *testing.T) {
	viewTags := map[string]bool{}
	vt := reflect.TypeOf(monitoringResultView{})
	for i := 0; i < vt.NumField(); i++ {
		tag := strings.Split(vt.Field(i).Tag.Get("json"), ",")[0]
		viewTags[tag] = true
	}

	et := reflect.TypeOf(entity.MonitoringResult{})
	for i := 0; i < et.NumField(); i++ {
		f := et.Field(i)
		if !f.IsExported() {
			continue
		}
		key := goFieldToSnake(f.Name)
		if !viewTags[key] {
			t.Errorf("实体字段 %s 缺失于 API 视图（期望 json:%s）——实体演化未同步视图，序列化将静默丢字段", f.Name, key)
		}
	}
}

// goFieldToSnake PascalCase → snake_case（处理 ID 等连续大写缩略词）。
func goFieldToSnake(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			prevLower := i > 0 && !(runes[i-1] >= 'A' && runes[i-1] <= 'Z')
			nextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
			if i > 0 && (prevLower || nextLower) {
				b.WriteRune('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestGoFieldToSnake(t *testing.T) {
	cases := map[string]string{
		"ID":                   "id",
		"TenantID":             "tenant_id",
		"AvgPosition":          "avg_position",
		"SelfSourceCount":      "self_source_count",
		"CompetitorSentiments": "competitor_sentiments",
		"RawSample":            "raw_sample",
	}
	for in, want := range cases {
		if got := goFieldToSnake(in); got != want {
			t.Errorf("goFieldToSnake(%q) = %q, want %q", in, got, want)
		}
	}
}
