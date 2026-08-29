// timeline_handler.go B-Roll 台词时间轴端点（22 号计划 §5.4①②）。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/usecase/port"
)

// TimelineHandler 台词时间轴定位/读取。
type TimelineHandler struct {
	composer port.Composer
}

// NewTimelineHandler 创建。
func NewTimelineHandler(composer port.Composer) *TimelineHandler {
	return &TimelineHandler{composer: composer}
}

// HandleLocate POST /api/v1/generation/tasks/:id/timeline
//
//	请求体（均可选）：{"force": false, "lines_override": [{"index":0,"text":"修正文字"}]}
//	行为：lines_override 非空 → 只修正各行文字（时间窗不动）；否则定位（force=重跑）。
func (h *TimelineHandler) HandleLocate(c *gin.Context) {
	if h.composer == nil {
		fail(c, errComposerNil)
		return
	}
	var req struct {
		Force         bool                       `json:"force"`
		LinesOverride []port.TimelineLineOverride `json:"lines_override"`
	}
	_ = c.ShouldBindJSON(&req) // 空请求体合法（默认定位）

	lines, source, err := h.composer.LocateTimeline(
		c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"),
		req.Force, req.LinesOverride)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40000, "msg": err.Error()})
		return
	}
	success(c, gin.H{"lines": lines, "script_source": source})
}

// HandleGet GET /api/v1/generation/tasks/:id/timeline（读取已定位时间轴）。
func (h *TimelineHandler) HandleGet(c *gin.Context) {
	if h.composer == nil {
		fail(c, errComposerNil)
		return
	}
	lines, source, err := h.composer.GetTimeline(
		c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 40400, "msg": err.Error()})
		return
	}
	success(c, gin.H{"lines": lines, "script_source": source})
}

var errComposerNil = errComposerNotReady{}

type errComposerNotReady struct{}

func (errComposerNotReady) Error() string { return "B-Roll 服务未配置" }
