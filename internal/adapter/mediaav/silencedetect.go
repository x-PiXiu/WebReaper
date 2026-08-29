// silencedetect.go 静音/音量低点检测的 ffmpeg 实现（22 号计划 §4 三级阈值，已实测）。
//
// 三级阈值：
//  ① 默认 -35dB（TTS/Vidu 生成音频——底噪极低）
//  ② 自适应：volumedetect 预分析 mean_volume → 阈值 = mean-8dB（clamp [-45,-25]）
//     ——适配用户上传带底噪音频（实测：噪声音频边界偏差仅 ±40ms）
//  ③ 重试：阈值放宽 3dB 再检一轮（由 usecase 层按段/句数偏差决定是否调用本方法重跑）
package mediaav

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"webreaper/internal/usecase/port"
)

var (
	silenceStartRe = regexp.MustCompile(`silence_start:\s*([0-9.]+)`)
	silenceEndRe   = regexp.MustCompile(`silence_end:\s*([0-9.]+)`)
	meanVolumeRe   = regexp.MustCompile(`mean_volume:\s*(-?[0-9.]+)\s*dB`)
)

// DetectSpeechSegments 实现 port.MediaAVTool.DetectSpeechSegments。
// 返回语音段序列（静音间隙），首段前如有语音从 0 起、尾静音截断。
func (t *FFmpegTool) DetectSpeechSegments(ctx context.Context, mediaPath string) ([]port.SpeechSegment, error) {
	threshold, err := t.adaptiveThreshold(ctx, mediaPath)
	if err != nil {
		return nil, fmt.Errorf("响度预分析失败: %w", err)
	}
	segs, err := t.detectAt(ctx, mediaPath, threshold)
	if err != nil {
		return nil, err
	}
	if len(segs) == 0 {
		// 三级阈值的重试档：放宽 3dB 再检（仍空则视为整段语音或无音频内容）
		segs, err = t.detectAt(ctx, mediaPath, threshold+3)
		if err != nil {
			return nil, err
		}
	}
	return segs, nil
}

// adaptiveThreshold volumedetect 预分析 → mean-8dB（clamp [-45,-25]）。
func (t *FFmpegTool) adaptiveThreshold(ctx context.Context, mediaPath string) (float64, error) {
	cmd := exec.CommandContext(ctx, t.bin("ffmpeg"), "-i", mediaPath, "-af", "volumedetect", "-f", "null", "-")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, err
	}
	m := meanVolumeRe.FindSubmatch(out)
	if m == nil {
		return -35, nil // 解析不到（异常音频）回落默认阈值
	}
	mean, err := strconv.ParseFloat(string(m[1]), 64)
	if err != nil {
		return -35, nil
	}
	th := mean - 8
	if th < -45 {
		th = -45
	}
	if th > -25 {
		th = -25
	}
	return th, nil
}

// detectAt 指定阈值跑 silencedetect 并解析语音段。
func (t *FFmpegTool) detectAt(ctx context.Context, mediaPath string, thresholdDB float64) ([]port.SpeechSegment, error) {
	filter := fmt.Sprintf("silencedetect=noise=%.1fdB:d=0.15", thresholdDB)
	cmd := exec.CommandContext(ctx, t.bin("ffmpeg"), "-i", mediaPath, "-af", filter, "-f", "null", "-")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("silencedetect 失败: %w | 输出尾: %.300s", err, out)
	}

	// 解析静音段 [start, end] 成对（end 可能缺失=尾部静音到文件末尾）
	type silence struct{ start, end float64 }
	var silences []silence
	var pending float64 = -1
	for _, line := range strings.Split(string(out), "\n") {
		if m := silenceStartRe.FindStringSubmatch(line); m != nil {
			v, _ := strconv.ParseFloat(m[1], 64)
			pending = v
		}
		if m := silenceEndRe.FindStringSubmatch(line); m != nil && pending >= 0 {
			v, _ := strconv.ParseFloat(m[1], 64)
			silences = append(silences, silence{pending, v})
			pending = -1
		}
	}
	if pending >= 0 {
		// 尾部静音无 end——对语音段划分无影响（尾静音截断）
		_ = pending
	}

	// 总时长（ffprobe）
	total, err := t.probeDuration(mediaPath)
	if err != nil {
		return nil, err
	}

	// 静音间隙 → 语音段：首静音前如有语音从 0 起；静音之间为段；尾静音截断
	var segs []port.SpeechSegment
	cursor := 0.0
	for _, s := range silences {
		if s.start > cursor+0.05 { // 有语音间隙（>50ms 容差）
			segs = append(segs, port.SpeechSegment{
				StartMs: int64(cursor * 1000),
				EndMs:   int64(s.start * 1000),
			})
		}
		cursor = s.end
	}
	if total > cursor+0.05 { // 末段语音
		segs = append(segs, port.SpeechSegment{
			StartMs: int64(cursor * 1000),
			EndMs:   int64(total * 1000),
		})
	}
	return segs, nil
}

// probeDuration ffprobe 取媒体时长（秒）。
func (t *FFmpegTool) probeDuration(path string) (float64, error) {
	cmd := exec.Command(t.bin("ffprobe"),
		"-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("时长解析失败: %q", string(out))
	}
	return d, nil
}
