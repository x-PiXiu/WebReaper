package kuaishou

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/chromedp"
)

// 快手 __NS_hxfalcon 签名实现（参考 MediaCrawler kuaishou/help.py）。
//
// 签名算法在快手页面的混淆 JS 中，无法逆向，必须在浏览器内执行。
// 流程：
//  1. 注入捕获脚本，监听 Object.prototype.caver 的 setter
//  2. 快手自己的 JS 初始化时会设置 caver，脚本捕获到 window.__ks_realm
//  3. 调用 window.__ks_realm.$encode() 生成签名

// KS_SIGN_CAPTURE_SCRIPT 捕获脚本（参考 MediaCrawler kuaishou/help.py 第 29-46 行）。
const KS_SIGN_CAPTURE_SCRIPT = `
(() => {
    if (window.__ks_realm) return;
    let done = false;
    const setter = function(v) {
        if (!done && this && typeof this === "object" && this !== window &&
            typeof this.$encode === "function" &&
            typeof this.$getCatVersion === "function") {
            done = true;
            window.__ks_realm = this;
            try { delete Object.prototype.caver; } catch (e) {}
        }
        Object.defineProperty(this, "caver", {
            value: v, writable: true, enumerable: true, configurable: true,
        });
    };
    try {
        Object.defineProperty(Object.prototype, "caver", { set: setter, configurable: true });
    } catch (e) {}
})();
`

// KuaishouSigner 快手签名器。
type KuaishouSigner struct {
	browserCtx context.Context
	ready      bool
}

// NewKuaishouSigner 创建快手签名器。
func NewKuaishouSigner(browserCtx context.Context) *KuaishouSigner {
	return &KuaishouSigner{
		browserCtx: browserCtx,
	}
}

// Init 初始化签名环境（注入捕获脚本 + 导航到快手页面）。
func (s *KuaishouSigner) Init(ctx context.Context) error {
	// 1. 注入捕获脚本
	if err := chromedp.Run(s.browserCtx, chromedp.Evaluate(KS_SIGN_CAPTURE_SCRIPT, nil)); err != nil {
		return fmt.Errorf("注入捕获脚本失败: %w", err)
	}

	// 2. 导航到快手页面（触发 JS 初始化）
	if err := chromedp.Run(s.browserCtx, chromedp.Navigate("https://www.kuaishou.com")); err != nil {
		return fmt.Errorf("导航到快手页面失败: %w", err)
	}

	// 3. 等待页面加载
	time.Sleep(3 * time.Second)

	// 4. 等待 __ks_realm 初始化
	if err := s.waitForRealm(ctx, 15*time.Second); err != nil {
		return fmt.Errorf("等待 __ks_realm 初始化失败: %w", err)
	}

	s.ready = true
	log.Printf("[kuaishou] 签名环境初始化成功")
	return nil
}

// waitForRealm 等待 window.__ks_realm 初始化。
func (s *KuaishouSigner) waitForRealm(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var exists bool
		err := chromedp.Run(s.browserCtx, chromedp.Evaluate(`() => !!window.__ks_realm`, &exists))
		if err == nil && exists {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("超时：window.__ks_realm 未初始化")
}

// Sign 调用页面内的签名函数生成 __NS_hxfalcon。
func (s *KuaishouSigner) Sign(ctx context.Context, url string, body map[string]any) (string, error) {
	if !s.ready {
		return "", fmt.Errorf("签名器未初始化，请先调用 Init()")
	}

	// 构造签名请求
	bodyJSON, _ := json.Marshal(body)
	signScript := fmt.Sprintf(`
		new Promise((resolve, reject) => {
			if (!window.__ks_realm) {
				reject(new Error("__ks_realm not found"));
				return;
			}
			window.__ks_realm.$encode([
				{ url: %q, query: {caver: 2}, form: {}, requestBody: %s },
				{ suc: s => resolve(s), err: e => reject(String(e)) }
			]);
		})
	`, url, string(bodyJSON))

	var signature string
	if err := chromedp.Run(s.browserCtx, chromedp.Evaluate(signScript, &signature)); err != nil {
		return "", fmt.Errorf("签名失败: %w", err)
	}

	if signature == "" {
		return "", fmt.Errorf("签名结果为空")
	}

	return signature, nil
}

// IsReady 检查签名器是否就绪。
func (s *KuaishouSigner) IsReady() bool {
	return s.ready
}
