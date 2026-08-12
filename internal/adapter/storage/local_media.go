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
func (s *LocalMediaStore) SaveFile(ctx context.Context, tenantID, brandID, ownerType string, data []byte, mime, ext string) (entity.MediaAsset, error) {
	name := fmt.Sprintf("%s-%d%s", tenantID, time.Now().UnixNano(), ext)
	path := filepath.Join(s.dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return entity.MediaAsset{}, fmt.Errorf("素材保存失败: %w", err)
	}
	asset := entity.MediaAsset{
		ID:        fmt.Sprintf("ma-%d", time.Now().UnixNano()),
		TenantID:  tenantID,
		BrandID:   brandID,
		OwnerType: ownerType,
		SourceURL: s.publicURL(name),
		StoredURL: s.publicURL(name),
		Mime:      mime,
		SizeBytes: int64(len(data)),
		CreatedAt: time.Now(),
	}
	return asset, nil
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
	name := fmt.Sprintf("c-%s-%d%s", tenantID, time.Now().UnixNano(), ext)
	if err := os.WriteFile(filepath.Join(s.dir, name), data, 0o644); err != nil {
		return "", fmt.Errorf("转存写入失败: %w", err)
	}
	return s.publicURL(name), nil
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
