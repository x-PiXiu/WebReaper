// Package douyinweb 实现抖音站内搜索/详情（port.DouyinSearcher）。
//
// 协议知识来源（MediaCrawler 项目验证过的 web 接口行为，不复制其代码）：
//   - 搜索：GET /aweme/v1/web/general/search/single/ —— 免 a_bogus 签名；
//     通用参数（aid=6383/device_platform=webapp/webid/msToken…）齐全 + 登录 cookie 才放行
//   - 详情：GET /aweme/v1/web/aweme/detail/?aweme_id=
//   - 排序/时间过滤：sort_type=1（最多点赞）、publish_time=7（一周内）——"最近很火"
//
// 执行姿势：chromedp 携账号 cookie 打开 douyin.com → 页面内同源 fetch
// （credentials:include —— cookie/referer/UA 由真实浏览器环境携带，无需伪造签名）。
// 账号自包含：从账号库选租户下任一健康抖音 cookie 账号（搜索只读，任意登录态可用）。
package douyinweb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"webreaper/internal/adapter/chromedputil"
	"webreaper/internal/usecase/port"
)

// searchFetchJS 页面内搜索（参数从页面环境现算，比硬编码伪装真实）。
// kw=关键词；limit=条数。返回原始响应 JSON 文本。
const searchFetchJS = `(async (kw, limit) => {
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
    from_group_id: '', offset: '0', count: String(limit),
    need_filter_settings: '1', list_type: 'multi',
    version_code: '170400', pc_client_type: '1',
    cookie_enabled: navigator.cookieEnabled ? 'true' : 'false',
    browser_language: navigator.language, browser_platform: navigator.platform,
    browser_name: 'Chrome', browser_online: navigator.onLine ? 'true' : 'false',
    engine_name: 'Blink', os_name: '', os_version: '',
    cpu_core_num: String(navigator.hardwareConcurrency || 8),
    platform: 'PC', screen_width: String(screen.width), screen_height: String(screen.height),
    effective_type: '4g', round_trip_time: '50',
    webid: uuid(), msToken,
  });
  try {
    const r = await fetch('/aweme/v1/web/general/search/single/?' + params.toString(), {
      credentials: 'include',
      headers: { 'Referer': location.origin + '/search/' + encodeURIComponent(kw) + '?type=general' },
    });
    return await r.text();
  } catch (e) {
    return JSON.stringify({ fetch_error: String(e) });
  }
})`

// Searcher port.DouyinSearcher 实现。
type Searcher struct {
	accountRepo port.AccountRepository
	vault       port.CookieVault
}

func NewSearcher(ar port.AccountRepository, vault port.CookieVault) *Searcher {
	return &Searcher{accountRepo: ar, vault: vault}
}

// pickCookie 选租户下一个健康的抖音 cookie 账号并解密 cookie。
// 没有可用账号返回错误（调用方降级到通用搜索引擎链路）。
func (s *Searcher) pickCookie(ctx context.Context, tenantID string) (string, error) {
	accounts, err := s.accountRepo.ListByPlatform(ctx, tenantID, "douyin")
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
	return "", fmt.Errorf("无可用抖音 cookie 账号（需浏览器扫码绑定一个）")
}

// SearchHotVideos 站内搜索一周内最多点赞的热门视频。
func (s *Searcher) SearchHotVideos(ctx context.Context, tenantID, keyword string, limit int) ([]port.DouyinVideo, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	var out []port.DouyinVideo
	err := s.withDouyinPage(ctx, tenantID, func(pctx context.Context) error {
		raw, err := evalAwait(pctx, fmt.Sprintf(`(%s)(%q, %d)`, searchFetchJS, keyword, limit))
		if err != nil {
			return err
		}
		var sr searchResp
		if jErr := json.Unmarshal([]byte(raw), &sr); jErr != nil {
			return fmt.Errorf("响应解析失败: %v (首部=%.200s)", jErr, raw)
		}
		if sr.StatusCode != 0 {
			return fmt.Errorf("接口 status_code=%d %s", sr.StatusCode, sr.StatusMsg)
		}
		for _, d := range sr.Data {
			if d.AwemeInfo.AwemeID == "" {
				continue
			}
			a := d.AwemeInfo
			out = append(out, port.DouyinVideo{
				AwemeID:      a.AwemeID,
				Desc:         a.Desc,
				Author:       a.Author.Nickname,
				URL:          "https://www.douyin.com/video/" + a.AwemeID,
				PlayCount:    a.Statistics.PlayCount,
				DiggCount:    a.Statistics.DiggCount,
				CommentCount: a.Statistics.CommentCount,
				ShareCount:   a.Statistics.ShareCount,
				CreateTime:   a.CreateTime,
			})
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

// GetVideoDetail 单视频详情（数据回读用）。注意：detail 端点 MediaCrawler 标记需要
// a_bogus 签名——页面内 fetch 若被拒，后续需补签名 JS（搜索端点免签已验证）。
func (s *Searcher) GetVideoDetail(ctx context.Context, tenantID, awemeID string) (*port.DouyinVideo, error) {
	var out *port.DouyinVideo
	err := s.withDouyinPage(ctx, tenantID, func(pctx context.Context) error {
		raw, err := evalAwait(pctx, fmt.Sprintf(
			`(async (id) => { try { const r = await fetch('/aweme/v1/web/aweme/detail/?aweme_id=' + id + '&device_platform=webapp&aid=6383', { credentials: 'include' }); return await r.text(); } catch (e) { return JSON.stringify({ fetch_error: String(e) }); } })(%q)`, awemeID))
		if err != nil {
			return err
		}
		var dr detailResp
		if jErr := json.Unmarshal([]byte(raw), &dr); jErr != nil {
			return fmt.Errorf("详情解析失败: %v", jErr)
		}
		if dr.StatusCode != 0 {
			return fmt.Errorf("详情接口 status_code=%d", dr.StatusCode)
		}
		a := dr.AwemeDetail
		out = &port.DouyinVideo{
			AwemeID:      a.AwemeID,
			Desc:         a.Desc,
			Author:       a.Author.Nickname,
			URL:          "https://www.douyin.com/video/" + a.AwemeID,
			PlayCount:    a.Statistics.PlayCount,
			DiggCount:    a.Statistics.DiggCount,
			CommentCount: a.Statistics.CommentCount,
			ShareCount:   a.Statistics.ShareCount,
			CreateTime:   a.CreateTime,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// withDouyinPage 启动浏览器 → 注入账号 cookie → 打开 douyin.com → 执行 fn。
// cookie 域 .douyin.com；登录态由 cookie 携带（2483=请先登录 → cookie 失效/账号未绑）。
func (s *Searcher) withDouyinPage(ctx context.Context, tenantID string, fn func(pctx context.Context) error) error {
	cookie, err := s.pickCookie(ctx, tenantID)
	if err != nil {
		return err
	}

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
		network.SetCookies(parseCookies(cookie, ".douyin.com")),
		chromedp.Navigate("https://www.douyin.com"),
		chromedp.Sleep(3*time.Second),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		return fmt.Errorf("打开抖音失败: %w", err)
	}
	if strings.Contains(currentURL, "login") {
		return fmt.Errorf("cookie 失效（重定向登录页），请重新绑定抖音账号")
	}
	return fn(sessionCtx)
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
	StatusCode   int       `json:"status_code"`
	AwemeDetail  awemeInfo `json:"aweme_detail"`
}

var _ port.DouyinSearcher = (*Searcher)(nil)
