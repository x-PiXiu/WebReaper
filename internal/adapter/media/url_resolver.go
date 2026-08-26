package media

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"

	"webreaper/internal/usecase/port"
)

// DefaultMaterialURLResolver MaterialURLResolver 的默认实现。
//
// 策略：
//   - 公网 URL → 原样返回（Vidu 可直接访问）
//   - 私网 + /media/ 路径 → 读取本地文件 → base64 内联（≤MaxInlineBytes）
//   - 私网 + /media/ + 超大文件 → 返回错误（需配置 PUBLIC_BASE_URL 或 OSS）
//   - 私网 + 非 /media/ → 原样保留（外部 localhost 服务，交给厂商报错）
type DefaultMaterialURLResolver struct {
	AssetStore      port.MediaAssetStore // 读取本地文件
	MaxInlineBytes  int                  // base64 内联上限（默认 8MB）
}

func NewDefaultMaterialURLResolver(asset port.MediaAssetStore, maxInlineMB int) *DefaultMaterialURLResolver {
	if maxInlineMB <= 0 {
		maxInlineMB = 8
	}
	return &DefaultMaterialURLResolver{
		AssetStore:     asset,
		MaxInlineBytes: maxInlineMB << 20,
	}
}

func (r *DefaultMaterialURLResolver) Resolve(ctx context.Context, rawURL string) (string, bool, error) {
	if rawURL == "" || !isPrivateHost(rawURL) {
		return rawURL, false, nil
	}
	if !strings.Contains(rawURL, "/media/") {
		return rawURL, false, nil
	}
	if r.AssetStore == nil {
		return rawURL, false, nil
	}
	data, mime, ok := r.AssetStore.ReadLocal(ctx, rawURL)
	if !ok {
		return rawURL, false, nil
	}
	if len(data) > r.MaxInlineBytes {
		return "", false, fmt.Errorf("素材 %s 为 %dMB，超出本地内联上限 %dMB——请配置公网可达的 PUBLIC_BASE_URL",
			truncateURL(rawURL), len(data)>>20, r.MaxInlineBytes>>20)
	}
	log.Printf("[MaterialURLResolver] %s → base64 data URI（mime=%s %d字节）", truncateURL(rawURL), mime, len(data))
	return toDataURI(data, mime), true, nil
}

// isPrivateHost 判断 URL 是否指向私网/环回地址。
func isPrivateHost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "[::1]" {
		return true
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return true // 解析失败按私网处理
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

// toDataURI 文件字节 → data URI。
func toDataURI(data []byte, mime string) string {
	if mime == "" {
		mime = "application/octet-stream"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func truncateURL(u string) string {
	if len(u) > 80 {
		return u[:80] + "…"
	}
	return u
}

// 确保实现接口
var _ port.MaterialURLResolver = (*DefaultMaterialURLResolver)(nil)
