package qrlogin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"

	"webreaper/internal/adapter/chromedputil"
	"webreaper/internal/config"
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
			"zhihu":  "",              // 默认知乎App扫码（无需点第三方按钮）
			"wechat": "ZDI--Wechat24", // 微信登录
			"qq":     "ZDI--Qq24",     // QQ登录
			"weibo":  "ZDI--Weibo24",  // 微博登录
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
	// 抖音创作者中心（获客智能体转型：视频分发主战场）
	// 登录页默认显示二维码；可能先弹滑块验证——Agent 处理后回到二维码流程。
	"douyin": {
		LoginURL:    "https://creator.douyin.com/creator-micro/home",
		TabText:     "",
		AuthCookies: []string{"sessionid", "sessionid_ss"},
		LoginMethods: map[string]string{
			"douyin": "", // 抖音App扫码（默认）
		},
	},
	// 快手创作者中心（需要先跳转到登录页，再点击扫码登录 tab）
	"kuaishou": {
		LoginURL:    "https://passport.kuaishou.com/pc/account/login/?sid=kuaishou.web.cp.api&callback=https%3A%2F%2Fcp.kuaishou.com%2Frest%2Finfra%2Fsts%3FfollowUrl%3Dhttps%253A%252F%252Fcp.kuaishou.com%252Fprofile%26setRootDomain%3Dtrue",
		TabText:     "扫码登录",
		AuthCookies: []string{"passToken", "kuaishou.server.webday7_st"},
		LoginMethods: map[string]string{
			"kuaishou": "", // 快手App扫码（默认）
		},
	},
	// B站创作者中心
	"bilibili": {
		LoginURL:    "https://member.bilibili.com/platform/home",
		TabText:     "扫码登录",
		AuthCookies: []string{"SESSDATA", "bili_jct"},
		LoginMethods: map[string]string{
			"bilibili": "", // B站App扫码（默认）
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
  if (method === 'zhihu' || method === 'xiaohongshu' || method === 'douyin' || method === 'kuaishou' || method === 'bilibili' || method === '' ) {
    const canvases = document.querySelectorAll('canvas');
    let canvasIdx = 0;
    for (const c of canvases) {
      const w = c.width, h = c.height;
      if (w >= 60 && w <= 500 && h >= 60 && h <= 500) {
        const ratio = w / Math.max(h, 1);
        if (ratio > 0.85 && ratio < 1.15) {
          // Go 侧已判定该 canvas 导出的是空白图（抖音篡改 toDataURL），跳过换下一个
          if (c.getAttribute('data-qr-blank') !== '1') {
            try {
              const dataURL = c.toDataURL('image/png');
              if (dataURL && dataURL.length > 100) {
                return JSON.stringify({ found: true, type: 'canvas-dataurl', width: w, height: h, canvasIdx: canvasIdx, className: (c.className||'').toString().slice(0,80), dataURL: dataURL });
              }
            } catch(e) {
              c.setAttribute('data-qr-shot', canvasIdx);
              return JSON.stringify({ found: true, type: 'canvas-screenshot', width: w, height: h, canvasIdx: canvasIdx, className: (c.className||'').toString().slice(0,80), jsPath: 'document.querySelector(\'canvas[data-qr-shot=\"' + canvasIdx + '\"]\')' });
            }
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
    const ariaLabel = (img.getAttribute('aria-label') || '').toLowerCase();
    const altText = (img.alt || '').toLowerCase();
    const hasQRClass = cls.includes('qr') || cls.includes('code') || cls.includes('qrcode') || cls.includes('qrimg')
      || parentCls.includes('qr') || parentCls.includes('qrcode') || parentCls.includes('qr-code')
      || ariaLabel.includes('二维码') || ariaLabel.includes('qrcode')  // 抖音用 aria-label="二维码"
      || altText.includes('scan me');  // B站用 alt="Scan me!"

    // class 含 qr 的图片直接返回（QQ 的 qrImg、微信的 qrcode_img、微博父容器 qr-code）
    // 抖音的 img class="RhjdbXj8" 通过 aria-label="二维码" 匹配
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

// findQRCandidatesJS 枚举页面上所有可能承载二维码的 canvas（≥100x100），
// 返回候选列表（带 jsPath），排序：可见的正方形优先、可见的次之、其余最后。
// 不做内容判断——由 Go 侧逐个元素截图并用 gozxing 解码验证（唯一可信的判定标准）。
// 背景：抖音登录页 4 个 180x180 装饰 canvas + 1 个 902x552 登录卡大 canvas，
// 真二维码渲染在大 canvas 区域的右下角——尺寸/比例启发式无法定位，只能穷举+解码。
const findQRCandidatesJS = `(() => {
  const out = [];
  document.querySelectorAll('canvas').forEach((c, i) => {
    if (c.width < 100 || c.height < 100) return;
    const r = c.getBoundingClientRect();
    const bigger = Math.max(c.width, c.height);
    out.push({
      kind: 'canvas', idx: i, w: c.width, h: c.height,
      square: Math.abs(c.width - c.height) < bigger * 0.2,
      visible: r.width > 0 && r.height > 0,
      jsPath: 'document.querySelectorAll(\'canvas\')[' + i + ']'
    });
  });
  const score = (c) => (c.visible && c.square ? 0 : c.visible ? 1 : 2);
  out.sort((a, b) => score(a) - score(b));
  return JSON.stringify({ candidates: out.slice(0, 10) });
})()`

// qrCandidate findQRCandidatesJS 返回的候选元素。
type qrCandidate struct {
	Kind    string `json:"kind"`
	Idx     int    `json:"idx"`
	W       int    `json:"w"`
	H       int    `json:"h"`
	Square  bool   `json:"square"`
	Visible bool   `json:"visible"`
	JSPath  string `json:"jsPath"`
}

// scanQRCandidates 阶段 A2：候选 canvas 逐个元素截图 + gozxing 解码验证。
// 命中即裁剪出二维码区域写入会话；全部失败返回 false（继续走后续阶段）。
func (q *ChromedpQRLogin) scanQRCandidates(ctx context.Context, sessionID string) bool {
	candCtx, candCancel := context.WithTimeout(ctx, 10*time.Second)
	var candJSON string
	err := chromedp.Run(candCtx,
		chromedp.Sleep(time.Second),
		chromedp.Evaluate(findQRCandidatesJS, &candJSON),
	)
	candCancel()
	if err != nil {
		log.Printf("[QRLogin:%s] 候选枚举失败: %v", sessionID, err)
		return false
	}
	var parsed struct {
		Candidates []qrCandidate `json:"candidates"`
	}
	if jErr := json.Unmarshal([]byte(candJSON), &parsed); jErr != nil {
		log.Printf("[QRLogin:%s] 候选解析失败: %v (raw=%s)", sessionID, jErr, candJSON)
		return false
	}
	log.Printf("[QRLogin:%s] 候选扫描开始：共 %d 个 canvas 候选", sessionID, len(parsed.Candidates))

	for i, cand := range parsed.Candidates {
		if q.isSessionClosed(sessionID) {
			return false
		}
		shotCtx, shotCancel := context.WithTimeout(ctx, 6*time.Second)
		var qrBytes []byte
		sErr := chromedp.Run(shotCtx, chromedp.Screenshot(cand.JSPath, &qrBytes, chromedp.ByJSPath))
		shotCancel()
		if sErr != nil || len(qrBytes) <= 500 {
			log.Printf("[QRLogin:%s] 候选 %d/%d canvas[%d]（%dx%d）截图失败: %v（%d 字节）",
				sessionID, i+1, len(parsed.Candidates), cand.Idx, cand.W, cand.H, sErr, len(qrBytes))
			continue
		}
		if text := decodeQRText(qrBytes); text != "" {
			log.Printf("[QRLogin:%s] 候选 %d/%d canvas[%d]（%dx%d）解出二维码 → %s",
				sessionID, i+1, len(parsed.Candidates), cand.Idx, cand.W, cand.H, truncURL(text))
			q.setSessionQRImage(sessionID, base64.StdEncoding.EncodeToString(cropToQR(qrBytes)))
			return true
		}
		log.Printf("[QRLogin:%s] 候选 %d/%d canvas[%d]（%dx%d）不是二维码，继续",
			sessionID, i+1, len(parsed.Candidates), cand.Idx, cand.W, cand.H)
	}
	return false
}

// findQRContainerJS 在页面内查找二维码容器元素并标记（用于元素截图）。
// 与 findQRElementJS（提取 canvas/img 图片数据）不同，本脚本找的是二维码的**容器 div**——
// 不关心内部是 canvas/img/SVG/iframe，找到后由 Go 侧对该元素截图。
// 这是对话式扫码登录的核心策略（参考 MediaCrawler）：截图不依赖 DOM 内部结构，平台改版不影响。
//
// 查找策略（按优先级）：
//   1. class/id 含 QR 相关关键词的元素（最可靠）
//   2. 页面中央区域的正方形中等大小元素（登录弹窗中的二维码通常在中央）
const findQRContainerJS = `(() => {
  // 策略 1：class/id 含 QR 关键词
  const kwSelectors = [
    '[class*="qrcode"]', '[class*="qr-code"]', '[class*="qr_code"]',
    '[class*="QRCode"]', '[class*="QrCode"]',
    '[id*="qrcode"]', '[id*="qr-code"]', '[id*="qr_code"]',
    '[class*="scan-code"]', '[class*="code-container"]', '[class*="code_container"]',
    '[class*="web-login"]', '[class*="login-qr"]', '[class*="login_qr"]',
  ];
  for (const sel of kwSelectors) {
    try {
      const els = document.querySelectorAll(sel);
      for (const el of els) {
        const rect = el.getBoundingClientRect();
        if (rect.width >= 100 && rect.width <= 500 && rect.height >= 100 && rect.height <= 500) {
          const ratio = rect.width / Math.max(rect.height, 1);
          if (ratio > 0.6 && ratio < 1.6) {
            el.setAttribute('data-qr-container', '1');
            return JSON.stringify({
              found: true, type: 'container-screenshot',
              width: Math.round(rect.width), height: Math.round(rect.height),
              className: (el.className || '').toString().slice(0, 80),
              jsPath: 'document.querySelector("[data-qr-container=\\"1\\"]")',
            });
          }
        }
      }
    } catch(e) {}
  }

  // 策略 2：页面中央区域的正方形中等大小元素
  const allEls = document.querySelectorAll('div, section');
  const vw = window.innerWidth, vh = window.innerHeight;
  const cx = vw / 2, cy = vh / 2;
  let best = null, bestScore = 0;

  for (const el of allEls) {
    const rect = el.getBoundingClientRect();
    if (rect.width < 120 || rect.width > 450) continue;
    if (rect.height < 120 || rect.height > 450) continue;

    const ratio = rect.width / Math.max(rect.height, 1);
    if (ratio < 0.75 || ratio > 1.3) continue;

    // 必须靠近页面中央（登录弹窗位置）
    const elCX = rect.x + rect.width / 2;
    const elCY = rect.y + rect.height / 2;
    const dist = Math.sqrt((elCX - cx) ** 2 + (elCY - cy) ** 2);
    if (dist > 350) continue;

    // 必须有视觉内容（不是空 div）
    if (el.children.length === 0 && !(el.textContent || '').trim()) continue;

    // 排除明显的非二维码元素
    const cls = (el.className || '').toString().toLowerCase();
    if (cls.includes('nav') || cls.includes('header') || cls.includes('footer')) continue;
    if (cls.includes('logo') || cls.includes('banner') || cls.includes('carousel')) continue;

    const squareness = 1 - Math.abs(1 - ratio);
    const score = squareness * (1 - dist / 500);
    if (score > bestScore) {
      bestScore = score;
      best = el;
    }
  }

  if (best && bestScore > 0.3) {
    best.setAttribute('data-qr-container', '1');
    const rect = best.getBoundingClientRect();
    return JSON.stringify({
      found: true, type: 'container-screenshot',
      width: Math.round(rect.width), height: Math.round(rect.height),
      className: (best.className || '').toString().slice(0, 80),
      jsPath: 'document.querySelector("[data-qr-container=\\"1\\"]")',
    });
  }

  return JSON.stringify({ found: false });
})()`

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
	opts := chromedputil.HeadlessOptions(config.IsBrowserHeaded())
	opts = append(opts,
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"),
		// 每次用独立的临时用户数据目录，确保不残留上次登录的 cookie/session
		// （否则上次扫码登录的状态会被新会话继承，导致"未扫码就显示已登录"）
		chromedp.Flag("incognito", true),
	)
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
					if !strings.Contains(t.URL, "zhihu.com/signin") && !strings.Contains(t.URL, "xiaohongshu.com") && !strings.Contains(t.URL, "creator.douyin.com") && !strings.Contains(t.URL, "cp.kuaishou.com") && !strings.Contains(t.URL, "passport.kuaishou.com") && !strings.Contains(t.URL, "member.bilibili.com") {
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

	// 容器截图 / 元素截图：对二维码容器/元素做截图（不关心内部是 canvas/img/SVG/iframe）
	if det.Type == "container-screenshot" || det.Type == "canvas-screenshot" || det.Type == "img-screenshot" {
		jsPath := det.JSPath
		if jsPath == "" {
			// 容器检测可能只返回了 found=true 但没带 jsPath，用通用选择器
			jsPath = `document.querySelector("[data-qr-container=\"1\"]")`
		}
		shotCtx, shotCancel := context.WithTimeout(ctx, 8*time.Second)
		var qrBytes []byte
		shotErr := chromedp.Run(shotCtx,
			chromedp.Screenshot(jsPath, &qrBytes, chromedp.ByJSPath),
		)
		shotCancel()
		if shotErr == nil && len(qrBytes) > 500 {
			// 解码验证：只有 gozxing 解出二维码内容才接受（装饰插画会被拦截）
			if text := decodeQRText(qrBytes); text != "" {
				log.Printf("[QRLogin:%s] 元素截图解出二维码（%d 字节）→ %s", sessionID, len(qrBytes), truncURL(text))
				q.setSessionQRImage(sessionID, base64.StdEncoding.EncodeToString(cropToQR(qrBytes)))
				return true
			}
			log.Printf("[QRLogin:%s] 元素截图未解出二维码，拒绝（%d 字节，空白=%v）", sessionID, len(qrBytes), isBlankImage(qrBytes))
			return false
		}
		log.Printf("[QRLogin:%s] 元素截图失败: %v (bytes=%d)", sessionID, shotErr, len(qrBytes))
		return false
	}

	qrBase64 := det.DataURL
	if strings.HasPrefix(qrBase64, "data:image/") {
		// data URL：去掉前缀，只保留 base64
		commaIdx := strings.Index(qrBase64, ",")
		if commaIdx < 0 {
			log.Printf("[QRLogin:%s] data URL 格式异常（无逗号分隔符）", sessionID)
			return false
		}
		mime := qrBase64[:commaIdx]
		qrBase64 = qrBase64[commaIdx+1:]
		if len(qrBase64) > 100 {
			if raw, dErr := base64.StdEncoding.DecodeString(qrBase64); dErr == nil {
				// 解码验证：canvas.toDataURL 可能被平台篡改（返回空白）或命中装饰插画，
				// 只有解出二维码内容才接受
				if text := decodeQRText(raw); text != "" {
					log.Printf("[QRLogin:%s] data URL 解出二维码 → %s", sessionID, truncURL(text))
					q.setSessionQRImage(sessionID, base64.StdEncoding.EncodeToString(cropToQR(raw)))
					return true
				}
				log.Printf("[QRLogin:%s] data URL 未解出二维码（空白=%v, mime=%s），尝试元素截图", sessionID, isBlankImage(raw), mime)
			}
			// 解码失败（空白/非二维码/SVG 等不可解格式）：canvas 改用元素截图走渲染管线
			if det.Type == "canvas-dataurl" && q.screenshotCanvasByIdx(ctx, sessionID, det.CanvasIdx) {
				return true
			}
			if det.Type == "canvas-dataurl" {
				q.markCanvasBlank(ctx, det.CanvasIdx)
			}
			return false
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
		// 服务端下载验证（仅记录结论，不改变行为）：快手登录二维码是艺术字样式，
		// gozxing 可能解不出来但手机可扫——下载失败/解码失败都不拦截 URL 返回
		if raw, ok := downloadImage(qrBase64); ok {
			log.Printf("[QRLogin:%s] 图片 URL 下载验证：%s", sessionID, map[bool]string{true: "解出二维码", false: "未解出（可能是样式化二维码）"}[decodeQRText(raw) != ""])
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

	// 阶段 A：用原有 JS 提取二维码图片数据（知乎/小红书的 canvas/img 方式）
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

	// 阶段 A2：候选 canvas 穷举扫描 + 解码验证。
	// 抖音的真二维码在 902x552 大 canvas 区域内，阶段 A 的尺寸启发式（60-500 正方形）
	// 永远命中不了它，只能逐个截图解码。
	if q.isSessionClosed(sessionID) {
		return
	}
	if q.scanQRCandidates(ctx, sessionID) {
		return
	}

	// 阶段 B：容器截图方式（抖音/快手等平台二维码不暴露 canvas/img，用容器元素截图）
	// 参考 MediaCrawler：不关心二维码内部渲染方式，直接对二维码容器截图
	if !q.isSessionClosed(sessionID) {
		log.Printf("[QRLogin:%s] 阶段 A 未找到，尝试容器截图方式", sessionID)
		for attempt := 1; attempt <= 2; attempt++ {
			if q.isSessionClosed(sessionID) {
				return
			}

			containerCtx, containerCancel := context.WithTimeout(ctx, 10*time.Second)
			var containerJSON string
			err := chromedp.Run(containerCtx,
				chromedp.Sleep(3*time.Second), // 等页面/JS 完全渲染
				chromedp.Evaluate(findQRContainerJS, &containerJSON),
			)
			containerCancel()
			if err != nil {
				log.Printf("[QRLogin:%s] 容器检测 JS 执行失败: %v", sessionID, err)
				continue
			}

			det, parseErr := parseQRDetection(containerJSON)
			if parseErr != nil {
				log.Printf("[QRLogin:%s] 容器检测结果解析失败: %v (raw=%s)", sessionID, parseErr, containerJSON)
				continue
			}
			if !det.Found {
				log.Printf("[QRLogin:%s] 容器截图尝试 %d：未找到二维码容器", sessionID, attempt)
				continue
			}

			log.Printf("[QRLogin:%s] 容器截图尝试 %d：找到容器 %s (%vx%v)", sessionID, attempt, det.Class, det.Width, det.Height)
			if q.processQRDetection(ctx, sessionID, det) {
				return
			}
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
		// 尽力裁剪：整页截图里若能解出二维码，裁出来给用户（比整页更清晰可扫）
		if text := decodeQRText(fullBytes); text != "" {
			log.Printf("[QRLogin:%s] 整页截图解出二维码 → %s", sessionID, truncURL(text))
			q.setSessionQRImage(sessionID, base64.StdEncoding.EncodeToString(cropToQR(fullBytes)))
			return
		}
		log.Printf("[QRLogin:%s] 整页截图成功（%d 字节）", sessionID, len(fullBytes))
		q.setSessionQRImage(sessionID, base64.StdEncoding.EncodeToString(fullBytes))
	} else {
		log.Printf("[QRLogin:%s] 整页截图为空", sessionID)
		q.setSessionError(sessionID, "页面可能未正确加载")
	}
}

// decodeQRText 用 gozxing 解码图片中的二维码，返回解码文本（失败为空串）。
//
// 这是二维码提取的唯一可信判定标准。踩坑记录（2026-08-20）：
//   - 抖音篡改 canvas.toDataURL 返回空白的 PNG；
//   - 抖音登录页有 4 个 180x180 装饰 canvas + 1 个 902x552 登录卡大 canvas，
//     真二维码渲染在大 canvas 区域右下角——尺寸/比例/内容启发式全部失效；
//   - 视觉 LLM 在引导性提问下会把装饰插画"确认"为二维码。
// 只有解码成功才能证明图片是可扫描的二维码。
func decodeQRText(data []byte) string {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return ""
	}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, nil)
	if err != nil || res == nil {
		return ""
	}
	return res.GetText()
}

// cropToQR 从截图中裁出二维码区域（四周加 25% 边距作静区），解码失败时原样返回。
// 场景：抖音二维码在 902x552 大 canvas 截图的右下角——裁剪后用户看到的就是干净二维码。
func cropToQR(data []byte) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return data
	}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, nil)
	if err != nil || res == nil {
		return data
	}
	pts := res.GetResultPoints()
	if len(pts) < 3 {
		return data
	}
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for _, p := range pts {
		x, y := int(p.GetX()), int(p.GetY())
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	padX, padY := max((maxX-minX)/4, 12), max((maxY-minY)/4, 12)
	rect := image.Rect(minX-padX, minY-padY, maxX+padX, maxY+padY).Intersect(b)
	type subImager interface{ SubImage(r image.Rectangle) image.Image }
	si, ok := img.(subImager)
	if !ok || rect.Empty() {
		return data
	}
	var buf bytes.Buffer
	if png.Encode(&buf, si.SubImage(rect)) == nil && buf.Len() > 500 {
		return buf.Bytes()
	}
	return data
}

// truncURL 日志用：截断长 URL（扫码链接带长 token，全量打印会刷屏）。
func truncURL(s string) string {
	if len(s) > 90 {
		return s[:90] + "..."
	}
	return s
}

// downloadImage 服务端下载图片 URL（验证 http 形式的 img 二维码用）。
func downloadImage(url string) ([]byte, bool) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return data, true
}

// qrDetection JS 检测到的二维码信息。
type qrDetection struct {
	Found     bool    `json:"found"`
	Type      string  `json:"type"` // canvas-dataurl / canvas-screenshot / img / container-screenshot
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	Class     string  `json:"className"`
	DataURL   string  `json:"dataURL"`   // base64 图片或 http URL（canvas-dataurl/img 类型有值）
	JSPath    string  `json:"jsPath"`    // 元素截图路径（canvas-screenshot 类型有值）
	CanvasIdx int     `json:"canvasIdx"` // canvas 在 document.querySelectorAll('canvas') 中的序号（canvas-dataurl 用）
}

// isBlankImage 判断图片是否基本纯色（无可见内容）。入参兼容 base64 字符串与原始字节。
//
// 背景：抖音对 canvas.toDataURL/getContext 做了反爬篡改——返回有效长度但内容全白的 PNG，
// 导致"提取成功"的二维码其实是空白图。这里解码图片抽样计算亮度极差：
// 极差 < 24 视为空白（真二维码黑白模块极差通常 > 150）。
// 解码失败（如 SVG/GIF）不拦截，返回 false。
func isBlankImage(data []byte) bool {
	raw, err := base64.StdEncoding.DecodeString(string(data))
	if err == nil {
		data = raw // 入参是 base64
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		img, err = jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			return false
		}
	}
	b := img.Bounds()
	stepX := max(1, b.Dx()/64)
	stepY := max(1, b.Dy()/64)
	mn, mx := uint32(1<<30), uint32(0)
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			r, g, bl, _ := img.At(x, y).RGBA()
			lum := (r/257*299 + g/257*587 + bl/257*114) / 1000 // 0-255 亮度
			if lum < mn {
				mn = lum
			}
			if lum > mx {
				mx = lum
			}
		}
	}
	return mx-mn < 24
}

// screenshotCanvasByIdx 对指定序号的 canvas 做元素截图（绕过 toDataURL 篡改，
// 截图走浏览器渲染管线）。成功且解出二维码内容时写入会话并返回 true。
func (q *ChromedpQRLogin) screenshotCanvasByIdx(ctx context.Context, sessionID string, idx int) bool {
	if idx < 0 {
		return false
	}
	markCtx, markCancel := context.WithTimeout(ctx, 5*time.Second)
	var marked bool
	err := chromedp.Run(markCtx, chromedp.Evaluate(
		fmt.Sprintf(`(() => { const c = document.querySelectorAll('canvas')[%d]; if (!c) return false; c.setAttribute('data-qr-shot', '%d'); return true; })()`, idx, idx),
		&marked))
	markCancel()
	if err != nil || !marked {
		log.Printf("[QRLogin:%s] canvas[%d] 标记失败: %v", sessionID, idx, err)
		return false
	}

	shotCtx, shotCancel := context.WithTimeout(ctx, 8*time.Second)
	var qrBytes []byte
	err = chromedp.Run(shotCtx,
		chromedp.Screenshot(fmt.Sprintf(`document.querySelector('canvas[data-qr-shot="%d"]')`, idx), &qrBytes, chromedp.ByJSPath))
	shotCancel()
	if err == nil && len(qrBytes) > 500 {
		if text := decodeQRText(qrBytes); text != "" {
			log.Printf("[QRLogin:%s] canvas[%d] 元素截图解出二维码（%d 字节）→ %s", sessionID, idx, len(qrBytes), truncURL(text))
			q.setSessionQRImage(sessionID, base64.StdEncoding.EncodeToString(cropToQR(qrBytes)))
			return true
		}
		log.Printf("[QRLogin:%s] canvas[%d] 元素截图未解出二维码（%d 字节）", sessionID, idx, len(qrBytes))
		return false
	}
	log.Printf("[QRLogin:%s] canvas[%d] 元素截图失败: %v（%d 字节）", sessionID, idx, err, len(qrBytes))
	return false
}

// markCanvasBlank 给指定 canvas 打空白标记，findQRElementJS 下次提取时跳过它。
func (q *ChromedpQRLogin) markCanvasBlank(ctx context.Context, idx int) {
	if idx < 0 {
		return
	}
	markCtx, markCancel := context.WithTimeout(ctx, 3*time.Second)
	_ = chromedp.Run(markCtx, chromedp.Evaluate(
		fmt.Sprintf(`(() => { const c = document.querySelectorAll('canvas')[%d]; if (c) c.setAttribute('data-qr-blank', '1'); })()`, idx),
		nil))
	markCancel()
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
				// 长度校验的适用性按平台而异：小红书 web_session 访客 38/登录 100+ 适用；
				// 但抖音 sessionid 固定 32 字符 hex（登录后也是 32）——单靠长度必误杀。
				// 改用登录伴随 cookie 判定：sid_guard/uid_tt 只在扫码登录成功后签发，访客没有。
				if _, ok2 := cookieMap["sid_guard"]; ok2 {
					log.Printf("[QRLogin] cookie %s 值短（%d 字符）但 sid_guard 存在，判定为登录态", name, len(v))
				} else {
					log.Printf("[QRLogin] cookie %s 存在但值太短（%d 字符）且无 sid_guard，视为访客 session，跳过", name, len(v))
					continue
				}
			} else {
				log.Printf("[QRLogin] cookie %s 值长度 %d，判定为登录态", name, len(v))
			}
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
