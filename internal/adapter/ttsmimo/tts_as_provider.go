package ttsmimo

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// MiMoAsGenerationProvider 将小米MiMo TTS包装为GenerationProvider接口。
//
// 设计动机：
//   - GenerationUseCase通过providers map选择provider
//   - 小米MiMo TTS是同步接口，返回base64音频
//   - 需要实现GenerationProvider接口以加入providers map
type MiMoAsGenerationProvider struct {
	tts *MiMoTTSProvider
}

var _ port.GenerationProvider = (*MiMoAsGenerationProvider)(nil)

func NewMiMoAsGenerationProvider(tts *MiMoTTSProvider) *MiMoAsGenerationProvider {
	return &MiMoAsGenerationProvider{tts: tts}
}

func (p *MiMoAsGenerationProvider) Name() string { return "xiaomi-mimo" }

// Submit 同步TTS/声音克隆：直接返回音频URL（base64内联）。
//
// 根据endpoint参数区分：
//   - /ent/v2/audio-tts 或 tts：标准TTS
//   - /ent/v2/audio-clone 或 voice_clone：声音克隆
func (p *MiMoAsGenerationProvider) Submit(ctx context.Context, endpoint string, body map[string]any) (port.SubmitResult, error) {
	// 提取text
	text, _ := body["text"].(string)
	if text == "" {
		return port.SubmitResult{}, fmt.Errorf("text参数为空")
	}

	// 判断是TTS还是声音克隆
	isVoiceClone := endpoint == "/ent/v2/audio-clone" || endpoint == "voice_clone"
	
	var audioData []byte
	var err error
	
	if isVoiceClone {
		// 声音克隆：需要 audio_url（参考音频）和 voice_id
		audioURL, _ := body["audio_url"].(string)
		if audioURL == "" {
			return port.SubmitResult{}, fmt.Errorf("声音克隆需要 audio_url 参数")
		}
		
		// 下载参考音频
		sampleData, downloadErr := p.downloadAudio(ctx, audioURL)
		if downloadErr != nil {
			return port.SubmitResult{}, fmt.Errorf("下载参考音频失败: %w", downloadErr)
		}
		
		// 转换为base64编码（小米MiMo API要求）
		sampleBase64 := base64.StdEncoding.EncodeToString(sampleData)
		
		// 调用声音克隆
		audioData, _, err = p.tts.SynthesizeClone(ctx, sampleBase64, text)
	} else {
		// 标准TTS
		voiceID, _ := body["voice_setting_voice_id"].(string)
		if voiceID == "" || voiceID == "default" {
			voiceID = "mimo_default"
		}
		audioData, _, err = p.tts.Synthesize(ctx, text, voiceID)
	}
	
	if err != nil {
		return port.SubmitResult{}, fmt.Errorf("MiMo 生成失败: %w", err)
	}

	// 返回同步结果（base64内联，无需轮询）
	base64Audio := "data:audio/mp3;base64," + base64.StdEncoding.EncodeToString(audioData)
	return port.SubmitResult{
		TaskID: "mimo-" + fmt.Sprintf("%d", len(audioData)),
		State:  entity.TaskStateSuccess,
		Creations: []entity.CreationItem{
			{ID: "audio", URL: base64Audio},
		},
	}, nil
}

// downloadAudio 下载音频文件（支持HTTP URL和data:URI）。
func (p *MiMoAsGenerationProvider) downloadAudio(ctx context.Context, url string) ([]byte, error) {
	// 支持 data:audio/mpeg;base64,... 格式（本地素材内联）
	if len(url) > 5 && url[:5] == "data:" {
		// 提取 base64 数据
		idx := strings.Index(url, ";base64,")
		if idx < 0 {
			return nil, fmt.Errorf("无效的data URI格式")
		}
		b64 := url[idx+8:]
		return base64.StdEncoding.DecodeString(b64)
	}
	
	// HTTP URL 下载
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载音频失败: HTTP %d", resp.StatusCode)
	}
	
	return io.ReadAll(resp.Body)
}

func (p *MiMoAsGenerationProvider) Poll(ctx context.Context, taskID string) (port.GenerationStatus, error) {
	return port.GenerationStatus{State: entity.TaskStateSuccess}, nil
}

func (p *MiMoAsGenerationProvider) Cancel(ctx context.Context, taskID string) error {
	return nil
}

func (p *MiMoAsGenerationProvider) VerifyCallback(ctx context.Context, header http.Header, body []byte, requestURI string) error {
	return nil
}

func (p *MiMoAsGenerationProvider) QueryCredits(ctx context.Context) (int, error) {
	return 999999, nil // MiMo免费额度
}

func (p *MiMoAsGenerationProvider) TranslateError(code string) string {
	return "MiMo错误: " + code
}
