package moderation

// moderation_test.go —— 32号 P2：机审判定解析纯函数单测。

import "testing"

func TestIsHighRisk(t *testing.T) {
	// 二批：阻断档范围——高危三类拒绝，营销夸大类仅标记不阻断
	if !IsHighRisk(textVerdict{Flag: true, Category: "politics"}) {
		t.Error("politics 应属高危")
	}
	if !IsHighRisk(textVerdict{Flag: true, Category: "porn"}) {
		t.Error("porn 应属高危")
	}
	if !IsHighRisk(textVerdict{Flag: true, Category: "violence"}) {
		t.Error("violence 应属高危")
	}
	if IsHighRisk(textVerdict{Flag: true, Category: "medical_exaggeration"}) {
		t.Error("营销夸大类不阻断（仅标记——处置权在管理员）")
	}
	if IsHighRisk(textVerdict{Flag: true, Category: "illegal_ad"}) {
		t.Error("违法广告暂列标记档（可按运营策略调整）")
	}
	if IsHighRisk(textVerdict{Flag: false, Category: "porn"}) {
		t.Error("未标记不构成高危")
	}
}

func TestParseVerdict(t *testing.T) {
	// 标准 JSON
	v := parseVerdict(`{"flag":true,"category":"illegal_ad","reason":"涉嫌违法广告"}`)
	if !v.Flag || v.Category != "illegal_ad" || v.Reason != "涉嫌违法广告" {
		t.Errorf("标准解析错误: %+v", v)
	}
	// markdown 围栏包裹（LLM 常见输出形态）
	v = parseVerdict("好的，审核结果：\n```json\n{\"flag\":true,\"category\":\"porn\",\"reason\":\"低俗内容\"}\n```")
	if !v.Flag || v.Category != "porn" {
		t.Errorf("围栏宽容解析错误: %+v", v)
	}
	// 通过
	v = parseVerdict(`{"flag":false,"category":"","reason":""}`)
	if v.Flag {
		t.Error("flag=false 应通过")
	}
	// 非 JSON 输出（模型格式漂移）——保守通过（宁漏勿误杀）
	v = parseVerdict("这段内容看起来没有问题，可以放行。")
	if v.Flag {
		t.Error("无法解析时应保守通过（不误标用户）")
	}
	// 空/残缺
	v = parseVerdict("")
	if v.Flag {
		t.Error("空输出应通过")
	}
}
