// admin_voice_handler.go 管理后台官方音色管理（27 号优化——运营可管理官方音色）。
//
// 功能：
//   - 创建官方音色（上传音频样本 或 音频URL + 文本 → 克隆 + TTS 生成试听）
//   - 列表/搜索/上下架/删除
package handler

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// AdminVoiceHandler 管理后台官方音色管理。
type AdminVoiceHandler struct {
	voiceRepo  port.VoiceLibrary
	audioSynth port.AudioSynthesizer // 可选；nil 时不可创建音色
	mediaStore port.MediaAssetStore  // 可选；nil 时试听音频不转存
}

func NewAdminVoiceHandler(voices port.VoiceLibrary, synth port.AudioSynthesizer, store port.MediaAssetStore) *AdminVoiceHandler {
	return &AdminVoiceHandler{voiceRepo: voices, audioSynth: synth, mediaStore: store}
}

// HandleCreateVoice POST /api/admin/voices
// 创建官方音色：音频样本（文件上传或URL）+ 文本 → 克隆音色 → TTS 生成试听 → 写入 generation_voices。
//
// 请求方式：
//   - multipart/form-data：audio=文件, text=文本, name=名称, language=语言
//   - application/json：audio_url=音频URL, text=文本, name=名称, language=语言
func (h *AdminVoiceHandler) HandleCreateVoice(c *gin.Context) {
	if h.audioSynth == nil {
		fail(c, fmt.Errorf("音色合成功能未配置"))
		return
	}

	var audioData []byte
	var err error

	// 支持两种方式：文件上传 或 音频URL
	contentType := c.ContentType()
	if contentType == "application/json" {
		// JSON 方式：从 audio_url 下载音频
		var req struct {
			AudioURL string `json:"audio_url"`
			Text     string `json:"text"`
			Name     string `json:"name"`
			Language string `json:"language"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, fmt.Errorf("参数错误: %w", err))
			return
		}
		if req.AudioURL == "" {
			fail(c, fmt.Errorf("audio_url 不能为空"))
			return
		}
		// 下载音频
		audioData, err = downloadAudio(req.AudioURL)
		if err != nil {
			fail(c, fmt.Errorf("下载音频失败: %w", err))
			return
		}
		// 设置默认值
		if req.Text == "" {
			req.Text = "你好，我是您的专属音色助手，很高兴为您服务。"
		}
		if req.Name == "" {
			req.Name = fmt.Sprintf("音色-%d", time.Now().UnixNano()%10000)
		}
		if req.Language == "" {
			req.Language = "中文 (普通话)"
		}
		// 继续处理
		h.processVoice(c, audioData, req.Text, req.Name, req.Language)
		return
	}

	// multipart/form-data 方式
	file, _, fErr := c.Request.FormFile("audio")
	if fErr != nil {
		fail(c, fmt.Errorf("请上传音频样本或提供 audio_url"))
		return
	}
	defer file.Close()

	audioData, err = io.ReadAll(file)
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

	h.processVoice(c, audioData, text, name, language)
}

// processVoice 处理音色创建（文件上传和URL方式共用逻辑）。
func (h *AdminVoiceHandler) processVoice(c *gin.Context, audioData []byte, text, name, language string) {
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

// HandleUpdateVoice PUT /api/admin/voices/:id
// 更新音色信息（名称/语言/状态）。
func (h *AdminVoiceHandler) HandleUpdateVoice(c *gin.Context) {
	voiceID := c.Param("id")
	if voiceID == "" {
		fail(c, fmt.Errorf("缺少音色ID"))
		return
	}

	var req struct {
		Name     string `json:"name"`
		Language string `json:"language"`
		Status   string `json:"status"` // active/disabled
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, fmt.Errorf("参数错误: %w", err))
		return
	}

	// 查询现有音色
	voices, err := h.voiceRepo.List(c.Request.Context(), "", voiceID)
	if err != nil || len(voices) == 0 {
		fail(c, fmt.Errorf("音色不存在"))
		return
	}

	// 更新字段
	voice := voices[0]
	if req.Name != "" {
		voice.Name = req.Name
	}
	if req.Language != "" {
		voice.Language = req.Language
	}
	if req.Status != "" {
		voice.Status = req.Status
	}

	if err := h.voiceRepo.Upsert(c.Request.Context(), voice); err != nil {
		fail(c, fmt.Errorf("更新音色失败: %w", err))
		return
	}

	success(c, gin.H{"updated": voiceID})
}

// HandleDeleteVoice DELETE /api/admin/voices/:id
// 删除音色。
func (h *AdminVoiceHandler) HandleDeleteVoice(c *gin.Context) {
	voiceID := c.Param("id")
	if voiceID == "" {
		fail(c, fmt.Errorf("缺少音色ID"))
		return
	}

	// 检查是否为平台音色（只允许删除 platform scope）
	voices, err := h.voiceRepo.List(c.Request.Context(), "", voiceID)
	if err != nil || len(voices) == 0 {
		fail(c, fmt.Errorf("音色不存在"))
		return
	}
	if voices[0].Scope != "platform" {
		fail(c, fmt.Errorf("只能删除平台创建的音色"))
		return
	}

	// 标记为删除（不物理删除，保留历史记录）
	voice := voices[0]
	voice.Status = "deleted"
	if err := h.voiceRepo.Upsert(c.Request.Context(), voice); err != nil {
		fail(c, fmt.Errorf("删除音色失败: %w", err))
		return
	}

	success(c, gin.H{"deleted": voiceID})
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

	// 按 scope 过滤
	scope := c.Query("scope")
	if scope != "" {
		var filtered []entity.GenerationVoice
		for _, v := range voices {
			if v.Scope == scope {
				filtered = append(filtered, v)
			}
		}
		voices = filtered
	}

	success(c, gin.H{"voices": voices})
}

// downloadAudio 从 URL 下载音频。
func downloadAudio(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 50<<20)) // 50MB 上限
}
