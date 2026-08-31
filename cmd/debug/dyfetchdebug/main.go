// dyfetchdebug：抖音分享链全链路拉取诊断工具（复现 LinkResolver 主链路，游客模式）。
//
// 逐步复现项目主链路的每一步并打印结果，用于定位 403 出处：
//  ① 短链 302 跟随 → 提取 aweme_id（纯 HTTP）
//  ② chromedp 打开视频页（游客，无 Cookie）→ 同步 XHR 调详情接口
//  ③ play_addr 域名替换（aweme.snssdk.com → www.douyin.com）→ 纯 HTTP GET 下载
//  ④ 对照组：原始 play_addr 不替换直接下载
//
// 用法：go run ./cmd/dyfetchdebug -url https://v.douyin.com/xxxx/
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"webreaper/internal/adapter/chromedputil"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

var awemeIDRe = regexp.MustCompile(`/video/(\d+)`)

func main() {
	rawURL := flag.String("url", "", "抖音分享短链或视频页链接")
	headed := flag.Bool("headed", false, "显示浏览器窗口")
	flag.Parse()
	if *rawURL == "" {
		fmt.Println("用法: go run ./cmd/dyfetchdebug -url <分享链>")
		os.Exit(1)
	}

	// ① 短链 → aweme_id
	videoID := ""
	if m := awemeIDRe.FindStringSubmatch(*rawURL); m != nil {
		videoID = m[1]
		fmt.Printf("① 链接已含视频 ID: %s\n", videoID)
	} else {
		final, err := followRedirect(*rawURL)
		if err != nil {
			fmt.Printf("① 短链解析失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("① 短链 302 → %s\n", trunc(final, 120))
		m := awemeIDRe.FindStringSubmatch(final)
		if m == nil {
			fmt.Println("① ✗ 最终链接里没有 /video/{id}")
			os.Exit(1)
		}
		videoID = m[1]
		fmt.Printf("① ✓ 提取 aweme_id = %s\n", videoID)
	}

	// ② 视频页 XHR 详情（游客模式，空响应风控自动重试 3 次）
	var info *detailInfo
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		info, err = guestDetail(*headed, videoID)
		if err == nil {
			break
		}
		fmt.Printf("② 第 %d 次失败: %v\n", attempt, err)
		if attempt < 3 {
			fmt.Println("② 等 25 秒避风控后重试...")
			time.Sleep(25 * time.Second)
		}
	}
	if err != nil {
		fmt.Printf("② ✗ 视频页详情失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("② ✓ 详情 OK：desc=%q play_addr=%s\n", trunc(info.Desc, 40), trunc(info.PlayURL, 140))
	_ = os.WriteFile("dyfetchdebug_url.txt", []byte(info.PlayURL), 0644)

	// ③ 域名替换后下载（项目 safeDownload 的行为）
	replaced := strings.Replace(info.PlayURL, "aweme.snssdk.com", "www.douyin.com", 1)
	fmt.Printf("③ 替换域名后下载（safeDownload 行为）...\n   URL: %s\n", trunc(replaced, 140))
	st3 := probeDownload(replaced)
	fmt.Printf("③ HTTP %s（Content-Type=%s 长度=%s）\n", st3.status, st3.ctype, st3.length)
	printVerdict("③", st3)

	// ④ 对照组：原始 URL 直接下载
	if replaced != info.PlayURL {
		fmt.Printf("④ 对照组：原始域名直接下载...\n   URL: %s\n", trunc(info.PlayURL, 140))
		st4 := probeDownload(info.PlayURL)
		fmt.Printf("④ HTTP %s（Content-Type=%s 长度=%s）\n", st4.status, st4.ctype, st4.length)
		printVerdict("④", st4)
	}

	// ⑤ url_list 全量探测（每个地址 2 轮，验证 CDN 节点差异）
	fmt.Printf("⑤ url_list 共 %d 个地址，逐个探测（×2 轮）...\n", len(info.AllURLs))
	for i, u := range info.AllURLs {
		host := ""
		if p := strings.SplitN(strings.TrimPrefix(u, "https://"), "/", 2); len(p) > 0 {
			host = p[0]
		}
		r1 := probeDownloadReferer(u, refererFor(u))
		time.Sleep(500 * time.Millisecond)
		r2 := probeDownloadReferer(u, refererFor(u))
		fmt.Printf("   [%d] %s → 第1轮 %s / 第2轮 %s\n", i, host, r1.status, r2.status)
		// 403 的节点试无 Referer 对照（定位防盗链差异）
		if strings.HasPrefix(r1.status, "403") || strings.HasPrefix(r2.status, "403") {
			stNo := probeDownload(u)
			fmt.Printf("       └ 无 Referer 对照 → %s\n", stNo.status)
		}
	}

	// ⑥ 加 Referer 对照
	if len(info.AllURLs) > 0 {
		fmt.Println("⑥ 加 Referer: https://www.douyin.com/ 对照...")
		st6 := probeDownloadReferer(info.AllURLs[0], "https://www.douyin.com/")
		fmt.Printf("⑥ HTTP %s（Content-Type=%s）\n", st6.status, st6.ctype)
	}
}

// probeDownloadReferer 带 Referer 的下载探测。
func probeDownloadReferer(u, referer string) dlResult {
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("User-Agent", userAgent)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return dlResult{status: "ERR " + err.Error()}
	}
	defer resp.Body.Close()
	return dlResult{status: resp.Status, ctype: resp.Header.Get("Content-Type"), length: resp.Header.Get("Content-Length")}
}

// refererFor 复刻 videotranscript.downloadOne 的按域补 Referer 逻辑。
func refererFor(u string) string {
	if strings.Contains(u, "douyinvod.com") || strings.Contains(u, "douyin.com") ||
		strings.Contains(u, "snssdk.com") || strings.Contains(u, "iesdouyin.com") {
		return "https://www.douyin.com/"
	}
	if strings.Contains(u, "bilivideo.com") || strings.Contains(u, "hdslb.com") {
		return "https://www.bilibili.com"
	}
	return ""
}

type dlResult struct {
	status string
	ctype  string
	length string
}

// probeDownload 带项目同款 UA 发 GET，只读响应头与前 1KB（不落盘）。
func probeDownload(u string) dlResult {
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("User-Agent", userAgent)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return dlResult{status: "ERR " + err.Error()}
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	sniff := ""
	if resp.StatusCode == http.StatusOK && len(buf) > 0 {
		if strings.HasPrefix(string(buf[:min(12, len(buf))]), "\x00\x00\x00") {
			sniff = " [mp4 魔数 ✓]"
		}
	}
	_ = sniff
	return dlResult{
		status: resp.Status,
		ctype:  resp.Header.Get("Content-Type"),
		length: resp.Header.Get("Content-Length"),
	}
}

func printVerdict(step string, r dlResult) {
	if strings.HasPrefix(r.status, "200") {
		fmt.Printf("%s ✓ 可下载\n", step)
	} else if strings.HasPrefix(r.status, "403") {
		fmt.Printf("%s ✗ 403——这就是你本地报的错\n", step)
	} else {
		fmt.Printf("%s ? 非 200 状态\n", step)
	}
}

// followRedirect 复刻 resolver.followRedirect。
func followRedirect(rawURL string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest(http.MethodHead, rawURL, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		req.Method = http.MethodGet
		resp, err = client.Do(req)
		if err != nil {
			return "", err
		}
	}
	defer resp.Body.Close()
	return resp.Request.URL.String(), nil
}

type detailInfo struct {
	Desc    string
	PlayURL string
	AllURLs []string
}

// guestDetail 复刻 searcher.getAwemeDetail（游客无 Cookie 版）。
func guestDetail(headed bool, videoID string) (*detailInfo, error) {
	opts := chromedputil.HeadlessOptions(headed)
	opts = append(opts,
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent(userAgent),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	browserCtx, cancel2 := chromedp.NewContext(allocCtx)
	defer cancel2()
	ctx, cancel3 := context.WithTimeout(browserCtx, 60*time.Second)
	defer cancel3()

	videoURL := "https://www.douyin.com/video/" + videoID
	var currentURL string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(videoURL),
		chromedp.Sleep(4*time.Second),
		chromedp.Location(&currentURL),
	); err != nil {
		return nil, fmt.Errorf("打开视频页失败: %w", err)
	}
	fmt.Printf("② 视频页已打开: %s\n", trunc(currentURL, 100))

	paramsJS := fmt.Sprintf(`new URLSearchParams({
  device_platform: 'webapp', aid: '6383', channel: 'channel_pc_web',
  cookie_enabled: 'true', browser_language: navigator.language,
  browser_platform: navigator.platform, browser_name: 'Chrome',
  browser_online: 'true', platform: 'PC',
  screen_width: String(screen.width), screen_height: String(screen.height),
  aweme_id: %q,
}).toString()`, videoID)
	js := fmt.Sprintf(`(() => {
  try {
    const xhr = new XMLHttpRequest();
    xhr.open('GET', '/aweme/v1/web/aweme/detail/?' + %s, false);
    xhr.send();
    return JSON.stringify({http: xhr.status, body: xhr.responseText});
  } catch (e) { return JSON.stringify({error: String(e)}); }
})()`, paramsJS)

	var raw string
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error {
		res, _, e := runtime.Evaluate(js).WithReturnByValue(true).Do(c)
		if e != nil {
			return e
		}
		if res == nil || res.Value == nil {
			return fmt.Errorf("空结果")
		}
		return json.Unmarshal([]byte(string(res.Value)), &raw)
	}))
	if err != nil {
		return nil, fmt.Errorf("XHR 执行失败: %w", err)
	}

	var wrapper struct {
		HTTP int    `json:"http"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		return nil, fmt.Errorf("包装解析失败: %w raw=%.200s", err, raw)
	}
	fmt.Printf("② 详情 XHR HTTP %d，body %d 字节\n", wrapper.HTTP, len(wrapper.Body))
	if strings.TrimSpace(wrapper.Body) == "" {
		return nil, fmt.Errorf("详情接口空响应（风控/Cookie 需求）")
	}

	var dr struct {
		StatusCode int `json:"status_code"`
		AwemeDetail struct {
			Desc  string `json:"desc"`
			Video struct {
				PlayAddr struct {
					URLList []string `json:"url_list"`
				} `json:"play_addr"`
			} `json:"video"`
		} `json:"aweme_detail"`
	}
	if err := json.Unmarshal([]byte(wrapper.Body), &dr); err != nil {
		return nil, fmt.Errorf("详情 JSON 解析失败: %v | 首部: %.200s", err, wrapper.Body)
	}
	if dr.StatusCode != 0 {
		return nil, fmt.Errorf("接口 status_code=%d（如 2483=Cookie 失效）", dr.StatusCode)
	}
	if len(dr.AwemeDetail.Video.PlayAddr.URLList) == 0 {
		return nil, fmt.Errorf("详情无播放地址，首部: %.300s", wrapper.Body)
	}
	return &detailInfo{Desc: dr.AwemeDetail.Desc, PlayURL: dr.AwemeDetail.Video.PlayAddr.URLList[0], AllURLs: dr.AwemeDetail.Video.PlayAddr.URLList}, nil
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
