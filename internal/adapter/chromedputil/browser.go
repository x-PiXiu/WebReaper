// Package chromedputil 提供 chromedp 浏览器启动参数的公共工厂。
//
// 设计（整洁架构）：
//   - 位于 adapter 层内部共享——qrlogin（扫码登录）与 publisher（RPA 发布）
//     都启动浏览器，参数必须一致，避免两份代码各自维护导致漂移
//     （本次事故：两边都用旧版 --headless 语法，容器内新版 Chromium 崩溃）
//   - 业务性参数（窗口尺寸/UA/incognito 等）由各调用方自管；
//     本包只负责「环境安全参数」：容器适配 + headless 决策
package chromedputil

import (
	"os"
	"runtime"

	"github.com/chromedp/chromedp"
)

// HeadlessOptions 返回浏览器启动参数（容器安全集 + headless 决策）。
//
// 关键决策（踩坑记录）：
//  1. headless 必须用新版语法 --headless=new：
//     Chromium 132+ 已移除旧 --headless（chromedp.Headless 生成的旧参数会被
//     静默忽略 → 浏览器以普通模式启动 → 无 X 容器报
//     "Missing X server or $DISPLAY" → platform failed to initialize）
//     --headless=new 同时兼容 Chromium 112-131（新 headless 自 112 引入）。
//  2. no-sandbox：容器内以 root 运行 Chromium 必须，否则 sandbox 初始化失败。
//  3. disable-dev-shm-usage：容器 /dev/shm 默认仅 64MB，Chromium 渲染常崩
//     （"DevTools listening ... Render process gone"）。
//  4. disable-gpu：无 GPU 容器避免 GPU 进程初始化报错。
//  5. headed 模式在 Linux 无 DISPLAY 环境（容器/无头服务器）自动降级 headless——
//     防止管理后台误开"显示窗口"导致全部 RPA 直接崩溃。
func HeadlessOptions(headed bool) []chromedp.ExecAllocatorOption {
	// Linux 无 DISPLAY = 容器/无头服务器——headed 请求自动降级
	// （Windows/macOS 桌面开发不受影响，桌面有 GUI 无 DISPLAY 变量）
	if headed && runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" {
		headed = false
	}

	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	}
	if headed {
		opts = append(opts, chromedp.Flag("headless", false))
	} else {
		opts = append(opts,
			chromedp.Flag("headless", "new"), // 新版 headless（Chromium 132+ 只认这个）
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("hide-scrollbars", true),
			chromedp.Flag("mute-audio", true),
		)
	}
	return opts
}
