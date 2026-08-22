package mediaav

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFFmpegAvailable(t *testing.T) {
	tool := NewFFmpegTool("")
	if !tool.Available() {
		t.Skip("ffmpeg 不在 PATH 中——跳过（CI/无 ffmpeg 环境）")
	}
	t.Log("ffmpeg 可用 ✓")
}

func TestExtractAudio(t *testing.T) {
	tool := NewFFmpegTool("")
	if !tool.Available() {
		t.Skip("ffmpeg 不可用")
	}
	dir := t.TempDir()
	// 生成 3 秒测试音频（ffmpeg 内置 sine 源）
	src := filepath.Join(dir, "test.mp4")
	if err := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-c:a", "aac", src).Run(); err != nil {
		t.Fatalf("生成测试音频失败: %v", err)
	}
	audioPath, err := tool.ExtractAudio(context.Background(), src)
	if err != nil {
		t.Fatalf("抽音轨失败: %v", err)
	}
	defer os.Remove(audioPath)
	st, err := os.Stat(audioPath)
	if err != nil || st.Size() == 0 {
		t.Fatalf("抽音轨产物为空: %v", err)
	}
	t.Logf("抽音轨成功: %s (%d bytes)", audioPath, st.Size())
}

func TestExtractSubtitleAbsent(t *testing.T) {
	tool := NewFFmpegTool("")
	if !tool.Available() {
		t.Skip("ffmpeg 不可用")
	}
	dir := t.TempDir()
	// 生成纯音频（无字幕轨）——应返回 ok=false
	src := filepath.Join(dir, "nosub.mp4")
	if err := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		src).Run(); err != nil {
		t.Fatalf("生成测试文件失败: %v", err)
	}
	_, ok, err := tool.ExtractSubtitle(context.Background(), src)
	if err != nil {
		t.Fatalf("探测失败: %v", err)
	}
	if ok {
		t.Error("纯音频文件不应有字幕轨")
	}
	t.Log("无字幕轨正确返回 ok=false ✓")
}
