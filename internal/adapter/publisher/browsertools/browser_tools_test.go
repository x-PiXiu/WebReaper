package browsertools

import (
	"encoding/json"
	"testing"
)

// TestGenerateSliderTrajectory 测试滑块轨迹生成。
// 验证：步数合理、起始位置正确、终点到达、延迟递增（ease-out 模式）。
func TestGenerateSliderTrajectory(t *testing.T) {
	startX, startY, distance := 100.0, 200.0, 300.0

	steps := generateSliderTrajectory(startX, startY, distance)

	if len(steps) < 10 {
		t.Errorf("步数太少：%d（至少 10 步）", len(steps))
	}
	if len(steps) > 60 {
		t.Errorf("步数太多：%d（最多 ~53 步含微调）", len(steps))
	}

	// 第一步应接近起始位置
	if steps[0].x < startX || steps[0].x > startX+10 {
		t.Errorf("起始 X 应接近 %.0f，实际 %.1f", startX, steps[0].x)
	}

	// 最后一步应到达终点（startX + distance）
	lastX := steps[len(steps)-1].x
	if lastX < startX+distance-5 {
		t.Errorf("终点 X 应接近 %.0f，实际 %.1f", startX+distance, lastX)
	}

	// Y 应在 startY 附近（微小偏移，±5 以内）
	for i, step := range steps {
		if step.y < startY-5 || step.y > startY+5 {
			t.Errorf("步 %d 的 Y 偏移过大：%.1f（应在 ±5 内）", i, step.y-startY)
		}
	}

	// 延迟应为正数
	for i, step := range steps {
		if step.delayMs <= 0 {
			t.Errorf("步 %d 的延迟应 > 0，实际 %d", i, step.delayMs)
		}
	}
}

// TestSliderTrajectoryEasing 测试缓动曲线（ease-out：前 30% 覆盖 >50% 距离）。
func TestSliderTrajectoryEasing(t *testing.T) {
	startX, distance := 0.0, 300.0
	steps := generateSliderTrajectory(startX, 0, distance)

	// 前 30% 的步应覆盖超过 50% 的距离（ease-out 特征：先快后慢）
	idx30 := len(steps) * 30 / 100
	if idx30 >= len(steps) {
		idx30 = len(steps) - 1
	}
	xAt30 := steps[idx30].x
	progress30 := xAt30 / distance

	if progress30 < 0.5 {
		t.Errorf("ease-out 特征不满足：前 30%% 步只覆盖 %.1f%% 距离（应 >50%%）", progress30*100)
	}
}

// TestSliderTrajectorySmallDistance 短距离（小于 50px）也能生成合理轨迹。
func TestSliderTrajectorySmallDistance(t *testing.T) {
	steps := generateSliderTrajectory(50, 100, 30) // 30px 距离

	if len(steps) < 10 {
		t.Errorf("短距离步数太少：%d（至少 10 步，防止轨迹过于生硬）", len(steps))
	}

	// 终点检查
	lastX := steps[len(steps)-1].x
	if lastX < 75 { // startX + distance - 5 的容差
		t.Errorf("短距离终点 X 应接近 80，实际 %.1f", lastX)
	}
}

// TestSliderTrajectoryLargeDistance 长距离（500px+）轨迹合理。
func TestSliderTrajectoryLargeDistance(t *testing.T) {
	steps := generateSliderTrajectory(0, 0, 600)

	if len(steps) > 55 {
		t.Errorf("长距离步数太多：%d（含微调不应超过 ~53）", len(steps))
	}

	lastX := steps[len(steps)-1].x
	if lastX < 595 {
		t.Errorf("长距离终点 X 应接近 600，实际 %.1f", lastX)
	}
}

// TestAgentDecisionJSON Agent 决策 JSON 结构测试。
func TestAgentDecisionJSON(t *testing.T) {
	// 模拟 LLM 返回的 JSON
	jsonStr := `{"tool": "browser_click", "args": {"selector": "#close-btn"}, "reasoning": "看到弹窗需要关闭", "done": false}`

	var d AgentDecision
	if err := json.Unmarshal([]byte(jsonStr), &d); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	if d.Tool != "browser_click" {
		t.Errorf("Tool = %s, want browser_click", d.Tool)
	}
	if d.Done {
		t.Error("Done 应为 false")
	}
	if d.Reasoning != "看到弹窗需要关闭" {
		t.Errorf("Reasoning = %s", d.Reasoning)
	}

	// 验证 Args 可以二次解析
	var args struct {
		Selector string `json:"selector"`
	}
	if err := json.Unmarshal(d.Args, &args); err != nil {
		t.Fatalf("Args 解析失败: %v", err)
	}
	if args.Selector != "#close-btn" {
		t.Errorf("Args.Selector = %s, want #close-btn", args.Selector)
	}
}

// TestAllDeclarations 工具集声明完整性测试。
func TestAllDeclarations(t *testing.T) {
	// 注意：不能真正创建 BrowserSession（需要 chromedp context），
	// 但 Declaration 不依赖 context，所以可以用 nil session 测试声明。
	session := &BrowserSession{}
	decls := AllDeclarations(session)

	expectedTools := []string{
		"browser_screenshot", "browser_click", "browser_type",
		"browser_upload", "browser_navigate", "browser_wait",
		"browser_scroll", "browser_solve_slider", "browser_get_text",
		"browser_done",
	}

	if len(decls) != len(expectedTools) {
		t.Errorf("工具数量不匹配：%d（期望 %d）", len(decls), len(expectedTools))
	}

	found := make(map[string]bool)
	for _, d := range decls {
		found[d.Name] = true
		if d.Description == "" {
			t.Errorf("工具 %s 缺少描述", d.Name)
		}
		if d.InputSchema == nil {
			t.Errorf("工具 %s 缺少 InputSchema", d.Name)
		}
	}

	for _, name := range expectedTools {
		if !found[name] {
			t.Errorf("缺少工具 %s", name)
		}
	}
}
