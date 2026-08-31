// zhpubdebug：知乎"页面内 fetch 发布 API"可行性验证工具（MediaCrawler 思想写场景移植）。
//
// 命题：知乎 API 在页面上下文内 fetch 时 cookie 自动携带（z_c0/d_c0/x-xsrftoken），
// 发布类接口是否可用强签名（x-zse-96）拦截——本工具只验证读 API + 探测，
// 不发任何内容。验证通过 → 知乎发布从 DOM 六步升级为"页面内 API"。
//
// 用法：go run ./cmd/zhpubdebug
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"webreaper/internal/adapter/chromedputil"
	"webreaper/internal/adapter/crypto"
	"webreaper/internal/adapter/repository"
	"webreaper/internal/config"
)

const meFetchJS = `(async () => {
  const hasZc0 = document.cookie.includes('z_c0');
  const cookieKeys = document.cookie.split('; ').map(c => c.split('=')[0]).filter(k => k.includes('z_c') || k.includes('_xsrf') || k.includes('q_c'));
  try {
    const r = await fetch('https://www.zhihu.com/api/v4/me', { credentials: 'include', redirect: 'follow' });
    const text = await r.text();
    return JSON.stringify({ status: r.status, redirected: r.redirected, url: r.url.slice(0, 80), hasZc0, cookieKeys, body: text.slice(0, 200) });
  } catch (e) { return JSON.stringify({ error: String(e), hasZc0, cookieKeys }); }
})`

const draftProbeJS = `(async () => {
  const cookies = document.cookie.split('; ').map(c => c.split('=')[0]);
  try {
    const r = await fetch('https://zhuanlan.zhihu.com/api/articles/drafts', {
      method: 'OPTIONS', credentials: 'include',
    });
    return JSON.stringify({ cookieNames: cookies, optionsStatus: r.status, allow: r.headers.get('allow') || '' });
  } catch (e) { return JSON.stringify({ cookieNames: cookies, error: String(e) }); }
})()`

func main() {
	cfg := config.Load()
	if !cfg.DB.IsConfigured() || cfg.Publish.CookieSecret == "" {
		fmt.Println("需配置 DB + PUBLISH_COOKIE_SECRET")
		os.Exit(1)
	}
	db, err := repository.NewMySQLDBFromConfig(cfg.DB)
	if err != nil {
		fmt.Println("DB 连接失败:", err)
		os.Exit(1)
	}
	vault, err := crypto.NewAESCookieVault(cfg.Publish.CookieSecret)
	if err != nil {
		fmt.Println("vault 初始化失败:", err)
		os.Exit(1)
	}
	accounts, err := repository.NewGormAccountRepository(db).ListAll(context.Background())
	if err != nil {
		fmt.Println("查账号失败:", err)
		os.Exit(1)
	}
	var enc string
	for _, a := range accounts {
		if a.Platform == "zhihu" && a.IsHealthy() && a.CookieEncrypted != "" {
			enc = a.CookieEncrypted
			break
		}
	}
	if enc == "" {
		fmt.Println("无健康知乎 cookie 账号（先扫码绑定）")
		os.Exit(1)
	}
	cookie, err := vault.Decrypt(enc)
	if err != nil {
		fmt.Println("cookie 解密失败:", err)
		os.Exit(1)
	}

	opts := chromedputil.HeadlessOptions(false)
	opts = append(opts,
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	browserCtx, cancel2 := chromedp.NewContext(allocCtx)
	defer cancel2()
	ctx, cancel3 := context.WithTimeout(browserCtx, 60*time.Second)
	defer cancel3()

	fmt.Println("① 打开知乎（cookie 注入）…")
	var currentURL string
	err = chromedp.Run(ctx,
		network.Enable(),
		network.SetCookies(parseCookies(cookie, ".zhihu.com")),
		chromedp.Navigate("https://www.zhihu.com"),
		chromedp.Sleep(3*time.Second),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		fmt.Println("导航失败:", err)
		os.Exit(1)
	}
	fmt.Println("  URL:", currentURL)

	// CDP 网络监听：捕获 fetch 的真实请求/响应（document.cookie 看不到 httpOnly z_c0，
	// 网络层才是真相）
	type netResp struct{ url string; status int64 }
	var responses []netResp
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if r, ok := ev.(*network.EventResponseReceived); ok && r != nil {
			u := r.Response.URL
			if strings.Contains(u, "zhihu.com/api") {
				responses = append(responses, netResp{u, r.Response.Status})
			}
		}
	})

	fmt.Println("② 页面内 fetch /api/v4/me …")
	var meRaw string
	if err := evalAwait(ctx, meFetchJS, &meRaw); err != nil {
		fmt.Println("  执行失败:", err)
		os.Exit(1)
	}
	var me struct {
		Status     int      `json:"status"`
		Body       string   `json:"body"`
		Error      string   `json:"error"`
		HasZc0     bool     `json:"hasZc0"`
		CookieKeys []string `json:"cookieKeys"`
		Redirected bool     `json:"redirected"`
		URL        string   `json:"url"`
	}
	_ = json.Unmarshal([]byte(meRaw), &me)
	if me.Error != "" {
		fmt.Println("  fetch 错误:", me.Error)
		os.Exit(1)
	}
	fmt.Printf("  z_c0=%v 关键cookie=%v\n", me.HasZc0, me.CookieKeys)
	fmt.Printf("  HTTP %d redirected=%v url=%s\n", me.Status, me.Redirected, me.URL)
	fmt.Println("  响应:", me.Body[:min(160, len(me.Body))])

	fmt.Println("  网络层捕获的 zhihu API 响应：")
	for _, r := range responses {
		fmt.Printf("    %s -> %d\n", r.url[:min(70, len(r.url))], r.status)
	}

	fmt.Println("③ 发布 API 探测（OPTIONS，不发内容）…")
	var probeRaw string
	if err := evalAwait(ctx, draftProbeJS, &probeRaw); err == nil {
		fmt.Println(" ", probeRaw[:min(280, len(probeRaw))])
	}

	fmt.Println("④ 写文章页草稿自动保存捕获（发现真实发布端点）…")
	{
		var posts []string
		chromedp.ListenTarget(ctx, func(ev interface{}) {
			if r, ok := ev.(*network.EventRequestWillBeSent); ok && r != nil {
				if r.Request.Method == "POST" && strings.Contains(r.Request.URL, "zhihu.com") {
					posts = append(posts, fmt.Sprintf("%s %s", r.Request.Method, r.Request.URL))
				}
			}
		})
		var titleBox string
		if e := chromedp.Run(ctx,
			chromedp.Navigate("https://zhuanlan.zhihu.com/write"),
			chromedp.Sleep(4*time.Second),
			chromedp.Location(&currentURL),
		); e == nil {
			fmt.Println("  write页 URL:", currentURL)
			// 输入标题触发自动保存（不发布）
			_ = chromedp.Run(ctx,
				chromedp.WaitVisible(`.WriteIndex-titleInput textarea, textarea[placeholder*="标题"]`, chromedp.ByQuery),
				chromedp.SendKeys(`.WriteIndex-titleInput textarea`, "webreaper-draft-probe", chromedp.ByQuery),
			)
			_ = titleBox
			chromedp.Sleep(8 * time.Second) // 等自动保存触发
			fmt.Printf("  捕获 %d 个 POST：\n", len(posts))
			for _, p := range posts {
				fmt.Println("   ", p[:min(150, len(p))])
			}
		} else {
			fmt.Println("  write 页打开失败:", e)
		}
	}

	if me.Status == 200 {
		fmt.Println("\n✅ 页面内 fetch 链路通（cookie 自动携带）——API 直调路线可行")
	} else if !me.HasZc0 {
		fmt.Println("\n❌ z_c0 缺失（登录态 cookie 未注入/过期）——重新扫码绑定")
	} else if me.Status == 401 {
		fmt.Println("\n❌ cookie 失效（401）——重新绑定后重试")
	} else {
		fmt.Printf("\n⚠️ 状态 %d redirected=%v——检查响应\n", me.Status, me.Redirected)
	}
}

func evalAwait(ctx context.Context, js string, out *string) error {
	return chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error {
		result, _, err := runtime.Evaluate(js).WithAwaitPromise(true).WithReturnByValue(true).Do(c)
		if err != nil {
			return err
		}
		if result == nil || result.Value == nil {
			return fmt.Errorf("空结果")
		}
		*out = string(result.Value)
		return nil
	}))
}

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
