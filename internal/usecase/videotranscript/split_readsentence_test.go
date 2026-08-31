package videotranscript

import (
	"strings"
	"testing"
)

// 真实 mimo-v2.5-asr 输出（22s 语音 116 字，仅 2 个句号、其余全逗号）——
// 修复前：2 行（45/71 字巨行，客户端"没按句显示"的根因样本）。
func TestSplitSentencesRealASRText(t *testing.T) {
	text := "Video是由声数科技联合清华大学正式发布的中国首个长时长、高一致性、高动态性视频大模型。Video在语义理解、推理速度、动态幅度等方面具备领先优势，并上线了全球首个多主体参考功能，突破视频模型一致性生成难题，开启了视觉上下文时代。"
	lines := splitSentences(text)
	t.Logf("行数=%d", len(lines))
	for i, l := range lines {
		t.Logf("  [%d] %d字: %s", i, len([]rune(l)), l)
	}
	if len(lines) < 5 {
		t.Fatalf("真实 ASR 稀疏标点文本应切出 ≥5 个读句，实际 %d 行", len(lines))
	}
	for i, l := range lines {
		// 聚合边界允许最后一个子句超出（子句不再拆散），其余行应在目标长度内
		if len([]rune(l)) > readSentenceMaxRunes+12 {
			t.Errorf("行[%d]过长（%d字）: %s", i, len([]rune(l)), l)
		}
	}
	// 行序拼接内容无损（去掉切分引入的空白后与原文一致）
	joined := strings.Join(lines, "")
	if joined != text {
		t.Errorf("切分内容有损:\n原: %s\n拼: %s", text, joined)
	}
}

// 无标点极端串（兜底硬切）
func TestSplitSentencesNoPunctuation(t *testing.T) {
	text := strings.Repeat("这是一个没有任何标点的超长语音识别结果", 8) // 176 字无标点
	lines := splitSentences(text)
	if len(lines) < 6 {
		t.Fatalf("无标点长串应硬切为多行，实际 %d 行", len(lines))
	}
	if strings.Join(lines, "") != text {
		t.Errorf("硬切内容有损")
	}
}

// 短句保持原样（不破坏既有短文本）
func TestSplitSentencesShortKept(t *testing.T) {
	text := "大家好。今天介绍一家成都老店！开了二十年了。"
	lines := splitSentences(text)
	if len(lines) != 3 {
		t.Fatalf("短句应 3 行，实际 %d: %v", len(lines), lines)
	}
}
