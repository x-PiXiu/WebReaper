// Package videotranscript 视频文案提取与改写（08 计划 D4/D2 第①②步）。
//
// 流程：resolve（分享链→直链）→ download（SSRF 防护下载）→ extract（软字幕轨
// 优先，无则 ffmpeg 抽音轨）→ transcribe（云 ASR）→ rewrite（LLM 清洗/改写双产出）。
// 原始视频即用即删（R5——只留提取产物，防 5GB 级临时文件吃爆磁盘）。
package videotranscript

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"webreaper/internal/usecase/port"
)

// UseCase 视频文案提取用例。
type UseCase struct {
	resolver    port.VideoLinkResolver  // 可选；nil=不支持分享链
	av          port.MediaAVTool        // 可选；nil/不可用=降级直传
	transcriber port.SpeechTranscriber  // 必需（未配置时报可读错误）
	ai          port.AIGenerator        // 改写双产出
	client      *http.Client
}

// NewUseCase 创建提取用例。
func NewUseCase(resolver port.VideoLinkResolver, av port.MediaAVTool, transcriber port.SpeechTranscriber, ai port.AIGenerator) *UseCase {
	return &UseCase{
		resolver: resolver, av: av, transcriber: transcriber, ai: ai,
		client: &http.Client{Timeout: 10 * time.Minute},
	}
}

// ExtractInput 提取输入（三选一：直链 / 分享链 / 平台视频 ID）。
type ExtractInput struct {
	TenantID  string
	VideoURL  string // 直接可下载的视频直链（优先）
	ShareURL  string // 分享链/网页链（resolver 解析）
	Title     string // 已知标题（灵感广场来源带出，可空）
}

// ExtractResult 提取产物。
type ExtractResult struct {
	RawText string // 说话内容原文（字幕或 ASR）
	Title   string
	Method  string // subtitle / asr / asr-direct（ffmpeg 缺席直传）
}

// Extract 提取视频说话内容。
func (uc *UseCase) Extract(ctx context.Context, in ExtractInput) (*ExtractResult, error) {
	if uc.transcriber == nil {
		return nil, fmt.Errorf("语音识别未配置")
	}
	videoURL := in.VideoURL
	title := in.Title
	if videoURL == "" && in.ShareURL != "" {
		if uc.resolver == nil {
			return nil, fmt.Errorf("分享链解析未启用（可下载视频后直接上传）")
		}
		var plat string
		var err error
		videoURL, title, plat, err = uc.resolver.Resolve(ctx, in.TenantID, in.ShareURL)
		if err != nil {
			return nil, err
		}
		_ = plat
	}
	if videoURL == "" {
		return nil, fmt.Errorf("请提供视频链接或分享链接")
	}

	// 下载（SSRF 防护：R1——仅 http/https、私网/环回拒、大小上限、超时）
	mediaPath, size, err := uc.safeDownload(ctx, videoURL)
	if err != nil {
		return nil, err
	}
	defer os.Remove(mediaPath) // R5：原始视频即用即删

	return uc.extractFromFile(ctx, mediaPath, size, title)
}

// extractFromFile 从本地媒体文件提取文本（音视频上传路径复用）。
func (uc *UseCase) extractFromFile(ctx context.Context, mediaPath string, size int64, title string) (*ExtractResult, error) {
	// ① 软字幕轨（免费且 100% 精确）
	if uc.av != nil && uc.av.Available() {
		if text, ok, err := uc.av.ExtractSubtitle(ctx, mediaPath); err == nil && ok && strings.TrimSpace(text) != "" {
			return &ExtractResult{RawText: text, Title: title, Method: "subtitle"}, nil
		}
		// ② 抽音轨（16k 单声道 mp3——绕开 ASR 文件上限）
		audioPath, aErr := uc.av.ExtractAudio(ctx, mediaPath)
		if aErr == nil {
			defer os.Remove(audioPath)
			audioStat, _ := os.Stat(audioPath)
			audioSize := int64(0)
			if audioStat != nil {
				audioSize = audioStat.Size()
			}
			text, tErr := uc.transcriber.Transcribe(ctx, audioPath, "audio/mpeg", audioSize)
			if tErr != nil {
				return nil, tErr
			}
			return &ExtractResult{RawText: text, Title: title, Method: "asr"}, nil
		}
		// 抽轨失败（如无音轨的纯字幕视频）→ 落到直传降级
	}
	// ③ 降级：ffmpeg 不可用/抽轨失败——视频 ≤25MB 整文件直传（多数 ASR 接受视频自动取音轨）
	if size > 25<<20 {
		return nil, fmt.Errorf("视频 %dMB 超过直传上限 25MB 且服务器未安装 ffmpeg——请联系管理员", size>>20)
	}
	mime := "video/mp4"
	text, err := uc.transcriber.Transcribe(ctx, mediaPath, mime, size)
	if err != nil {
		return nil, err
	}
	return &ExtractResult{RawText: text, Title: title, Method: "asr-direct"}, nil
}

// ExtractFromFile 用户直接上传的音视频文件提取（向导第①步上传路径）。
func (uc *UseCase) ExtractFromFile(ctx context.Context, tenantID, mediaPath, title string) (*ExtractResult, error) {
	st, err := os.Stat(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %w", err)
	}
	return uc.extractFromFile(ctx, mediaPath, st.Size(), title)
}

// safeDownload SSRF 防护下载（R1）：
// 仅 http/https；DNS 解析后拒绝私网/环回/链路本地地址；≤500MB；10 分钟超时。
func (uc *UseCase) safeDownload(ctx context.Context, rawURL string) (path string, size int64, err error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", 0, fmt.Errorf("仅支持 http/https 链接")
	}
	ips, err := net.LookupIP(u.Hostname())
	if err != nil {
		return "", 0, fmt.Errorf("链接域名无法解析")
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return "", 0, fmt.Errorf("链接指向内网地址，已拒绝下载")
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36")
	resp, err := uc.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("下载失败 HTTP %d", resp.StatusCode)
	}
	f, err := os.CreateTemp("", "vt-*.mp4")
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(resp.Body, 500<<20+1))
	if err != nil {
		os.Remove(f.Name())
		return "", 0, fmt.Errorf("下载中断: %w", err)
	}
	if n > 500<<20 {
		os.Remove(f.Name())
		return "", 0, fmt.Errorf("视频超过 500MB 上限")
	}
	return f.Name(), n, nil
}

// ---- 文案双产出（D2 第②步：清洗版 + 改写版）----

// ScriptResult LLM 双产出。
type ScriptResult struct {
	Clean   string `json:"clean"`   // 清洗版：加标点分段/去语气词/修同音错字（=「用原文」按钮）
	Rewrite string `json:"rewrite"` // 改写版：借结构 + 品牌内容替换（默认填入编辑框）
}

// RewriteScript 原文 → 双产出（一次 LLM 调用）。topic 为用户的一句话意图
//（品牌/产品上下文由调用方拼入——向导持品牌知识库）；rawText 为 ASR/字幕原文。
func (uc *UseCase) RewriteScript(ctx context.Context, rawText, topic string) (*ScriptResult, error) {
	if uc.ai == nil {
		return nil, fmt.Errorf("AI 服务未配置")
	}
	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return nil, fmt.Errorf("原文为空")
	}
	if len([]rune(rawText)) > 20000 {
		rawText = string([]rune(rawText)[:20000]) // 超长截断（约 80 分钟语料）
	}
	system := "你是口播视频文案编辑。基于参考视频的语音转录文本，产出两个版本。\n" +
		"clean：最小干预清洗——加标点分段、去除语气词/重复/口癖、修正明显同音错字，保留原话内容与风格。\n" +
		"rewrite：改写版——借鉴原文的结构（开头钩子→内容→行动号召），内容围绕给定主题重写，口语化适合口播，长度与原文相当（±20%）。\n" +
		"严格输出 JSON：{\"clean\":\"...\",\"rewrite\":\"...\"}"
	user := fmt.Sprintf("主题：%s\n\n参考视频转录原文：\n%s", topic, rawText)
	out, err := uc.ai.ChatStream(ctx, "", "default", []port.ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("文案生成失败: %w", err)
	}
	res := parseScriptJSON(out)
	if res == nil || (res.Clean == "" && res.Rewrite == "") {
		// JSON 解析失败降级：整段输出当清洗版
		return &ScriptResult{Clean: out, Rewrite: ""}, nil
	}
	if res.Clean == "" {
		res.Clean = rawText
	}
	if res.Rewrite == "" {
		res.Rewrite = res.Clean
	}
	return res, nil
}

// parseScriptJSON 从 LLM 输出解析双产出（容忍 ```json 包裹与思考标签）。
func parseScriptJSON(out string) *ScriptResult {
	s := out
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			s = s[i : j+1]
		}
	}
	var res ScriptResult
	if err := json.Unmarshal([]byte(s), &res); err != nil {
		return nil
	}
	return &res
}
