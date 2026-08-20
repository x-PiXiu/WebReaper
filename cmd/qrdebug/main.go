// qrdebug：二维码登录页调试工具。
//
// 背景：抖音扫码会话提取到的 canvas（270x270）经视觉模型确认是空白图——
// findQRElementJS 只检查 dataURL.length > 100，空白 canvas 也能通过。
// 本工具用与 qrlogin 完全一致的启动参数打开登录页，输出：
//   1. 整页截图（ground truth：页面上到底渲染了什么）
//   2. 所有 canvas 的像素空白检测（min/max 亮度差）+ img 元素清单 + 页面文案
// 用法：go run ./cmd/qrdebug -url https://creator.douyin.com -out data/qrdebug_douyin.png
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/chromedp"

	"webreaper/internal/adapter/chromedputil"
	"webreaper/internal/config"
)

// dumpPageJS 导出页面二维码相关元素的完整画像。
// canvas 空白判定：抽样像素的 min/max 亮度差 < 10 视为空白（纯色）。
const dumpPageJS = `JSON.stringify((() => {
  const dumpCanvas = (c) => {
    const info = { w: c.width, h: c.height, cls: (c.className||'').toString().slice(0,60), id: c.id || '' };
    try {
      const ctx = c.getContext('2d');
      if (!ctx || c.width === 0 || c.height === 0) { info.blank = 'no-ctx'; return info; }
      const d = ctx.getImageData(0, 0, Math.min(c.width, 270), Math.min(c.height, 270)).data;
      let mn = 255, mx = 0;
      for (let i = 0; i < d.length; i += 4) {
        const lum = 0.299*d[i] + 0.587*d[i+1] + 0.114*d[i+2];
        if (lum < mn) mn = lum;
        if (lum > mx) mx = lum;
      }
      info.minLum = Math.round(mn); info.maxLum = Math.round(mx);
      info.blank = (mx - mn) < 10;
    } catch (e) { info.blank = 'tainted: ' + (e.message||'').slice(0,50); }
    return info;
  };
  const canvases = [...document.querySelectorAll('canvas')].map(dumpCanvas);
  const imgs = [...document.querySelectorAll('img')].map(i => ({
    cls: (i.className||'').toString().slice(0,40), id: i.id || '',
    src: (i.src||'').slice(0,80),
    nw: i.naturalWidth, nh: i.naturalHeight, cw: i.clientWidth, ch: i.clientHeight
  }));
  return {
    title: document.title,
    url: location.href,
    bodyText: (document.body.innerText||'').replace(/\s+/g,' ').slice(0, 600),
    canvasCount: canvases.length, canvases,
    imgCount: imgs.length, imgs: imgs.slice(0, 20)
  };
})())`

func main() {
	url := flag.String("url", "https://creator.douyin.com", "登录页 URL")
	out := flag.String("out", "data/qrdebug.png", "整页截图输出路径")
	wait := flag.Duration("wait", 6*time.Second, "页面加载后等待时长")
	flag.Parse()

	opts := chromedputil.HeadlessOptions(config.IsBrowserHeaded())
	opts = append(opts,
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"),
		chromedp.Flag("incognito", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	browserCtx, cancel2 := chromedp.NewContext(allocCtx)
	defer cancel2()

	ctx, cancel3 := context.WithTimeout(browserCtx, 60*time.Second)
	defer cancel3()

	var dump string
	var fullPNG []byte
	var canvasCount int
	fmt.Printf("正在打开 %s ...\n", *url)
	err := chromedp.Run(ctx,
		chromedp.Navigate(*url),
		chromedp.Sleep(*wait),
		chromedp.Evaluate(dumpPageJS, &dump),
		chromedp.Evaluate(`document.querySelectorAll('canvas').length`, &canvasCount),
		chromedp.FullScreenshot(&fullPNG, 90),
	)
	if err != nil {
		fmt.Printf("执行失败: %v\n", err)
		os.Exit(1)
	}

	if jErr := os.WriteFile(*out, fullPNG, 0644); jErr != nil {
		fmt.Printf("截图写入失败: %v\n", jErr)
	} else {
		fmt.Printf("整页截图已保存: %s（%d 字节）\n", *out, len(fullPNG))
	}

	// 逐个 canvas 元素截图：验证元素截图能否拿到渲染后的真实内容
	// （抖音篡改了 toDataURL/getContext，元素截图走渲染管线不受影响）
	for i := 0; i < canvasCount && i < 8; i++ {
		var pngBytes []byte
		jsPath := fmt.Sprintf(`document.querySelectorAll('canvas')[%d]`, i)
		sErr := chromedp.Run(ctx, chromedp.Screenshot(jsPath, &pngBytes, chromedp.ByJSPath))
		if sErr != nil || len(pngBytes) == 0 {
			fmt.Printf("canvas[%d] 元素截图失败: %v（%d 字节）\n", i, sErr, len(pngBytes))
			continue
		}
		f := fmt.Sprintf("%s.canvas%d.png", *out, i)
		if wErr := os.WriteFile(f, pngBytes, 0644); wErr == nil {
			fmt.Printf("canvas[%d] 元素截图已保存: %s（%d 字节）\n", i, f, len(pngBytes))
		}
	}

	var pretty map[string]any
	if err := json.Unmarshal([]byte(dump), &pretty); err == nil {
		b, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(b))
	} else {
		fmt.Println(dump)
	}
}
