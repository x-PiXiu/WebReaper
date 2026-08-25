package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/usecase/inspiration"
	"webreaper/internal/usecase/port"
)

// InspirationHandler 灵感广场 API（用户端，无需登录平台账号）。
type InspirationHandler struct {
	uc        *inspiration.UseCase
	videoRepo port.InspirationVideoRepository
	brandRepo port.BrandRepository // 用于校验品牌归属
}

func NewInspirationHandler(uc *inspiration.UseCase, videoRepo port.InspirationVideoRepository) *InspirationHandler {
	return &InspirationHandler{uc: uc, videoRepo: videoRepo}
}

// SetBrandRepo 注入品牌仓储（用于租户隔离校验）。
func (h *InspirationHandler) SetBrandRepo(repo port.BrandRepository) {
	h.brandRepo = repo
}

// HandleList GET /api/v1/inspirations —— 灵感视频列表。
//
// Query 参数：
//   - brand_id: 品牌筛选（空=全部品牌）
//   - platform: 平台筛选（空=全部平台）
//   - keyword: 关键词搜索
//   - sort_by: 排序（viral_score/play_count/digg_count/publish_time）
//   - page: 页码（默认 1）
//   - page_size: 每页数量（默认 20）
func (h *InspirationHandler) HandleList(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("灵感服务未配置"))
		return
	}

	tenantID := middleware.CurrentTenantID(c)
	brandID := c.Query("brand_id")
	platform := c.Query("platform")
	keyword := c.Query("keyword")
	sortBy := c.DefaultQuery("sort_by", "viral_score")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// 租户隔离：校验品牌是否属于当前租户
	if brandID != "" && h.brandRepo != nil {
		if _, err := h.brandRepo.FindByID(c.Request.Context(), tenantID, brandID); err != nil {
			fail(c, fmt.Errorf("品牌不存在或无权访问"))
			return
		}
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}

	videos, total, err := h.uc.List(c.Request.Context(), brandID, platform, keyword, sortBy, page, pageSize)
	if err != nil {
		fail(c, err)
		return
	}

	// 构建响应
	resp := gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     videos,
	}

	// 无数据时返回采集状态提示
	if total == 0 {
		if brandID != "" {
			resp["status"] = "collecting"
			resp["message"] = "该品牌的热门视频正在采集中，请稍后再来查看"
		} else {
			resp["status"] = "collecting"
			resp["message"] = "热门视频正在采集中，请稍后再来查看"
		}
	} else {
		resp["status"] = "ready"
	}

	success(c, resp)
}

// HandleGet GET /api/v1/inspirations/:id —— 灵感视频详情。
func (h *InspirationHandler) HandleGet(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("灵感服务未配置"))
		return
	}

	video, err := h.uc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}

	success(c, video)
}

// HandleListPlatforms GET /api/v1/inspirations/platforms —— 已注册平台列表。
func (h *InspirationHandler) HandleListPlatforms(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("灵感服务未配置"))
		return
	}

	platforms := h.uc.ListPlatforms()
	success(c, gin.H{"platforms": platforms})
}

// HandleUpdateInspiration PUT /api/v1/admin/inspirations/:id —— 更新灵感（置顶/推荐/备注）。
func (h *InspirationHandler) HandleUpdateInspiration(c *gin.Context) {
	if h.videoRepo == nil {
		fail(c, fmt.Errorf("灵感服务未配置"))
		return
	}

	id := c.Param("id")
	video, err := h.videoRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		fail(c, fmt.Errorf("灵感不存在"))
		return
	}

	var req struct {
		IsPinned      *bool   `json:"is_pinned"`
		IsRecommended *bool   `json:"is_recommended"`
		AdminNote     *string `json:"admin_note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	if req.IsPinned != nil {
		video.IsPinned = *req.IsPinned
	}
	if req.IsRecommended != nil {
		video.IsRecommended = *req.IsRecommended
	}
	if req.AdminNote != nil {
		video.AdminNote = *req.AdminNote
	}

	if err := h.videoRepo.Update(c.Request.Context(), video); err != nil {
		fail(c, fmt.Errorf("更新失败: %w", err))
		return
	}

	// 前端期望 {msg, id}（此前连写两次 success() 输出双 JSON 拼接体，解析必失败）
	success(c, gin.H{"msg": "更新成功", "id": id})
}

// HandleDeleteInspiration DELETE /api/v1/admin/inspirations/:id —— 删除灵感。
func (h *InspirationHandler) HandleDeleteInspiration(c *gin.Context) {
	if h.videoRepo == nil {
		fail(c, fmt.Errorf("灵感服务未配置"))
		return
	}

	id := c.Param("id")
	if err := h.videoRepo.Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{"msg": "删除成功", "id": id})
}

// HandleBatchInspirations POST /api/v1/admin/inspirations/batch —— 批量操作。
func (h *InspirationHandler) HandleBatchInspirations(c *gin.Context) {
	if h.videoRepo == nil {
		fail(c, fmt.Errorf("灵感服务未配置"))
		return
	}

	var req struct {
		IDs    []string `json:"ids" binding:"required"`
		Action string   `json:"action" binding:"required"` // delete / pin / recommend
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	affected := 0
	for _, id := range req.IDs {
		switch req.Action {
		case "delete":
			if err := h.videoRepo.Delete(c.Request.Context(), id); err == nil {
				affected++
			}
		default:
			// pin/recommend 需要 Update 方法
		}
	}

	success(c, gin.H{"msg": "批量操作完成", "affected": affected})
}

// HandleBrandsStats GET /api/v1/inspirations/brands —— 各品牌灵感数量统计。
func (h *InspirationHandler) HandleBrandsStats(c *gin.Context) {
	if h.uc == nil || h.videoRepo == nil {
		fail(c, fmt.Errorf("灵感服务未配置"))
		return
	}

	brands, err := h.videoRepo.CountByBrand(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{"brands": brands})
}

// HandleStats GET /api/v1/admin/inspirations/stats —— 统计看板。
func (h *InspirationHandler) HandleStats(c *gin.Context) {
	if h.videoRepo == nil {
		fail(c, fmt.Errorf("灵感服务未配置"))
		return
	}

	platforms, err := h.videoRepo.CountByPlatform(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}

	brands, err := h.videoRepo.CountByBrand(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}

	totalVideos := 0
	for _, p := range platforms {
		totalVideos += p.Count
	}

	success(c, gin.H{
		"total_videos": totalVideos,
		"total_brands": len(brands),
		"by_platform":  platforms,
		"by_brand":     brands,
	})
}
