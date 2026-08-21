// dywebdebug：抖音 web 接口免签路径验证工具。
//
// 验证命题（MediaCrawler 协议知识）：/aweme/v1/web/general/search/single/ 等
// 接口在浏览器页面内同源 fetch 可用（免 a_bogus 签名），游客模式即可拿到数据。
// 通过 → 热门同款数据源直接升级为抖音站内真实搜索，无需绑定账号。
//
// 用法：go run ./cmd/dywebdebug -keyword "川菜 探店"
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"webreaper/internal/adapter/chromedputil"
)

// searchFetchJS 在 douyin.com 页面内同源 fetch 搜索接口（一周内+最多点赞=最近很火）。
// 参数含 MediaCrawler 协议知识的全套通用参数（aid/device_platform/webid/msToken…），
// 从页面环境现算（navigator/screen/localStorage），比硬编码伪装更真实。
const searchFetchJS = `(async (kw) => {
  const uuid = () => 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = Math.random() * 16 | 0;
    return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
  });
  let msToken = '';
  try { msToken = localStorage.getItem('xmst') || ''; } catch (e) {}
  const params = new URLSearchParams({
    device_platform: 'webapp', aid: '6383', channel: 'channel_pc_web',
    aweme_type: '0', keyword: kw, search_channel: 'aweme_video_web',
    enable_history: '1', search_source: 'tab_search', query_correct_type: '1',
    is_filter_search: '1',
    filter_selected: JSON.stringify({ sort_type: '1', publish_time: '7' }),
    from_group_id: '', offset: '0', count: '15', need_filter_settings: '1',
    list_type: 'multi', version_code: '170400', pc_client_type: '1',
    cookie_enabled: navigator.cookieEnabled ? 'true' : 'false',
    browser_language: navigator.language, browser_platform: navigator.platform,
    browser_name: 'Chrome', browser_online: navigator.onLine ? 'true' : 'false',
    engine_name: 'Blink', engine_version: '', os_name: '', os_version: '',
    cpu_core_num: String(navigator.hardwareConcurrency || 8),
    device_memory: String(navigator.deviceMemory || 8),
    platform: 'PC', screen_width: String(screen.width), screen_height: String(screen.height),
    effective_type: '4g', round_trip_time: '50',
    webid: uuid(), msToken,
  });
  try {
    const r = await fetch('/aweme/v1/web/general/search/single/?' + params.toString(), {
      credentials: 'include',
      headers: { 'Referer': location.origin + '/search/' + encodeURIComponent(kw) + '?type=general' },
    });
    const text = await r.text();
    return JSON.stringify({ status: r.status, len: text.length, body: text });
  } catch (e) {
    return JSON.stringify({ error: String(e) });
  }
})`

// 搜索响应结构（只取关心的字段）。
type searchResp struct {
	StatusCode int `json:"status_code"`
	Data []struct {
		Type      int `json:"type"`
		AwemeInfo struct {
			AwemeID   string `json:"aweme_id"`
			Desc      string `json:"desc"`
			Author    struct {
				Nickname string `json:"nickname"`
			} `json:"author"`
			Statistics struct {
				PlayCount    int `json:"play_count"`
				DiggCount    int `json:"digg_count"`
				CommentCount int `json:"comment_count"`
				ShareCount   int `json:"share_count"`
			} `json:"statistics"`
			CreateTime int64 `json:"create_time"`
		} `json:"aweme_info"`
	} `json:"data"`
}

func main() {
	keyword := flag.String("keyword", "川菜 探店", "搜索关键词")
	headed := flag.Bool("headed", false, "显示浏览器窗口")
	flag.Parse()

	opts := chromedputil.HeadlessOptions(*headed)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	browserCtx, cancel2 := chromedp.NewContext(allocCtx)
	defer cancel2()

	ctx, cancel3 := context.WithTimeout(browserCtx, 90*time.Second)
	defer cancel3()

	fmt.Printf("打开 douyin.com（游客模式）搜索 %q ...\n", *keyword)
	var respJSON string
	err := chromedp.Run(ctx,
		chromedp.Navigate("https://www.douyin.com"),
		chromedp.Sleep(4*time.Second), // 等 ttwid 等游客 cookie 自动签发
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 显式 AwaitPromise+ReturnByValue：页面内 async fetch 的结果直接回传
			result, _, err := runtime.Evaluate(fmt.Sprintf(`(%s)(%q)`, searchFetchJS, *keyword)).
				WithAwaitPromise(true).
				WithReturnByValue(true).
				Do(ctx)
			if err != nil {
				return err
			}
			if result == nil || result.Value == nil {
				return fmt.Errorf("空结果: %v", result)
			}
			return json.Unmarshal(result.Value, &respJSON)
		}),
	)
	if err != nil {
		fmt.Println("执行失败:", err)
		os.Exit(1)
	}

	var wrapper struct {
		Status int    `json:"status"`
		Len    int    `json:"len"`
		Body   string `json:"body"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal([]byte(respJSON), &wrapper); err != nil {
		fmt.Println("解析包装失败:", err)
		os.Exit(1)
	}
	if wrapper.Error != "" {
		fmt.Println("fetch 错误:", wrapper.Error)
		os.Exit(1)
	}
	fmt.Printf("HTTP %d，响应 %d 字节\n", wrapper.Status, wrapper.Len)

	var sr searchResp
	if err := json.Unmarshal([]byte(wrapper.Body), &sr); err != nil {
		fmt.Println("响应不是预期 JSON:", err)
		fmt.Println("首部:", first(wrapper.Body, 300))
		os.Exit(1)
	}
	if sr.StatusCode != 0 {
		fmt.Printf("❌ 接口 status_code=%d（可能需要登录/签名/验证）\n", sr.StatusCode)
		fmt.Println("首部:", first(wrapper.Body, 300))
		os.Exit(1)
	}
	n := 0
	for _, d := range sr.Data {
		if d.AwemeInfo.AwemeID == "" {
			continue
		}
		n++
		s := d.AwemeInfo.Statistics
		fmt.Printf("  %d. %s | @%s | 播放%d 赞%d 评%d | https://www.douyin.com/video/%s\n",
			n, first(d.AwemeInfo.Desc, 30), d.AwemeInfo.Author.Nickname,
			s.PlayCount, s.DiggCount, s.CommentCount, d.AwemeInfo.AwemeID)
	}
	if n == 0 {
		fmt.Println("❌ 响应无视频条目——首部:", first(wrapper.Body, 300))
		os.Exit(1)
	}
	fmt.Printf("\n✅ 免签路径可用：游客模式搜到 %d 条真实视频（含数据）\n", n)
}

func first(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
