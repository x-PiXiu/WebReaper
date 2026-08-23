package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/port"
)

// GenerationHandler 统一生成任务 API。
type GenerationHandler struct {
	uc     *generation.GenerationUseCase
	voices port.VoiceLibrary // 可选；nil=音色端点不注册
}

// NewGenerationHandler 创建生成任务 handler。
func NewGenerationHandler(uc *generation.GenerationUseCase) *GenerationHandler {
	return &GenerationHandler{uc: uc}
}

// SetVoiceLibrary 注入官方音色库（可选——main 装配 seed 完成后传入）。
func (h *GenerationHandler) SetVoiceLibrary(v port.VoiceLibrary) {
	h.voices = v
}

// HandleVoices GET /api/v1/generation/voices?language=&q= —— 官方音色库
//（TTS voice_setting_voice_id / 主体与数字人 voice_id 的取值来源）。
func (h *GenerationHandler) HandleVoices(c *gin.Context) {
	if h.voices == nil {
		fail(c, fmt.Errorf("音色库未配置"))
		return
	}
	list, err := h.voices.List(c.Request.Context(), c.Query("language"), c.Query("q"))
	if err != nil {
		fail(c, err)
		return
	}
	if list == nil {
		list = []entity.GenerationVoice{}
	}
	success(c, gin.H{"voices": list})
}

// HandleUnifiedSubmit POST /api/v1/generation/submit —— 统一提交（傻瓜式）。
//
// 客户端只需要：
//   - brand_id：品牌ID
//   - text：文本描述
//   - materials：素材ID列表（可选）
//   - template：模板ID（可选）
//   - duration：时长（可选）
//   - quality：质量（可选）
//
// 系统自动：
//   - 根据素材选择端点
//   - 选择默认模型
//   - 填充默认参数
func (h *GenerationHandler) HandleUnifiedSubmit(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("生成服务未配置"))
		return
	}

	var req struct {
		BrandID   string   `json:"brand_id"`
		Text      string   `json:"text"`
		Materials []string `json:"materials"`
		Template  string   `json:"template"`
		Type      string   `json:"type"`      // 生成类型：video/image/audio/voice
		Duration  int      `json:"duration"`
		Quality   string   `json:"quality"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	task, err := h.uc.UnifiedSubmit(c.Request.Context(), generation.UnifiedSubmitInput{
		TenantID:  middleware.CurrentTenantID(c),
		BrandID:   req.BrandID,
		Text:      req.Text,
		Materials: req.Materials,
		Template:  req.Template,
		Type:      req.Type,
		Duration:  req.Duration,
		Quality:   req.Quality,
	})
	if err != nil {
		// 参数校验类错误 400；配额 402 由 fail 统一映射
		c.JSON(http.StatusBadRequest, gin.H{"code": 40000, "msg": err.Error()})
		return
	}
	success(c, generationTaskToView(task))
}

// HandleGet GET /api/v1/generation/tasks/:id
func (h *GenerationHandler) HandleGet(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("生成服务未配置"))
		return
	}
	task, err := h.uc.Get(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, generationTaskToView(task))
}

// HandleList GET /api/v1/generation/tasks
func (h *GenerationHandler) HandleList(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("生成服务未配置"))
		return
	}
	tasks, err := h.uc.List(c.Request.Context(), middleware.CurrentTenantID(c), 50)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, generationTaskToView(t))
	}
	success(c, gin.H{"tasks": out})
}

// HandleTypes GET /api/v1/generation/types —— 端点类型 + 模型能力向量（前端表单驱动）。
func (h *GenerationHandler) HandleTypes(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("生成服务未配置"))
		return
	}
	types := h.uc.Types()
	out := make([]gin.H, 0, len(types))
	for _, t := range types {
		caps, err := h.uc.Capabilities(c.Request.Context(), t)
		if err != nil {
			continue
		}
		models := make([]gin.H, 0, len(caps))
		for _, cap := range caps {
			models = append(models, gin.H{"model": cap.Model, "capability": cap})
		}
		out = append(out, gin.H{"sub_type": t, "models": models})
	}
	success(c, gin.H{"types": out})
}

// HandleCancel POST /api/v1/generation/tasks/:id/cancel
func (h *GenerationHandler) HandleCancel(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("生成服务未配置"))
		return
	}
	if err := h.uc.Cancel(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"cancelled": c.Param("id")})
}

// HandleDelete DELETE /api/v1/generation/tasks/:id —— 删除本地产任务记录
//（资产库"删除数字人"；非终态先尽力取消上游。Vidu 无删主体 API，仅移除本地展示）。
func (h *GenerationHandler) HandleDelete(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("生成服务未配置"))
		return
	}
	if err := h.uc.DeleteTask(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": c.Param("id")})
}

// HandleCallback POST /api/v1/generation/callback —— Vidu 回调入口（验签 + 幂等推进）。
// 签名头在 X-HMAC-*；验签由注入的 provider 完成（mock 放行）。
func (h *GenerationHandler) HandleCallback(c *gin.Context, provider port.GenerationProvider) {
	if h.uc == nil || provider == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成服务未配置"})
		return
	}
	// ① nonce 防重放
	nonce := c.GetHeader("x-request-nonce")
	if nonce == "" || !h.uc.CheckCallbackNonce(c.Request.Context(), nonce) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "重复或缺失的 nonce"})
		return
	}
	// ② 验签（requestURI 从请求行还原——签名字符串基于 callback_url 的 path/query）
	body, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err := provider.VerifyCallback(c.Request.Context(), c.Request.Header, body, c.Request.URL.RequestURI()); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "签名校验失败"})
		return
	}
	// ③ 解析回调体（状态 + payload/id + 生成物——结构同查询任务 API 返回体）
	var payload struct {
		ID        string `json:"id"` // 服务商任务 ID（payload 未透传端点的定位兜底）
		State     string `json:"state"`
		ErrCode   string `json:"err_code"`
		Payload   string `json:"payload"`
		Creations []struct {
			ID             string `json:"id"`
			URL            string `json:"url"`
			CoverURL       string `json:"cover_url"`
			WatermarkedURL string `json:"watermarked_url"`
		} `json:"creations"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "回调体解析失败"})
		return
	}
	status := port.GenerationStatus{State: payload.State, ErrCode: payload.ErrCode}
	for _, cr := range payload.Creations {
		status.Creations = append(status.Creations, entity.CreationItem{
			ID: cr.ID, URL: cr.URL, CoverURL: cr.CoverURL, WatermarkedURL: cr.WatermarkedURL,
		})
	}
	// ④ 幂等推进（payload 优先关联本地任务；兜底按回调体 id → provider_task_id）
	_, err := h.uc.HandleCallback(c.Request.Context(), payload.Payload, payload.ID, status)
	if err != nil {
		// 任务不存在：先按 provider_task_id 兜底再失败
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// generationTaskToView 任务 → API 契约（snake_case）。
func generationTaskToView(t entity.GenerationTask) gin.H {
	creations := []gin.H{}
	if t.CreationsJSON != "" {
		var items []entity.CreationItem
		if json.Unmarshal([]byte(t.CreationsJSON), &items) == nil {
			for _, it := range items {
				creations = append(creations, gin.H{
					"id": it.ID, "url": it.URL, "cover_url": it.CoverURL,
					"watermarked_url": it.WatermarkedURL, "stored_url": it.StoredURL,
				})
			}
		}
	}
	return gin.H{
		"id": t.ID, "tenant_id": t.TenantID, "brand_id": t.BrandID,
		"type": t.Type, "sub_type": t.SubType, "model": t.Model,
		"provider": t.Provider, "provider_task_id": t.ProviderTaskID,
		"state": t.State, "err_code": t.ErrCode, "err_msg": t.ErrMsg,
		"params": t.ParamsJSON, "creations": creations,
		"credits": t.Credits, "off_peak": t.OffPeak, "watermark": t.Watermark,
		"retry_count": t.RetryCount,
		"created_at": t.CreatedAt, "finished_at": t.FinishedAt,
	}
}

// validateRefsOwnership 引用素材租户归属校验。
// 本站托管路径（/media/）的素材文件名以 {tenantID}- 为前缀（LocalMediaStore 命名规则）——
// 校验防止 A 租户引用 B 租户的素材（越权）。外部 URL（用户自己的图床等）放行。
func validateRefsOwnership(tenantID string, refs []entity.PromptRef) error {
	for _, r := range refs {
		// 仅校验本站 /media/ 托管路径；外部 URL（图床/OSS）不校验
		idx := strings.Index(r.URL, "/media/")
		if idx < 0 {
			continue
		}
		fileName := r.URL[idx+len("/media/"):]
		// 文件名必须以 {tenantID}- 开头（material 命名：{tenant}-{ts}{ext}；creation：c-{tenant}-{ts}{ext}）
		if !strings.HasPrefix(fileName, tenantID+"-") && !strings.HasPrefix(fileName, "c-"+tenantID+"-") {
			return fmt.Errorf("引用素材 %s 不属于当前租户", fileName)
		}
	}
	return nil
}
