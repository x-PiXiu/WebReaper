package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"webreaper/internal/domain/entity"
)

// OSSMediaStore 是 port.MediaAssetStore 的阿里云 OSS 实现。
//
// 设计（与 LocalMediaStore 可互换——策略模式）：
//   - 素材上传/产物转存都存 OSS（云端持久化，不依赖服务器磁盘）
//   - 公开 URL 直接指向 OSS（或绑定的 CDN 域名）
//   - 不需要 nginx /media 静态托管（OSS 自带 HTTP 访问）
//   - 切换：STORAGE_TYPE=oss 时 main 装配此实现，用例层零改动
type OSSMediaStore struct {
	bucket    *oss.Bucket
	publicBase string // URL 前缀（https://bucket.endpoint 或自定义域名）
	client    *http.Client
}

// NewOSSMediaStore 创建 OSS 媒体存储。
// endpoint: 公网或内网（云服务器用 internal endpoint 免流量费）
// publicDomain: 可选自定义域名（CDN/绑域名）；空=用 https://{bucket}.{endpoint}
func NewOSSMediaStore(endpoint, bucketName, accessKey, secretKey, publicDomain string) (*OSSMediaStore, error) {
	oClient, err := oss.New(endpoint, accessKey, secretKey)
	if err != nil {
		return nil, fmt.Errorf("OSS 连接失败: %w", err)
	}
	bucket, err := oClient.Bucket(bucketName)
	if err != nil {
		return nil, fmt.Errorf("OSS bucket 获取失败: %w", err)
	}
	pubBase := publicDomain
	if pubBase == "" {
		pubBase = fmt.Sprintf("https://%s.%s", bucketName, endpoint)
	}
	return &OSSMediaStore{
		bucket:     bucket,
		publicBase: strings.TrimRight(pubBase, "/"),
		client:     &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (s *OSSMediaStore) publicURL(key string) string {
	return s.publicBase + "/" + key
}

// ossProjectPrefix OSS key 统一项目前缀（和另一个项目共 bucket 时按前缀隔离）。
// 如 zhichen 项目用 "zhichen/"，WebReaper 用 "webreaper/"。
const ossProjectPrefix = "webreaper"

// SaveFile 上传素材文件到 OSS。
func (s *OSSMediaStore) SaveFile(ctx context.Context, tenantID, brandID, ownerType string, data []byte, mime, ext string) (entity.MediaAsset, error) {
	prefix := "media"
	if ownerType == entity.AssetTypeCreation {
		prefix = "creation"
	}
	// key 格式：webreaper/{media|creation}/{tenantID}/{date}/{shortID}.ext
	key := fmt.Sprintf("%s/%s/%s/%s/%s%s", ossProjectPrefix, prefix, tenantID, datePath(), shortID(), ext)
	if err := s.bucket.PutObject(key, bytes.NewReader(data)); err != nil {
		return entity.MediaAsset{}, fmt.Errorf("OSS 上传失败: %w", err)
	}
	url := s.publicURL(key)
	return entity.MediaAsset{
		ID: key, TenantID: tenantID, BrandID: brandID, OwnerType: ownerType,
		SourceURL: url, StoredURL: url, Mime: mime, SizeBytes: int64(len(data)),
		CreatedAt: time.Now(),
	}, nil
}

// List 列出某租户的资产（按 OSS 前缀 + ownerType 过滤）。
func (s *OSSMediaStore) List(ctx context.Context, tenantID, ownerType string) ([]entity.MediaAsset, error) {
	// 前缀按租户目录隔离：webreaper/media/{tenantID}/ 或 webreaper/creation/{tenantID}/
	p := ossProjectPrefix + "/"
	if ownerType == entity.AssetTypeMaterial {
		p += "media/" + tenantID + "/"
	} else if ownerType == entity.AssetTypeCreation {
		p += "creation/" + tenantID + "/"
	} else {
		p += tenantID
	}
	prefix := p
	result, err := s.bucket.ListObjects(oss.Prefix(prefix), oss.MaxKeys(200))
	if err != nil {
		return nil, fmt.Errorf("OSS 列出失败: %w", err)
	}
	out := make([]entity.MediaAsset, 0, len(result.Objects))
	// OSS ListObjects 默认按 key 字母序（升序）；倒序排列（新→旧）
	for i := len(result.Objects) - 1; i >= 0; i-- {
		obj := result.Objects[i]
		owner := entity.AssetTypeCreation
		if strings.Contains(obj.Key, "/media/") {
			owner = entity.AssetTypeMaterial
		}
		out = append(out, entity.MediaAsset{
			ID: obj.Key, TenantID: tenantID, OwnerType: owner,
			SourceURL: s.publicURL(obj.Key), StoredURL: s.publicURL(obj.Key),
			SizeBytes: obj.Size, CreatedAt: obj.LastModified,
		})
	}
	return out, nil
}

// Delete 删除 OSS 对象（校验 key 含 tenantID 防越权）。
func (s *OSSMediaStore) Delete(ctx context.Context, tenantID, assetID string) error {
	// Delete 校验 key 含 tenantID（目录路径隔离防越权）
	if assetID == "" || !strings.Contains(assetID, tenantID+"/") {
		return fmt.Errorf("无权删除该资产")
	}
	if err := s.bucket.DeleteObject(assetID); err != nil {
		return fmt.Errorf("OSS 删除失败: %w", err)
	}
	return nil
}

// DownloadAndStore 下载外部 URL → 上传 OSS（Vidu 产物 24h URL 永久化）。
func (s *OSSMediaStore) DownloadAndStore(ctx context.Context, tenantID, sourceURL string, meta map[string]string) (string, error) {
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
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	if err != nil {
		return "", err
	}
	// 扩展名推断（与 LocalMediaStore 一致）
	ext := ".bin"
	if meta != nil && meta["ext"] != "" {
		ext = meta["ext"]
	} else if ct := resp.Header.Get("Content-Type"); ct != "" {
		ext = extFromContentType(ct)
	}
	key := fmt.Sprintf("%s/creation/%s/%s/%s%s", ossProjectPrefix, tenantID, datePath(), shortID(), ext)
	if err := s.bucket.PutObject(key, bytes.NewReader(data)); err != nil {
		return "", fmt.Errorf("OSS 上传失败: %w", err)
	}
	return s.publicURL(key), nil
}

// CleanupBefore 清理早于 before 的资产（按 LastModified 过滤 + 批量删除）。
func (s *OSSMediaStore) CleanupBefore(ctx context.Context, before time.Time) (int, error) {
	// 列出 creation 目录所有对象
	result, err := s.bucket.ListObjects(oss.Prefix(ossProjectPrefix+"/creation/"), oss.MaxKeys(1000))
	if err != nil {
		return 0, err
	}
	var toDelete []string
	for _, obj := range result.Objects {
		if obj.LastModified.Before(before) {
			toDelete = append(toDelete, obj.Key)
		}
	}
	if len(toDelete) == 0 {
		return 0, nil
	}
	_, err = s.bucket.DeleteObjects(toDelete)
	if err != nil {
		return 0, fmt.Errorf("OSS 批量删除失败: %w", err)
	}
	return len(toDelete), nil
}

// extFromContentType Content-Type → 扩展名（与 LocalMediaStore 同逻辑）。
func extFromContentType(ct string) string {
	switch {
	case strings.Contains(ct, "mp4"):
		return ".mp4"
	case strings.Contains(ct, "webm"):
		return ".webm"
	case strings.Contains(ct, "mp3"):
		return ".mp3"
	case strings.Contains(ct, "m4a"):
		return ".m4a"
	case strings.Contains(ct, "wav"):
		return ".wav"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg") || strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "webp"):
		return ".webp"
	default:
		return ".bin"
	}
}
