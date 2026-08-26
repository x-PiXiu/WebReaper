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

func TestVideoCodecAndIsH264(t *testing.T) {
	tool := NewFFmpegTool("")
	if !tool.Available() {
		t.Skip("ffmpeg 不在 PATH 中")
	}

	// 生成一个 H.264 测试视频
	dir := t.TempDir()
	mp4 := filepath.Join(dir, "test.mp4")
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "color=c=blue:s=320x240:d=1",
		"-c:v", "libx264", "-preset", "ultrafast", mp4)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("生成测试视频失败: %v: %s", err, out)
	}

	ctx := context.Background()

	codec := tool.VideoCodec(ctx, mp4)
	if codec != "h264" {
		t.Errorf("VideoCodec = %q, want \"h264\"", codec)
	}
	t.Logf("VideoCodec: %s ✓", codec)

	if !tool.IsH264(ctx, mp4) {
		t.Error("H.264 视频 IsH264 应返回 true")
	}
	t.Log("IsH264: true ✓")

	// 生成一个 VP9 测试视频（非 H.264）
	webm := filepath.Join(dir, "test.webm")
	cmd2 := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "color=c=red:s=320x240:d=1",
		"-c:v", "libvpx-vp9", webm)
	if out, err := cmd2.CombinedOutput(); err != nil {
		t.Skipf("VP9 编码不可用，跳过非 H.264 测试: %v: %s", err, out)
	}

	codec2 := tool.VideoCodec(ctx, webm)
	if codec2 == "h264" {
		t.Errorf("VP9 视频 VideoCodec 不应返回 \"h264\"，得到 %q", codec2)
	}
	t.Logf("VP9 VideoCodec: %s ✓", codec2)

	if tool.IsH264(ctx, webm) {
		t.Error("VP9 视频 IsH264 应返回 false")
	}
	t.Log("VP9 IsH264: false ✓")
}
