package qrlogin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"webreaper/internal/usecase/port"
)

// ChromedpQRLogin 用 chromedp 浏览器实现扫码登录（port.QRLoginSession）。
//
// 异步设计（核心）：
//   StartLogin 只负责「开浏览器 + 导航」，立即返回 sessionID（不等二维码）。
//   后台 goroutine 负责「等二维码出现 → 截图 → 检测登录成功」。
//   前端通过 PollStatus 轮询拿到二维码图片和登录状态。
//
// 重要：chromedp 的 context 生命周期管理是这个模块最易出错的地方。
//   - NewExecAllocator → NewContext → 首次 Run 时才真正启动浏览器进程
//   - 后台 goroutine 必须用「独立的长生命周期 context」，不能被 HTTP 请求的 context 干扰
//   - 每个 chromedp.Run 调用都应有独立超时，避免单个操作卡死拖垮整个会话
type ChromedpQRLogin struct {
	mu       sync.Mutex
	sessions map[string]*loginSession
	headed   bool
}

// loginSession 一次扫码登录的会话状态。
type loginSession struct {
	cancel      context.CancelFunc
	closed      bool
	status      string
	qrImage     string
	cookie      string
	expiresAt   time.Time
	accountName string
	errMsg      string
	platform    string
	method      string
	created     time.Time
}

var _ port.QRLoginSession = (*ChromedpQRLogin)(nil)

func NewChromedpQRLogin(headed bool) *ChromedpQRLogin {
	return &ChromedpQRLogin{sessions: make(map[string]*loginSession), headed: headed}
}

// platformConfig 平台登录页配置。
type platformConfig struct {
	LoginURL     string            // 登录页地址
	TabText      string            // 扫码登录按钮的文本（用于通过文字内容点击）
	AuthCookies  []string          // 认证 Cookie 名——出现任一即判定登录成功
	LoginMethods map[string]string // 登录方式 → 第三方按钮 SVG class 片段（如 wechat→ZDI--Wechat24）
}

var platformConfigs = map[string]platformConfig{
	"zhihu": {
		LoginURL:    "https://www.zhihu.com/signin",
		TabText:     "扫码登录",
		AuthCookies: []string{"z_c0"},
		// 知乎支持多种登录方式：默认知乎App扫码，也可选微信/QQ/微博
		// 按钮通过 SVG class 区分：ZDI--Wechat24 / ZDI--Qq24 / ZDI--Weibo24
		LoginMethods: map[string]string{
			"zhihu":   "",               // 默认知乎App扫码（无需点第三方按钮）
			"wechat":  "ZDI--Wechat24",  // 微信登录
			"qq":      "ZDI--Qq24",      // QQ登录
			"weibo":   "ZDI--Weibo24",   // 微博登录
		},
	},
	"xiaohongshu": {
		LoginURL:    "https://www.xiaohongshu.com",
		TabText:     "",
		AuthCookies: []string{"id_token"},
		LoginMethods: map[string]string{
			"xiaohongshu": "", // 小红书只有自身扫码登录
		},
	},
}

// findQRElementJS 在页面内动态查找二维码并直接提取图片内容。
// 核心策略（绕过截图的 0x0 尺寸问题）：
//   - canvas：用 canvas.width/canvas.height（画布像素尺寸，非 CSS 尺寸）判断 + toDataURL() 直接导出图片
//   - img：用 naturalWidth/naturalHeight（图片原始尺寸）判断 + src 提取图片 URL
// 这样不依赖 getBoundingClientRect/offsetWidth（知乎/小红书的二维码元素 CSS 尺寸为 0）。
// 参考 MediaCrawler 的实现：用 wait_for_selector + element.screenshot() 获取二维码。
// findQRElementJS 在页面内动态查找二维码并提取图片。
// method 参数：如果是第三方登录（wechat/qq/weibo），跳过知乎自身 canvas，只查 img。
const findQRElementJS = `((method) => {
  // 默认登录方式（知乎App/小红书）：优先查 canvas
  if (method === 'zhihu' || method === 'xiaohongshu' || method === '' ) {
    const canvases = document.querySelectorAll('canvas');
    let canvasIdx = 0;
    for (const c of canvases) {
      const w = c.width, h = c.height;
      if (w >= 60 && w <= 500 && h >= 60 && h <= 500) {
        const ratio = w / Math.max(h, 1);
        if (ratio > 0.85 && ratio < 1.15) {
          try {
            const dataURL = c.toDataURL('image/png');
            if (dataURL && dataURL.length > 100) {
              return JSON.stringify({ found: true, type: 'canvas-dataurl', width: w, height: h, className: (c.className||'').toString().slice(0,80), dataURL: dataURL });
            }
          } catch(e) {
            c.setAttribute('data-qr-shot', canvasIdx);
            return JSON.stringify({ found: true, type: 'canvas-screenshot', width: w, height: h, className: (c.className||'').toString().slice(0,80), jsPath: 'document.querySelector(\'canvas[data-qr-shot=\"' + canvasIdx + '\"]\')' });
          }
        }
      }
      canvasIdx++;
    }
  }

  // 查找 img（小红书/微信/QQ/微博都用 img 渲染二维码）
  // QQ 登录页的二维码可能在 iframe 里，需要搜索 iframe 内容
  const allImgs = [...document.querySelectorAll('img')];
  // 搜索同源 iframe 内的 img
  let iframeCount = 0;
  try {
    const iframes = document.querySelectorAll('iframe');
    iframeCount = iframes.length;
    for (const iframe of iframes) {
      try {
        const iframeImgs = iframe.contentDocument.querySelectorAll('img');
        allImgs.push(...iframeImgs);
      } catch(e) {} // 跨域 iframe 会抛异常，忽略
    }
  } catch(e) {}

  // 调试日志：输出页面上的 img 和 iframe 信息
  const imgDebug = allImgs.map(img => ({
    cls: (img.className||'').toString().slice(0,40),
    id: img.id,
    src: (img.src||'').slice(0,60),
    w: img.naturalWidth || img.width,
    h: img.naturalHeight || img.height,
    display: img.style.display
  }));

  for (const img of allImgs) {
    const cls = (img.className || '').toString().toLowerCase();
    if (cls.includes('logo') || cls.includes('icon') || cls.includes('avatar')) continue;
    if (img.style.display === 'none') continue;

    const src = img.src;
    if (!src) continue;
    const isImgSrc = src.startsWith('data:') || src.startsWith('http') || src.startsWith('/');
    if (!isImgSrc) continue;

    let w = img.naturalWidth || img.width;
    let h = img.naturalHeight || img.height;
    // 检查 img 自身 class 和父容器 class 是否含 qr/code
    // 微博的 img 没有 class，但父容器 div class="qr-code" 有
    const parentCls = (img.parentElement && img.parentElement.className || '').toString().toLowerCase();
    const hasQRClass = cls.includes('qr') || cls.includes('code') || cls.includes('qrcode') || cls.includes('qrimg')
      || parentCls.includes('qr') || parentCls.includes('qrcode') || parentCls.includes('qr-code');

    // class 含 qr 的图片直接返回（QQ 的 qrImg、微信的 qrcode_img、微博父容器 qr-code）
    if (hasQRClass && isImgSrc) {
      return JSON.stringify({ found: true, type: 'img', width: w, height: h, className: (img.className||parentCls||'').toString().slice(0,80), dataURL: src });
    }

    if (w >= 80 && w <= 500 && h >= 80 && h <= 500) {
      const ratio = w / Math.max(h, 1);
      if (ratio > 0.85 && ratio < 1.15) {
        return JSON.stringify({ found: true, type: 'img', width: w, height: h, className: (img.className||'').toString().slice(0,80), dataURL: src });
      }
    } else if (hasQRClass && w === 0) {
      const idx = 'qri' + Math.random().toString(36).slice(2,8);
      img.setAttribute('data-qr-shot', idx);
      return JSON.stringify({ found: true, type: 'img-screenshot', width: 0, height: 0, className: (img.className||'').toString().slice(0,80), jsPath: 'document.querySelector(\'img[data-qr-shot=\"' + idx + '\"]\')' });
    }
  }

  return JSON.stringify({ found: false, debug: { imgCount: allImgs.length, iframeCount: iframeCount, imgs: imgDebug } });
})`

// clickTabByTextJS 通过文本内容点击扫码登录按钮的 JavaScript。
const clickTabByTextJS = `(text) => {
  const els = document.querySelectorAll('a, button, span, div, li');
  for (const el of els) {
    if (el.textContent && el.textContent.includes(text) && el.offsetParent !== null) {
      el.click();
      return true;
    }
  }
  return false;
}`

func (q *ChromedpQRLogin) allocOpts() []chromedp.ExecAllocatorOption {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.WindowSize(1280, 800),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"),
		// 每次用独立的临时用户数据目录，确保不残留上次登录的 cookie/session
		// （否则上次扫码登录的状态会被新会话继承，导致"未扫码就显示已登录"）
		chromedp.Flag("incognito", true),
	}
	if q.headed {
		opts = append(opts, chromedp.Flag("headless", false))
	} else {
		opts = append(opts, chromedp.Headless)
	}
	return opts
}

// StartLogin 启动浏览器并打开平台登录页，立即返回会话 ID。
// method 指定登录方式（zhihu/wechat/qq/weibo），空=平台默认扫码。
func (q *ChromedpQRLogin) StartLogin(_ context.Context, platform, method string) (sessionID string, err error) {
	pc, ok := platformConfigs[platform]
	if !ok {
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}
	if method == "" {
		for k := range pc.LoginMethods {
			method = k
			break
		}
	}

	log.Printf("[QRLogin] StartLogin 开始：platform=%s method=%s", platform, method)

	// 创建独立的浏览器 context（不继承 HTTP 请求的 context）
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), q.allocOpts()...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	// ⚠️ chromedp 关键陷阱：首次 chromedp.Run 会把浏览器 Tab 和 context 绑定。
	// 如果用临时 context（如 navCtx）做首次 Run，cancel 后 Tab 会被关闭，
	// 后台 goroutine 再用同一个 context 树的所有操作都会报 "context canceled"。
	// 正确做法：首次导航和后续后台操作共用同一个长生命周期 context。
	sessionCtx, sessionCancel := context.WithTimeout(browserCtx, 5*time.Minute)

	// 同步完成首次 Run（启动 Chrome 进程 + 导航到登录页）
	// 首次 Run 会真正启动浏览器，耗时较长（Chrome 冷启动），不能套短超时。
	err = chromedp.Run(sessionCtx,
		chromedp.Navigate(pc.LoginURL),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		sessionCancel()
		browserCancel()
		allocCancel()
		log.Printf("[QRLogin] StartLogin 失败：浏览器启动/导航错误: %v", err)
		return "", fmt.Errorf("启动浏览器失败（检查 Chrome 是否安装）: %w", err)
	}
	log.Printf("[QRLogin] StartLogin 成功：浏览器已启动并导航到 %s", pc.LoginURL)

	sessionID = fmt.Sprintf("qr-%d", time.Now().UnixNano())
	sess := &loginSession{
		status:   "preparing",
		platform: platform,
		method:   method,
		created:  time.Now(),
	}
	sess.cancel = func() {
		sess.closed = true
		sessionCancel()
		browserCancel()
		allocCancel()
	}

	q.mu.Lock()
	q.sessions[sessionID] = sess
	q.mu.Unlock()

	go q.runSession(sessionCtx, sessionCancel, sessionID, pc, method)

	log.Printf("[QRLogin] 会话已创建：sessionID=%s，后台 goroutine 已启动", sessionID)
	return sessionID, nil
}

// runSession 后台管理扫码登录会话的完整生命周期。
func (q *ChromedpQRLogin) runSession(ctx context.Context, sessionCancel context.CancelFunc, sessionID string, pc platformConfig, method string) {
	defer sessionCancel()

	log.Printf("[QRLogin:%s] runSession 启动", sessionID)

	// 阶段一：截图二维码
	q.captureQRCode(ctx, sessionID, pc, method)

	// 检查会话是否仍然有效（截图阶段可能已标记 error 或被关闭）
	q.mu.Lock()
	sess := q.sessions[sessionID]
	var status string
	var closed bool
	if sess != nil {
		status = sess.status
		closed = sess.closed
	} else {
		status = "error"
	}
	q.mu.Unlock()
	if sess == nil || closed || status == "error" {
		log.Printf("[QRLogin:%s] runSession 提前结束（status=%s, closed=%v, nil=%v）", sessionID, status, closed, sess == nil)
		return
	}

	// 阶段二：检测登录成功
	log.Printf("[QRLogin:%s] 进入登录态检测阶段", sessionID)
	q.pollLoginSuccess(ctx, sessionID, pc)
}

// captureQRCode 尝试提取二维码图片。
// method 指定登录方式：如果非默认方式（如 wechat/qq/weibo），先点击对应第三方登录按钮。
func (q *ChromedpQRLogin) captureQRCode(ctx context.Context, sessionID string, pc platformConfig, method string) {
	// 第三方登录：点击按钮后会弹出新窗口，需要切换到新窗口检测二维码
	if method != "" && pc.LoginMethods != nil {
		svgClass, isThirdParty := pc.LoginMethods[method]
		if isThirdParty && svgClass != "" {
			log.Printf("[QRLogin:%s] 点击第三方登录按钮: %s", sessionID, svgClass)

			// 点击第三方按钮
			clickCtx, clickCancel := context.WithTimeout(ctx, 5*time.Second)
			err := chromedp.Run(clickCtx,
				chromedp.Click("."+svgClass, chromedp.ByQuery),
			)
			clickCancel()
			if err != nil {
				log.Printf("[QRLogin:%s] 点击第三方按钮失败: %v", sessionID, err)
				q.captureQRFromPage(ctx, sessionID, method, pc)
				return
			}
			log.Printf("[QRLogin:%s] 第三方登录按钮已点击，等待新窗口...", sessionID)

			// 等 3 秒让新窗口创建
			time.Sleep(3 * time.Second)

			// 查找所有 target，获取弹出窗口的 URL
			targets, err := chromedp.Targets(ctx)
			if err != nil {
				log.Printf("[QRLogin:%s] 获取 targets 失败: %v", sessionID, err)
				q.captureQRFromPage(ctx, sessionID, method, pc)
				return
			}

			// 找弹出窗口的 URL
			var popupURL string
			for _, t := range targets {
				if t.Type == "page" && t.URL != "" && t.URL != "about:blank" {
					if !strings.Contains(t.URL, "zhihu.com/signin") && !strings.Contains(t.URL, "xiaohongshu.com") {
						popupURL = t.URL
						log.Printf("[QRLogin:%s] 发现弹出窗口: %s", sessionID, popupURL)
						break
					}
				}
			}

			if popupURL == "" {
				log.Printf("[QRLogin:%s] 未发现弹出窗口，在原页面检测", sessionID)
				q.captureQRFromPage(ctx, sessionID, method, pc)
				return
			}

			// 不切换 target（chromedp 的 target 切换不稳定），
			// 而是在原页面直接导航到弹出窗口的 URL，这样在同一个 target 里操作。
			log.Printf("[QRLogin:%s] 原页面导航到第三方登录页: %s", sessionID, popupURL)
			navCtx, navCancel := context.WithTimeout(ctx, 20*time.Second)
			err = chromedp.Run(navCtx,
				chromedp.Navigate(popupURL),
				chromedp.Sleep(3*time.Second), // 等第三方页面加载二维码
			)
			navCancel()
			if err != nil {
				log.Printf("[QRLogin:%s] 导航到第三方登录页失败: %v", sessionID, err)
				q.captureQRFromPage(ctx, sessionID, method, pc)
				return
			}

			// 在原页面（已导航到第三方登录页）提取二维码
			q.captureQRFromPage(ctx, sessionID, method, pc)
			return
		}
	}

	// 默认登录方式：在原页面检测二维码
	q.captureQRFromPage(ctx, sessionID, method, pc)
}


// processQRDetection 处理 JS 检测结果，返回是否成功提取二维码。
func (q *ChromedpQRLogin) processQRDetection(ctx context.Context, sessionID string, det *qrDetection) bool {
	log.Printf("[QRLogin:%s] 处理二维码: type=%s class=%s %vx%v dataURL长度=%d jsPath=%s",
		sessionID, det.Type, det.Class, det.Width, det.Height, len(det.DataURL), det.JSPath)

	if det.Type == "canvas-screenshot" || det.Type == "img-screenshot" {
		shotCtx, shotCancel := context.WithTimeout(ctx, 8*time.Second)
		var qrBytes []byte
		shotErr := chromedp.Run(shotCtx,
			chromedp.Screenshot(det.JSPath, &qrBytes, chromedp.ByJSPath),
		)
		shotCancel()
		if shotErr == nil && len(qrBytes) > 500 {
			log.Printf("[QRLogin:%s] 元素截图成功（%d 字节）", sessionID, len(qrBytes))
			q.setSessionQRImage(sessionID, base64.StdEncoding.EncodeToString(qrBytes))
			return true
		}
		log.Printf("[QRLogin:%s] 元素截图失败: %v (bytes=%d)", sessionID, shotErr, len(qrBytes))
		return false
	}

	qrBase64 := det.DataURL
	if strings.HasPrefix(qrBase64, "data:image/") {
		// data URL：去掉前缀，只保留 base64
		if idx := strings.Index(qrBase64, ","); idx > 0 {
			qrBase64 = qrBase64[idx+1:]
		}
		if len(qrBase64) > 100 {
			log.Printf("[QRLogin:%s] 二维码提取成功（data URL, %d 字符）", sessionID, len(qrBase64))
			q.setSessionQRImage(sessionID, qrBase64)
			return true
		}
		log.Printf("[QRLogin:%s] data URL 太短，可能无效", sessionID)
		return false
	}

	// http/https URL 或相对URL：直接作为图片 URL 返回给前端
	// 浏览器会自动把 <img src="/connect/qrcode/..."> 补全为完整 URL
	if strings.HasPrefix(qrBase64, "http") || strings.HasPrefix(qrBase64, "/") {
		// 如果是相对URL，补全为完整 URL
		if strings.HasPrefix(qrBase64, "/") {
			originCtx, originCancel := context.WithTimeout(ctx, 3*time.Second)
			var origin string
			_ = chromedp.Run(originCtx, chromedp.Location(&origin))
			originCancel()
			if origin != "" {
				qrBase64 = strings.TrimRight(origin, "/") + qrBase64
			}
		}
		log.Printf("[QRLogin:%s] 二维码提取成功（URL: %s）", sessionID, qrBase64)
		q.setSessionQRImage(sessionID, qrBase64)
		return true
	}

	log.Printf("[QRLogin:%s] dataURL 格式未知，可能无效: %s", sessionID, qrBase64[:min(50, len(qrBase64))])
	return false
}

// captureQRFromPage 在原页面检测二维码（知乎App/小红书默认方式）
func (q *ChromedpQRLogin) captureQRFromPage(ctx context.Context, sessionID, method string, pc platformConfig) {
	// 切换到扫码 tab（仅默认登录方式，第三方登录不需要点这个）
	if pc.TabText != "" && (method == "" || method == "zhihu" || method == "xiaohongshu") {
		log.Printf("[QRLogin:%s] 尝试点击扫码 tab（文本=%q）", sessionID, pc.TabText)
		clickCtx, clickCancel := context.WithTimeout(ctx, 10*time.Second)
		err := chromedp.Run(clickCtx,
			chromedp.ActionFunc(func(ctx context.Context) error {
				return chromedp.Evaluate(fmt.Sprintf(`(%s)("%s")`, clickTabByTextJS, pc.TabText), nil).Do(ctx)
			}),
			chromedp.Sleep(2*time.Second),
		)
		clickCancel()
		if err != nil {
			log.Printf("[QRLogin:%s] 点击扫码 tab 失败（不阻断）: %v", sessionID, err)
		} else {
			log.Printf("[QRLogin:%s] 扫码 tab 已点击", sessionID)
		}
	}

	// 用 JS 直接提取二维码图片（3 轮，每轮等 2s 让页面渲染）
	for attempt := 1; attempt <= 3; attempt++ {
		if q.isSessionClosed(sessionID) {
			return
		}
		log.Printf("[QRLogin:%s] 二维码提取尝试 %d/3", sessionID, attempt)

		detectCtx, detectCancel := context.WithTimeout(ctx, 8*time.Second)
		var resultJSON string
		err := chromedp.Run(detectCtx,
			chromedp.Sleep(2*time.Second),
			chromedp.Evaluate(fmt.Sprintf(`%s("%s")`, findQRElementJS, method), &resultJSON),
		)
		detectCancel()
		if err != nil {
			log.Printf("[QRLogin:%s] 提取 JS 执行失败: %v", sessionID, err)
			continue
		}

		// 解析 JS 返回的结果
		det, parseErr := parseQRDetection(resultJSON)
		if parseErr != nil {
			log.Printf("[QRLogin:%s] 解析结果失败: %v (raw=%s)", sessionID, parseErr, resultJSON)
			continue
		}
		if !det.Found {
			log.Printf("[QRLogin:%s] 提取尝试 %d：未找到二维码 (raw=%s)", sessionID, attempt, resultJSON)

			// 主文档没找到 → 尝试在 iframe 里搜索（QQ 二维码在跨域 iframe 里）
			// 用 chromedp.Targets 获取所有 frame target，在 iframe 里执行 JS
			if attempt == 1 {
				if q.searchIframesForQR(ctx, sessionID, method) {
					return
				}
			}
			continue
		}

		if q.processQRDetection(ctx, sessionID, det) {
			return
		}
	}

	// 降级：整页截图
	log.Printf("[QRLogin:%s] JS 提取全部失败，降级为整页截图", sessionID)
	if q.isSessionClosed(sessionID) {
		return
	}
	fullCtx, fullCancel := context.WithTimeout(ctx, 10*time.Second)
	var fullBytes []byte
	err := chromedp.Run(fullCtx, chromedp.CaptureScreenshot(&fullBytes))
	fullCancel()
	if err != nil {
		log.Printf("[QRLogin:%s] 整页截图失败: %v", sessionID, err)
		q.setSessionError(sessionID, fmt.Sprintf("截图失败: %v", err))
		return
	}
	if len(fullBytes) > 100 {
		log.Printf("[QRLogin:%s] 整页截图成功（%d 字节）", sessionID, len(fullBytes))
		q.setSessionQRImage(sessionID, base64.StdEncoding.EncodeToString(fullBytes))
	} else {
		log.Printf("[QRLogin:%s] 整页截图为空", sessionID)
		q.setSessionError(sessionID, "页面可能未正确加载")
	}
}

// qrDetection JS 检测到的二维码信息。
type qrDetection struct {
	Found   bool    `json:"found"`
	Type    string  `json:"type"`    // canvas-dataurl / canvas-screenshot / img
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
	Class   string  `json:"className"`
	DataURL string  `json:"dataURL"` // base64 图片或 http URL（canvas-dataurl/img 类型有值）
	JSPath  string  `json:"jsPath"`  // canvas 截图路径（canvas-screenshot 类型有值）
}

// parseQRDetection 解析 JS 返回的二维码检测结果。
func parseQRDetection(jsonStr string) (*qrDetection, error) {
	var det qrDetection
	if err := json.Unmarshal([]byte(jsonStr), &det); err != nil {
		return nil, err
	}
	return &det, nil
}

// setSessionQRImage 截图成功后设置图片，状态从 preparing 改为 waiting。
func (q *ChromedpQRLogin) setSessionQRImage(sessionID, qrBase64 string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if sess, ok := q.sessions[sessionID]; ok && (sess.status == "preparing" || sess.status == "error") {
		sess.qrImage = qrBase64
		sess.status = "waiting"
		sess.errMsg = ""
	}
}

// setSessionError 设置错误状态。
func (q *ChromedpQRLogin) setSessionError(sessionID, errMsg string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if sess, ok := q.sessions[sessionID]; ok {
		sess.status = "error"
		sess.errMsg = errMsg
	}
}

// isSessionClosed 检查会话是否已被取消。
func (q *ChromedpQRLogin) isSessionClosed(sessionID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	sess, ok := q.sessions[sessionID]
	if !ok {
		return true
	}
	return sess.closed
}

// pollLoginSuccess 后台检测登录是否成功。
func (q *ChromedpQRLogin) pollLoginSuccess(ctx context.Context, sessionID string, pc platformConfig) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			q.mu.Lock()
			sess := q.sessions[sessionID]
			isClosed := sess != nil && sess.closed
			currentStatus := ""
			if sess != nil {
				currentStatus = sess.status
			}
			q.mu.Unlock()
			if isClosed || currentStatus == "success" {
				log.Printf("[QRLogin:%s] context 结束（主动关闭或已成功），不标记过期", sessionID)
				return
			}
			log.Printf("[QRLogin:%s] 超时未登录，标记过期", sessionID)
			q.setSessionStatus(sessionID, "expired", "")
			return
		case <-ticker.C:
			q.mu.Lock()
			sess := q.sessions[sessionID]
			q.mu.Unlock()
			if sess == nil || sess.closed {
				return
			}
			// preparing / waiting 状态都检测登录（preparing 阶段用户也可能手动扫码了）
			if sess.status != "waiting" && sess.status != "preparing" {
				return
			}

			checkCtx, checkCancel := context.WithTimeout(ctx, 8*time.Second)
			loggedIn, cookie, expiresAt, err := q.checkAuthCookies(checkCtx, pc.AuthCookies)
			checkCancel()
			if err != nil {
				continue
			}
			if loggedIn {
				log.Printf("[QRLogin:%s] 检测到认证 Cookie，登录成功！过期时间: %s", sessionID, expiresAt.Format("2006-01-02 15:04"))
				// 先设置 success 状态（前端立即拿到登录成功）
				q.setSessionSuccess(sessionID, cookie, expiresAt, "")
				// 同步提取账号名——直接用 checkAuthCookies 返回的 cookie 字符串调 API
				accountName := q.extractAccountName(sess.platform, sess.method, cookie)
				q.mu.Lock()
				if s, ok := q.sessions[sessionID]; ok && s.status == "success" {
					s.accountName = accountName
				}
				q.mu.Unlock()
				return
			}
		}
	}
}

// checkAuthCookies 检查浏览器是否已设置认证 Cookie。
// ⚠️ 关键：必须先调 network.Enable() 激活 Network 域，否则 GetCookies 返回空列表。
// 返回：(是否登录, 完整cookie字符串, 认证cookie过期时间, 错误)
func (q *ChromedpQRLogin) checkAuthCookies(ctx context.Context, authCookieNames []string) (bool, string, time.Time, error) {
	var cookies []*network.Cookie
	err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	)
	if err != nil {
		return false, "", time.Time{}, err
	}

	cookieMap := make(map[string]string, len(cookies))
	expiresMap := make(map[string]float64, len(cookies))
	var parts []string
	var allNames []string
	for _, c := range cookies {
		cookieMap[c.Name] = c.Value
		expiresMap[c.Name] = c.Expires
		parts = append(parts, c.Name+"="+c.Value)
		allNames = append(allNames, c.Name)
	}

	log.Printf("[QRLogin] 当前 cookie（%d 个）: %s", len(cookies), strings.Join(allNames, ", "))

	for _, name := range authCookieNames {
		if v, ok := cookieMap[name]; ok && v != "" {
			// ⚠️ 值长度校验：访客 session 的 cookie 值通常很短（<50 字符），
			// 真正登录后的认证 cookie 值很长（如 z_c0 是 100+ 字符的 JWT token，
			// web_session 登录后 100+ 字符，访客 session 约 38 字符）。
			// 用这个区分"访客 cookie"和"登录 cookie"。
			if len(v) < 50 {
				log.Printf("[QRLogin] cookie %s 存在但值太短（%d 字符），可能是访客 session，跳过", name, len(v))
				continue
			}
			log.Printf("[QRLogin] cookie %s 值长度 %d，判定为登录态", name, len(v))
			// 提取该认证 cookie 的过期时间
			// Expires 是 Unix 时间戳（秒），-1 表示 session cookie（浏览器关闭即过期）
			var expiresAt time.Time
			exp := expiresMap[name]
			if exp > 0 {
				expiresAt = time.Unix(int64(exp), 0)
			} else {
				// session cookie，默认 24 小时后过期
				expiresAt = time.Now().Add(24 * time.Hour)
			}
			return true, strings.Join(parts, "; "), expiresAt, nil
		}
	}
	return false, "", time.Time{}, nil
}

// extractAccountName 登录成功后获取账号显示名。
// 知乎：用 cookie 直接调用 api.zhihu.com/people/self 获取用户信息。
// 小红书：暂用默认名（后续可用小红书 API 获取用户名）。
// cookieStr 是 checkAuthCookies 返回的完整 cookie 字符串。
func (q *ChromedpQRLogin) extractAccountName(platform, method, cookieStr string) string {
	var name string

	switch platform {
	case "zhihu":
		// 直接用 cookie 字符串调用知乎 API
		apiCtx, apiCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer apiCancel()

		req, _ := http.NewRequestWithContext(apiCtx, "GET", "https://api.zhihu.com/people/self", nil)
		req.Header.Set("Cookie", cookieStr)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("[QRLogin] 知乎 API 请求失败: %v", err)
		} else {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			log.Printf("[QRLogin] 知乎 API 响应状态: %d", resp.StatusCode)
			log.Printf("[QRLogin] 知乎 API 响应内容: %s", string(body))

			var userInfo struct {
				Name     string `json:"name"`
				Headline string `json:"headline"`
				ID       string `json:"id"`
				URLToken string `json:"url_token"`
			}
			if json.Unmarshal(body, &userInfo) == nil && userInfo.Name != "" {
				name = userInfo.Name
				log.Printf("[QRLogin] 知乎用户: name=%s headline=%s id=%s url_token=%s",
					userInfo.Name, userInfo.Headline, userInfo.ID, userInfo.URLToken)
			}
		}

	case "xiaohongshu":
		// 小红书暂用默认名（后续可用小红书 API 获取用户名）
		name = ""
	}

	// 生成显示名：包含登录方式标识
	methodLabel := methodDisplayName(platform, method)
	if name == "" {
		name = methodLabel
	} else {
		name = methodLabel + " " + name
	}
	log.Printf("[QRLogin] 账号名提取完成: %s", name)
	return name
}

// methodDisplayName 生成登录方式标识（如"知乎-微信"、"知乎-App"、"小红书"）。
func methodDisplayName(platform, method string) string {
	platformNames := map[string]string{
		"zhihu":       "知乎",
		"xiaohongshu": "小红书",
	}
	platformName := platformNames[platform]
	if platformName == "" {
		platformName = platform
	}
	if method == "" || method == platform {
		return platformName
	}
	methodNames := map[string]string{
		"wechat": "微信",
		"qq":     "QQ",
		"weibo":  "微博",
		"zhihu":  "App",
	}
	if mn, ok := methodNames[method]; ok {
		return platformName + "-" + mn
	}
	return platformName
}

// setSessionSuccess 设置登录成功状态及附带信息。
func (q *ChromedpQRLogin) setSessionSuccess(sessionID, cookie string, expiresAt time.Time, accountName string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if sess, ok := q.sessions[sessionID]; ok {
		sess.status = "success"
		sess.cookie = cookie
		sess.expiresAt = expiresAt
		sess.accountName = accountName
	}
}

func (q *ChromedpQRLogin) setSessionStatus(sessionID, status, cookie string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if sess, ok := q.sessions[sessionID]; ok {
		sess.status = status
		if cookie != "" {
			sess.cookie = cookie
		}
	}
}

// PollStatus 前端轮询登录状态和二维码图片。
func (q *ChromedpQRLogin) PollStatus(_ context.Context, sessionID string) (port.QRLoginResult, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	sess, ok := q.sessions[sessionID]
	if !ok {
		return port.QRLoginResult{Status: "error", Error: "session not found"}, fmt.Errorf("session not found: %s", sessionID)
	}
	result := port.QRLoginResult{
		Status:      sess.status,
		QRImage:     sess.qrImage,
		Cookie:      sess.cookie,
		ExpiresAt:   sess.expiresAt,
		AccountName: sess.accountName,
		Error:       sess.errMsg,
	}
	// 首次轮询到 waiting 状态时打日志（便于排查前端是否收到图片）
	if sess.status == "waiting" && sess.qrImage != "" {
		log.Printf("[QRLogin:%s] PollStatus 返回 waiting，二维码 %d 字符", sessionID, len(sess.qrImage))
	}
	return result, nil
}

// Cleanup 关闭浏览器会话。
func (q *ChromedpQRLogin) Cleanup(_ context.Context, sessionID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	sess, ok := q.sessions[sessionID]
	if !ok {
		return nil
	}
	log.Printf("[QRLogin:%s] Cleanup：关闭浏览器会话", sessionID)
	if sess.cancel != nil {
		sess.cancel()
	}
	delete(q.sessions, sessionID)
	return nil
}

// searchIframesForQR 遍历所有 iframe target 搜索二维码。
// QQ 登录页的二维码在跨域 iframe（xui.ptlogin2.qq.com）里，
// 主文档 JS 无法访问跨域 iframe 内容，需要用 CDP 在 iframe 的 target 里执行 JS。
func (q *ChromedpQRLogin) searchIframesForQR(ctx context.Context, sessionID, method string) bool {
	// 获取所有 target
	targets, err := chromedp.Targets(ctx)
	if err != nil {
		log.Printf("[QRLogin:%s] 搜索 iframe：获取 targets 失败: %v", sessionID, err)
		return false
	}

	// 找到 iframe target（type=iframe）
	for _, t := range targets {
		if t.Type != "iframe" {
			continue
		}
		log.Printf("[QRLogin:%s] 发现 iframe target: %s (url=%s)", sessionID, t.TargetID, t.URL[:min(60, len(t.URL))])

		// 在 iframe target 里执行 JS 搜索二维码
		// 用 browserCtx 派生新 context 指向 iframe target
		q.mu.Lock()
		sess := q.sessions[sessionID]
		q.mu.Unlock()
		if sess == nil {
			return false
		}

		// 用 sessionCtx 的父级 browserCtx 创建 iframe context
		// chromedp.NewContext 需要从 browser context 派生
		iframeCtx, iframeCancel := chromedp.NewContext(ctx, chromedp.WithTargetID(t.TargetID))

		// 首次 Run attach 到 iframe target
		attachCtx, attachCancel := context.WithTimeout(iframeCtx, 10*time.Second)
		err := chromedp.Run(attachCtx, chromedp.Sleep(1*time.Second))
		attachCancel()
		if err != nil {
			log.Printf("[QRLogin:%s] iframe attach 失败: %v", sessionID, err)
			iframeCancel()
			continue
		}

		// 在 iframe 里执行简单 JS 搜索二维码 img
		simpleJS := `(() => {
			const imgs = document.querySelectorAll('img');
			for (const img of imgs) {
				const cls = (img.className || '').toString().toLowerCase();
				const id = (img.id || '').toString().toLowerCase();
				if (cls.includes('qr') || id.includes('qr') || cls.includes('qrimg')) {
					return JSON.stringify({found: true, src: img.src, className: cls.slice(0,80), id: img.id});
				}
			}
			// 降级：找任何有 src 的 img
			for (const img of imgs) {
				if (img.src && img.src.startsWith('http')) {
					return JSON.stringify({found: true, src: img.src, className: 'fallback', id: img.id});
				}
			}
			return JSON.stringify({found: false, imgCount: imgs.length});
		})()`

		jsCtx, jsCancel := context.WithTimeout(iframeCtx, 8*time.Second)
		var resultJSON string
		err = chromedp.Run(jsCtx,
			chromedp.Evaluate(simpleJS, &resultJSON),
		)
		jsCancel()
		iframeCancel()
		if err != nil {
			log.Printf("[QRLogin:%s] iframe JS 执行失败: %v", sessionID, err)
			continue
		}

		log.Printf("[QRLogin:%s] iframe JS 返回: %s", sessionID, resultJSON)

		var result struct {
			Found     bool   `json:"found"`
			Src       string `json:"src"`
			ClassName string `json:"className"`
			ID        string `json:"id"`
			ImgCount  int    `json:"imgCount"`
		}
		if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
			log.Printf("[QRLogin:%s] iframe 结果解析失败: %v", sessionID, err)
			continue
		}
		if !result.Found || result.Src == "" {
			log.Printf("[QRLogin:%s] iframe 里未找到二维码（imgCount=%d）", sessionID, result.ImgCount)
			continue
		}

		log.Printf("[QRLogin:%s] iframe 里提取到二维码: id=%s class=%s src=%s", sessionID, result.ID, result.ClassName, result.Src[:min(50, len(result.Src))])
		q.setSessionQRImage(sessionID, result.Src)
		return true
	}

	log.Printf("[QRLogin:%s] 所有 iframe 搜索完毕，未找到二维码", sessionID)
	return false
}
