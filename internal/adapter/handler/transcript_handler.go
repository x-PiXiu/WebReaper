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

// HandleExtractAsync POST /api/v1/generation/transcript/extract/async
// 长视频防前端超时：立即返回 task_id，前端轮询 HandleGetAsyncTask。
// 请求体同 HandleExtract（video_url / share_url 二选一；asset_url 不支持异步）。
func (h *TranscriptHandler) HandleExtractAsync(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("提取服务未配置"))
		return
	}
	var req struct {
		VideoURL string `json:"video_url"`
		ShareURL string `json:"share_url"`
		Title    string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if req.VideoURL == "" && req.ShareURL == "" {
		fail(c, fmt.Errorf("请提供视频链接或分享链接"))
		return
	}
	taskID, err := h.uc.ExtractAsync(videotranscript.ExtractInput{
		TenantID: middleware.CurrentTenantID(c),
		VideoURL: req.VideoURL, ShareURL: req.ShareURL, Title: req.Title,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"task_id": taskID, "status": "pending"})
}

// HandleGetAsyncTask GET /api/v1/generation/transcript/extract/tasks/:id
// 轮询异步提取任务：done 返回与同步接口同构的 result；error 返回原因。
func (h *TranscriptHandler) HandleGetAsyncTask(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("提取服务未配置"))
		return
	}
	task, ok := h.uc.GetAsyncTask(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 40400, "msg": "任务不存在或已过期（超过 30 分钟）"})
		return
	}
	switch task.Status {
	case "done":
		h.respond(c, task.Result, nil)
	case "error":
		c.JSON(http.StatusBadRequest, gin.H{"code": 40000, "msg": task.Err, "status": "error"})
	default:
		success(c, gin.H{"status": "pending", "task_id": task.ID})
	}
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
// 请求体：{raw_text, topic, requirement}；topic 为用户一句话意图（向导侧拼品牌上下文）。
func (h *TranscriptHandler) HandleRewrite(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("提取服务未配置"))
		return
	}
	var req struct {
		RawText     string `json:"raw_text" binding:"required"`
		Topic       string `json:"topic"`
		Requirement string `json:"requirement"` // 可选润色需求（23 号 §3.1：如"更口语化"——只影响 rewrite 版方向）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	res, err := h.uc.RewriteScript(c.Request.Context(), req.RawText, req.Topic, req.Requirement)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40000, "msg": err.Error()})
		return
	}
	success(c, gin.H{"clean": res.Clean, "rewrite": res.Rewrite})
}

var _ = context.Background
