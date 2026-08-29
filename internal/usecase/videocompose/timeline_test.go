// timeline_test.go 配对算法/ASR 分行/边界语义单测（22 号计划 §4）。
package videocompose

import (
	"testing"

	"webreaper/internal/usecase/port"
)

func TestAlignSegmentsDirect(t *testing.T) {
	// M==N 直配
	segs := []port.SpeechSegment{
		{StartMs: 500, EndMs: 3120},
		{StartMs: 3400, EndMs: 6850},
	}
	lines := []string{"大家好今天给大家介绍", "我们家的酸菜鱼是现杀的"}
	meta := alignSegments(segs, lines)

	if meta.AlignMode != port.TimelineAlignDirect {
		t.Fatalf("应 direct 配对，得到 %s", meta.AlignMode)
	}
	if len(meta.Lines) != 2 {
		t.Fatalf("行数=%d 期望 2", len(meta.Lines))
	}
	// 窗口边界语义：第一行起点= start-150ms（无前静音段）；第二行起点=前一行 End 与本行 start 的中点
	if meta.Lines[0].StartMs != 350 { // 500-150
		t.Errorf("第0行起点=%d 期望 350", meta.Lines[0].StartMs)
	}
	if meta.Lines[0].EndMs != 3120 {
		t.Errorf("第0行终点=%d 期望 3120", meta.Lines[0].EndMs)
	}
	if meta.Lines[1].StartMs != (3120+3400)/2 { // 静音中点
		t.Errorf("第1行起点=%d 期望 %d", meta.Lines[1].StartMs, (3120+3400)/2)
	}
}

func TestAlignSegmentsMerge(t *testing.T) {
	// M>N：5 段 vs 3 行——按字符比例合并
	segs := []port.SpeechSegment{
		{0, 1000}, {1000, 2000}, {2000, 3000}, {3000, 4000}, {4000, 5000},
	}
	lines := []string{"第一句比较短", "第二句稍微长一些", "第三句"}
	meta := alignSegments(segs, lines)
	if meta.AlignMode != port.TimelineAlignEstimated {
		t.Fatalf("M>N 应 estimated，得到 %s", meta.AlignMode)
	}
	if len(meta.Lines) != 3 {
		t.Fatalf("行数=%d 期望 3", len(meta.Lines))
	}
	// 验证窗口连续无重叠
	for i := 1; i < len(meta.Lines); i++ {
		if meta.Lines[i].StartMs < meta.Lines[i-1].EndMs {
			t.Errorf("行 %d 起点重叠上一行终点", i)
		}
	}
}

func TestSplitBySegmentsASR(t *testing.T) {
	// C 路径 ASR 分行：全文按段时长比例切
	segs := []port.SpeechSegment{
		{0, 3000}, {3500, 8000}, {8500, 10000},
	}
	meta := splitBySegments(segs, "第一段文字内容 第二段文字更长的内容 第三段")
	if meta.ScriptSource != port.TimelineScriptSourceASR {
		t.Fatalf("script_source=%s 期望 asr", meta.ScriptSource)
	}
	if len(meta.Lines) != 3 {
		t.Fatalf("行数=%d 期望 3（与段一致）", len(meta.Lines))
	}
	if meta.Lines[0].StartMs != 0 || meta.Lines[0].EndMs != 3000 {
		t.Errorf("第0行窗口 [%d,%d] 期望 [0,3000]", meta.Lines[0].StartMs, meta.Lines[0].EndMs)
	}
	if meta.Lines[2].Text == "" {
		t.Error("末行文字为空")
	}
}

func TestParseTimelineEmpty(t *testing.T) {
	if _, ok := parseTimeline(""); ok {
		t.Error("空串应视为未定位")
	}
	if _, ok := parseTimeline("{bad json"); ok {
		t.Error("损坏 JSON 应视为未定位")
	}
	meta, ok := parseTimeline(`{"lines":[{"index":0,"text":"a","start_ms":0,"end_ms":100}]}`)
	if !ok || len(meta.Lines) != 1 {
		t.Error("合法 JSON 应解析成功")
	}
}

func TestSplitLines(t *testing.T) {
	got := splitLines("第一行\n\n  第二行  \n第三行")
	if len(got) != 3 {
		t.Fatalf("行数=%d 期望 3", len(got))
	}
	if got[1] != "第二行" {
		t.Errorf("第1行=%q 应已 trim", got[1])
	}
}
