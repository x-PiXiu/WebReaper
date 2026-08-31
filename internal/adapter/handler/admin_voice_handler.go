// admin_voice_handler.go 管理后台官方音色管理（27 号优化——运营可管理官方音色）。
//
// 功能：
//   - 创建官方音色（上传音频样本 + 文本 → 克隆 + TTS 生成试听）
//   - 列表/搜索/上下架/删除
package handler

import (
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// AdminVoiceHandler 管理后台官方音色管理。
type AdminVoiceHandler struct {
	voiceRepo   port.VoiceLibrary
	audioSynth  port.AudioSynthesizer // 可选；nil 时不可创建音色
	mediaStore  port.MediaAssetStore  // 可选；nil 时试听音频不转存
}

func NewAdminVoiceHandler(voices port.VoiceLibrary, synth port.AudioSynthesizer, store port.MediaAssetStore) *AdminVoiceHandler {
	return &AdminVoiceHandler{voiceRepo: voices, audioSynth: synth, mediaStore: store}
}

// HandleCreateVoice POST /api/admin/voices
// 创建官方音色：上传音频样本 → 克隆音色 → TTS 生成试听 → 写入 generation_voices。
func (h *AdminVoiceHandler) HandleCreateVoice(c *gin.Context) {
	if h.audioSynth == nil {
		fail(c, fmt.Errorf("音色合成功能未配置"))
		return
	}

	// 支持 multipart/form-data（音频文件 + 文本）
	file, _, fErr := c.Request.FormFile("audio")
	if fErr != nil {
		fail(c, fmt.Errorf("请上传音频样本"))
		return
	}
	defer file.Close()

	audioData, err := io.ReadAll(file)
	if err != nil {
		fail(c, fmt.Errorf("读取音频失败: %w", err))
		return
	}

	text := c.PostForm("text")
	if text == "" {
		text = "你好，我是您的专属音色助手，很高兴为您服务。"
	}
	name := c.PostForm("name")
	if name == "" {
		name = fmt.Sprintf("音色-%d", time.Now().UnixNano()%10000)
	}
	language := c.PostForm("language")
	if language == "" {
		language = "中文 (普通话)"
	}

	// 调用声音克隆
	sampleBase64 := base64.StdEncoding.EncodeToString(audioData)
	audioBytes, format, err := h.audioSynth.SynthesizeClone(c.Request.Context(), sampleBase64, text)
	if err != nil {
		fail(c, fmt.Errorf("声音克隆失败: %w", err))
		return
	}

	// 上传试听到媒体存储
	var sampleURL string
	if h.mediaStore != nil {
		asset, uploadErr := h.mediaStore.SaveFile(
			c.Request.Context(),
			middleware.CurrentTenantID(c),
			"", // brandID
			"creation",
			audioBytes,
			"audio/"+format,
			"."+format,
		)
		if uploadErr == nil {
			sampleURL = asset.SourceURL
		}
	}

	// 生成 voice_id
	voiceID := fmt.Sprintf("platform-%d", time.Now().UnixNano())

	// 写入 generation_voices（scope=platform）
	voice := entity.GenerationVoice{
		VoiceID:   voiceID,
		Language:  language,
		Name:      name,
		SampleURL: sampleURL,
		Scope:     "platform",
		Status:    "active",
	}
	if err := h.voiceRepo.Upsert(c.Request.Context(), voice); err != nil {
		fail(c, fmt.Errorf("保存音色失败: %w", err))
		return
	}

	success(c, gin.H{
		"voice_id":   voiceID,
		"name":       name,
		"language":   language,
		"sample_url": sampleURL,
		"status":     "active",
	})
}

// HandleListVoices GET /api/admin/voices?language=&q=&scope=
func (h *AdminVoiceHandler) HandleListVoices(c *gin.Context) {
	language := c.Query("language")
	q := c.Query("q")

	voices, err := h.voiceRepo.List(c.Request.Context(), language, q)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"voices": voices})
}
