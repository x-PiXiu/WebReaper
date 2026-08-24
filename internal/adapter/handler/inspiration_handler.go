package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"webreaper/internal/usecase/inspiration"
)

// InspirationHandler 灵感广场 API（用户端，无需登录）。
type InspirationHandler struct {
	uc *inspiration.UseCase
}

func NewInspirationHandler(uc *inspiration.UseCase) *InspirationHandler {
	return &InspirationHandler{uc: uc}
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

	success(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     videos,
	})
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
