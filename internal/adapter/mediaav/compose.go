// compose.go B-Roll 画面插入合成的 ffmpeg 实现（22 号计划 §3——命令已实测验证）。
//
// 滤镜链（每片段一段）：
//
//	[i:v]scale=W:H:force_original_aspect_ratio=increase,crop=W:H,setsar=1,
//	     tpad=stop_mode=clone:stop_duration=3600[si];
//	[prev][si]overlay=0:0:enable='between(t,S,E)':shortest=1[next]
//
// 音频只映射主视频流（-map 0:a:0 -c:a copy 直拷）——片段音轨不映射即剥离。
package mediaav

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"webreaper/internal/usecase/port"
)

// ComposeInsertSegments 实现 port.MediaAVTool.ComposeInsertSegments。
func (t *FFmpegTool) ComposeInsertSegments(ctx context.Context, mainVideoPath string, segs []port.InsertSegmentSpec, outPath string) error {
	if len(segs) == 0 {
		return fmt.Errorf("无插入片段")
	}
	// 主视频分辨率探测（片段 cover 适配目标）
	w, h, err := t.probeVideoSize(mainVideoPath)
	if err != nil {
		return fmt.Errorf("探测主视频分辨率失败: %w", err)
	}

	args := []string{"-y", "-i", mainVideoPath}
	filterParts := make([]string, 0, len(segs)*2+1)
	prev := "[0:v]"
	for i, seg := range segs {
		inputIdx := i + 1
		args = append(args, "-i", seg.MediaPath)
		out := fmt.Sprintf("[v%d]", inputIdx)
		// 29号改进：只做缩放+裁剪，不加 tpad（不定格）
		// 片段播完后 overlay 自动失效，原片自然显示
		filterParts = append(filterParts, fmt.Sprintf(
			"[%d:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,setsar=1[s%d]",
			inputIdx, w, h, w, h, inputIdx))
		// 29号改进：移除 shortest=1，让片段自然播放
		// enable='between(t,S,E)' 控制显示时间窗
		// 片段比时间窗长时继续播放（不截断），片段比时间窗短时原片自然显示（不定格）
		filterParts = append(filterParts, fmt.Sprintf(
			"%s[s%d]overlay=0:0:enable='between(t,%.3f,%.3f)'%s",
			prev, inputIdx, float64(seg.StartMs)/1000, float64(seg.EndMs)/1000, out))
		prev = out
	}
	filter := strings.Join(filterParts, ";")

	args = append(args,
		"-filter_complex", filter,
		"-map", prev,
		"-map", "0:a:0", // 口播音频直拷；片段音轨不映射即剥离（22 计划已确认）
		"-c:v", "libx264", "-preset", "veryfast",
		"-c:a", "copy",
		outPath,
	)

	cmd := exec.CommandContext(ctx, t.bin("ffmpeg"), args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg 合成失败: %w | 输出尾: %.400s", err, out)
	}
	return nil
}

// probeVideoSize ffprobe 取视频流宽高。
func (t *FFmpegTool) probeVideoSize(path string) (w, h int, err error) {
	cmd := exec.Command(t.bin("ffprobe"),
		"-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-of", "csv=p=0", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("ffprobe 输出异常: %q", string(out))
	}
	if w, err = strconv.Atoi(strings.TrimSpace(parts[0])); err != nil {
		return 0, 0, err
	}
	if h, err = strconv.Atoi(strings.TrimSpace(parts[1])); err != nil {
		return 0, 0, err
	}
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("分辨率非法 %dx%d", w, h)
	}
	return w, h, nil
}

// ProbeHasVideoStream 实现 port.MediaAVTool.ProbeHasVideoStream。
func (t *FFmpegTool) ProbeHasVideoStream(ctx context.Context, mediaPath string) (bool, error) {
	cmd := exec.CommandContext(ctx, t.bin("ffprobe"),
		"-v", "error", "-select_streams", "v",
		"-show_entries", "stream=codec_type", "-of", "csv=p=0", mediaPath)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("ffprobe 探测失败: %w", err)
	}
	return strings.Contains(string(out), "video"), nil
}

// StaticImageToVideo 图片转静态视频（compose 片段输入统一为视频形态）。
// 效果：图片静态显示 durSec 秒（不循环动画），到时间窗结束自动切换回原片。
// 技术：ffmpeg -loop 1 保持图片持续输出，-t 截止到指定时长。
func (t *FFmpegTool) StaticImageToVideo(ctx context.Context, imgPath string, durSec float64, outPath string) error {
	cmd := exec.CommandContext(ctx, t.bin("ffmpeg"), "-y",
		"-loop", "1", "-t", fmt.Sprintf("%.2f", durSec), "-i", imgPath,
		"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("图片转视频失败: %w | %.300s", err, out)
	}
	return nil
}
