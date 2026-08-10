package ai

import (
	"testing"

	"webreaper/internal/domain/entity"
)

// parseGeoScoreJSON 测试：验证标准 JSON、markdown 包裹、解析失败降级。
func TestParseGeoScoreJSON(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		expect  entity.GEOScore
		comment string
	}{
		{
			name:  "标准 JSON",
			input: `{"authority":85,"specificity":70,"structure":80,"uniqueness":65,"recency":75}`,
			expect: entity.GEOScore{
				Authority: 85, Specificity: 70, Structure: 80, Uniqueness: 65, Recency: 75,
			},
		},
		{
			name:  "markdown 包裹",
			input: "```json\n{\"authority\":90,\"specificity\":60,\"structure\":70,\"uniqueness\":80,\"recency\":50}\n```",
			expect: entity.GEOScore{
				Authority: 90, Specificity: 60, Structure: 70, Uniqueness: 80, Recency: 50,
			},
		},
		{
			name:  "多余文字",
			input: "评分结果如下：{\"authority\":100,\"specificity\":100,\"structure\":100,\"uniqueness\":100,\"recency\":100}，请参考。",
			expect: entity.GEOScore{
				Authority: 100, Specificity: 100, Structure: 100, Uniqueness: 100, Recency: 100,
			},
		},
		{
			name:   "解析失败降级全 50",
			input:  "模型没有返回 JSON，只说了一堆话。",
			expect: entity.GEOScore{Authority: 50, Specificity: 50, Structure: 50, Uniqueness: 50, Recency: 50},
		},
		{
			name:  "数值越界钳制",
			input: `{"authority":150,"specificity":-20,"structure":50,"uniqueness":50,"recency":50}`,
			expect: entity.GEOScore{
				Authority: 100, Specificity: 0, Structure: 50, Uniqueness: 50, Recency: 50,
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseGeoScoreJSON(c.input)
			if got != c.expect {
				t.Errorf("parseGeoScoreJSON(%q) = %+v, want %+v", c.input, got, c.expect)
			}
		})
	}
}

// truncateForGeo 是跨文件共用辅助，验证行为。
func TestTruncateForGeo(t *testing.T) {
	if got := truncateForGeo("短文本", 10); got != "短文本" {
		t.Errorf("短文本不应截断: %q", got)
	}
	long := "这是一段很长的内容，用来测试截断行为。"
	if got := truncateForGeo(long, 10); len([]rune(got)) != 13 {
		t.Errorf("截断后应 10 字 + '...' = 13 runes，实际 %d: %q", len([]rune(got)), got)
	}
}
