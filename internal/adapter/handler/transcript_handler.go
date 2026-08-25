package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/videotranscript"
)

// TranscriptHandler 视频文案提取（08 计划 D4——向导第①步的提取入口）。
type TranscriptHandler struct {
	uc   *videotranscript.UseCase
	asset port.MediaAssetStore // 可选；asset_url → 本地文件提取
}

// NewTranscriptHandler 创建提取 handler。
func NewTranscriptHandler(uc *videotranscript.UseCase, asset port.MediaAssetStore) *TranscriptHandler {
	return &TranscriptHandler{uc: uc, asset: asset}
}

// HandleExtract POST /api/v1/generation/transcript/extract
// 请求体（三选一）：{video_url 直链 | share_url 分享链 | asset_url 本站上传资产}。
func (h *TranscriptHandler) HandleExtract(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("提取服务未配置"))
		return
	}
	var req struct {
		VideoURL string `json:"video_url"`
		ShareURL string `json:"share_url"`
		AssetURL string `json:"asset_url"`
		Title    string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	// 本站上传资产：ReadLocal 读字节落临时文件（safeDownload 会拒本站 URL——环回）
	if req.VideoURL == "" && req.ShareURL == "" && req.AssetURL != "" {
		if h.asset == nil {
			fail(c, fmt.Errorf("素材存储未配置"))
			return
		}
		data, _, ok := h.asset.ReadLocal(c.Request.Context(), req.AssetURL)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"code": 40000, "msg": "素材不存在或非本站托管"})
			return
		}
		f, err := os.CreateTemp("", "vt-up-*.mp4")
		if err != nil {
			fail(c, err)
			return
		}
		defer os.Remove(f.Name())
		if _, err := f.Write(data); err != nil {
			f.Close()
			fail(c, err)
			return
		}
		f.Close()
		res, err := h.uc.ExtractFromFile(c.Request.Context(), tenantID, f.Name(), req.Title)
		h.respond(c, res, err)
		return
	}
	res, err := h.uc.Extract(c.Request.Context(), videotranscript.ExtractInput{
		TenantID: tenantID, VideoURL: req.VideoURL, ShareURL: req.ShareURL, Title: req.Title,
	})
	h.respond(c, res, err)
}

func (h *TranscriptHandler) respond(c *gin.Context, res *videotranscript.ExtractResult, err error) {
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40000, "msg": err.Error()})
		return
	}
	success(c, gin.H{
		"raw_text": res.RawText, "title": res.Title, "method": res.Method,
		"raw_text_lines": res.Lines, // 按句切分的行（一行一句——raw_text 已按换行分行，数组供结构化渲染）
	})
}

// HandleRewrite POST /api/v1/generation/transcript/rewrite —— 原文 → 双产出。
// 请求体：{raw_text, topic}；topic 为用户一句话意图（向导侧拼品牌上下文）。
func (h *TranscriptHandler) HandleRewrite(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("提取服务未配置"))
		return
	}
	var req struct {
		RawText string `json:"raw_text" binding:"required"`
		Topic   string `json:"topic"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	res, err := h.uc.RewriteScript(c.Request.Context(), req.RawText, req.Topic)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40000, "msg": err.Error()})
		return
	}
	success(c, gin.H{"clean": res.Clean, "rewrite": res.Rewrite})
}

var _ = context.Background
