// datauri.go data: URI 解析（缺口B修复——MiMo 同步产物内联 base64 的转存通道）。
//
// 背景：小米 MiMo TTS/克隆是同步接口，产物以 data:audio/mp3;base64,... 内联返回。
// 此前 DownloadAndStore 走 http.Get → "unsupported protocol scheme" 必然失败，
// 导致 MiMo 产物永远无 stored_url，且物化回退把几十万字符的 base64 写进
// generation_voices.sample_url（VARCHAR(512)）→ Data too long 静默失败。
package storage

import (
	"encoding/base64"
	"strings"
)

// ParseDataURI 解析 data:[<mime>][;base64],<payload> 形态。
// 非 data: URI 返回 ok=false（调用方走原 HTTP 下载路径）。
// 返回：解码后的字节、按 mime 推断的扩展名（含点，兜底 .bin）。
func ParseDataURI(uri string) (payload []byte, ext string, ok bool) {
	if !strings.HasPrefix(uri, "data:") {
		return nil, "", false
	}
	rest := uri[len("data:"):]
	comma := strings.Index(rest, ",")
	if comma < 0 {
		return nil, "", false
	}
	header, encoded := rest[:comma], rest[comma+1:]

	mime := "application/octet-stream"
	isBase64 := false
	for _, part := range strings.Split(header, ";") {
		if part == "base64" {
			isBase64 = true
		} else if part != "" && strings.Contains(part, "/") {
			mime = part
		}
	}

	var decoded []byte
	if isBase64 {
		d, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, "", false
		}
		decoded = d
	} else {
		decoded = []byte(encoded) // 罕见：非 base64 明文（媒体产物不会用）
	}
	return decoded, extFromContentType(mime), true
}

// dataURIExtOr meta 显式扩展名优先（与 HTTP 路径的 meta["ext"] 口径一致）。
func dataURIExtOr(meta map[string]string, ext string) string {
	if meta != nil && meta["ext"] != "" {
		return meta["ext"]
	}
	return ext
}
