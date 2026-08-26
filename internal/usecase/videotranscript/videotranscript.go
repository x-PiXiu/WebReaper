// Package videotranscript 视频文案提取与改写（08 计划 D4/D2 第①②步）。
//
// 流程：resolve（分享链→直链）→ download（SSRF 防护下载）→ extract（软字幕轨
// 优先，无则 ffmpeg 抽音轨）→ transcribe（云 ASR）→ rewrite（LLM 清洗/改写双产出）。
// 原始视频即用即删（R5——只留提取产物，防 5GB 级临时文件吃爆磁盘）。
package videotranscript

import (
	"context"
	"encoding/hex"
	"crypto/sha256"
	"log"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
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
	cache       port.CacheStore         // 可选；提取结果缓存（同 URL 24h 内免重复 ASR 计费）
}

// SetCache 注入缓存（可选；nil=不缓存——每次全量提取）。
func (uc *UseCase) SetCache(c port.CacheStore) { uc.cache = c }

// transcriptCacheKey 提取缓存键（视频/分享 URL 哈希——同视频多形态 URL 分别缓存）。
func transcriptCacheKey(url string) string {
	sum := sha256.Sum256([]byte(url))
	return "transcript:extract:v1:" + hex.EncodeToString(sum[:16])
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
	RawText string // 说话内容原文（按句分行，换行符连接——编辑友好）
	Title   string
	Method  string // subtitle / asr / asr / asr-direct（ffmpeg 缺席直传）
	Lines   []string // 按句切分的行（口播逐句单位——服务端规范化，多消费方共享）
}

// splitSentences 按句末标点（。！？!?；;…）切分为行；超长句（>80 字）按逗号/顿号
// 二次切分为 ≤40 字的读句（口播稿的自然单位——一行一句便于确认与编辑）。
func splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var primary []string
	var cur strings.Builder
	for _, r := range text {
		cur.WriteRune(r)
		if strings.ContainsRune("。！？!?；;…", r) {
			if t := strings.TrimSpace(cur.String()); t != "" {
				primary = append(primary, t)
			}
			cur.Reset()
		}
	}
	if t := strings.TrimSpace(cur.String()); t != "" {
		primary = append(primary, t) // 尾句（可能无句末标点）
	}
	// 超长句二次切分（按逗号/顿号，目标 ≤40 字/行）
	var out []string
	for _, l := range primary {
		runes := []rune(l)
		if len(runes) <= 80 {
			out = append(out, l)
			continue
		}
		var seg strings.Builder
		for _, r := range runes {
			seg.WriteRune(r)
			if strings.ContainsRune("，、,--", r) && len([]rune(seg.String())) >= 40 {
				if t := strings.TrimSpace(seg.String()); t != "" {
					out = append(out, t)
				}
				seg.Reset()
			}
		}
		if t := strings.TrimSpace(seg.String()); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// withLines 规范化提取产物：切分行 + RawText 分行化（单一句读规范，前端零改动）。
func withLines(r *ExtractResult) *ExtractResult {
	if r == nil || strings.TrimSpace(r.RawText) == "" {
		return r
	}
	r.Lines = splitSentences(r.RawText)
	r.RawText = strings.Join(r.Lines, "\n")
	return r
}

// transcribeSegments 长音频分段转写：5 分钟/段 → 逐段 ASR → 按序拼接。
// 段文件即用即删；任一段失败返回错误（调用方可回落单次直传）。
func (uc *UseCase) transcribeSegments(ctx context.Context, audioPath string) (string, error) {
	segments, err := uc.av.SegmentAudio(ctx, audioPath, 300)
	if err != nil {
		return "", err
	}
	defer func() {
		for _, seg := range segments {
			_ = os.Remove(seg)
		}
	}()
	var parts []string
	for i, seg := range segments {
		st, _ := os.Stat(seg)
		size := int64(0)
		if st != nil {
			size = st.Size()
		}
		text, tErr := uc.transcriber.Transcribe(ctx, seg, "audio/mpeg", size)
		if tErr != nil {
			return "", fmt.Errorf("第 %d/%d 段转写失败: %w", i+1, len(segments), tErr)
		}
		if strings.TrimSpace(text) != "" {
			parts = append(parts, strings.TrimSpace(text))
		}
		uc.debugLog(ctx, "分段转写进度", "segment", fmt.Sprintf("%d/%d", i+1, len(segments)), "chars", len(text))
	}
	return strings.Join(parts, ""), nil
}

// debugLog DEBUG 级过程日志（本地排查链路用——服务 logger 未注入本用例，
// 走标准库与前缀标记；LOG_LEVEL 由服务统一控制时这里始终输出 INFO 以下粒度可见性不受影响）。
func (uc *UseCase) debugLog(_ context.Context, msg string, kv ...any) {
	log.Printf("[Transcript][DEBUG] %s %v", msg, kv)
}

// truncURLForLog 截断 URL（直链含长签名参数）。
func truncURLForLog(u string) string {
	if i := strings.Index(u, "?"); i > 0 {
		u = u[:i] + "?…"
	}
	if len(u) > 90 {
		return u[:90] + "…"
	}
	return u
}

// Extract 提取视频说话内容。
// Extract 提取视频说话内容（带 24h 结果缓存——同 URL 重复提取免 ASR 计费）。
func (uc *UseCase) Extract(ctx context.Context, in ExtractInput) (result *ExtractResult, err error) {
	if uc.transcriber == nil {
		return nil, fmt.Errorf("语音识别未配置")
	}
	cacheKey := ""
	if uc.cache != nil {
		if u := in.ShareURL; u != "" {
			cacheKey = transcriptCacheKey(u)
		} else if u := in.VideoURL; u != "" {
			cacheKey = transcriptCacheKey(u)
		}
		if cacheKey != "" {
			if hit, ok, _ := uc.cache.Get(ctx, cacheKey); ok && hit != "" {
				var cached ExtractResult
				if json.Unmarshal([]byte(hit), &cached) == nil && cached.RawText != "" {
					uc.debugLog(ctx, "提取缓存命中（免 ASR 计费）", "chars", len(cached.RawText))
					return &cached, nil
				}
			}
		}
	}
	defer func() {
		// 成功结果回填缓存（24h；异常静默——缓存故障不阻断提取）
		if cacheKey != "" && uc.cache != nil && result != nil && err == nil {
			if b, jErr := json.Marshal(result); jErr == nil {
				_ = uc.cache.Set(ctx, cacheKey, string(b), 24*time.Hour)
			}
		}
	}()
	return uc.extract(ctx, in)
}

// extract 无缓存的原始提取流程。
func (uc *UseCase) extract(ctx context.Context, in ExtractInput) (*ExtractResult, error) {
	videoURL := in.VideoURL
	title := in.Title
	if videoURL == "" && in.ShareURL != "" {
		// BE-CRAWL-02：口令全文预处理——从分享文本中抽取 URL，避免 url.Parse 遇到非 URL 文本报错
		shareURL := extractShareURL(in.ShareURL)
		if shareURL == "" {
			return nil, fmt.Errorf("未能从分享文本中提取到有效链接（请粘贴含 https:// 的完整链接）")
		}
		if uc.resolver == nil {
			return nil, fmt.Errorf("分享链解析未启用（可下载视频后直接上传）")
		}
		var plat, localPath string
		var err error
		videoURL, title, plat, localPath, err = uc.resolver.Resolve(ctx, in.TenantID, shareURL)
		if err != nil {
			return nil, err
		}
		uc.debugLog(ctx, "分享链解析完成", "platform", plat, "title_len", len(title), "video_url", truncURLForLog(videoURL))
		// 浏览器上下文已下载（快手 pkey 会话签名场景）——跳过 safeDownload 直接提取
		if localPath != "" {
			defer os.Remove(localPath)
			st, _ := os.Stat(localPath)
			size := int64(0)
			if st != nil {
				size = st.Size()
			}
			uc.debugLog(ctx, "解析器已完成浏览器内下载", "bytes", size)
			return uc.extractFromFile(ctx, localPath, size, title)
		}
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
	uc.debugLog(ctx, "视频已下载", "bytes", size)

	return uc.extractFromFile(ctx, mediaPath, size, title)
}

// extractFromFile 从本地媒体文件提取文本（音视频上传路径复用）。
func (uc *UseCase) extractFromFile(ctx context.Context, mediaPath string, size int64, title string) (*ExtractResult, error) {
	// ① 软字幕轨（免费且 100% 精确）
	if uc.av != nil && uc.av.Available() {
		if text, ok, err := uc.av.ExtractSubtitle(ctx, mediaPath); err == nil && ok && strings.TrimSpace(text) != "" {
			uc.debugLog(ctx, "提取方式=软字幕轨（免费精确）", "chars", len(text))
			return withLines(&ExtractResult{RawText: text, Title: title, Method: "subtitle"}), nil
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
			// 长音频分段转写（>10MB ≈ 20 分钟+：半小时/1 小时教程视频的解决方案）——
			// 按 5 分钟/段切开循环 ASR 后拼接；单段 ~2.4MB 远离任何 ASR 上限
			if audioSize > 10<<20 {
				text, sErr := uc.transcribeSegments(ctx, audioPath)
				if sErr == nil {
					uc.debugLog(ctx, "提取方式=分段转写", "audio_bytes", audioSize, "chars", len(text))
					return withLines(&ExtractResult{RawText: text, Title: title, Method: "asr-segments"}), nil
				}
				uc.debugLog(ctx, "分段转写失败，回落单次直传", "err", sErr.Error())
			}
			text, tErr := uc.transcriber.Transcribe(ctx, audioPath, "audio/mpeg", audioSize)
			if tErr != nil {
				return nil, tErr
			}
			uc.debugLog(ctx, "提取方式=ffmpeg抽音轨+ASR", "audio_bytes", audioSize, "chars", len(text))
			return withLines(&ExtractResult{RawText: text, Title: title, Method: "asr"}), nil
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
	return withLines(&ExtractResult{RawText: text, Title: title, Method: "asr-direct"}), nil
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
	// B站 CDN 防盗链（缺失 Referer 返回 403）——抖音 CDN 无此校验不受影响
	if strings.Contains(u.Host, "bilivideo.com") || strings.Contains(u.Host, "hdslb.com") {
		req.Header.Set("Referer", "https://www.bilibili.com")
	}
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

// shareURLRe 从分享口令/文本中提取 http/https 链接（BE-CRAWL-02）。
// 抖音口令示例："5.84 :1pm 01/05 v@f.Bg xFU:/ 普通人怎样白手起家 https://v.douyin.com/xxx"
var shareURLRe = regexp.MustCompile(`https?://[^\s\)\]\}，。；、""''<>]+`)

// extractShareURL 从任意文本中抽取第一个 http/https 链接。
// 用户粘贴抖音/快手分享口令时，文本中嵌入了短链但前后有中文、标点等非 URL 字符。
// 返回空字符串表示未找到有效 URL。
func extractShareURL(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	// 纯 URL（无空格、scheme+host 合法）→ 原样返回（去除尾部中文标点）
	if !strings.ContainsAny(text, " \t\n") {
		if u, err := url.Parse(text); err == nil && u.Scheme != "" && u.Host != "" {
			return strings.TrimRight(text, ",;:!?。；，")
		}
	}
	// 正则提取第一个 http/https 链接
	m := shareURLRe.FindString(text)
	if m == "" {
		return ""
	}
	// 去除尾部标点（中英文）
	m = strings.TrimRight(m, ",;:!?。；，")
	return m
}
