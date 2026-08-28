// bilidecl：B站创作声明挂载位置诊断（只读不发布）。
//
// 回答三个问题：
//  ① .creation-statement-container 在主文档 querySelector 能否命中
//  ② micro-app 元素的 shadowRoot/iframe 结构（子应用渲染容器）
//  ③ 滚动到底/等待后声明 input 是否出现，出现的精确路径
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"webreaper/internal/adapter/crypto"
	"webreaper/internal/adapter/publisher/humanize"
)

func main() {
	_ = godotenv.Load("configs/.env")
	// ① 取账号 cookie
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME"))
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("DB 连接失败:", err)
		os.Exit(1)
	}
	var enc string
	db.Raw("SELECT cookie_encrypted FROM geo_accounts WHERE platform='bilibili' ORDER BY bound_at DESC LIMIT 1").Scan(&enc)
	vault, err := crypto.NewAESCookieVault(os.Getenv("PUBLISH_COOKIE_SECRET"))
	if err != nil {
		fmt.Println("vault 失败:", err)
		os.Exit(1)
	}
	cookie, err := vault.Decrypt(enc)
	if err != nil {
		fmt.Println("解密失败:", err)
		os.Exit(1)
	}
	fmt.Println("✅ cookie 就绪（解密", len(cookie), "字节）")

	// ② 打开发布页（与通道同款配置）
	opts := humanize.StealthOptions()
	opts = append(opts,
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	browserCtx, cancel2 := chromedp.NewContext(allocCtx)
	defer cancel2()
	ctx, cancel3 := context.WithTimeout(browserCtx, 90*time.Second)
	defer cancel3()

	var cur string
	if e := chromedp.Run(ctx,
		network.Enable(),
		network.SetCookies(parseCookies(cookie, ".bilibili.com")),
		chromedp.Navigate("https://member.bilibili.com/platform/upload/video/frame"),
		chromedp.Sleep(6*time.Second),
		chromedp.Location(&cur),
	); e != nil {
		fmt.Println("导航失败:", e)
		os.Exit(1)
	}
	fmt.Println("✅ 页面已打开:", cur)

	// ③ 分阶段探测：立即 → 滚动后 → 再等 10s
	probe := func(stage string) {
		var report string
		_ = chromedp.Run(ctx, chromedp.Evaluate(`(() => {
			const lines = [];
			// 主文档
			const main = document.querySelector('.creation-statement-container');
			lines.push('主文档 container: ' + (main ? '✅ 命中' : '❌ 无'));
			const input = document.querySelector('input[placeholder*="创作声明"]');
			lines.push('主文档声明 input: ' + (input ? '✅ 命中' : '❌ 无'));
			// micro-app 元素
			const mapps = document.querySelectorAll('micro-app');
			lines.push('micro-app 元素: ' + mapps.length + ' 个');
			for (const m of mapps) {
				lines.push('  name=' + (m.getAttribute('name') || '?') +
					' shadowRoot=' + (m.shadowRoot ? '有' : '无') +
					' childElementCount=' + m.childElementCount);
				if (m.shadowRoot) {
					const inner = m.shadowRoot.querySelector('.creation-statement-container');
					lines.push('  shadow 内 container: ' + (inner ? '✅ 命中' : '❌ 无'));
				}
			}
			// iframe
			const iframes = document.querySelectorAll('iframe');
			lines.push('iframe: ' + iframes.length + ' 个');
			for (const f of iframes) {
				let hit = '无法访问(跨域)';
				try { hit = f.contentDocument && f.contentDocument.querySelector('.creation-statement-container') ? '✅ 命中' : '无'; } catch (e) {}
				lines.push('  iframe src=' + (f.src || '(空)').slice(0, 80) + ' 内部 container: ' + hit);
			}
			// 递归找所有 shadowRoot 中的声明 input
			const deepSearch = (root, depth) => {
				if (!root || depth > 5) return null;
				const all = root.querySelectorAll('*');
				for (const el of all) {
					if (el.shadowRoot) {
						const found = el.shadowRoot.querySelector('input[placeholder*="创作声明"], .creation-statement-container');
						if (found) return el.tagName + '(shadow)';
					}
				}
				return null;
			};
			const deep = deepSearch(document, 0);
			lines.push('递归 shadow 搜索: ' + (deep ? '✅ ' + deep : '未找到'));
			return lines.join('\n');
		})()`, &report))
		fmt.Printf("\n===== %s =====\n%s\n", stage, report)
	}

	// micro-app 内部结构（子应用渲染容器类型判断）
	var mappDump string
	_ = chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const m = document.querySelector('micro-app');
		if (!m) return '无 micro-app';
		const inner = m.innerHTML || '';
		const kids = [...m.children].map(c => c.tagName + (c.id ? '#'+c.id : '') + (c.className ? '.'+String(c.className).slice(0,40) : ''));
		return 'children: ' + kids.join(' | ') + '
innerHTML 前600: ' + inner.slice(0, 600);
	})()`, &mappDump))
	fmt.Println("===== micro-app 内部结构 =====")
	fmt.Println(mappDump)

	probe("T+6s 页面加载后立即")
	_ = chromedp.Run(ctx, chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil))
	chromedp.Sleep(2 * time.Second)
	probe("滚动到底后")
	chromedp.Sleep(10 * time.Second)
	probe("再等 10s 后")

	// ④ 若命中：试触发声明下拉并 dump 选项
	var declTest string
	_ = chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const findInput = () => {
			let el = document.querySelector('input[placeholder*="创作声明"]');
			if (el) return {el, where: '主文档'};
			for (const m of document.querySelectorAll('micro-app')) {
				if (m.shadowRoot) {
					const s = m.shadowRoot.querySelector('input[placeholder*="创作声明"]');
					if (s) return {el: s, where: 'micro-app shadow'};
				}
			}
			return {};
		};
		const {el, where} = findInput();
		if (!el) return '声明 input 仍未找到';
		el.scrollIntoView({block: 'center'});
		const sel = el.closest('.bcc-select') || el;
		sel.click(); el.click();
		return '已点击触发器（位置: ' + where + '），placeholder=' + el.placeholder;
	})()`, &declTest))
	fmt.Println("\n触发测试:", declTest)
	chromedp.Sleep(2 * time.Second)
	var opts2 string
	_ = chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const findOpts = () => {
			let list = document.querySelectorAll('.bcc-select-option-list .bcc-option span');
			if (list.length) return list;
			for (const m of document.querySelectorAll('micro-app')) {
				if (m.shadowRoot) {
					const l = m.shadowRoot.querySelectorAll('.bcc-option span');
					if (l.length) return l;
				}
			}
			return [];
		};
		const spans = findOpts();
		return '选项数=' + spans.length + ': ' + [...spans].map(s => s.textContent.trim()).join(' | ');
	})()`, &opts2))
	fmt.Println("展开选项:", opts2)

	// ⑤ 截图留证
	var shot []byte
	if e := chromedp.Run(ctx, chromedp.CaptureScreenshot(&shot)); e == nil {
		_ = os.WriteFile("data/bilidecl-probe.png", shot, 0o644)
		fmt.Println("\n截图: data/bilidecl-probe.png")
	}
}

func parseCookies(cookieStr, domain string) []*network.CookieParam {
	var out []*network.CookieParam
	for _, part := range splitSemicolon(cookieStr) {
		kv := splitEq(part)
		if len(kv) == 2 {
			out = append(out, &network.CookieParam{Name: kv[0], Value: kv[1], Domain: domain, Path: "/"})
		}
	}
	return out
}

func splitSemicolon(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ';' {
			out = append(out, trimSpace(s[start:i]))
			start = i + 1
		}
	}
	out = append(out, trimSpace(s[start:]))
	return out
}

func splitEq(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return []string{trimSpace(s[:i]), s[i+1:]}
		}
	}
	return nil
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
