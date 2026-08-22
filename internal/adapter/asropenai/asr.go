// Package asropenai 提供 OpenAI 兼容语音识别（port.SpeechTranscriber 实现）。
//
// 动态配置（08 计划 D5）：端点/Key/模型存 provider_configs 表（provider=asr，
// 多条目 + extra_json.active 标记启用的那一条），运行时切换即时生效（10s TTL
// 缓存——与 Vidu SwitchingProvider 同模式）。默认推荐硅基流动 SenseVoiceSmall
//（免费、/v1/audio/transcriptions、mp3/wav/m4a/flac/ogg、≤50MB/≤1h）。
package asropenai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"webreaper/internal/usecase/port"
)

// Transcriber port.SpeechTranscriber 的 OpenAI 兼容实现。
type Transcriber struct {
	// resolve 当前生效 ASR 配置：{endpoint, apiKey, model}（由 main 从 provider_configs 聚合）。
	resolve func() ASRConfig
	client  *http.Client

	mu       sync.Mutex
	cache    ASRConfig
	cachedAt time.Time
	ttl      time.Duration
}

// ASRConfig 一条 ASR 服务商配置。
type ASRConfig struct {
	Endpoint string `json:"endpoint"` // 如 https://api.siliconflow.cn/v1/audio/transcriptions
	APIKey   string `json:"api_key"`
	Model    string `json:"model"` // 如 FunAudioLLM/SenseVoiceSmall
	// ResponseStyle 返回格式：空=标准 transcription（{"text":...}）；
	// "chat"=智谱 GLM-ASR / 小米 MiMo 风格（choices[].message.content）
	ResponseStyle string `json:"response_style"`
	// Protocol 请求格式：空/"openai"=multipart file 上传（硅基流动/OpenAI）；
	// "openai-chat"=JSON chat/completions + input_audio base64（小米 MiMo）
	Protocol string `json:"protocol"`
	// ASRLanguage 语种（openai-chat 协议专用——小米 MiMo asr_options.language）。
	// 空="auto"（自动检测）；"zh"=中文；"en"=英文。
	ASRLanguage string `json:"asr_language"`
}

// NewTranscriber 创建 ASR 客户端。resolve 应返回当前启用的配置（未配置返回零值）。
func NewTranscriber(resolve func() ASRConfig) *Transcriber {
	return &Transcriber{resolve: resolve, client: &http.Client{Timeout: 10 * time.Minute}, ttl: 10 * time.Second}
}

// current TTL 缓存读取当前配置。
func (t *Transcriber) current() ASRConfig {
	t.mu.Lock()
	defer t.mu.Unlock()
	if time.Since(t.cachedAt) > t.ttl {
		t.cache, t.cachedAt = t.resolve(), time.Now()
	}
	return t.cache
}

// Refresh 配置变更后主动刷新缓存（管理后台保存路径调用，切换即时生效）。
func (t *Transcriber) Refresh(cfg ASRConfig) {
	t.mu.Lock()
	t.cache, t.cachedAt = cfg, time.Now()
	t.mu.Unlock()
}

// Transcribe 音频文件 → 文本（按协议分支：openai multipart / openai-chat JSON+base64）。
func (t *Transcriber) Transcribe(ctx context.Context, audioPath, mime string, fileSize int64) (string, error) {
	cfg := t.current()
	if cfg.APIKey == "" || cfg.Endpoint == "" {
		return "", fmt.Errorf("语音识别未配置（管理员在第三方集成中配置 ASR 服务商）")
	}
	if fileSize > 50<<20 {
		return "", fmt.Errorf("音频超过 50MB 上限（%dMB）——请缩短视频", fileSize>>20)
	}

	var (
		reqBody    io.Reader
		contentTyp string
		err        error
	)

	switch cfg.Protocol {
	case "openai-chat":
		reqBody, contentTyp, err = t.buildChatAudioRequest(ctx, audioPath, mime, cfg)
	default:
		reqBody, contentTyp, err = t.buildMultipartRequest(audioPath, mime, cfg)
	}
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, reqBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentTyp)
	if cfg.Protocol == "openai-chat" {
		req.Header.Set("api-key", cfg.APIKey) // 小米 MiMo 用 api-key 头
	} else {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ASR 请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ASR 失败 HTTP %d: %s", resp.StatusCode, truncStr(string(raw), 200))
	}
	return parseText(raw, cfg.ResponseStyle)
}

// buildMultipartRequest 标准 OpenAI multipart（硅基流动/OpenAI）。
func (t *Transcriber) buildMultipartRequest(audioPath string, mime string, cfg ASRConfig) (io.Reader, string, error) {
	f, err := os.Open(audioPath)
	if err != nil {
		return nil, "", fmt.Errorf("读取音频失败: %w", err)
	}
	defer f.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("model", cfg.Model); err != nil {
		return nil, "", err
	}
	fileName := filepath.Base(audioPath)
	if ext := filepath.Ext(fileName); ext != "" && mime != "" {
		if !strings.Contains(fileName, ".") {
			fileName += extByMime(mime)
		}
	}
	hw, _ := mw.CreateFormFile("file", fileName)
	if _, err := io.Copy(hw, f); err != nil {
		return nil, "", fmt.Errorf("读取音频失败: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return &body, mw.FormDataContentType(), nil
}

// buildChatAudioRequest 小米 MiMo JSON+base64（chat/completions + input_audio）。
func (t *Transcriber) buildChatAudioRequest(ctx context.Context, audioPath, mime string, cfg ASRConfig) (io.Reader, string, error) {
	data, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, "", fmt.Errorf("读取音频失败: %w", err)
	}
	// data URL 格式：data:{mime};base64,{data}（小米 ASR 支持）
	if mime == "" {
		mime = "audio/mpeg"
	}
	dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)

	payload := map[string]any{
		"model": cfg.Model,
		"messages": []map[string]any{{
			"role": "user",
			"content": []map[string]any{{
				"type": "input_audio",
				"input_audio": map[string]any{
					"data": dataURL,
				},
			}},
		}},
	}
	// asr_options.language（小米 MiMo 语种控制：auto/zh/en）
	lang := cfg.ASRLanguage
	if lang == "" {
		lang = "auto"
	}
	payload["asr_options"] = map[string]any{"language": lang}

	b, _ := json.Marshal(payload)
	return bytes.NewReader(b), "application/json", nil
}

// parseText 兼容两种返回结构：标准 {"text":...} / 智谱 chat.completion 风格。
func parseText(raw []byte, style string) (string, error) {
	if style == "chat" {
		var cr struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &cr); err == nil && len(cr.Choices) > 0 {
			return cr.Choices[0].Message.Content, nil
		}
	}
	var tr struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &tr); err != nil || tr.Text == "" {
		return "", fmt.Errorf("ASR 响应解析失败: %s", truncStr(string(raw), 200))
	}
	return tr.Text, nil
}

func extByMime(mime string) string {
	switch {
	case strings.Contains(mime, "mpeg"), strings.Contains(mime, "mp3"):
		return ".mp3"
	case strings.Contains(mime, "wav"):
		return ".wav"
	case strings.Contains(mime, "mp4"), strings.Contains(mime, "m4a"):
		return ".m4a"
	default:
		return ""
	}
}

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

var _ port.SpeechTranscriber = (*Transcriber)(nil)
