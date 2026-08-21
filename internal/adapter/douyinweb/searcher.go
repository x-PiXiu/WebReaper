// Package douyinweb 实现抖音站内搜索/详情/评论（port.SocialSearcher 的 douyin 平台实现）。
//
// 执行模式（完整调研后的最终解——搜索页上下文 + 同步 XHR）：
//   1. chromedp 携 cookie 导航到 /search/{keyword} 页面（页面加载后安全 SDK 自动初始化）
//   2. 在该页面上下文中执行同步 XMLHttpRequest（不走 fetch/SW 管道，不被安全 SDK 挂起）
//   3. XHR 响应体从 JS 层直接返回（ReturnByValue）
//
// 踩坑记录（为什么这么绕）：
//   - 页面内 fetch：请求发出但 r.text() 被安全 SDK 无限挂起
//   - Go 直调（带 cookie+msToken）：被 verify_check 风控拦截（data=[]）
//   - 首页上下文 XHR：同样 verify_check
//   - 搜索页上下文 XHR：✅ 成功返回真实数据——搜索页有完整的安全上下文
//
// 协议知识来源（MediaCrawler 项目验证过的 web 接口行为，不复制其代码）：
//   - 搜索：GET /aweme/v1/web/general/search/single/ —— 搜索页上下文免签
//   - 详情：GET /aweme/v1/web/aweme/detail/?aweme_id= —— 视频页上下文
//   - 评论：GET /aweme/v1/web/comment/list/ —— cursor 分页
package douyinweb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"webreaper/internal/adapter/chromedputil"
	"webreaper/internal/usecase/port"
)

const platform = "douyin"
const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// Searcher port.SocialSearcher 的 douyin 实现。
type Searcher struct {
	accountRepo port.AccountRepository
	vault       port.CookieVault
}

func NewSearcher(ar port.AccountRepository, vault port.CookieVault) *Searcher {
	return &Searcher{accountRepo: ar, vault: vault}
}

func (s *Searcher) SupportedPlatforms() []string { return []string{platform} }

// pickCookie 选健康的抖音 cookie 账号并解密。
// 优先取平台工作账号（role=platform）——搜索是只读操作，风控风险集中到平台可控账号，
// 不消耗商户账号的信任额度。无平台账号时回退商户账号（兼容期）。
func (s *Searcher) pickCookie(ctx context.Context, tenantID, plat string) (string, error) {
	accounts, err := s.accountRepo.ListAll(ctx)
	if err != nil {
		return "", err
	}
	// 第一优先：平台工作账号（跨租户共享）
	for _, acc := range accounts {
		if acc.Platform != plat || !acc.IsHealthy() || acc.IsOAuth() || acc.CookieEncrypted == "" {
			continue
		}
		if acc.Role == "platform" {
			if cookie, dErr := s.vault.Decrypt(acc.CookieEncrypted); dErr == nil && cookie != "" {
				log.Printf("[douyinweb] 使用平台工作账号 %s（风控集中）", acc.ID[:12])
				return cookie, nil
			}
		}
	}
	// 兼容回退：商户自己的账号
	for _, acc := range accounts {
		if acc.Platform != plat || acc.TenantID != tenantID || !acc.IsHealthy() || acc.IsOAuth() || acc.CookieEncrypted == "" {
			continue
		}
		if cookie, dErr := s.vault.Decrypt(acc.CookieEncrypted); dErr == nil && cookie != "" {
			log.Printf("[douyinweb] 回退使用商户账号 %s（建议管理员绑定平台工作账号）", acc.ID[:12])
			return cookie, nil
		}
	}
	return "", fmt.Errorf("无可用 %s cookie 账号（建议管理员绑定平台工作账号）", plat)
}

// withSearchPage 打开搜索页 → 执行 fn（fn 内发 XHR）。
// 搜索页上下文是关键——首页/精选页的 XHR 会被 verify_check 拦截。
func (s *Searcher) withSearchPage(ctx context.Context, tenantID, plat, keyword string, fn func(pctx context.Context) error) error {
	cookie, err := s.pickCookie(ctx, tenantID, plat)
	if err != nil {
		return err
	}

	opts := chromedputil.HeadlessOptions(false)
	opts = append(opts,
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent(userAgent),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()
	sessionCtx, sessionCancel := context.WithTimeout(browserCtx, 45*time.Second)
	defer sessionCancel()

	searchURL := "https://www.douyin.com/search/" + keyword + "?type=general"
	var currentURL string
	err = chromedp.Run(sessionCtx,
		network.Enable(),
		network.SetCookies(parseCookies(cookie, ".douyin.com")),
		chromedp.Navigate(searchURL),
		chromedp.Sleep(8*time.Second), // 等安全 SDK 完整初始化（搜索页安全上下文需要更长）
		chromedp.Location(&currentURL),
	)
	if err != nil {
		return fmt.Errorf("打开搜索页失败: %w", err)
	}
	if strings.Contains(currentURL, "login") {
		return fmt.Errorf("cookie 失效（重定向登录页），请重新绑定账号")
	}
	log.Printf("[douyinweb] 搜索页已打开: %s", currentURL)

	return fn(sessionCtx)
}

// withVideoPage 打开视频详情页（详情/评论接口的上下文）。
func (s *Searcher) withVideoPage(ctx context.Context, tenantID, plat, videoID string, fn func(pctx context.Context) error) error {
	cookie, err := s.pickCookie(ctx, tenantID, plat)
	if err != nil {
		return err
	}

	opts := chromedputil.HeadlessOptions(false)
	opts = append(opts,
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent(userAgent),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()
	sessionCtx, sessionCancel := context.WithTimeout(browserCtx, 45*time.Second)
	defer sessionCancel()

	videoURL := "https://www.douyin.com/video/" + videoID
	var currentURL string
	err = chromedp.Run(sessionCtx,
		network.Enable(),
		network.SetCookies(parseCookies(cookie, ".douyin.com")),
		chromedp.Navigate(videoURL),
		chromedp.Sleep(4*time.Second),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		return fmt.Errorf("打开视频页失败: %w", err)
	}
	log.Printf("[douyinweb] 视频页已打开: %s", currentURL)

	return fn(sessionCtx)
}

// xhrSyncJS 生成同步 XHR 调用 JS（在页面上下文中执行，返回响应文本）。
// 同步 XHR 不走 fetch/SW 管道——响应体不被安全 SDK 挂起。
func xhrSyncJS(apiPath string, paramsJS string) string {
	return fmt.Sprintf(`(() => {
  try {
    const xhr = new XMLHttpRequest();
    xhr.open('GET', '%s?' + %s, false);
    xhr.send();
    return xhr.responseText;
  } catch (e) { return JSON.stringify({error: String(e)}); }
})()`, apiPath, paramsJS)
}

// evalSync 同步 Evaluate（不需要 awaitPromise——XHR 是同步的）。
func evalSync(ctx context.Context, js string) (string, error) {
	var out string
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error {
		res, _, e := runtime.Evaluate(js).WithReturnByValue(true).Do(c)
		if e != nil {
			return e
		}
		if res != nil && res.Value != nil {
			return json.Unmarshal([]byte(string(res.Value)), &out)
		}
		return fmt.Errorf("空结果")
	}))
	return out, err
}

// buildParamsJS 生成 URLSearchParams 构造 JS。
func buildParamsJS(extra map[string]string) string {
	var entries []string
	for k, v := range extra {
		kj, _ := json.Marshal(k)
		vj, _ := json.Marshal(v)
		entries = append(entries, fmt.Sprintf("%s: %s", kj, vj))
	}
	return fmt.Sprintf(`new URLSearchParams({
  device_platform: 'webapp', aid: '6383', channel: 'channel_pc_web',
  cookie_enabled: 'true', browser_language: navigator.language,
  browser_platform: navigator.platform, browser_name: 'Chrome',
  browser_online: 'true', platform: 'PC',
  screen_width: String(screen.width), screen_height: String(screen.height),
  %s
}).toString()`, strings.Join(entries, ",\n  "))
}

// SearchHotVideos 站内搜索一周内最多点赞的热门视频。
func (s *Searcher) SearchHotVideos(ctx context.Context, tenantID, plat, keyword string, limit int) ([]port.SocialVideo, error) {
	if plat != platform {
		return nil, fmt.Errorf("douyinweb 不支持平台 %s", plat)
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	var out []port.SocialVideo
	err := s.withSearchPage(ctx, tenantID, plat, keyword, func(pctx context.Context) error {
		paramsJS := buildParamsJS(map[string]string{
			"keyword":          keyword,
			"search_channel":   "aweme_video_web",
			"search_source":    "tab_search",
			"is_filter_search": "1",
			"filter_selected":  `{"sort_type":"1","publish_time":"7"}`,
			"offset":           "0",
			"count":            fmt.Sprintf("%d", limit),
			"list_type":        "multi",
		})
		js := xhrSyncJS("/aweme/v1/web/general/search/single/", paramsJS)
		raw, e := evalSync(pctx, js)
		if e != nil {
			return fmt.Errorf("XHR 执行失败: %w", e)
		}

		var sr searchResp
		if jErr := json.Unmarshal([]byte(raw), &sr); jErr != nil {
			return fmt.Errorf("搜索响应解析失败: %v (首部=%.200s)", jErr, raw)
		}
		if sr.StatusCode != 0 {
			return statusErr(sr.StatusCode, sr.StatusMsg)
		}
		for _, d := range sr.Data {
			if d.AwemeInfo.AwemeID == "" {
				continue
			}
			out = append(out, toSocialVideo(d.AwemeInfo))
		}
		log.Printf("[douyinweb] search %q -> %d items (status=%d) raw=%.200s", keyword, len(out), sr.StatusCode, raw)
		if len(out) == 0 {
			return fmt.Errorf("搜索无结果 keyword=%q status=%d raw=%.200s", keyword, sr.StatusCode, raw)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetVideoDetail 单视频详情（数据回读用）。
func (s *Searcher) GetVideoDetail(ctx context.Context, tenantID, plat, videoID string) (*port.SocialVideo, error) {
	if plat != platform {
		return nil, fmt.Errorf("douyinweb 不支持平台 %s", plat)
	}

	var out *port.SocialVideo
	err := s.withVideoPage(ctx, tenantID, plat, videoID, func(pctx context.Context) error {
		paramsJS := buildParamsJS(map[string]string{"aweme_id": videoID})
		js := xhrSyncJS("/aweme/v1/web/aweme/detail/", paramsJS)
		raw, e := evalSync(pctx, js)
		if e != nil {
			return fmt.Errorf("XHR 执行失败: %w", e)
		}
		var dr detailResp
		if jErr := json.Unmarshal([]byte(raw), &dr); jErr != nil {
			return fmt.Errorf("详情解析失败: %v", jErr)
		}
		if dr.StatusCode != 0 {
			return statusErr(dr.StatusCode, "")
		}
		v := toSocialVideo(dr.AwemeDetail)
		out = &v
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetComments 视频评论（cursor 分页）。
func (s *Searcher) GetComments(ctx context.Context, tenantID, plat, videoID string, cursor, limit int) ([]port.SocialComment, error) {
	if plat != platform {
		return nil, fmt.Errorf("douyinweb 不支持平台 %s", plat)
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	var out []port.SocialComment
	err := s.withVideoPage(ctx, tenantID, plat, videoID, func(pctx context.Context) error {
		paramsJS := buildParamsJS(map[string]string{
			"aweme_id":  videoID,
			"cursor":    fmt.Sprintf("%d", cursor),
			"count":     fmt.Sprintf("%d", limit),
			"item_type": "0",
		})
		js := xhrSyncJS("/aweme/v1/web/comment/list/", paramsJS)
		raw, e := evalSync(pctx, js)
		if e != nil {
			return fmt.Errorf("XHR 执行失败: %w", e)
		}
		var cr commentResp
		if jErr := json.Unmarshal([]byte(raw), &cr); jErr != nil {
			return fmt.Errorf("评论解析失败: %v", jErr)
		}
		if cr.StatusCode != 0 {
			return statusErr(cr.StatusCode, "")
		}
		for _, c := range cr.Comments {
			out = append(out, port.SocialComment{
				CommentID:  c.CID,
				Content:    c.Text,
				User:       c.User.Nickname,
				DiggCount:  c.DiggCount,
				CreateTime: c.CreateTime,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ResolveShortURL 短链归一化：v.douyin.com/xxx 302 跟随取 aweme_id（纯 HTTP，无需浏览器）。
func (s *Searcher) ResolveShortURL(shortURL string) (string, string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodGet, shortURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" {
		loc = shortURL
	}
	for _, pat := range []string{"/video/", "/note/"} {
		if i := strings.Index(loc, pat); i >= 0 {
			tail := loc[i+len(pat):]
			id := tail
			if j := strings.IndexAny(id, "/?&"); j >= 0 {
				id = id[:j]
			}
			if len(id) >= 15 {
				return platform, id, nil
			}
		}
	}
	return "", "", fmt.Errorf("短链解析失败：%s → %s", shortURL, loc)
}

// IsAlive 登录态心跳：打开首页检查是否被重定向到登录页。
func (s *Searcher) IsAlive(ctx context.Context, tenantID, plat string) bool {
	if plat != platform {
		return false
	}
	cookie, err := s.pickCookie(ctx, tenantID, plat)
	if err != nil {
		return false
	}
	opts := chromedputil.HeadlessOptions(false)
	opts = append(opts, chromedp.WindowSize(1280, 800), chromedp.UserAgent(userAgent))
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()
	browserCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	pctx, pcancel := context.WithTimeout(browserCtx, 20*time.Second)
	defer pcancel()

	var cur string
	if e := chromedp.Run(pctx,
		network.Enable(),
		network.SetCookies(parseCookies(cookie, ".douyin.com")),
		chromedp.Navigate("https://www.douyin.com"),
		chromedp.Sleep(3*time.Second),
		chromedp.Location(&cur),
	); e != nil {
		return false
	}
	return !strings.Contains(cur, "login")
}

// statusErr 抖音错误码翻译。
func statusErr(code int, msg string) error {
	if code == 2483 {
		return fmt.Errorf("cookie 失效（请先登录，status_code=2483）")
	}
	return fmt.Errorf("接口错误 status_code=%d %s", code, msg)
}

// parseCookies cookie 字符串 → chromedp CookieParam。
func parseCookies(cookieStr, domain string) []*network.CookieParam {
	var out []*network.CookieParam
	for _, part := range strings.Split(cookieStr, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 || kv[0] == "" {
			continue
		}
		out = append(out, &network.CookieParam{Name: kv[0], Value: kv[1], Domain: domain, Path: "/"})
	}
	return out
}

// ---- 响应结构 ----

type videoStats struct {
	PlayCount    int `json:"play_count"`
	DiggCount    int `json:"digg_count"`
	CommentCount int `json:"comment_count"`
	ShareCount   int `json:"share_count"`
}

type awemeInfo struct {
	AwemeID    string `json:"aweme_id"`
	Desc       string `json:"desc"`
	Author     struct {
		Nickname string `json:"nickname"`
	} `json:"author"`
	Statistics videoStats `json:"statistics"`
	CreateTime int64      `json:"create_time"`
}

type searchResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
	Data       []struct {
		Type      int       `json:"type"`
		AwemeInfo awemeInfo `json:"aweme_info"`
	} `json:"data"`
}

type detailResp struct {
	StatusCode  int       `json:"status_code"`
	AwemeDetail awemeInfo `json:"aweme_detail"`
}

type commentResp struct {
	StatusCode int `json:"status_code"`
	Comments   []struct {
		CID        string `json:"cid"`
		Text       string `json:"text"`
		DiggCount  int    `json:"digg_count"`
		CreateTime int64  `json:"create_time"`
		User       struct {
			Nickname string `json:"nickname"`
		} `json:"user"`
	} `json:"comments"`
}

func toSocialVideo(a awemeInfo) port.SocialVideo {
	return port.SocialVideo{
		Platform:     platform,
		VideoID:      a.AwemeID,
		Desc:         a.Desc,
		Author:       a.Author.Nickname,
		URL:          "https://www.douyin.com/video/" + a.AwemeID,
		PlayCount:    a.Statistics.PlayCount,
		DiggCount:    a.Statistics.DiggCount,
		CommentCount: a.Statistics.CommentCount,
		ShareCount:   a.Statistics.ShareCount,
		CreateTime:   a.CreateTime,
	}
}

var _ port.SocialSearcher = (*Searcher)(nil)
