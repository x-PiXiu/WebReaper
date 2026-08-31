// admin_subject_handler.go 管理后台官方主体管理（27 号优化——运营可管理官方主体）。
//
// 功能：
//   - 创建官方主体（上传形象图 → 调 Vidu 注册 → 写 subject_assets）
//   - 列表/搜索/上下架/删除
//   - 为官方主体生成形象视频
package handler

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/port"
)

// AdminSubjectHandler 管理后台官方主体管理。
type AdminSubjectHandler struct {
	genUC          *generation.GenerationUseCase
	subjectAssetRepo port.SubjectAssetRepository
	mediaStore       port.MediaAssetStore
}

func NewAdminSubjectHandler(genUC *generation.GenerationUseCase, repo port.SubjectAssetRepository, store port.MediaAssetStore) *AdminSubjectHandler {
	return &AdminSubjectHandler{genUC: genUC, subjectAssetRepo: repo, mediaStore: store}
}

// HandleCreateOfficialSubject POST /api/admin/subjects
// 创建官方主体：上传形象图 → 调 Vidu 注册 → 写 subject_assets(scope=official)。
func (h *AdminSubjectHandler) HandleCreateOfficialSubject(c *gin.Context) {
	var req struct {
		Name      string   `json:"name" binding:"required"`
		Images    []string `json:"images" binding:"required,min=1"`
		VoiceID   string   `json:"voice_id"`
		Kind      string   `json:"kind"` // person/scene，默认 person
		Tags      string   `json:"tags"`
		SortOrder int      `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, fmt.Errorf("参数错误: %w", err))
		return
	}
	if req.Kind == "" {
		req.Kind = entity.SubjectKindPerson
	}

	// 调用 Vidu 注册主体（通过统一提交，sub_type=subject）
	task, err := h.genUC.Submit(c.Request.Context(), generation.SubmitInput{
		TenantID: middleware.CurrentTenantID(c),
		SubType:  "subject",
		Params: entity.GenerationParams{
			"name":   req.Name,
			"images": req.Images,
			"kind":   req.Kind,
		},
	})
	if err != nil {
		fail(c, fmt.Errorf("主体注册失败: %w", err))
		return
	}
	if task.State != entity.TaskStateSuccess {
		fail(c, fmt.Errorf("主体注册未成功（状态: %s）: %s", task.State, task.ErrMsg))
		return
	}

	// 提取 server_id
	serverID := task.ProviderTaskID
	if serverID == "" {
		serverID = firstCreationID(task.CreationsJSON)
	}
	if serverID == "" {
		fail(c, fmt.Errorf("主体注册成功但未获取到 server_id"))
		return
	}

	// 写入 subject_assets (scope=official)
	asset := entity.SubjectAsset{
		ID:           fmt.Sprintf("official-%d", time.Now().UnixNano()),
		TenantID:     middleware.CurrentTenantID(c),
		Scope:        entity.SubjectScopeOfficial,
		Kind:         req.Kind,
		Name:         req.Name,
		ServerID:     serverID,
		PortraitURL:  req.Images[0],
		VoiceID:      req.VoiceID,
		Tags:         req.Tags,
		SortOrder:    req.SortOrder,
		Status:       entity.SubjectStatusActive,
		SourceTaskID: task.ID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := h.subjectAssetRepo.Upsert(c.Request.Context(), asset); err != nil {
		fail(c, fmt.Errorf("保存主体资产失败: %w", err))
		return
	}

	// 自动触发链式形象视频
	go func() {
		_, _ = h.genUC.RetryAvatarVideo(c.Request.Context(), middleware.CurrentTenantID(c), task.ID)
	}()

	success(c, gin.H{
		"id":        asset.ID,
		"server_id": serverID,
		"name":      req.Name,
		"status":    "active",
	})
}

// HandleListOfficialSubjects GET /api/admin/subjects?kind=&limit=&offset=
func (h *AdminSubjectHandler) HandleListOfficialSubjects(c *gin.Context) {
	kind := c.Query("kind")
	limit := 20
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v >= 0 {
		offset = v
	}

	assets, total, err := h.subjectAssetRepo.ListByTenant(
		c.Request.Context(), middleware.CurrentTenantID(c),
		entity.SubjectScopeOfficial, kind, limit, offset,
	)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"subjects": assets, "total": total})
}

// HandleUpdateOfficialSubject PUT /api/admin/subjects/:id
func (h *AdminSubjectHandler) HandleUpdateOfficialSubject(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name      string `json:"name"`
		Tags      string `json:"tags"`
		SortOrder *int   `json:"sort_order"`
		Status    string `json:"status"` // active/disabled
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, fmt.Errorf("参数错误: %w", err))
		return
	}

	asset, err := h.subjectAssetRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		fail(c, fmt.Errorf("主体不存在"))
		return
	}

	if req.Name != "" {
		asset.Name = req.Name
	}
	if req.Tags != "" {
		asset.Tags = req.Tags
	}
	if req.SortOrder != nil {
		asset.SortOrder = *req.SortOrder
	}
	if req.Status != "" && req.Status != asset.Status {
		if err := h.subjectAssetRepo.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
			fail(c, fmt.Errorf("更新状态失败: %w", err))
			return
		}
	}
	asset.UpdatedAt = time.Now()
	if err := h.subjectAssetRepo.Upsert(c.Request.Context(), asset); err != nil {
		fail(c, fmt.Errorf("保存失败: %w", err))
		return
	}
	success(c, gin.H{"updated": id})
}

// HandleDeleteOfficialSubject DELETE /api/admin/subjects/:id
func (h *AdminSubjectHandler) HandleDeleteOfficialSubject(c *gin.Context) {
	id := c.Param("id")
	if err := h.subjectAssetRepo.Delete(c.Request.Context(), id); err != nil {
		fail(c, fmt.Errorf("删除失败: %w", err))
		return
	}
	success(c, gin.H{"deleted": id})
}

// firstCreationID 解析 creations JSON 的首个 id。
func firstCreationID(creationsJSON string) string {
	if creationsJSON == "" {
		return ""
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(creationsJSON), &arr); err != nil || len(arr) == 0 {
		return ""
	}
	if id, ok := arr[0]["id"].(string); ok {
		return id
	}
	return ""
}
