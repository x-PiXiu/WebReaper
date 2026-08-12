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
	asset := entity.MediaAsset{
		ID:        id,
		TenantID:  tenantID,
		BrandID:   brandID,
		OwnerType: ownerType,
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
		out = append(out, entity.MediaAsset{
			ID: id, TenantID: tenantID, OwnerType: fileOwnerType(name),
			SourceURL: s.publicURL(id), StoredURL: s.publicURL(id),
			Mime: mimeFromExt(filepath.Ext(name)), SizeBytes: info.Size(),
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

// CleanupBefore 清理过期文件（定时任务；简化：删除 data 目录下早于 before 的文件）。
func (s *LocalMediaStore) CleanupBefore(ctx context.Context, before time.Time) (int, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(before) {
			if os.Remove(filepath.Join(s.dir, e.Name())) == nil {
				n++
			}
		}
	}
	return n, nil
}

var _ port.MediaAssetStore = (*LocalMediaStore)(nil)

// 保持 import 稳定（json 备用）。
