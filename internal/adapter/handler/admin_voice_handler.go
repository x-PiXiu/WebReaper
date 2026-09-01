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
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/port"
)

// AdminVoiceHandler 管理后台官方音色管理。
type AdminVoiceHandler struct {
	voiceRepo  port.VoiceLibrary
	audioSynth port.AudioSynthesizer // 可选；nil 时不可创建音色
	mediaStore port.MediaAssetStore  // 可选；nil 时试听音频不转存
	genUC      *generation.GenerationUseCase // 可选；31号 L4-② 创建后异步预热 Vidu 注册
}

func NewAdminVoiceHandler(voices port.VoiceLibrary, synth port.AudioSynthesizer, store port.MediaAssetStore) *AdminVoiceHandler {
	return &AdminVoiceHandler{voiceRepo: voices, audioSynth: synth, mediaStore: store}
}

// SetGenerationUC 注入生成用例（可选——平台音色创建后异步预热 Vidu 注册，
// 首次口播命中窗口零开销；未注入则跳过，按需保障兜底）。
func (h *AdminVoiceHandler) SetGenerationUC(uc *generation.GenerationUseCase) {
	if uc != nil {
		h.genUC = uc
	}
}

// HandleCreateFromVidu POST /api/admin/voices/from-vidu
// 从 Vidu 音色克隆平台音色（白牌化——管理后台用上游音色做种子，产出平台自建音色）：
// 选择一个 Vidu voice_id → 直接下载其 sample_url 官方试听音频作为克隆样本 →
// 克隆出平台 voice_id → 写入 generation_voices(scope=platform)。
// （跳过 TTS 环节——MiMo 不认 Vidu voice_id，且 sample_url 音频品质最接近原声）
func (h *AdminVoiceHandler) HandleCreateFromVidu(c *gin.Context) {
	if h.audioSynth == nil {
		fail(c, fmt.Errorf("音色合成功能未配置"))
		return
	}
	var req struct {
		ViduVoiceID string `json:"vidu_voice_id" binding:"required"` // Vidu 音色 ID（scope=vidu 的参考源）
		Text        string `json:"text"`                              // 试听文本（克隆合成的朗读内容）
		Name        string `json:"name"`                              // 平台音色名称
		Language    string `json:"language"`                          // 语言/分组
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, fmt.Errorf("参数错误: %w", err))
		return
	}
	if req.Text == "" {
		req.Text = "欢迎来到智宸AI，用一句话介绍你的店铺，让更多客人找到你。"
	}
	if req.Name == "" {
		req.Name = "平台音色-" + req.ViduVoiceID[:min(8, len(req.ViduVoiceID))]
	}
	if req.Language == "" {
		req.Language = "平台精选"
	}

	// ① 查 Vidu 音色的 sample_url（官方试听音频，5-15 秒人声——克隆最佳样本）
	vd, err := h.voiceRepo.ListForAdmin(c.Request.Context(), "vidu")
	if err != nil {
		fail(c, err)
		return
	}
	var source *entity.GenerationVoice
	for i := range vd {
		if vd[i].VoiceID == req.ViduVoiceID {
			source = &vd[i]
			break
		}
	}
	if source == nil || source.SampleURL == "" {
		fail(c, fmt.Errorf("Vidu 音色 %s 不存在或无试听音频", req.ViduVoiceID))
		return
	}

	// ② 直接以 Vidu 官方试听音频（公网 S3 URL）为样本写入平台音色。
	// 不再"下载→MiMo 重合成→转存本站"：样本音质最接近原声（04号 §2.3 本意），
	// 且公网可达——Vidu audio-clone 仅收 audio_url（无 base64 字段，实测 data URL
	// 返回 500），本地开发环境的注册链（ensureViduVoiceID）也能直接使用。
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = source.Name
	}
	language := strings.TrimSpace(req.Language)
	if language == "" {
		language = "平台精选"
	}
	voiceID := fmt.Sprintf("platform-%d", time.Now().UnixNano())
	voice := entity.GenerationVoice{
		VoiceID:   voiceID,
		Language:  language,
		Name:      name,
		SampleURL: source.SampleURL, // Vidu 公网源（试听即原声）
		Scope:     "platform",
		Status:    "active",
	}
	if err := h.voiceRepo.Upsert(c.Request.Context(), voice); err != nil {
		fail(c, fmt.Errorf("保存音色失败: %w", err))
		return
	}
	// 31号 L4-②：创建后异步预热 Vidu 注册（公网样本——本地环境亦可成功）
	if h.genUC != nil {
		h.genUC.WarmUpVoiceRegistration(voiceID)
	}
	success(c, gin.H{
		"voice_id":    voiceID,
		"name":        name,
		"language":    language,
		"sample_url":  source.SampleURL,
		"source_vidu": source.VoiceID,
	})
}

// HandleListViduVoices GET /api/admin/voices/vidu-sources
// 列出 Vidu 上游音色（scope=vidu）——仅管理端可见，作为克隆参考源。
func (h *AdminVoiceHandler) HandleListViduVoices(c *gin.Context) {
	voices, err := h.voiceRepo.ListForAdmin(c.Request.Context(), "vidu")
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"voices": voices})
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
	// 31号 L4-②：创建后异步预热 Vidu 注册（上传/URL/from-vidu 三路共用——
	// 首次口播命中窗口零开销；失败静默由按需保障兜底）
	if h.genUC != nil {
		h.genUC.WarmUpVoiceRegistration(voiceID)
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

	// 查询现有音色（全量后按 voice_id 精确匹配——音色无单查接口）
	all, err := h.voiceRepo.ListForAdmin(c.Request.Context(), "")
	if err != nil {
		fail(c, err)
		return
	}
	var found *entity.GenerationVoice
	for i := range all {
		if all[i].VoiceID == voiceID {
			found = &all[i]
			break
		}
	}
	if found == nil {
		fail(c, fmt.Errorf("音色不存在"))
		return
	}

	// 更新字段
	voice := *found
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
	all, err := h.voiceRepo.ListForAdmin(c.Request.Context(), "")
	if err != nil {
		fail(c, err)
		return
	}
	var target *entity.GenerationVoice
	for i := range all {
		if all[i].VoiceID == voiceID {
			target = &all[i]
			break
		}
	}
	if target == nil {
		fail(c, fmt.Errorf("音色不存在"))
		return
	}
	if target.Scope != "platform" {
		fail(c, fmt.Errorf("只能删除平台创建的音色"))
		return
	}

	// 标记为删除（不物理删除，保留历史记录）
	voice := *target
	voice.Status = "deleted"
	if err := h.voiceRepo.Upsert(c.Request.Context(), voice); err != nil {
		fail(c, fmt.Errorf("删除音色失败: %w", err))
		return
	}

	success(c, gin.H{"deleted": voiceID})
}

// HandleListVoices GET /api/admin/voices?scope=vidu|platform|clone
// 管理端全量音色：scope=vidu（克隆参考源）/ platform（平台音色管理）/ clone（用户克隆）。
func (h *AdminVoiceHandler) HandleListVoices(c *gin.Context) {
	voices, err := h.voiceRepo.ListForAdmin(c.Request.Context(), c.Query("scope"))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"voices": voices})
}

// HandleSetDefaultVoice PUT /api/admin/voices/:id/default
// 设为平台默认音色（scope=platform 内仅一条 is_default=true）。
func (h *AdminVoiceHandler) HandleSetDefaultVoice(c *gin.Context) {
	voiceID := c.Param("id")
	if voiceID == "" {
		fail(c, fmt.Errorf("缺少音色 ID"))
		return
	}
	if err := h.voiceRepo.SetDefault(c.Request.Context(), voiceID); err != nil {
		fail(c, fmt.Errorf("设置默认音色失败: %w", err))
		return
	}
	success(c, gin.H{"set_default": voiceID})
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
