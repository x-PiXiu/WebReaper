package geo

import (
	"fmt"
	"net/url"
	"strconv"
)

// ---- 高德静态地图 URL 拼装（P2：文章页门店位置图）----
//
// 说明：静态地图本质是"拼 URL 拿图片"（v3/staticmap，GET）。
// Key 不能暴露给浏览器——由服务端拼 URL 后 302 重定向（见 public_handler.go），
// 或服务端代理下载。本文件只提供纯函数拼装（零 HTTP 依赖，可单测）。

// StaticMapURL 拼装静态地图 URL。
//
//   - center：地图中心（门店经纬度）
//   - label：可选标注文字（如门店名，≤15 字符；空=仅图钉）
//   - size：图片尺寸（如 "400x300"，高德格式为 宽*高，函数内部转换）
//   - zoom：缩放级别 [1,17]（0 表示自动）
//
// 返回形如：
//
//	https://restapi.amap.com/v3/staticmap?key=K&location=116.47,39.99&zoom=15&size=400*300&markers=mid,,:116.47,39.99&labels=店名,2,0,14,0xFFFFFF,0x008000:116.47,39.99
func StaticMapURL(apiKey string, centerLat, centerLng float64, label, size string, zoom int) string {
	u := fmt.Sprintf("https://restapi.amap.com/v3/staticmap?key=%s&location=%f,%f",
		url.QueryEscape(apiKey), centerLng, centerLat)
	if zoom > 0 {
		if zoom > 17 {
			zoom = 17
		}
		u += fmt.Sprintf("&zoom=%d", zoom)
	}
	if size == "" {
		size = "400x300"
	}
	// 高德尺寸格式为 宽*高（星号），前端习惯 x——统一转换
	u += "&size=" + normalizeSize(size)
	// 图钉标注（mid 大小、默认色）
	u += fmt.Sprintf("&markers=mid,,:%f,%f", centerLng, centerLat)
	// 文字标签（微软雅黑、粗体、14 号、白字绿底）
	if label != "" {
		runes := []rune(label)
		if len(runes) > 15 {
			label = string(runes[:15])
		}
		u += fmt.Sprintf("&labels=%s,2,1,14,0xFFFFFF,0x008000:%f,%f",
			url.QueryEscape(label), centerLng, centerLat)
	}
	return u
}

// normalizeSize "400x300" → "400*300"（高德用星号；x/× 兼容）。
func normalizeSize(size string) string {
	out := make([]rune, 0, len(size))
	for _, r := range size {
		if r == 'x' || r == 'X' || r == '×' {
			out = append(out, '*')
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

// StaticMapSize 便捷函数：返回"宽*高"格式（供测试/调试）。
func StaticMapSize(w, h int) string {
	return strconv.Itoa(w) + "*" + strconv.Itoa(h)
}
