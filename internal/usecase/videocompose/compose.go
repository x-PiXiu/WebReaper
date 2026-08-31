// compose.go B-Roll 合成任务编排（22 号计划阶段三——§5.3 校验/§10.1 执行设计）。
package videocompose

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// composeSem ffmpeg 并发闸门（22 计划 §10.1②：每个 compose 是吃 CPU 的编码进程，
// 信号量限 2 并发——排队不拒绝；任务取消时 ctx 让位退出排队）。
var composeSem = make(chan struct{}, 2)

// composeTimeout 单次合成上限（3 分钟视频 veryfast 实测秒级；10 分钟兜底）。
const composeTimeout = 10 * time.Minute

// maxMediaBytes 单个下载文件上限（源成片/片段——对齐 safeDownload 500MB）。
const maxMediaBytes = 500 << 20

// SubmitCompose 提交合成：校验 → 创建 compose 任务 → 后台执行。
// 实现 port.Composer.SubmitCompose（generation.UnifiedSubmit 的 compose 类型分发入口）。
func (uc *UseCase) SubmitCompose(ctx context.Context, in port.ComposeInput) (port.ComposeResult, error) {
	if uc.av == nil || !uc.av.Available() {
		return port.ComposeResult{}, fmt.Errorf("ffmpeg 不可用——请联系管理员安装")
	}
	if in.SourceTaskID == "" || len(in.Segments) == 0 {
		return port.ComposeResult{}, fmt.Errorf("缺少源任务或插入片段")
	}
	if len(in.Segments) > 20 {
		return port.ComposeResult{}, fmt.Errorf("插入片段最多 20 个（当前 %d）", len(in.Segments))
	}

	// 源任务与时间轴（链式继承：compose 产物的 timeline 直接复用——音频直拷时间轴不变）
	src, err := uc.tasks.FindByID(ctx, in.TenantID, in.SourceTaskID)
	if err != nil {
		return port.ComposeResult{}, fmt.Errorf("源任务不存在")
	}
	meta, ok := parseTimeline(src.TimelineJSON)
	if !ok {
		return port.ComposeResult{}, fmt.Errorf("源成片尚未定位台词时间轴——请先调用 POST timeline 定位")
	}

	// §5.3 校验：句号有效 + 同句重复检测（29 号：重叠检测已移除——重叠时后续片段优先）
	var resolved []resolvedSeg
	for _, s := range in.Segments {
		if s.SentenceIndex < 0 || s.SentenceIndex >= len(meta.Lines) {
			return port.ComposeResult{}, fmt.Errorf("句号越界: %d（有效范围 0~%d）", s.SentenceIndex, len(meta.Lines)-1)
		}
		if s.MediaURL == "" {
			return port.ComposeResult{}, fmt.Errorf("第 %d 句的片段地址为空", s.SentenceIndex)
		}
		line := meta.Lines[s.SentenceIndex]
		if line.EndMs <= line.StartMs {
			return port.ComposeResult{}, fmt.Errorf("第 %d 句时间窗为空", s.SentenceIndex)
		}
		resolved = append(resolved, resolvedSeg{
			spec: port.InsertSegmentSpec{StartMs: line.StartMs, EndMs: line.EndMs},
			idx:  s.SentenceIndex,
			url:  s.MediaURL,
		})
	}
	// 29号改进：保留重复检测，移除重叠检测（重叠时后续片段优先）
	for i := 0; i < len(resolved); i++ {
		for j := i + 1; j < len(resolved); j++ {
			if resolved[i].idx == resolved[j].idx {
				return port.ComposeResult{}, fmt.Errorf("第 %d 句重复配置片段", resolved[i].idx)
			}
		}
	}

	// 创建 compose 任务（状态机：queueing → processing → success/failed/cancelled）
	now := time.Now()
	params := map[string]any{
		"source_task_id": in.SourceTaskID,
		"segments":       in.Segments,
		"script":         taskScript(src), // 台词随链继承（链式再 compose 直接可用）
	}
	paramsJSON, _ := json.Marshal(params)
	task := entity.GenerationTask{
		ID:           fmt.Sprintf("comp-%d", now.UnixNano()),
		TenantID:     in.TenantID,
		BrandID:      in.BrandID,
		Type:         entity.GenerationTypeOther,
		SubType:      "compose",
		Model:        "local-ffmpeg",
		Provider:     "local",
		State:        entity.TaskStateQueueing,
		ParamsJSON:   string(paramsJSON),
		TimelineJSON: src.TimelineJSON, // 链式继承（§10.1④：禁止重检测）
	}
	if err := uc.tasks.Save(ctx, task); err != nil {
		return port.ComposeResult{}, fmt.Errorf("创建合成任务失败: %w", err)
	}

	// 后台执行（闭包拷贝所需数据——不复用请求 ctx）
	go uc.execute(task.ID, in.TenantID, taskCreationURL(src), resolved)
	return port.ComposeResult{TaskID: task.ID, State: task.State}, nil
}

// resolvedSeg 校验通过的片段（窗口 + 句号 + 素材 URL）。
type resolvedSeg struct {
	spec port.InsertSegmentSpec
	idx  int
	url  string
}

// execute 后台执行：排队（闸门）→ 下载（SSRF 校验）→ 预检（视频流/图片预转）→ 合成 → 产物回写。
func (uc *UseCase) execute(taskID, tenantID, mainURL string, segs []resolvedSeg) {
	ctx, cancel := context.WithTimeout(context.Background(), composeTimeout)
	defer cancel()

	// 状态回写用独立短超时 ctx（主 ctx 超时后仍可写终态）
	setState := func(state, errMsg string) {
		wctx, wcancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer wcancel()
		t, err := uc.tasks.FindByID(wctx, tenantID, taskID)
		if err != nil {
			log.Printf("[videocompose] 任务读取失败 %s: %v", taskID, err)
			return
		}
		t.State = state
		t.ErrMsg = errMsg
		_ = uc.tasks.Save(wctx, t)
	}

	// ① 并发闸门（ctx 取消即让位——任务被 Cancel 时退出排队）
	select {
	case composeSem <- struct{}{}:
		defer func() { <-composeSem }()
	case <-ctx.Done():
		setState(entity.TaskStateCancelled, "排队中超时/取消")
		return
	}
	setState(entity.TaskStateProcessing, "")

	// ② 安全下载全部媒体（SSRF 校验 + 500MB 上限——§10.1①）
	mainPath, cleanupMain, err := safeDownloadMedia(ctx, mainURL)
	if err != nil {
		setState(entity.TaskStateFailed, fmt.Sprintf("源成片下载失败: %v", err))
		return
	}
	defer cleanupMain()

	segPaths := make([]string, len(segs))
	for i, s := range segs {
		p, cleanup, dErr := safeDownloadMedia(ctx, s.url)
		if dErr != nil {
			setState(entity.TaskStateFailed, fmt.Sprintf("片段下载失败（第 %d 句）: %v", s.idx, dErr))
			return
		}
		defer cleanup()
		segPaths[i] = p
	}

	// ③ 片段预检（§5.3-4：视频流校验——图片 mjpeg 单帧也过；纯音频拒绝）+ 图片预转视频
	specs := make([]port.InsertSegmentSpec, len(segs))
	for i, r := range segs {
		segPath := segPaths[i]
		hasVideo, perr := uc.av.ProbeHasVideoStream(ctx, segPath)
		if perr != nil {
			setState(entity.TaskStateFailed, fmt.Sprintf("片段探测失败（第 %d 句）: %v", r.idx, perr))
			return
		}
		if !hasVideo {
			setState(entity.TaskStateFailed, fmt.Sprintf("第 %d 句的素材不含画面（纯音频不支持插入）", r.idx))
			return
		}
		// 图片形态预转 loop 视频（compose 输入统一视频形态）
		if isImagePath(segPath) {
			dur := float64(r.spec.EndMs-r.spec.StartMs) / 1000
			loopPath := segPath + ".loop.mp4"
			if lerr := uc.av.(imageLooper).StaticImageToVideo(ctx, segPath, dur, loopPath); lerr != nil {
				setState(entity.TaskStateFailed, fmt.Sprintf("图片转视频失败（第 %d 句）: %v", r.idx, lerr))
				return
			}
			segPath = loopPath
		}
		specs[i] = port.InsertSegmentSpec{StartMs: r.spec.StartMs, EndMs: r.spec.EndMs, MediaPath: segPath}
	}

	// ④ 合成（滤镜链细节全在 adapter）
	outPath := mainPath + ".broll.mp4"
	if cerr := uc.av.ComposeInsertSegments(ctx, mainPath, specs, outPath); cerr != nil {
		setState(entity.TaskStateFailed, fmt.Sprintf("合成失败: %v", cerr))
		return
	}
	defer os.Remove(outPath)

	// ⑤ 产物入库（窄接口上传——未注入 store 时报明确错误）
	outURL, upErr := uc.uploadCreation(ctx, tenantID, outPath)
	if upErr != nil {
		setState(entity.TaskStateFailed, fmt.Sprintf("产物上传失败: %v", upErr))
		return
	}
	wctx, wcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer wcancel()
	t, terr := uc.tasks.FindByID(wctx, tenantID, taskID)
	if terr == nil {
		t.State = entity.TaskStateSuccess
		creations, _ := json.Marshal([]map[string]any{{"id": taskID, "url": outURL, "stored_at": time.Now().Format(time.RFC3339)}})
		t.CreationsJSON = string(creations)
		_ = uc.tasks.Save(wctx, t)
	}
	log.Printf("[videocompose] 合成完成 %s → %s", taskID, outURL)
}

// imageLooper StaticImageToVideo 的窄接口（port.MediaAVTool 主体外的 adapter 附加能力）。
type imageLooper interface {
	StaticImageToVideo(ctx context.Context, imgPath string, durSec float64, outPath string) error
}

// CreationUploader 产物上传窄接口（main 装配注入 mediaStore 适配器）。
type CreationUploader interface {
	Upload(ctx context.Context, tenantID, localPath, kind string) (url string, err error)
}

// safeDownloadMedia SSRF 防护下载（§10.1①——与 videotranscript.safeDownload 同级校验）。
func safeDownloadMedia(ctx context.Context, rawURL string) (path string, cleanup func(), err error) {
	u, perr := url.Parse(rawURL)
	if perr != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", nil, fmt.Errorf("仅支持 http/https")
	}
	ips, lerr := net.LookupIP(u.Hostname())
	if lerr != nil {
		return "", nil, fmt.Errorf("域名无法解析")
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return "", nil, fmt.Errorf("地址被拒绝")
		}
	}
	req, rerr := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if rerr != nil {
		return "", nil, rerr
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36")
	resp, derr := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if derr != nil {
		return "", nil, derr
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	ext := ".mp4"
	if isImagePath(rawURL) {
		ext = pathExtDefault(rawURL, ".jpg")
	}
	f, ferr := os.CreateTemp("", "broll-*"+ext)
	if ferr != nil {
		return "", nil, ferr
	}
	cleanup = func() { f.Close(); os.Remove(f.Name()) }
	n, cerr := io.Copy(f, io.LimitReader(resp.Body, maxMediaBytes+1))
	if cerr != nil {
		cleanup()
		return "", nil, cerr
	}
	if n > maxMediaBytes {
		cleanup()
		return "", nil, fmt.Errorf("超过 500MB 上限")
	}
	if n == 0 {
		cleanup()
		return "", nil, fmt.Errorf("内容为空")
	}
	if n < 1024 {
		cleanup()
		return "", nil, fmt.Errorf("文件过小（%d 字节）——可能是错误页/重定向页而非实际媒体", n)
	}
	f.Close()
	return f.Name(), cleanup, nil
}

// safeDownloadTaskMedia 成片获取（timeline 定位用——复用 SSRF 下载）。
func (uc *UseCase) safeDownloadTaskMedia(ctx context.Context, task entity.GenerationTask) (string, func(), error) {
	u := taskCreationURL(task)
	if u == "" {
		return "", func() {}, fmt.Errorf("任务无成片产物")
	}
	return safeDownloadMedia(ctx, u)
}

// taskCreationURL 任务产物 URL（CreationsJSON 首个 url）。
func taskCreationURL(t entity.GenerationTask) string {
	var cs []map[string]any
	if err := json.Unmarshal([]byte(t.CreationsJSON), &cs); err != nil || len(cs) == 0 {
		return ""
	}
	if u, ok := cs[0]["url"].(string); ok {
		return u
	}
	return ""
}

func isImagePath(p string) bool {
	l := strings.ToLower(p)
	return strings.Contains(l, ".jpg") || strings.Contains(l, ".jpeg") || strings.Contains(l, ".png") || strings.Contains(l, ".webp")
}

func pathExtDefault(p, def string) string {
	if i := strings.LastIndex(p, "."); i > 0 {
		return p[i:]
	}
	return def
}
