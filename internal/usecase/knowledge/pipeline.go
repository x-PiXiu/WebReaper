package knowledge

import (
	"encoding/json"
	"net/url"
	"strings"
)

// searchCandidate 搜索候选（标题 + URL）。
type searchCandidate struct {
	Title string
	URL   string
}

// searchResultPayload SearchCrawler 返回的候选列表 JSON 结构
// （Content = {"query":...,"results":[{title,url,snippet}]}）。
type searchResultPayload struct {
	Query   string `json:"query"`
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Snippet string `json:"snippet"`
	} `json:"results"`
}

// parseSearchResults 解析 SearchCrawler 输出的候选列表（非法输入返回空列表）。
func parseSearchResults(content string) ([]searchCandidate, error) {
	var payload searchResultPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return nil, err
	}
	out := make([]searchCandidate, 0, len(payload.Results))
	for _, r := range payload.Results {
		if r.URL == "" {
			continue
		}
		out = append(out, searchCandidate{Title: r.Title, URL: r.URL})
	}
	return out, nil
}

// normalizeURL 统一 URL 去重粒度：去 fragment + 常见跟踪参数（utm_* / fbclid 等）。
// 不同跟踪参数指向同一页面，指纹必须一致才能正确去重。
func normalizeURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	u.Fragment = ""
	q := u.Query()
	for k := range q {
		if strings.HasPrefix(k, "utm_") || k == "fbclid" || k == "gclid" {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// runeLen 按字符数计算长度（中文等宽字符场景必须 rune 计数）。
func runeLen(s string) int {
	return len([]rune(s))
}

// truncateRunes 按字符数截断（超长截断；不破坏 UTF-8）。
func truncateRunes(s string, max int) string {
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max])
}
