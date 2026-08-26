// Package mediaav 提供 ffmpeg/ffprobe 本地媒体处理（port.MediaAVTool 实现）。
//
// 职责（08 计划 D4 提取管线）：
//   - 软字幕轨探测/抽取：免费且 100% 精确（抖音/快手几乎全是硬字幕走不通；
//     B 站等有软字幕来源可白嫖）
//   - 音轨剥离：16kHz 单声道 mp3——5 分钟仅 2-3MB，绕开 ASR 接口 25MB 上限
//
// 二进制缺失（Available=false）时由用例层降级：视频 ≤25MB 整文件直传 ASR。
package mediaav

import (
	"context"
	"sort"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FFmpegTool port.MediaAVTool 的 ffmpeg 实现。
type FFmpegTool struct {
	binDir string // ffmpeg/ffprobe 所在目录（空=PATH 查找）

	availableOnce sync.Once
	available     bool
}

// NewFFmpegTool 创建媒体处理工具。binDir 为空时从 PATH 查找。
func NewFFmpegTool(binDir string) *FFmpegTool {
	return &FFmpegTool{binDir: binDir}
}

func (t *FFmpegTool) bin(name string) string {
	if t.binDir != "" {
		return filepath.Join(t.binDir, name)
	}
	return name
}

// Available ffmpeg/ffprobe 是否可用（首次调用探测后缓存）。
func (t *FFmpegTool) Available() bool {
	t.availableOnce.Do(func() {
		t.available = t.probe("ffprobe") == nil && t.probe("ffmpeg") == nil
	})
	return t.available
}

func (t *FFmpegTool) probe(bin string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, t.bin(bin), "-version").Run()
}

// ffprobeStreams ffprobe -show_streams 的流清单。
type ffprobeStreams struct {
	Streams []struct {
		CodecType string `json:"codec_type"` // video/audio/subtitle
		CodecName string `json:"codec_name"` // h264/hevc/vp9/aac/mp3 等
	} `json:"streams"`
}

// VideoCodec 返回视频流的编码名称（h264/hevc/vp9 等）；无视频流返回空字符串。
func (t *FFmpegTool) VideoCodec(ctx context.Context, mediaPath string) string {
	if !t.Available() {
		return ""
	}
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(pctx, t.bin("ffprobe"),
		"-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=codec_name",
		"-of", "json", mediaPath).Output()
	if err != nil {
		return ""
	}
	var streams ffprobeStreams
	if json.Unmarshal(raw, &streams) != nil || len(streams.Streams) == 0 {
		return ""
	}
	return streams.Streams[0].CodecName
}

// IsH264 视频是否为 H.264 编码（lip-sync 要求）。
func (t *FFmpegTool) IsH264(ctx context.Context, mediaPath string) bool {
	return t.VideoCodec(ctx, mediaPath) == "h264"
}

// ExtractSubtitle 探测软字幕轨并抽取文本（srt → 纯文本拼接）。
// ok=false：无字幕轨（调用方走音轨路径）。
func (t *FFmpegTool) ExtractSubtitle(ctx context.Context, mediaPath string) (string, bool, error) {
	if !t.Available() {
		return "", false, nil
	}
	// ① 探测字幕流
	pctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(pctx, t.bin("ffprobe"),
		"-v", "error", "-show_streams", "-select_streams", "s", "-of", "json", mediaPath).Output()
	if err != nil {
		return "", false, nil // 探测失败按无字幕处理（不阻断音轨路径）
	}
	var streams ffprobeStreams
	if json.Unmarshal(raw, &streams) != nil || len(streams.Streams) == 0 {
		return "", false, nil
	}
	// ② 抽取第一条字幕轨为 srt
	srtPath := mediaPath + ".srt"
	pctx2, cancel2 := context.WithTimeout(ctx, 60*time.Second)
	defer cancel2()
	if out, err := exec.CommandContext(pctx2, t.bin("ffmpeg"),
		"-y", "-i", mediaPath, "-map", "0:s:0", srtPath).CombinedOutput(); err != nil {
		return "", false, fmt.Errorf("抽字幕失败: %v: %s", err, truncate(string(out), 200))
	}
	data, err := os.ReadFile(srtPath)
	_ = os.Remove(srtPath) // 临时文件即用即删
	if err != nil || len(data) == 0 {
		return "", false, nil
	}
	return srtToText(string(data)), true, nil
}

// ExtractAudio 抽音轨为 16kHz 单声道 mp3（ASR 标准输入）。
func (t *FFmpegTool) ExtractAudio(ctx context.Context, mediaPath string) (string, error) {
	if !t.Available() {
		return "", fmt.Errorf("ffmpeg 不可用")
	}
	audioPath := mediaPath + ".16k.mp3"
	pctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if out, err := exec.CommandContext(pctx, t.bin("ffmpeg"),
		"-y", "-i", mediaPath, "-vn", "-ac", "1", "-ar", "16000", "-b:a", "64k",
		"-codec:a", "libmp3lame", audioPath).CombinedOutput(); err != nil {
		return "", fmt.Errorf("抽音轨失败: %v: %s", err, truncate(string(out), 200))
	}
	return audioPath, nil
}

// SegmentAudio 把音频按 segSeconds 切段（16kHz 单声道 mp3——与 ExtractAudio 同规格，
// segment muxer 流复制不重编码，秒级完成）。返回按序段文件路径（调用方负责删除）。
func (t *FFmpegTool) SegmentAudio(ctx context.Context, audioPath string, segSeconds int) ([]string, error) {
	if !t.Available() {
		return nil, fmt.Errorf("ffmpeg 不可用")
	}
	if segSeconds <= 0 {
		segSeconds = 300 // 默认 5 分钟/段
	}
	outPattern := audioPath + ".seg%03d.mp3"
	pctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if out, err := exec.CommandContext(pctx, t.bin("ffmpeg"),
		"-y", "-i", audioPath, "-f", "segment",
		"-segment_time", fmt.Sprintf("%d", segSeconds),
		"-ac", "1", "-ar", "16000", "-b:a", "64k", "-codec:a", "libmp3lame",
		outPattern).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("音频分段失败: %v: %s", err, truncate(string(out), 200))
	}
	// 按序收集段文件
	matches, _ := filepath.Glob(audioPath + ".seg*.mp3")
	sort.Strings(matches)
	if len(matches) == 0 {
		return nil, fmt.Errorf("分段后未产出段文件")
	}
	return matches, nil
}

// srtToText srt 字幕 → 纯文本（去序号/时间轴，按行合并为段落）。
func srtToText(srt string) string {
	var lines []string
	for _, line := range strings.Split(srt, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "-->") {
			continue
		}
		if isAllDigits(line) { // 序号行
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, " ")
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
