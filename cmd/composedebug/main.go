// composedebug：B-Roll 画面插入合成 CLI 验证工具（22 号计划阶段一）。
//
// 用真实文件直接合成，验证滤镜链（窗口切换/口型/音频直拷/定格/多片段）：
//
//	go run ./cmd/composedebug -main main.mp4 -out out.mp4 \
//	  -seg 4-9=broll.mp4 -seg 20-25=other.jpg
//
// seg 格式：起秒-止秒=片段路径（可多个；图片自动走 loop 形态由 ffmpeg 输入探测处理——
// 本工具对图片补 -loop 1 -t 参数）。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"webreaper/internal/adapter/mediaav"
	"webreaper/internal/usecase/port"
)

func main() {
	mainPath := flag.String("main", "", "主视频（口播成片）路径")
	outPath := flag.String("out", "data/composedebug-out.mp4", "输出路径")
	segArg := flag.String("seg", "", "插入片段，格式 起秒-止秒=路径（逗号分隔多个）")
	flag.Parse()
	if *mainPath == "" || *segArg == "" {
		fmt.Println("用法: go run ./cmd/composedebug -main main.mp4 -seg 4-9=broll.mp4[,20-25=x.jpg] [-out out.mp4]")
		os.Exit(1)
	}

	tool := mediaav.NewFFmpegTool("")
	if !tool.Available() {
		fmt.Println("❌ ffmpeg/ffprobe 不可用")
		os.Exit(1)
	}

	var specs []port.InsertSegmentSpec
	for _, part := range strings.Split(*segArg, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		eq := strings.Index(part, "=")
		if eq < 0 {
			fmt.Printf("❌ seg 格式错误（缺 =）: %q\n", part)
			os.Exit(1)
		}
		window, path := part[:eq], strings.TrimSpace(part[eq+1:])
		dash := strings.Index(window, "-")
		if dash < 0 {
			fmt.Printf("❌ seg 时间窗格式错误（缺 -）: %q\n", part)
			os.Exit(1)
		}
		start, err1 := strconv.ParseFloat(window[:dash], 64)
		end, err2 := strconv.ParseFloat(window[dash+1:], 64)
		if err1 != nil || err2 != nil || end <= start {
			fmt.Printf("❌ seg 时间窗非法: %q\n", part)
			os.Exit(1)
		}
		// 图片补 loop 输入形态（视频原样）
		p := path
		if strings.HasSuffix(strings.ToLower(p), ".jpg") || strings.HasSuffix(strings.ToLower(p), ".png") {
			loopPath := p + ".loop.mp4"
			if e := loopImage(tool, p, end-start, loopPath); e != nil {
				fmt.Printf("❌ 图片转视频失败: %v\n", e)
				os.Exit(1)
			}
			p = loopPath
		}
		specs = append(specs, port.InsertSegmentSpec{
			StartMs:   int64(start * 1000),
			EndMs:     int64(end * 1000),
			MediaPath: p,
		})
		fmt.Printf("  片段: %.1fs~%.1fs ← %s\n", start, end, path)
	}

	fmt.Printf("合成中：%s + %d 片段 → %s\n", *mainPath, len(specs), *outPath)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	started := time.Now()
	if err := tool.ComposeInsertSegments(ctx, *mainPath, specs, *outPath); err != nil {
		fmt.Println("❌ 合成失败:", err)
		os.Exit(1)
	}
	fmt.Printf("✅ 合成完成（耗时 %s）→ %s\n", time.Since(started).Round(time.Millisecond), *outPath)

	// 顺带跑一次静音检测（阶段二自验）
	segOut, err := tool.DetectSpeechSegments(ctx, *mainPath)
	if err == nil {
		fmt.Printf("\n静音检测（同文件语音段，供阶段二核对）：\n")
		for i, s := range segOut {
			fmt.Printf("  [%d] %.3fs ~ %.3fs\n", i, float64(s.StartMs)/1000, float64(s.EndMs)/1000)
		}
	}
}

// loopImage 图片转 loop 视频（compose 输入统一为视频形态——adapter 的 tpad 定格
// 对视频最稳；图片由 CLI/usecase 层预转，adapter 保持单一输入形态）。
func loopImage(tool *mediaav.FFmpegTool, imgPath string, dur float64, outPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return tool.LoopImageToVideo(ctx, imgPath, dur, outPath)
}
