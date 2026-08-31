// admin_health_handler.go 管理后台系统健康总览（Admin LLM Tools 配套）。
//
// GET /api/v1/admin/system/health —— 聚合系统各维度的实时健康数据：
//   - 生成任务：活跃数/最近失败率/排队积压
//   - Vidu：积分余额 + API 可达性
//   - 存储：用量统计
//   - 音色/主体：平台资产数量
//   - 服务商：MiMo/ASR 配置状态
package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/port"
)

// AdminHealthHandler 系统健康总览。
type AdminHealthHandler struct {
	genUC       *generation.GenerationUseCase
	voiceRepo   port.VoiceLibrary
	subjectRepo port.SubjectAssetRepository
	settingRepo port.SystemSettingRepository
	provider    port.GenerationProvider
}

func NewAdminHealthHandler(
	genUC *generation.GenerationUseCase,
	voices port.VoiceLibrary,
	subjects port.SubjectAssetRepository,
	settings port.SystemSettingRepository,
	provider port.GenerationProvider,
) *AdminHealthHandler {
	return &AdminHealthHandler{
		genUC: genUC, voiceRepo: voices, subjectRepo: subjects,
		settingRepo: settings, provider: provider,
	}
}

// HandleSystemHealth GET /api/v1/admin/system/health
func (h *AdminHealthHandler) HandleSystemHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	health := gin.H{"timestamp": time.Now().Format(time.RFC3339)}

	// ① 生成任务统计（跨租户——活跃 + 最近失败分开查）
	if h.genUC != nil {
		active, activeErr := h.genUC.ListActiveAll(ctx, 200)
		failed, failedErr := h.genUC.ListRecentFailed(ctx, 50)
		if activeErr == nil {
			var queueing int
			for _, t := range active {
				if t.State == entity.TaskStateQueueing || t.State == entity.TaskStateCreated {
					queueing++
				}
			}
			health["tasks"] = gin.H{
				"active":   len(active),
				"queueing": queueing,
				"failed":   maxInt(failedErr == nil, len(failed)),
			}
		}
	}

	// ② Vidu 积分 + 可达性
	if h.provider != nil {
		credits, err := h.provider.QueryCredits(ctx)
		status := "ok"
		if err != nil {
			status = "error: " + err.Error()
			credits = -1
		}
		health["vidu"] = gin.H{"credits": credits, "status": status}
	}

	// ③ 平台资产
	if h.voiceRepo != nil {
		platformVoices, _ := h.voiceRepo.ListForAdmin(ctx, "platform")
		viduVoices, _ := h.voiceRepo.ListForAdmin(ctx, "vidu")
		health["voices"] = gin.H{
			"platform": len(platformVoices),
			"vidu_refs": len(viduVoices),
		}
	}
	if h.subjectRepo != nil {
		officialSubjects, _, _ := h.subjectRepo.ListByTenant(ctx, "", "official", "", 100, 0)
		health["subjects"] = gin.H{"official": len(officialSubjects)}
	}

	// ④ 存储配置
	if h.settingRepo != nil {
		autoMonitor, _ := h.settingRepo.Get(ctx, "auto_monitor_enabled")
		browserHeaded, _ := h.settingRepo.Get(ctx, "browser_headed")
		chainAvatar, _ := h.settingRepo.Get(ctx, "gen_chain_avatar_video")
		health["settings"] = gin.H{
			"auto_monitor":      autoMonitor.Value,
			"browser_headed":    browserHeaded.Value,
			"chain_avatar":      chainAvatar.Value,
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": health})
}

func maxInt(cond bool, v int) int {
	if cond {
		return v
	}
	return 0
}

// HandleListAllTasks GET /api/v1/admin/tasks?state=&limit=
// 跨租户生成任务列表（管理后台监控用——按状态筛选，含失败原因）。
func (h *AdminHealthHandler) HandleListAllTasks(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	state := c.Query("state")
	limit := 50
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 200 {
		limit = v
	}

	var tasks []entity.GenerationTask
	var err error
	switch state {
	case "failed":
		tasks, err = h.genUC.ListRecentFailed(ctx, limit)
	case "active":
		tasks, err = h.genUC.ListActiveAll(ctx, limit)
	default:
		tasks, err = h.genUC.ListActiveAll(ctx, limit)
		if err == nil {
			failed, ferr := h.genUC.ListRecentFailed(ctx, limit)
			if ferr == nil {
				tasks = append(tasks, failed...)
			}
		}
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50000, "msg": err.Error()})
		return
	}

	// 转视图（不返回完整 params）
	type taskView struct {
		ID        string `json:"id"`
		TenantID  string `json:"tenant_id"`
		SubType   string `json:"sub_type"`
		State     string `json:"state"`
		Model     string `json:"model"`
		ErrMsg    string `json:"err_msg,omitempty"`
		CreatedAt string `json:"created_at"`
	}
	views := make([]taskView, 0, len(tasks))
	for _, t := range tasks {
		views = append(views, taskView{
			ID: t.ID, TenantID: t.TenantID[:min(20, len(t.TenantID))],
			SubType: t.SubType, State: t.State, Model: t.Model,
			ErrMsg: t.ErrMsg, CreatedAt: t.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"tasks": views, "total": len(views)}})
}

// HandleAdminCancelTask POST /api/v1/admin/tasks/:id/cancel
// 管理员跨租户取消任务。
func (h *AdminHealthHandler) HandleAdminCancelTask(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40000, "msg": "缺少任务 ID"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.genUC.Cancel(ctx, "", taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50000, "msg": "取消失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"cancelled": taskID}})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// HandleGetGenSettings GET /api/v1/admin/settings/gen
// 读取生成域业务配置项（gen_* 前缀的 system_settings）。
func (h *AdminHealthHandler) HandleGetGenSettings(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	keys := []string{
		"gen_chain_avatar_video",
		"gen_default_avatar_prompt",
		"gen_default_voice_id",
		"gen_default_resolution",
		"gen_default_aspect_ratio",
		"gen_default_watermark",
		"gen_default_off_peak",
		"auto_publish_enabled",
	}
	result := make(map[string]string)
	for _, key := range keys {
		v := ""
		if h.settingRepo != nil {
			if st, err := h.settingRepo.Get(ctx, key); err == nil {
				v = st.Value
			}
		}
		result[key] = v
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": result})
}

// HandleSetGenSetting PUT /api/v1/admin/settings/gen/:key
// 设置单个生成域配置项。
func (h *AdminHealthHandler) HandleSetGenSetting(c *gin.Context) {
	key := c.Param("key")
	if key == "" || !strings.HasPrefix(key, "gen_") {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40000, "msg": "仅支持 gen_* 前缀配置项"})
		return
	}
	var req struct{ Value string `json:"value"` }
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40000, "msg": "参数错误"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := h.settingRepo.Save(ctx, entity.SystemSetting{Key: key, Value: req.Value}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50000, "msg": "保存失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"key": key, "value": req.Value}})
}
