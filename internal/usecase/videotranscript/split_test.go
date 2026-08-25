package videotranscript

import (
	"strings"
	"testing"
)

func TestSplitSentences(t *testing.T) {
	raw := "嗨，大家好，我是新路，欢迎来到我们正式的第一节课。那这节课我们就要来创建一个全新的项目，这套教程呢，我会全程使用五点七点四来录制，那对于大家而言呢，建议大家选择五点五以上的版本，不要选择过老的版本。好，那我们现在就来启动引擎？下节课再见，拜拜！"
	lines := splitSentences(raw)
	t.Logf("切分出 %d 行：", len(lines))
	for i, l := range lines {
		t.Logf("  %2d| %s", i+1, l)
	}
	if len(lines) < 4 {
		t.Fatalf("句号/问号/感叹号切分不足: %d", len(lines))
	}
	for _, l := range lines {
		if strings.Count(l, "\n") > 0 {
			t.Fatal("行内含换行")
		}
	}
}

func TestSplitLongSentence(t *testing.T) {
	// 超长句（>80字）按逗号二次切分
	long := make([]byte, 0, 300)
	for i := 0; i < 20; i++ {
		long = append(long, []byte("这是一个非常长的句子片段，")...)
	}
	long = append(long, []byte("最后结束。")...)
	lines := splitSentences(string(long))
	t.Logf("超长句（%d字）切分为 %d 行", len([]rune(string(long))), len(lines))
	if len(lines) < 2 {
		t.Fatal("超长句未二次切分")
	}
	for _, l := range lines {
		if len([]rune(l)) > 100 {
			t.Fatalf("存在过长行 %d 字", len([]rune(l)))
		}
	}
}
