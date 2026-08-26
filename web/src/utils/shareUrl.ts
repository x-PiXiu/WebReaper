/**
 * 从分享口令/粘贴文案中抽出可解析的视频链接。
 * 抖音复制常带「x.xx xx/xx xxx:/ 标题 https://v.douyin.com/xxx/ 复制此链接…」整段文字，
 * 直接当 URL 传给后端会触发 Go url.Parse 报 first path segment cannot contain colon。
 */
const SHARE_URL_RE =
  /https?:\/\/(?:v\.douyin\.com|www\.douyin\.com|www\.iesdouyin\.com|www\.bilibili\.com|b23\.tv|m\.bilibili\.com)\/[^\s\u4e00-\u9fff]+/i

/** 短链兜底：无协议但含平台域名时补 https:// */
const BARE_HOST_RE =
  /(?:^|[\s:])((?:v\.douyin\.com|www\.douyin\.com|b23\.tv|www\.bilibili\.com)\/[A-Za-z0-9_\-/.?=&%]+)/i

export function extractShareUrl(raw: string): string | null {
  const text = (raw || '').trim()
  if (!text) return null

  const m = text.match(SHARE_URL_RE)
  if (m?.[0]) {
    return cleanUrl(m[0])
  }

  const bare = text.match(BARE_HOST_RE)
  if (bare?.[1]) {
    return cleanUrl(`https://${bare[1]}`)
  }

  // 已是干净单行链接
  if (/^https?:\/\//i.test(text) && !/\s/.test(text)) {
    return cleanUrl(text)
  }

  return null
}

function cleanUrl(u: string): string {
  return u.replace(/[)，。；、！？'"”’>\]]+$/g, '').trim()
}

export function isKuaishouUrl(raw: string): boolean {
  return /kuaishou\.com|v\.kuaishou\.com|gifshow\.com/i.test(raw || '')
}
