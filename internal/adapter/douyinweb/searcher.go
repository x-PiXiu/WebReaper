// Package douyinweb 实现抖音站内搜索/详情/评论（port.SocialSearcher 的 douyin 平台实现）。
//
// 执行模式（MediaCrawler 原版思想的 Go 落地——踩坑后确认的正确姿势）：
//   浏览器只做两件事：① cookie 会话维持 ② 从 localStorage 提取 msToken
//   实际数据请求由 Go net/http 直调（cookie + 通用参数 + UA/Referer）——
//   响应体在 Go 侧解析，完全绕开页面 JS 环境。
//
// 踩坑记录（为什么不能页面内 fetch）：
//   - 页面内 fetch 请求确实发出、服务器 200 响应（CDP 网络层确认）
//   - 但 r.text()/r.arrayBuffer() 的 Promise 被抖音安全 SDK（__security_mc_1_s_sdk_*）
//     无限期挂起——核心数据接口（搜索/详情）被保护，静态资源不受影响
//   - 结论：MediaCrawler 的 "Playwright 签名机 + httpx 直调" 模式是唯一可行路径
//
// 协议知识来源（MediaCrawler 项目验证过的 web 接口行为，不复制其代码）：
//   - 搜索：GET /aweme/v1/web/general/search/single/ —— 免 a_bogus 签名，需登录 cookie
//   - 详情：GET /aweme/v1/web/aweme/detail/?aweme_id=
//   - 评论：GET /aweme/v1/web/comment/list/ —— cursor 分页
package douyinweb

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
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
	httpClient  *http.Client
}

func NewSearcher(ar port.AccountRepository, vault port.CookieVault) *Searcher {
	return &Searcher{
		accountRepo: ar,
		vault:       vault,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		},
	}
}

func (s *Searcher) SupportedPlatforms() []string { return []string{platform} }

// pickCookie 选租户下一个健康的抖音 cookie 账号并解密 cookie。
func (s *Searcher) pickCookie(ctx context.Context, tenantID, plat string) (string, error) {
	accounts, err := s.accountRepo.ListByPlatform(ctx, tenantID, plat)
	if err != nil {
		return "", err
	}
	for _, acc := range accounts {
		if !acc.IsHealthy() || acc.IsOAuth() || acc.CookieEncrypted == "" {
			continue
		}
		cookie, dErr := s.vault.Decrypt(acc.CookieEncrypted)
		if dErr == nil && cookie != "" {
			return cookie, nil
		}
	}
	return "", fmt.Errorf("无可用 %s cookie 账号（需浏览器扫码绑定一个）", plat)
}

// pageEnv 浏览器页面环境（msToken 从 localStorage 提取）。
type pageEnv struct {
	msToken string
}

// withPageEnv 启动浏览器 → cookie 注入 → 打开 douyin.com → 提取 msToken → 执行 fn。
// 浏览器只活在这个函数里——数据请求由 fn 内的 Go HTTP 发出。
func (s *Searcher) withPageEnv(ctx context.Context, tenantID, plat string, fn func(env *pageEnv) error) error {
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
	sessionCtx, sessionCancel := context.WithTimeout(browserCtx, 30*time.Second)
	defer sessionCancel()

	var currentURL string
	err = chromedp.Run(sessionCtx,
		network.Enable(),
		network.SetCookies(parseCookies(cookie, ".douyin.com")),
		chromedp.Navigate("https://www.douyin.com"),
		chromedp.Sleep(3*time.Second),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		return fmt.Errorf("打开 %s 失败: %w", plat, err)
	}
	if strings.Contains(currentURL, "login") {
		return fmt.Errorf("cookie 失效（重定向登录页），请重新绑定账号")
	}
	log.Printf("[douyinweb] 已导航 %s，提取 msToken…", currentURL)

	// msToken 从 localStorage 提取（同步 JS，无 promise 问题）
	var msToken string
	_ = chromedp.Run(sessionCtx, chromedp.Evaluate(`(localStorage.getItem('xmst') || '') + ''`, &msToken))
	env := &pageEnv{msToken: msToken}
	log.Printf("[douyinweb] msToken=%d 字符", len(msToken))

	return fn(env)
}

// buildCommonParams 通用参数（MediaCrawler 协议知识：aid=6383 等）。
func buildCommonParams(keyword string, extra map[string]string) url.Values {
	q := url.Values{
		"device_platform": {"webapp"},
		"aid":             {"6383"},
		"channel":         {"channel_pc_web"},
		"cookie_enabled":  {"true"},
		"browser_language": {"zh-CN"},
		"browser_platform": {"Win32"},
		"browser_name":    {"Chrome"},
		"browser_online":  {"true"},
		"platform":        {"PC"},
		"screen_width":    {"1920"},
		"screen_height":   {"1080"},
		"webid":           {randWebID()},
	}
	if keyword != "" {
		q.Set("keyword", keyword)
	}
	for k, v := range extra {
		q.Set(k, v)
	}
	return q
}

func randWebID() string {
	const hex = "0123456789abcdef"
	b := make([]byte, 36)
	for i := range b {
		b[i] = hex[rand.Intn(16)]
	}
	b[14] = '4'
	return string(b)
}

// httpGet Go HTTP 直调抖音 web 接口（带 cookie + UA + Referer）。
func (s *Searcher) httpGet(ctx context.Context, cookie, apiPath string, params url.Values, referer string) ([]byte, error) {
	u := "https://www.douyin.com" + apiPath + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求抖音失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %.200s", resp.StatusCode, body)
	}
	return body, nil
}

// SearchHotVideos 站内搜索一周内最多点赞的热门视频（douyin 平台）。
func (s *Searcher) SearchHotVideos(ctx context.Context, tenantID, plat, keyword string, limit int) ([]port.SocialVideo, error) {
	if plat != platform {
		return nil, fmt.Errorf("douyinweb 不支持平台 %s", plat)
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	cookie, err := s.pickCookie(ctx, tenantID, plat)
	if err != nil {
		return nil, err
	}

	params := buildCommonParams(keyword, map[string]string{
		"search_channel":    "aweme_video_web",
		"enable_history":    "1",
		"search_source":     "tab_search",
		"query_correct_type": "1",
		"is_filter_search":  "1",
		"filter_selected":   `{"sort_type":"1","publish_time":"7"}`,
		"offset":            "0",
		"count":             fmt.Sprintf("%d", limit),
		"list_type":         "multi",
	})
	referer := "https://www.douyin.com/search/" + url.QueryEscape(keyword) + "?type=general"
	raw, err := s.httpGet(ctx, cookie, "/aweme/v1/web/general/search/single/", params, referer)
	if err != nil {
		return nil, err
	}

	var sr searchResp
	if jErr := json.Unmarshal(raw, &sr); jErr != nil {
		return nil, fmt.Errorf("搜索响应解析失败: %v (首部=%.200s)", jErr, raw)
	}
	if sr.StatusCode != 0 {
		return nil, statusErr(sr.StatusCode, sr.StatusMsg)
	}
	var out []port.SocialVideo
	for _, d := range sr.Data {
		if d.AwemeInfo.AwemeID == "" {
			continue
		}
		out = append(out, toSocialVideo(d.AwemeInfo))
	}
	log.Printf("[douyinweb] search %q -> %d items (status=%d)", keyword, len(out), sr.StatusCode)
	if len(out) == 0 {
		return nil, fmt.Errorf("搜索无结果 keyword=%q status=%d raw=%.200s", keyword, sr.StatusCode, raw)
	}
	return out, nil
}

// GetVideoDetail 单视频详情（数据回读用）。
func (s *Searcher) GetVideoDetail(ctx context.Context, tenantID, plat, videoID string) (*port.SocialVideo, error) {
	if plat != platform {
		return nil, fmt.Errorf("douyinweb 不支持平台 %s", plat)
	}
	cookie, err := s.pickCookie(ctx, tenantID, plat)
	if err != nil {
		return nil, err
	}
	params := buildCommonParams("", map[string]string{"aweme_id": videoID})
	referer := "https://www.douyin.com/video/" + videoID
	raw, err := s.httpGet(ctx, cookie, "/aweme/v1/web/aweme/detail/", params, referer)
	if err != nil {
		return nil, err
	}
	var dr detailResp
	if jErr := json.Unmarshal(raw, &dr); jErr != nil {
		return nil, fmt.Errorf("详情解析失败: %v", jErr)
	}
	if dr.StatusCode != 0 {
		return nil, statusErr(dr.StatusCode, "")
	}
	v := toSocialVideo(dr.AwemeDetail)
	return &v, nil
}

// GetComments 视频评论（cursor 分页）。
func (s *Searcher) GetComments(ctx context.Context, tenantID, plat, videoID string, cursor, limit int) ([]port.SocialComment, error) {
	if plat != platform {
		return nil, fmt.Errorf("douyinweb 不支持平台 %s", plat)
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	cookie, err := s.pickCookie(ctx, tenantID, plat)
	if err != nil {
		return nil, err
	}
	params := buildCommonParams("", map[string]string{
		"aweme_id": videoID,
		"cursor":   fmt.Sprintf("%d", cursor),
		"count":    fmt.Sprintf("%d", limit),
		"item_type": "0",
	})
	referer := "https://www.douyin.com/video/" + videoID
	raw, err := s.httpGet(ctx, cookie, "/aweme/v1/web/comment/list/", params, referer)
	if err != nil {
		return nil, err
	}
	var cr commentResp
	if jErr := json.Unmarshal(raw, &cr); jErr != nil {
		return nil, fmt.Errorf("评论解析失败: %v", jErr)
	}
	if cr.StatusCode != 0 {
		return nil, statusErr(cr.StatusCode, "")
	}
	var out []port.SocialComment
	for _, c := range cr.Comments {
		out = append(out, port.SocialComment{
			CommentID:  c.CID,
			Content:    c.Text,
			User:       c.User.Nickname,
			DiggCount:  c.DiggCount,
			CreateTime: c.CreateTime,
		})
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

// IsAlive 登录态心跳（pong）：GET 我的个人信息接口，200+非 2483 即活。
func (s *Searcher) IsAlive(ctx context.Context, tenantID, plat string) bool {
	if plat != platform {
		return false
	}
	cookie, err := s.pickCookie(ctx, tenantID, plat)
	if err != nil {
		return false
	}
	params := buildCommonParams("", nil)
	raw, err := s.httpGet(ctx, cookie, "/aweme/v1/web/im/user/info/", params, "https://www.douyin.com/")
	if err != nil {
		return false
	}
	var r struct {
		StatusCode int `json:"status_code"`
	}
	if json.Unmarshal(raw, &r) != nil {
		return false
	}
	return r.StatusCode != 2483 && r.StatusCode != 8
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
