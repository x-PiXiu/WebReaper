// dysharedebug：抖音 iesdouyin 分享页通道 Go 移植验证（free-video-downloader/douyin.py 同款路线）。
//
// 验证命题：Go net/http（TLS 指纹不同于 curl/requests）访问分享页能否拿到
// SSR 的 _ROUTER_DATA（含 videoInfoRes.item_list 播放地址）——
// 能 → 该通道可移植进 douyinweb resolver 做免浏览器免 cookie 降级链。
//
// 用法：go run ./cmd/dysharedebug -url https://v.douyin.com/xxxx/
package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const mobileUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1"

var shellHTML string

var (
	urlRe      = regexp.MustCompile(`https?://[^\s]+`)
	videoIDRe  = regexp.MustCompile(`/(?:video|note)/(\d{8,24})`)
	idFallback = regexp.MustCompile(`(\d{15,24})`)
	routerMark = "window._ROUTER_DATA = "
)

func main() {
	raw := flag.String("url", "", "抖音分享口令/短链/视频页链接")
	h1 := flag.Bool("h1", false, "强制 HTTP/1.1（禁 ALPN h2——对照 requests 行为）")
	flag.Parse()
	if *raw == "" {
		fmt.Println("用法: go run ./cmd/dysharedebug -url <分享链>")
		os.Exit(1)
	}
	transport := &http.Transport{}
	if *h1 {
		transport.ForceAttemptHTTP2 = false
		transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		fmt.Println("⇒ HTTP/1.1 模式（对照 requests）")
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: transport}

	// ① 口令抽 URL
	shareURL := *raw
	if m := urlRe.FindString(*raw); m != "" {
		shareURL = strings.TrimRight(strings.Trim(m, "\"'"), ").,;!?")
	}
	fmt.Printf("① 分享链接: %s\n", shareURL)

	// ② 跟随 302 拿 video_id
	req, _ := http.NewRequest(http.MethodGet, shareURL, nil)
	req.Header.Set("User-Agent", mobileUA)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("② ✗ 短链请求失败:", err)
		os.Exit(1)
	}
	finalURL := resp.Request.URL.String()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	fmt.Printf("② 302 → %s\n", trunc(finalURL, 100))

	m := videoIDRe.FindStringSubmatch(finalURL)
	if m == nil {
		m = idFallback.FindStringSubmatch(finalURL)
	}
	if m == nil {
		fmt.Println("② ✗ 提取不到 video_id")
		os.Exit(1)
	}
	videoID := m[1]
	fmt.Printf("② ✓ video_id = %s\n", videoID)

	// 对照实验：分享页连续抓 6 次，统计 SSR 数据页出现率
	okN := 0
	for i := 1; i <= 6; i++ {
		h, _, e := fetchSharePage(client, "https://www.iesdouyin.com/share/video/"+videoID+"/")
		if e != nil {
			fmt.Printf("  [%d] 请求失败 %v\n", i, e)
			continue
		}
		if d := extractRouterData(h); d != nil && findItem(d) != nil {
			okN++
		}
		time.Sleep(700 * time.Millisecond)
	}
	fmt.Printf("③b 对照：Go 连续 6 次分享页，SSR 数据页 %d/6\n", okN)

	// ③ 分享页 SSR
	_ = os.WriteFile("dysharedebug_shell.html", []byte(shellHTML), 0644)
	sharePage := "https://www.iesdouyin.com/share/video/" + videoID + "/"
	if strings.Contains(finalURL, "iesdouyin.com") {
		sharePage = finalURL
	}
	html, waf, err := fetchSharePage(client, sharePage)
	shellHTML = html
	if err != nil {
		fmt.Println("③ ✗ 分享页请求失败:", err)
		os.Exit(1)
	}
	fmt.Printf("③ 分享页 %d 字节（WAF 挑战=%v）\n", len(html), waf)
	if waf {
		fmt.Println("③ 检测到 WAF 挑战（Please wait）——Go 版暂未实现 PoW 破解，解析可能失败")
	}

	// ④ _ROUTER_DATA 提取
	data := extractRouterData(html)
	if data == nil {
		fmt.Println("④ ✗ 提取不到 _ROUTER_DATA（可能是 JS 渲染壳页——WAF 按 TLS 指纹分流）")
		os.Exit(1)
	}
	fmt.Println("④ ✓ _ROUTER_DATA 提取成功")
	_ = os.WriteFile("dysharedebug_router.json", []byte(mustJSON(data)), 0644)

	// ⑤ videoInfoRes.item_list
	item := findItem(data)
	if item == nil {
		fmt.Println("⑤ ✗ 无 videoInfoRes.item_list（壳页变体）")
		os.Exit(1)
	}
	desc, _ := item["desc"].(string)
	fmt.Printf("⑤ ✓ item_list 命中：desc=%q\n", trunc(desc, 50))

	video, _ := item["video"].(map[string]any)
	playAddr, _ := video["play_addr"].(map[string]any)
	urlList, _ := playAddr["url_list"].([]any)
	if len(urlList) == 0 {
		fmt.Println("⑥ ✗ play_addr.url_list 为空")
		os.Exit(1)
	}
	playURL, _ := urlList[0].(string)
	playURL = strings.Replace(playURL, "playwm", "play", 1)
	fmt.Printf("⑥ ✓ 播放直链: %s\n", trunc(playURL, 110))

	// ⑦ 下载验证（前 1MB）
	dl, _ := http.NewRequest(http.MethodGet, playURL, nil)
	dl.Header.Set("User-Agent", mobileUA)
	dl.Header.Set("Referer", "https://www.douyin.com/")
	dresp, err := client.Do(dl)
	if err != nil {
		fmt.Println("⑦ ✗ 下载失败:", err)
		os.Exit(1)
	}
	defer dresp.Body.Close()
	head := make([]byte, 16)
	n, _ := dresp.Body.Read(head)
	fmt.Printf("⑦ 下载探测: HTTP %s Content-Type=%s 长度=%s 文件头=%x\n",
		dresp.Status, dresp.Header.Get("Content-Type"), dresp.Header.Get("Content-Length"), head[:n])
	if dresp.StatusCode == 200 && (strings.HasPrefix(fmt.Sprintf("%x", head[:n]), "000000") || strings.Contains(dresp.Header.Get("Content-Type"), "video")) {
		fmt.Println("\n✅✅ Go 分享页通道全链路可用——可移植进 douyinweb resolver")
	} else {
		fmt.Println("\n❌ 下载探测异常")
	}
}

func fetchSharePage(client *http.Client, u string) (string, bool, error) {
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("User-Agent", mobileUA)
	req.Header.Set("Accept", "text/html,application/json,*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.douyin.com/")
	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", false, err
	}
	html := string(body)
	waf := strings.Contains(html, "Please wait") && strings.Contains(html, "wci=")
	return html, waf, nil
}

// extractRouterData 括号配对提取 window._ROUTER_DATA = {...}（douyin.py 同款算法）。
func extractRouterData(html string) map[string]any {
	start := strings.Index(html, routerMark)
	if start < 0 {
		return nil
	}
	idx := start + len(routerMark)
	for idx < len(html) && (html[idx] == ' ' || html[idx] == '\t' || html[idx] == '\n' || html[idx] == '\r') {
		idx++
	}
	if idx >= len(html) || html[idx] != '{' {
		return nil
	}
	depth, inStr, escaped := 0, false, false
	for cursor := idx; cursor < len(html); cursor++ {
		ch := html[cursor]
		if inStr {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return parseJSONFlexible(html[idx : cursor+1])
			}
		}
	}
	return nil
}

// parseJSONFlexible 兼容原文 JSON 与 URL 编码 JSON 两种变体。
func parseJSONFlexible(s string) map[string]any {
	var out map[string]any
	if json.Unmarshal([]byte(s), &out) == nil {
		return out
	}
	if dec, err := urlDecode(s); err == nil {
		json.Unmarshal([]byte(dec), &out)
		return out
	}
	return nil
}

func urlDecode(s string) (string, error) {
	// 仅处理 %XX；避免引入 net/url 的 + 号语义差异
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			var v byte
			ok1 := hexVal(s[i+1], &v)
			var v2 byte
			ok2 := hexVal(s[i+2], &v2)
			if ok1 && ok2 {
				b.WriteByte(v<<4 | v2)
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String(), nil
}

func hexVal(c byte, out *byte) bool {
	switch {
	case c >= '0' && c <= '9':
		*out = c - '0'
	case c >= 'a' && c <= 'f':
		*out = c - 'a' + 10
	case c >= 'A' && c <= 'F':
		*out = c - 'A' + 10
	default:
		return false
	}
	return true
}

// findItem 遍历 loaderData 找 videoInfoRes.item_list[0]。
func findItem(data map[string]any) map[string]any {
	loader, _ := data["loaderData"].(map[string]any)
	for _, node := range loader {
		nd, ok := node.(map[string]any)
		if !ok {
			continue
		}
		vir, ok := nd["videoInfoRes"].(map[string]any)
		if !ok {
			continue
		}
		items, ok := vir["item_list"].([]any)
		if ok && len(items) > 0 {
			if it, ok := items[0].(map[string]any); ok {
				return it
			}
		}
	}
	return nil
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
