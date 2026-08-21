// Package douyinweb 实现抖音站内搜索/详情/评论（port.SocialSearcher 的 douyin 平台实现）。
//
// 协议知识来源（MediaCrawler 项目验证过的 web 接口行为，不复制其代码）：
//   - 搜索：GET /aweme/v1/web/general/search/single/ —— 免 a_bogus 签名；
//     通用参数（aid=6383/device_platform=webapp/webid/msToken…）齐全 + 登录 cookie 才放行
//   - 详情：GET /aweme/v1/web/aweme/detail/?aweme_id=
//   - 评论：GET /aweme/v1/web/comment/list/ —— cursor 分页，20 条/页
//   - 排序/时间过滤：sort_type=1（最多点赞）、publish_time=7（一周内）——"最近很火"
//
// 执行姿势：chromedp 携账号 cookie 打开 douyin.com → 页面内同源 fetch
// （credentials:include —— cookie/referer/UA 由真实浏览器环境携带，无需伪造签名）。
// 账号自包含：从账号库选租户下任一健康抖音 cookie 账号（只读操作，任意登录态可用）。
//
// 多平台演进（方案 B）：本包只管 douyin；新平台（kuaishou/xhs/…）= 新增同构适配器
// + main 注册进 registry，用例层以 platform 参数调用零改动。
package douyinweb

import (
	"context"
	"encoding/json"
	"fmt"
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

// commonParamsJS 通用参数片段（页面环境现算，比硬编码伪装真实）——供各 fetch JS 复用。
const commonParamsJS = `(() => {
  const uuid = () => 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = Math.random() * 16 | 0;
    return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
  });
  let msToken = '';
  try { msToken = localStorage.getItem('xmst') || ''; } catch (e) {}
  return {
    device_platform: 'webapp', aid: '6383', channel: 'channel_pc_web',
    version_code: '170400', pc_client_type: '1',
    cookie_enabled: navigator.cookieEnabled ? 'true' : 'false',
    browser_language: navigator.language, browser_platform: navigator.platform,
    browser_name: 'Chrome', browser_online: navigator.onLine ? 'true' : 'false',
    engine_name: 'Blink', os_name: '', os_version: '',
    cpu_core_num: String(navigator.hardwareConcurrency || 8),
    platform: 'PC', screen_width: String(screen.width), screen_height: String(screen.height),
    effective_type: '4g', round_trip_time: '50',
    webid: uuid(), msToken,
  };
})()`

// searchFetchJS 页面内搜索（一周内+最多点赞 = 最近很火）。
const searchFetchJS = `(async (kw, limit) => {
  const cp = ${commonParamsJS};
  const params = new URLSearchParams({ ...cp,
    aweme_type: '0', keyword: kw, search_channel: 'aweme_video_web',
    enable_history: '1', search_source: 'tab_search', query_correct_type: '1',
    is_filter_search: '1',
    filter_selected: JSON.stringify({ sort_type: '1', publish_time: '7' }),
    from_group_id: '', offset: '0', count: String(limit),
    need_filter_settings: '1', list_type: 'multi',
  });
  try {
    const r = await fetch('/aweme/v1/web/general/search/single/?' + params.toString(), {
      credentials: 'include',
      headers: { 'Referer': location.origin + '/search/' + encodeURIComponent(kw) + '?type=general' },
    });
    return await r.text();
  } catch (e) { return JSON.stringify({ fetch_error: String(e) }); }
})`

// commentsFetchJS 页面内拉评论（cursor 分页）。
const commentsFetchJS = `(async (vid, cursor, count) => {
  const cp = ${commonParamsJS};
  const params = new URLSearchParams({ ...cp,
    aweme_id: vid, cursor: String(cursor), count: String(count), item_type: '0',
  });
  try {
    const r = await fetch('/aweme/v1/web/comment/list/?' + params.toString(), {
      credentials: 'include',
      headers: { 'Referer': location.origin + '/video/' + vid },
    });
    return await r.text();
  } catch (e) { return JSON.stringify({ fetch_error: String(e) }); }
})`

// detailFetchJS 页面内查单视频详情。
const detailFetchJS = `(async (vid) => {
  const cp = ${commonParamsJS};
  const params = new URLSearchParams({ ...cp, aweme_id: vid });
  try {
    const r = await fetch('/aweme/v1/web/aweme/detail/?' + params.toString(), {
      credentials: 'include',
      headers: { 'Referer': location.origin + '/video/' + vid },
    });
    return await r.text();
  } catch (e) { return JSON.stringify({ fetch_error: String(e) }); }
})`

// Searcher port.SocialSearcher 的 douyin 实现。
type Searcher struct {
	accountRepo port.AccountRepository
	vault       port.CookieVault
}

func NewSearcher(ar port.AccountRepository, vault port.CookieVault) *Searcher {
	return &Searcher{accountRepo: ar, vault: vault}
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

// SearchHotVideos 站内搜索一周内最多点赞的热门视频（douyin 平台）。
func (s *Searcher) SearchHotVideos(ctx context.Context, tenantID, plat, keyword string, limit int) ([]port.SocialVideo, error) {
	if plat != platform {
		return nil, fmt.Errorf("douyinweb 不支持平台 %s", plat)
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	var out []port.SocialVideo
	err := s.withPage(ctx, tenantID, plat, func(pctx context.Context) error {
		raw, err := evalAwait(pctx, fmt.Sprintf(`(%s)(%q, %d)`, searchFetchJS, keyword, limit))
		if err != nil {
			return err
		}
		var sr searchResp
		if jErr := json.Unmarshal([]byte(raw), &sr); jErr != nil {
			return fmt.Errorf("响应解析失败: %v (首部=%.200s)", jErr, raw)
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
		if len(out) == 0 {
			return fmt.Errorf("搜索无结果（keyword=%q）", keyword)
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
	err := s.withPage(ctx, tenantID, plat, func(pctx context.Context) error {
		raw, err := evalAwait(pctx, fmt.Sprintf(`(%s)(%q)`, detailFetchJS, videoID))
		if err != nil {
			return err
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

// GetComments 视频评论（cursor 分页）。线索挖掘只拉前几页即可——勿全量翻页。
func (s *Searcher) GetComments(ctx context.Context, tenantID, plat, videoID string, cursor, limit int) ([]port.SocialComment, error) {
	if plat != platform {
		return nil, fmt.Errorf("douyinweb 不支持平台 %s", plat)
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var out []port.SocialComment
	err := s.withPage(ctx, tenantID, plat, func(pctx context.Context) error {
		raw, err := evalAwait(pctx, fmt.Sprintf(`(%s)(%q, %d, %d)`, commentsFetchJS, videoID, cursor, limit))
		if err != nil {
			return err
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
			return http.ErrUseLastResponse // 不真正跟随，取第一个 302 的 Location
		},
	}
	req, err := http.NewRequest(http.MethodGet, shortURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" {
		// 短链可能已直接 200；从原 URL 兜底提取
		loc = shortURL
	}
	for _, pat := range []string{"/video/", "/note/"} {
		if i := strings.Index(loc, pat); i >= 0 {
			tail := loc[i+len(pat):]
			id := tail
			if j := strings.IndexAny(id, "/?&"); j >= 0 {
				id = id[:j]
			}
			if len(id) >= 15 { // 抖音 aweme_id 为 15+ 位数字
				return platform, id, nil
			}
		}
	}
	return "", "", fmt.Errorf("短链解析失败：%s → %s", shortURL, loc)
}

// IsAlive 登录态心跳（pong）：用 count=1 的搜索请求实测——
// status_code=0 活；2483（请先登录）/8（未登录）死。比 ExpiresAt 猜测真实。
func (s *Searcher) IsAlive(ctx context.Context, tenantID, plat string) bool {
	if plat != platform {
		return false
	}
	var alive bool
	_ = s.withPage(ctx, tenantID, plat, func(pctx context.Context) error {
		raw, err := evalAwait(pctx, fmt.Sprintf(`(%s)(%q, 1)`, searchFetchJS, "test"))
		if err != nil {
			return err
		}
		var sr searchResp
		if json.Unmarshal([]byte(raw), &sr) == nil {
			alive = sr.StatusCode != 2483 && sr.StatusCode != 8
		}
		return nil
	})
	return alive
}

// withPage 启动浏览器 → 注入账号 cookie → 打开平台首页 → 执行 fn。
func (s *Searcher) withPage(ctx context.Context, tenantID, plat string, fn func(pctx context.Context) error) error {
	cookie, err := s.pickCookie(ctx, tenantID, plat)
	if err != nil {
		return err
	}
	domain := "." + plat + ".com"

	opts := chromedputil.HeadlessOptions(false)
	opts = append(opts,
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()
	sessionCtx, sessionCancel := context.WithTimeout(browserCtx, 60*time.Second)
	defer sessionCancel()

	// 首次 Run + cookie 注入 + 导航必须同一 context（qrlogin/publisher 同款陷阱）
	var currentURL string
	err = chromedp.Run(sessionCtx,
		network.Enable(),
		network.SetCookies(parseCookies(cookie, domain)),
		chromedp.Navigate("https://www."+plat+".com"),
		chromedp.Sleep(3*time.Second),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		return fmt.Errorf("打开 %s 失败: %w", plat, err)
	}
	if strings.Contains(currentURL, "login") {
		return fmt.Errorf("cookie 失效（重定向登录页），请重新绑定账号")
	}
	return fn(sessionCtx)
}

// statusErr 抖音错误码翻译（2483=请先登录 → cookie 失效语义，健康检查消费）。
func statusErr(code int, msg string) error {
	if code == 2483 {
		return fmt.Errorf("cookie 失效（请先登录，status_code=2483）")
	}
	return fmt.Errorf("接口错误 status_code=%d %s", code, msg)
}

// evalAwait 页面内执行 async JS 并等待结果（AwaitPromise+ReturnByValue）。
func evalAwait(ctx context.Context, js string) (string, error) {
	result, _, err := runtime.Evaluate(js).
		WithAwaitPromise(true).
		WithReturnByValue(true).
		Do(ctx)
	if err != nil {
		return "", err
	}
	if result == nil || result.Value == nil {
		return "", fmt.Errorf("空结果")
	}
	var s string
	if jErr := json.Unmarshal(result.Value, &s); jErr != nil {
		return "", fmt.Errorf("结果非字符串: %v", jErr)
	}
	return s, nil
}

// parseCookies cookie 字符串 → chromedp CookieParam（与 publisher 同模式）。
func parseCookies(cookieStr, domain string) []*network.CookieParam {
	var cookies []*network.CookieParam
	for _, part := range strings.Split(cookieStr, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 || kv[0] == "" {
			continue
		}
		cookies = append(cookies, &network.CookieParam{
			Name: kv[0], Value: kv[1], Domain: domain, Path: "/",
		})
	}
	return cookies
}

// ---- 响应结构（只取关心的字段）----

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
