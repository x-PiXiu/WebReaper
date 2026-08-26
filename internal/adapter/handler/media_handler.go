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

// HandleUpload POST /api/v1/media/assets —— multipart 上传素材（图片/音频/视频）。
// 返回 {id, url, mime, size_bytes, type, name}——url 直接可用于生成任务的 images/audio_url
// 参数及发布向导的 media_urls（视频发布素材）。
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
	mime := header.Header.Get("Content-Type")
	ext := strings.ToLower(filepath.Ext(header.Filename))
	// 通用 MIME 类型降级：curl/form 上传时常发 application/octet-stream，
	// 按扩展名修正为实际类型（否则 InferTypeFromMime 返回空，素材类型丢失）
	if mime == "application/octet-stream" || mime == "application/json" {
		mime = extToMime(ext)
	}
	// 类型白名单与分级大小限制：图片/音频 20MB（Vidu POST body 上限）；视频 200MB
	//（发布素材——口播成片几十~几百 MB，20MB 必然拒之门外；不进 Vidu 不受其限制）。
	isVideo := ext == ".mp4" || ext == ".webm" || ext == ".mov"
	limit := 21 << 20
	if isVideo {
		limit = 201 << 20
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)))
	if err != nil {
		fail(c, fmt.Errorf("读取文件失败: %w", err))
		return
	}
	if len(data) > limit-1 {
		fail(c, fmt.Errorf("素材超过 %dMB 上限", (limit-1)>>20))
		return
	}
	allowed := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".webp": true, // 图片
		".mp3": true, ".m4a": true, ".wav": true, // 音频
		".mp4": true, ".webm": true, ".mov": true, // 视频（发布素材）
	}
	if !allowed[ext] {
		fail(c, fmt.Errorf("仅支持图片(png/jpg/webp)、音频(mp3/m4a/wav)与视频(mp4/webm/mov)素材，收到 %s", ext))
		return
	}
	asset, err := h.store.SaveFile(c.Request.Context(), middleware.CurrentTenantID(c), c.PostForm("brand_id"), "material", data, mime, ext)
	if err != nil {
		fail(c, err)
		return
	}

	// 从文件名提取素材名称（去掉扩展名）
	assetName := strings.TrimSuffix(header.Filename, ext)

	success(c, gin.H{
		"id": asset.ID, "url": asset.SourceURL, "mime": mime,
		"size_bytes": len(data), "owner_type": asset.OwnerType,
		"type": asset.Type, "name": assetName, // 新增：素材类型和名称
	})
}

// HandleList GET /api/v1/media/assets —— 素材库列表（material 素材；创建时间倒序）。
func (h *MediaHandler) HandleList(c *gin.Context) {
	if h.store == nil {
		fail(c, fmt.Errorf("素材存储未配置"))
		return
	}
	// owner 过滤：material（默认，向后兼容——配图/配音场景）/ creation（AI 产物，含成片视频）/
	// all（素材+产物——发布向导选发视频用：成片主要落在 creation）。
	owner := c.DefaultQuery("owner", entity.AssetTypeMaterial)
	if owner == "all" {
		owner = "" // store.List 语义：空 = 全部
	}
	assets, err := h.store.List(c.Request.Context(), middleware.CurrentTenantID(c), owner)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(assets))
	for _, a := range assets {
		out = append(out, gin.H{
			"id": a.ID, "owner_type": a.OwnerType, "url": a.SourceURL,
			"mime": a.Mime, "size_bytes": a.SizeBytes, "created_at": a.CreatedAt,
			"type": a.Type, "name": a.Name, // 新增：素材类型和名称
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

// extToMime 文件扩展名 → MIME 类型映射（上传时 Content-Type 为
// application/octet-stream 的降级兜底）。
func extToMime(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	default:
		return "application/octet-stream"
	}
}
