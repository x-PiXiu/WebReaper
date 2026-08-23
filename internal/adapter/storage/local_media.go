// Package storage 提供媒体资产存储实现（素材托管 + 产物转存适配器）。
//
// 设计（Docs/Plans/03 §P1）：MediaAssetStore 是 port 接口——本实现为本地文件
// 存储（数据目录 + 静态托管 URL），P2 换 OSS = 新实现 + main 装配一行，用例零改动。
// 两个职责：
//   - 素材托管：用户上传图片/音频 → 本地文件 → 返回可访问 URL（供 Vidu 引用，
//     避开 20MB POST body 限制与外部 URL 失效问题）
//   - 产物转存：Vidu 生成物 URL 24h 过期 → 下载到本地 → stored_url 永久化
package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// LocalMediaStore 是 port.MediaAssetStore 的本地文件实现。
type LocalMediaStore struct {
	dir      string // 数据目录（如 ./data/media）
	baseURL  string // 公开站根地址（生成可访问 URL，如 http://localhost:8082）
	client   *http.Client
}

// NewLocalMediaStore 创建本地媒体存储。
func NewLocalMediaStore(dir, baseURL string) (*LocalMediaStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建媒体目录失败: %w", err)
	}
	return &LocalMediaStore{
		dir:     dir,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// publicURL 本地文件 → 公网可访问 URL（静态路由 /media 由框架层托管）。
func (s *LocalMediaStore) publicURL(rel string) string {
	return s.baseURL + "/media/" + rel
}

func (s *LocalMediaStore) relPath(filename string) string {
	return filename
}

// SaveFile 保存素材文件（handler 上传后调用；返回可访问 URL 与资产信息）。
// 资产 ID = {tenantID}/{shortID}.ext（租户子目录隔离 + 短 ID 文件名）
//
// 整洁架构设计：
//   - Type 字段根据 MIME 类型自动推断（image/video/audio）
//   - Name 字段从文件名提取（去掉扩展名）
//   - Width/Height/Duration 字段由调用方填充（图片/视频解析在 handler 层）
func (s *LocalMediaStore) SaveFile(ctx context.Context, tenantID, brandID, ownerType string, data []byte, mime, ext string) (entity.MediaAsset, error) {
	name := shortID() + ext
	relPath := filepath.Join(tenantID, datePath(), name)
	fullPath := filepath.Join(s.dir, relPath)
	// 确保租户子目录存在
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return entity.MediaAsset{}, fmt.Errorf("创建租户目录失败: %w", err)
	}
	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return entity.MediaAsset{}, fmt.Errorf("素材保存失败: %w", err)
	}
	id := filepath.ToSlash(relPath) // ID 用 / 分隔（跨平台 URL 友好）

	// 自动推断素材类型（image/video/audio）
	materialType := entity.InferTypeFromMime(mime)

	asset := entity.MediaAsset{
		ID:        id,
		TenantID:  tenantID,
		BrandID:   brandID,
		OwnerType: ownerType,
		Type:      materialType, // 新增：素材类型
		SourceURL: s.publicURL(id),
		StoredURL: s.publicURL(id),
		Mime:      mime,
		SizeBytes: int64(len(data)),
		CreatedAt: time.Now(),
	}
	return asset, nil
}

// fileOwnerType 文件名 → 资产类型（material=上传素材 / creation=转存产物）。
func fileOwnerType(name string) string {
	if strings.HasPrefix(name, "c-") {
		return entity.AssetTypeCreation
	}
	return entity.AssetTypeMaterial
}

// List 列出某租户资产（递归扫描日期子目录：{dir}/{tenantID}/{date}/{shortID}.ext）
func (s *LocalMediaStore) List(ctx context.Context, tenantID, ownerType string) ([]entity.MediaAsset, error) {
	tenantDir := filepath.Join(s.dir, tenantID)
	var out []entity.MediaAsset
	_ = filepath.Walk(tenantDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		isCreation := strings.HasPrefix(name, "c-")
		if ownerType == entity.AssetTypeMaterial && isCreation {
			return nil
		}
		if ownerType == entity.AssetTypeCreation && !isCreation {
			return nil
		}
		rel, _ := filepath.Rel(s.dir, path) // {tenantID}/{date}/{name}
		id := filepath.ToSlash(rel)
		mime := mimeFromExt(filepath.Ext(name))
		out = append(out, entity.MediaAsset{
			ID: id, TenantID: tenantID, OwnerType: fileOwnerType(name),
			Type: entity.InferTypeFromMime(mime), // 新增：自动推断素材类型
			SourceURL: s.publicURL(id), StoredURL: s.publicURL(id),
			Mime: mime, SizeBytes: info.Size(),
			CreatedAt: info.ModTime(),
		})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// Delete 删除资产（assetID={tenantID}/{shortID}.ext；校验目录归属防越权）
func (s *LocalMediaStore) Delete(ctx context.Context, tenantID, assetID string) error {
	if assetID == "" || strings.Contains(assetID, "..") {
		return fmt.Errorf("非法资产 ID")
	}
	relPath := filepath.FromSlash(assetID)
	if !strings.HasPrefix(relPath, tenantID+string(filepath.Separator)) {
		return fmt.Errorf("无权删除该资产")
	}
	if err := os.Remove(filepath.Join(s.dir, relPath)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("资产不存在")
		}
		return fmt.Errorf("删除失败: %w", err)
	}
	return nil
}

// mimeFromExt 扩展名 → MIME（列表展示用；上传时用真实 Content-Type）。
func mimeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}

// DownloadAndStore 下载外部 URL 到本地（转存：Vidu 产物 24h 过期 → 永久化）。
func (s *LocalMediaStore) DownloadAndStore(ctx context.Context, tenantID, sourceURL string, meta map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20)) // 256MB 上限
	if err != nil {
		return "", err
	}
	// 扩展名：优先 meta；否则按 Content-Type 推断
	ext := ".bin"
	if meta != nil && meta["ext"] != "" {
		ext = meta["ext"]
	} else if ct := resp.Header.Get("Content-Type"); ct != "" {
		switch {
		case strings.Contains(ct, "mp4"):
			ext = ".mp4"
		case strings.Contains(ct, "webm"):
			ext = ".webm"
		case strings.Contains(ct, "mp3"):
			ext = ".mp3"
		case strings.Contains(ct, "m4a"):
			ext = ".m4a"
		case strings.Contains(ct, "wav"):
			ext = ".wav"
		case strings.Contains(ct, "png"):
			ext = ".png"
		case strings.Contains(ct, "jpeg") || strings.Contains(ct, "jpg"):
			ext = ".jpg"
		case strings.Contains(ct, "webp"):
			ext = ".webp"
		}
	}
	name := "c-" + shortID() + ext
	relPath := filepath.Join(tenantID, datePath(), name)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(s.dir, relPath)), 0o755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(s.dir, relPath), data, 0o644); err != nil {
		return "", fmt.Errorf("转存写入失败: %w", err)
	}
	return s.publicURL(filepath.ToSlash(relPath)), nil
}

// CleanupBefore 清理过期文件（定时任务）。R1 修复两处：
//   - 引用排除：excludeURLs 命中的文件跳过（仍被任务引用的产物不删）
//   - 目录遍历：改为 Walk 递归（此前 ReadDir 只扫第一层，{tenant}/{date}/ 子目录
//     里的文件从未被清理——与 SaveFile 的存储结构不匹配，属静默失效）
func (s *LocalMediaStore) CleanupBefore(ctx context.Context, before time.Time, excludeURLs map[string]bool) (int, error) {
	n := 0
	_ = filepath.Walk(s.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !info.ModTime().Before(before) {
			return nil
		}
		rel, rErr := filepath.Rel(s.dir, path)
		if rErr != nil {
			return nil
		}
		id := filepath.ToSlash(rel)
		if excludeURLs != nil && excludeURLs[s.publicURL(id)] {
			return nil // 仍被引用——保留
		}
		if os.Remove(path) == nil {
			n++
		}
		return nil
	})
	return n, nil
}

// ReadLocal 读取本站托管 URL 对应的本地文件（URL 前缀 {baseURL}/media/ 判定归属；
// 兼容只有路径的旧格式 /media/...）。非本站托管返回 ok=false（外部图床/OSS）。
func (s *LocalMediaStore) ReadLocal(ctx context.Context, url string) ([]byte, string, bool) {
	if url == "" || strings.Contains(url, "..") {
		return nil, "", false
	}
	var rel string
	switch {
	case strings.HasPrefix(url, s.baseURL+"/media/"):
		rel = strings.TrimPrefix(url, s.baseURL+"/media/")
	case strings.HasPrefix(url, "/media/"):
		rel = strings.TrimPrefix(url, "/media/")
	default:
		return nil, "", false
	}
	data, err := os.ReadFile(filepath.Join(s.dir, filepath.FromSlash(rel)))
	if err != nil {
		return nil, "", false
	}
	return data, mimeFromExt(filepath.Ext(rel)), true
}

var _ port.MediaAssetStore = (*LocalMediaStore)(nil)

// 保持 import 稳定（json 备用）。
