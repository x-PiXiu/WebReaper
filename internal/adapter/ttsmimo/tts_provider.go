package ttsmimo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"webreaper/internal/usecase/port"
)

// MiMoTTSProvider 小米MiMo TTS适配器。
//
// 设计动机：
//   - 小米MiMo TTS返回base64音频，无需轮询，延迟更低
//   - 支持9种预置音色
//   - 支持声音克隆
//   - 支持音色设计
type MiMoTTSProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

var _ port.AudioSynthesizer = (*MiMoTTSProvider)(nil)

// NewMiMoTTSProvider 创建小米MiMo TTS适配器。
func NewMiMoTTSProvider(apiKey, baseURL string) *MiMoTTSProvider {
	if baseURL == "" {
		baseURL = "https://token-plan-cn.xiaomimimo.com/v1"
	}
	return &MiMoTTSProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		// 真机实测（2026-09-01）：MiMo voiceclone 合成耗时随文本长度线性增长
		//（2049 字实测 166s）——固定 30s 必超。客户端用上限 240s，实际时限由
		// Submit 按文本长度自适应的 ctx 控制（见 mimoDeadlineForText）。
		client: &http.Client{Timeout: 240 * time.Second},
	}
}

// mimoDeadlineForText 按文本长度自适应时限：30s 基线 + 每 20 字 2s，封顶 240s
//（实测速率约 12 字/秒，留 2.5 倍余量）。
func mimoDeadlineForText(text string) time.Duration {
	n := len([]rune(text))
	d := 30*time.Second + time.Duration(n/20)*2*time.Second
	if d > 240*time.Second {
		d = 240 * time.Second
	}
	return d
}

// Synthesize 标准TTS（预置音色）。
func (p *MiMoTTSProvider) Synthesize(ctx context.Context, text string, voiceID string) (audio []byte, format string, err error) {
	if voiceID == "" {
		voiceID = "mimo_default"
	}

	// 构建请求
	req := map[string]any{
		"model": "mimo-v2.5-tts",
		"messages": []map[string]string{
			{"role": "assistant", "content": text},
		},
		"audio": map[string]any{
			"format": "mp3",
			"voice":  voiceID,
		},
	}

	// 发送请求
	resp, err := p.sendRequest(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("MiMo TTS 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	audioData, err := p.parseAudioResponse(resp)
	if err != nil {
		return nil, "", err
	}

	return audioData, "mp3", nil
}

// SynthesizeDesign 音色设计（自然语言描述音色风格）。
func (p *MiMoTTSProvider) SynthesizeDesign(ctx context.Context, text string, styleDesc string) (audio []byte, format string, err error) {
	// 构建请求
	req := map[string]any{
		"model": "mimo-v2.5-tts-voicedesign",
		"messages": []map[string]string{
			{"role": "user", "content": styleDesc},
			{"role": "assistant", "content": text},
		},
		"audio": map[string]any{
			"format": "mp3",
		},
	}

	// 发送请求
	resp, err := p.sendRequest(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("MiMo TTS 音色设计请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	audioData, err := p.parseAudioResponse(resp)
	if err != nil {
		return nil, "", err
	}

	return audioData, "mp3", nil
}

// SynthesizeClone 声音克隆（传入音频样本base64 + 合成文本）。
//
// 小米MiMo声音克隆API要求 voice 字段为 DataURL 格式（data:audio/mpeg;base64,...）。
func (p *MiMoTTSProvider) SynthesizeClone(ctx context.Context, sampleBase64 string, text string) (audio []byte, format string, err error) {
	// 确保 sampleBase64 是 DataURL 格式
	voiceDataURI := sampleBase64
	if !strings.HasPrefix(sampleBase64, "data:") {
		// 转换为 DataURL 格式
		voiceDataURI = "data:audio/mpeg;base64," + sampleBase64
	}

	// 构建请求
	req := map[string]any{
		"model": "mimo-v2.5-tts-voiceclone",
		"messages": []map[string]string{
			{"role": "assistant", "content": text},
		},
		"audio": map[string]any{
			"format": "mp3",
			"voice":  voiceDataURI,
		},
	}

	// 发送请求
	resp, err := p.sendRequest(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("MiMo TTS 声音克隆请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	audioData, err := p.parseAudioResponse(resp)
	if err != nil {
		return nil, "", err
	}

	return audioData, "mp3", nil
}

// sendRequest 发送HTTP请求。
func (p *MiMoTTSProvider) sendRequest(ctx context.Context, req any) (*http.Response, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 小米MiMo使用api-key头
	httpReq.Header.Set("api-key", p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("MiMo TTS 返回 HTTP %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// parseAudioResponse 解析音频响应。
func (p *MiMoTTSProvider) parseAudioResponse(resp *http.Response) ([]byte, error) {
	var result struct {
		Choices []struct {
			Message struct {
				Audio struct {
					Data string `json:"data"`
				} `json:"audio"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("无音频数据")
	}

	audioData := result.Choices[0].Message.Audio.Data
	if audioData == "" {
		return nil, fmt.Errorf("音频数据为空")
	}

	// 解码base64
	decoded, err := base64.StdEncoding.DecodeString(audioData)
	if err != nil {
		return nil, fmt.Errorf("解码base64失败: %w", err)
	}

	return decoded, nil
}
