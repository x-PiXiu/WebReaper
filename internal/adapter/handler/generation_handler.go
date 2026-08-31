package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/port"
)

// GenerationHandler 统一生成任务 API。
type GenerationHandler struct {
	uc               *generation.GenerationUseCase
	voices           port.VoiceLibrary            // 可选；nil=音色端点不注册
	subjectAssetRepo port.SubjectAssetRepository  // 可选；26 号计划——资产读路径
}

// NewGenerationHandler 创建生成任务 handler。
func NewGenerationHandler(uc *generation.GenerationUseCase) *GenerationHandler {
	return &GenerationHandler{uc: uc}
}

// SetVoiceLibrary 注入官方音色库（可选——main 装配 seed 完成后传入）。
func (h *GenerationHandler) SetVoiceLibrary(v port.VoiceLibrary) {
	h.voices = v
}

// SetSubjectAssetRepo 注入主体资产仓储（可选——26 号计划读路径）。
func (h *GenerationHandler) SetSubjectAssetRepo(r port.SubjectAssetRepository) {
	h.subjectAssetRepo = r
}

// HandleVoices GET /api/v1/generation/voices?language=&q= —— 官方音色库
// （TTS voice_setting_voice_id / 主体与数字人 voice_id 的取值来源）。
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
		BrandID      string             `json:"brand_id"`
		Text         string             `json:"text"`
		Materials    []string           `json:"materials"`
		Template     string             `json:"template"`
		Type         string             `json:"type"` // 生成类型：video/image/audio/voice
		Duration     int                `json:"duration"`
		Quality      string             `json:"quality"`
		AspectRatio  string             `json:"aspect_ratio"` // 画面比例（9:16 等——竖版封面/配图必需，此前全链丢弃致恒 16:9）
		Params       map[string]any     `json:"params"`       // 高级参数透传（seed/style/voice_setting_* 等白名单合并）
		Refs         []entity.PromptRef `json:"refs"`         // BE-GEN-06：@引用素材（translateRefs 按端点翻译）
		Watermark    bool               `json:"watermark"`    // 带水印（傻瓜式客户端不传——管理后台默认值通道）
		OffPeak      bool               `json:"off_peak"`     // 错峰生成（更便宜但更慢；同上）
		SubType      string             `json:"sub_type"`     // 显式端点覆盖（subject 创建主体等——空=自动选择）
		BrollSegments []generation.BrollSegment `json:"broll_segments"` // 29号计划：B-Roll配置
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	task, err := h.uc.UnifiedSubmit(c.Request.Context(), generation.UnifiedSubmitInput{
		TenantID:      middleware.CurrentTenantID(c),
		BrandID:       req.BrandID,
		Text:          req.Text,
		Materials:     req.Materials,
		Template:      req.Template,
		Type:          req.Type,
		Duration:      req.Duration,
		Quality:       req.Quality,
		AspectRatio:   req.AspectRatio,
		Params:        req.Params,
		Refs:          req.Refs, // BE-GEN-06：透传 @引用
		Watermark:     req.Watermark,
		OffPeak:       req.OffPeak,
		SubType:       req.SubType,
		BrollSegments: req.BrollSegments, // 29号计划：B-Roll配置
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
		if len(models) == 0 {
			continue // 无可用模型的端点不下发——前端 CapabilityBanner 据此告警（此前空 models 仍返回致永不告警）
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
// （资产库"删除数字人"；非终态先尽力取消上游。Vidu 无删主体 API，仅移除本地展示）。
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
// retryHintFromErrCode err_code → 前端失败处理建议 key（RetryAuto 稍后重试 /
// RetryManual 改参数重试 / RetryTerminal 不可重试）。
//
// BE-GEN-09：与 usecase.ClassifyError 对齐——此前默认分支过宽（CreditInsufficient
// 等也标 RetryAuto），且空 err_code 返回 RetryAuto 而非 RetryTerminal。
func retryHintFromErrCode(errCode string) string {
	switch {
	// RetryAuto：限流/内部错误/超时——稍后自动重试
	case errCode == "TooManyRequests" || errCode == "SystemThrottling" ||
		errCode == "InternalServiceFailure" || errCode == "OperationInProcess" ||
		errCode == "LocalStuckTimeout":
		return "RetryAuto"
	// RetryManual：积分/风控/内容违规——用户改参数后重试
	case errCode == "CreditInsufficient" || errCode == "TaskPromptPolicyViolation" ||
		errCode == "AuditSubmitIllegal" || errCode == "CreationPolicyViolation" ||
		errCode == "ModelUnavailable" || errCode == "UserCancelled" ||
		errCode == "TaskNotFound":
		return "RetryManual"
	// RetryTerminal：配额耗尽/素材问题/未知错误——不可重试
	case errCode == "QuotaExceeded":
		return "RetryTerminal"
	default:
		// 空 err_code 或未知错误码：保守标记 RetryAuto（Vidu 未返回原因时仍允许重试）
		if errCode == "" {
			return "RetryAuto"
		}
		return "RetryTerminal"
	}
}

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
		"retry_hint": retryHintFromErrCode(t.ErrCode),
		"params":     t.ParamsJSON, "creations": creations,
		"credits": t.Credits, "off_peak": t.OffPeak, "watermark": t.Watermark,
		"retry_count": t.RetryCount,
		"created_at":  t.CreatedAt, "finished_at": t.FinishedAt,
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

// HandleRetryAvatarVideo POST /api/v1/generation/tasks/:id/avatar-video
// 重试/补建分身形象视频（25 号阶段二 D4——幂等：未终态链式任务直接返回）。
func (h *GenerationHandler) HandleRetryAvatarVideo(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("生成服务未配置"))
		return
	}
	task, err := h.uc.RetryAvatarVideo(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, generationTaskToView(task))
}

// HandleListSubjects GET /api/v1/subjects?ownership=system|official|private&page_token=&count=
// 主体库代理（25 号阶段一 + 27 号优化）：
//   - ownership=system：官方主体缓存代理（前端读本地端点，不直连 Vidu）
//   - ownership=official：管理后台创建的官方主体（从 subject_assets 表读取 scope=official）
//   - ownership=private：个人分身列表（本地 generation_tasks 聚合已注册成功的主体）
//   - ownership 空/不传：默认 system（向后兼容）
func (h *GenerationHandler) HandleListSubjects(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("生成服务未配置"))
		return
	}
	ownership := c.DefaultQuery("ownership", "system")
	count := 20
	if v, err := strconv.Atoi(c.Query("count")); err == nil && v > 0 {
		count = v
	}
	if count > 100 {
		count = 100
	}

	switch ownership {
	case "system":
		res, err := h.uc.ListOfficialSubjects(c.Request.Context(), c.Query("page_token"), count)
		if err != nil {
			fail(c, err)
			return
		}
		success(c, res)
	case "official":
		// 管理后台创建的官方主体（从 subject_assets 表读取 scope=official）
		if h.subjectAssetRepo == nil {
			fail(c, fmt.Errorf("主体资产服务未配置"))
			return
		}
		assets, total, err := h.subjectAssetRepo.ListByTenant(
			c.Request.Context(), middleware.CurrentTenantID(c),
			entity.SubjectScopeOfficial, "", count, 0,
		)
		if err != nil {
			fail(c, err)
			return
		}
		success(c, gin.H{"subjects": assets, "total": total})
	case "private":
		res, err := h.uc.ListPersonalSubjects(c.Request.Context(), middleware.CurrentTenantID(c), count)
		if err != nil {
			fail(c, err)
			return
		}
		success(c, res)
	default:
		fail(c, fmt.Errorf("不支持的 ownership 值: %s（可选 system / official / private）", ownership))
	}
}

// HandleListSubjectAssets GET /api/v1/subjects/mine?kind=&limit=&offset=
// 个人主体资产列表（26 号计划——从 subject_assets 表读取，失败任务天然不出现）。
func (h *GenerationHandler) HandleListSubjectAssets(c *gin.Context) {
	if h.subjectAssetRepo == nil {
		fail(c, fmt.Errorf("主体资产服务未配置"))
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	kind := c.Query("kind") // person / scene / 空=全部
	limit := 20
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v >= 0 {
		offset = v
	}

	assets, total, err := h.subjectAssetRepo.ListByTenant(c.Request.Context(), tenantID, entity.SubjectScopePersonal, kind, limit, offset)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"assets": assets, "total": total})
}
