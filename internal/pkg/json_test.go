package pkg

import (
	"encoding/json"
	"testing"
)

func TestExtractJSONBlock(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"纯 JSON", `{"a":1}`, `{"a":1}`},
		{"think 包裹", `<think>让我分析</think>{"a":1}`, `{"a":1}`},
		{"think+代码块", "<think>分析</think>\n```json\n{\"a\":1}\n```", `{"a":1}`},
		{"说明文字+JSON", `结果是：{"a":1}，请查收`, `{"a":1}`},
		{"think 未闭合", `<think>没闭合`, ``},
		{"无 JSON", `纯文本说明`, `纯文本说明`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExtractJSONBlock(c.in)
			if got != c.want {
				t.Errorf("ExtractJSONBlock(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestExtractJSONBlock_ParsesKeywordList(t *testing.T) {
	// 回归：真实场景——LLM 忽略 DisableThinking 输出 think 块 + json 代码块
	in := "<think>生成关键词</think>\n```json\n{\"keywords\":[{\"term\":\"望京川菜\",\"intent\":\"品牌词\"}]}\n```"
	var kl struct {
		Keywords []struct {
			Term string `json:"term"`
		} `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(ExtractJSONBlock(in)), &kl); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(kl.Keywords) != 1 || kl.Keywords[0].Term != "望京川菜" {
		t.Errorf("unexpected result: %+v", kl)
	}
}
