// Package ttsmimo 小米 MiMo 同步 TTS（port.AudioSynthesizer 实现）。
//
// 三种模式共用 /v1/chat/completions 端点，通过 model 名区分：
//   - mimo-v2.5-tts：标准 TTS（9 种预置音色，audio.voice = 音色 ID）
//   - mimo-v2.5-tts-voicedesign：音色设计（user.content = 风格描述，audio.voice 不支持）
//   - mimo-v2.5-tts-voiceclone：声音克隆（audio.voice = 音频样本 base64）
//
// 所有模式同步返回 base64 音频（choices[0].message.audio.data），无需轮询。
package ttsmimo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"webreaper/internal/usecase/port"
)

// Synthesizer port.AudioSynthesizer 的小米 MiMo 实现。
type Synthesizer struct {
	resolve func() Config
	client  *http.Client

	mu       sync.Mutex
	cache    Config
	cachedAt time.Time
	ttl      time.Duration
}

// Config 小米 TTS 配置（从 CapabilityResolver 解析）。
type Config struct {
	Endpoint string // "https://token-plan-cn.xiaomimimo.com/v1/chat/completions"
	APIKey   string
	// Models: 标准=StandardModel, 音色设计=DesignModel, 声音克隆=CloneModel
	StandardModel string // 默认 "mimo-v2.5-tts"
	DesignModel   string // 默认 "mimo-v2.5-tts-voicedesign"
	CloneModel    string // 默认 "mimo-v2.5-tts-voiceclone"
	AudioFormat   string // 默认 "wav"
}

const (
	StandardModel = "mimo-v2.5-tts"
	DesignModel   = "mimo-v2.5-tts-voicedesign"
	CloneModel    = "mimo-v2.5-tts-voiceclone"
)

// NewSynthesizer 创建小米 TTS。resolve 返回当前配置。
func NewSynthesizer(resolve func() Config) *Synthesizer {
	return &Synthesizer{resolve: resolve, client: &http.Client{Timeout: 60 * time.Second}, ttl: 10 * time.Second}
}

func (s *Synthesizer) current() Config {
	s.mu.Lock()
	if time.Since(s.cachedAt) > s.ttl {
		s.cache, s.cachedAt = s.resolve(), time.Now()
	}
	cfg := s.cache
	s.mu.Unlock()
	if cfg.StandardModel == "" { cfg.StandardModel = StandardModel }
	if cfg.DesignModel == "" { cfg.DesignModel = DesignModel }
	if cfg.CloneModel == "" { cfg.CloneModel = CloneModel }
	if cfg.AudioFormat == "" { cfg.AudioFormat = "wav" }
	return cfg
}

// Refresh 管理后台保存后即时刷新。
func (s *Synthesizer) Refresh(cfg Config) {
	s.mu.Lock()
	s.cache, s.cachedAt = cfg, time.Now()
	s.mu.Unlock()
}

// Synthesize 标准 TTS（预置音色）。
func (s *Synthesizer) Synthesize(ctx context.Context, text string, voiceID string) ([]byte, string, error) {
	cfg := s.current()
	if cfg.APIKey == "" || cfg.Endpoint == "" {
		return nil, "", fmt.Errorf("TTS 未配置")
	}
	if voiceID == "" {
		voiceID = "mimo_default"
	}
	payload := map[string]any{
		"model": cfg.StandardModel,
		"messages": []map[string]any{
			{"role": "user", "content": text},
			{"role": "assistant", "content": text},
		},
		"audio": map[string]any{
			"format": cfg.AudioFormat,
			"voice":  voiceID,
		},
	}
	return s.doRequest(ctx, cfg, payload)
}

// SynthesizeDesign 音色设计（自然语言描述风格 → 音频）。
func (s *Synthesizer) SynthesizeDesign(ctx context.Context, text string, styleDesc string) ([]byte, string, error) {
	cfg := s.current()
	if cfg.APIKey == "" || cfg.Endpoint == "" {
		return nil, "", fmt.Errorf("TTS 未配置")
	}
	payload := map[string]any{
		"model": cfg.DesignModel,
		"messages": []map[string]any{
			{"role": "user", "content": styleDesc},
			{"role": "assistant", "content": text},
		},
		"audio": map[string]any{
			"format":              cfg.AudioFormat,
			"optimize_text_preview": false,
		},
	}
	return s.doRequest(ctx, cfg, payload)
}

// SynthesizeClone 声音克隆（音频样本 base64 + 合成文本）。
func (s *Synthesizer) SynthesizeClone(ctx context.Context, sampleBase64 string, text string) ([]byte, string, error) {
	cfg := s.current()
	if cfg.APIKey == "" || cfg.Endpoint == "" {
		return nil, "", fmt.Errorf("TTS 未配置")
	}
	payload := map[string]any{
		"model": cfg.CloneModel,
		"messages": []map[string]any{
			{"role": "user", "content": "使用提供的音色样本朗读以下文本"},
			{"role": "assistant", "content": text},
		},
		"audio": map[string]any{
			"format": cfg.AudioFormat,
			"voice":  sampleBase64, // 小米 voiceclone：audio.voice = 音频样本 base64
		},
	}
	return s.doRequest(ctx, cfg, payload)
}

// doRequest 统一请求 + 响应解析（提取 choices[0].message.audio.data）。
func (s *Synthesizer) doRequest(ctx context.Context, cfg Config, payload map[string]any) ([]byte, string, error) {
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", cfg.APIKey) // 小米用 api-key 头

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("TTS 请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // base64 音频可达数 MB
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("TTS 失败 HTTP %d: %s", resp.StatusCode, truncStr(string(raw), 200))
	}
	// 解析 choices[0].message.audio.data
	var result struct {
		Choices []struct {
			Message struct {
				Audio struct {
					Data string `json:"data"` // base64 编码音频
				} `json:"audio"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, "", fmt.Errorf("TTS 响应解析失败: %w", err)
	}
	if len(result.Choices) == 0 || result.Choices[0].Message.Audio.Data == "" {
		return nil, "", fmt.Errorf("TTS 响应无音频数据")
	}
	audioBytes, err := base64.StdEncoding.DecodeString(result.Choices[0].Message.Audio.Data)
	if err != nil {
		return nil, "", fmt.Errorf("TTS 音频解码失败: %w", err)
	}
	return audioBytes, cfg.AudioFormat, nil
}

func truncStr(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n] + "..."
}

var _ port.AudioSynthesizer = (*Synthesizer)(nil)
