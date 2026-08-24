package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"webreaper/internal/usecase/inspiration"
	"webreaper/internal/usecase/port"
)

// InspirationHandler 灵感广场 API（用户端，无需登录平台账号）。
type InspirationHandler struct {
	uc        *inspiration.UseCase
	videoRepo port.InspirationVideoRepository
}

func NewInspirationHandler(uc *inspiration.UseCase, videoRepo port.InspirationVideoRepository) *InspirationHandler {
	return &InspirationHandler{uc: uc, videoRepo: videoRepo}
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

	brandID := c.Query("brand_id")
	platform := c.Query("platform")
	keyword := c.Query("keyword")
	sortBy := c.DefaultQuery("sort_by", "viral_score")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

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
