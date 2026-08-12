package handler

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// MediaHandler 素材上传托管（用户上传图片/音频 → 本地存储 → 可访问 URL 供 Vidu 引用）。
type MediaHandler struct {
	store port.MediaAssetStore
}

// NewMediaHandler 创建素材 handler。
func NewMediaHandler(store port.MediaAssetStore) *MediaHandler {
	return &MediaHandler{store: store}
}

// HandleUpload POST /api/v1/media/assets —— multipart 上传素材（图片/音频）。
// 返回 {id, url, mime, size_bytes}——url 直接可用于生成任务的 images/audio_url 参数。
func (h *MediaHandler) HandleUpload(c *gin.Context) {
	if h.store == nil {
		fail(c, fmt.Errorf("素材存储未配置"))
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		fail(c, fmt.Errorf("缺少文件字段 file: %w", err))
		return
	}
	defer file.Close()
	// 大小限制 20MB（Vidu POST body 上限；素材单文件限制）
	data, err := io.ReadAll(io.LimitReader(file, 21<<20))
	if err != nil {
		fail(c, fmt.Errorf("读取文件失败: %w", err))
		return
	}
	if len(data) > 20<<20 {
		fail(c, fmt.Errorf("素材超过 20MB 上限"))
		return
	}
	// 类型白名单：图片（png/jpeg/jpg/webp）+ 音频（mp3/m4a/wav）
	mime := header.Header.Get("Content-Type")
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".mp3": true, ".m4a": true, ".wav": true}
	if !allowed[ext] {
		fail(c, fmt.Errorf("仅支持图片(png/jpg/webp)与音频(mp3/m4a/wav)素材，收到 %s", ext))
		return
	}
	asset, err := h.store.SaveFile(c.Request.Context(), middleware.CurrentTenantID(c), c.PostForm("brand_id"), "material", data, mime, ext)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{
		"id": asset.ID, "url": asset.SourceURL, "mime": mime,
		"size_bytes": len(data), "owner_type": asset.OwnerType,
	})
}

// HandleList GET /api/v1/media/assets —— 素材库列表（material 素材；创建时间倒序）。
func (h *MediaHandler) HandleList(c *gin.Context) {
	if h.store == nil {
		fail(c, fmt.Errorf("素材存储未配置"))
		return
	}
	assets, err := h.store.List(c.Request.Context(), middleware.CurrentTenantID(c), entity.AssetTypeMaterial)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(assets))
	for _, a := range assets {
		out = append(out, gin.H{
			"id": a.ID, "owner_type": a.OwnerType, "url": a.SourceURL,
			"mime": a.Mime, "size_bytes": a.SizeBytes, "created_at": a.CreatedAt,
		})
	}
	success(c, gin.H{"assets": out})
}

// HandleDelete DELETE /api/v1/media/assets/:id —— 删除素材（租户校验在 store 内）。
func (h *MediaHandler) HandleDelete(c *gin.Context) {
	if h.store == nil {
		fail(c, fmt.Errorf("素材存储未配置"))
		return
	}
	if err := h.store.Delete(c.Request.Context(), middleware.CurrentTenantID(c), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": c.Param("id")})
}
