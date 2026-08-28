"""抖音分享链 Python 解析 sidecar（Go 主服务子挂载）。

为什么存在（2026-08 实测结论）：抖音 WAF 按 TLS 指纹分流——Python
requests（OpenSSL 指纹）访问 iesdouyin 分享页能拿到 SSR 的
_ROUTER_DATA（含播放直链），Go crypto/tls 只会拿到 JS 渲染壳页（0/6）。
故由 Go 主服务在解析链最前端挂载本脚本做"快路径"：纯 HTTP、免浏览器、
免登录账号；失败时 Go 侧自动降级回 chromedp 通道。

协议（stdin/stdout 各一行 JSON）：
  请求: {"url": "分享口令全文或链接"}
  成功: {"ok": true, "video_id": "...", "title": "...", "author": "...",
         "duration": 122, "urls": ["直链1", "直链2", ...]}   ← url_list 全量候选
  失败: {"ok": false, "error": "原因"}

实现来源：free-video-downloader/backend/douyin.py（三级降级 + WAF PoW 破解）。
依赖：python3 + requests（部署机需 pip install requests）。
"""
import base64
import hashlib
import json
import re
import sys
import time
from pathlib import Path
from urllib.parse import urlparse

import requests

MOBILE_HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) "
        "AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 "
        "Mobile/15E148 Safari/604.1"
    ),
    "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
    "Referer": "https://www.douyin.com/",
}

URL_PATTERN = re.compile(r"https?://[^\s]+", re.IGNORECASE)
ITEMINFO_API = "https://www.iesdouyin.com/web/api/v2/aweme/iteminfo/"
MAX_RETRIES = 3


def log(msg):
    """诊断输出走 stderr（stdout 只放协议 JSON）。"""
    print("[dy-sidecar] %s" % msg, file=sys.stderr, flush=True)


def extract_url(text):
    m = URL_PATTERN.search(text or "")
    if not m:
        raise ValueError("未找到有效链接")
    return m.group(0).strip().strip("\"'").rstrip(").,;!?")


class Resolver:
    def __init__(self):
        self.session = requests.Session()
        self.session.headers.update(MOBILE_HEADERS)
        self.timeout = (10, 30)

    def resolve(self, raw):
        share_url = extract_url(raw)
        video_id = self._video_id(share_url)
        item = self._item_info(video_id, share_url)
        return self._build(video_id, item)

    def _video_id(self, share_url):
        try:
            resp = self.session.get(share_url, timeout=self.timeout, allow_redirects=True)
            resp.raise_for_status()
            final = resp.url
        except requests.RequestException as e:
            raise ValueError("短链解析失败: %s" % e)
        for pat in (r"/video/(\d{8,24})", r"/note/(\d{8,24})"):
            m = re.search(pat, urlparse(final).path)
            if m:
                return m.group(1)
        m = re.search(r"(\d{15,24})", final)
        if m:
            return m.group(1)
        raise ValueError("链接里提取不到视频 ID")

    def _item_info(self, video_id, share_url):
        # ① iteminfo 公开 API（大概率已被下线——保留碰运气）
        try:
            return self._via_api(video_id)
        except Exception as e:
            log("iteminfo API 失败(%s)，走分享页" % e)
        # ② 分享页 SSR _ROUTER_DATA（主通道；被 WAF 拦则破解挑战重试）
        return self._via_share_page(video_id, share_url)

    def _via_api(self, video_id):
        for attempt in range(MAX_RETRIES):
            try:
                resp = self.session.get(
                    ITEMINFO_API, params={"item_ids": video_id}, timeout=self.timeout)
                resp.raise_for_status()
                items = (resp.json() or {}).get("item_list") or []
                if items:
                    return items[0]
                raise ValueError("空数据")
            except Exception:
                if attempt == MAX_RETRIES - 1:
                    raise
                time.sleep(2 ** attempt)

    def _via_share_page(self, video_id, share_url):
        parsed = urlparse(share_url)
        page = share_url if "iesdouyin.com" in (parsed.netloc or "") \
            else "https://www.iesdouyin.com/share/video/%s/" % video_id
        for attempt in range(MAX_RETRIES):
            resp = self.session.get(page, timeout=self.timeout)
            resp.raise_for_status()
            html = resp.text or ""
            if "Please wait" in html and "wci=" in html:
                html = self._solve_waf(html, page)
            data = extract_router_data(html)
            item = find_item(data)
            if item:
                return item
            log("分享页第 %d 次为壳页（无 item_list），重试" % (attempt + 1))
            if attempt < MAX_RETRIES - 1:
                time.sleep(1.5 * (attempt + 1))
        raise ValueError("分享页未返回视频数据（连续壳页）")

    def _solve_waf(self, html, page_url):
        """抖音 WAF 算力挑战：SHA256 暴力碰撞 nonce 构造答案 cookie。"""
        m = re.search(r'wci="([^"]+)"\s*,\s*cs="([^"]+)"', html)
        if not m:
            return html
        cookie_name, blob = m.groups()
        try:
            challenge = json.loads(_b64(blob).decode("utf-8"))
            prefix = _b64(challenge["v"]["a"])
            expected = _b64(challenge["v"]["c"]).hex()
        except (KeyError, ValueError):
            return html
        for candidate in range(1_000_001):
            digest = hashlib.sha256(prefix + str(candidate).encode()).hexdigest()
            if digest == expected:
                challenge["d"] = base64.b64encode(str(candidate).encode()).decode()
                value = base64.b64encode(
                    json.dumps(challenge, separators=(",", ":")).encode()).decode()
                domain = urlparse(page_url).hostname or "www.iesdouyin.com"
                self.session.cookies.set(cookie_name, value, domain=domain, path="/")
                resp = self.session.get(page_url, timeout=self.timeout)
                log("WAF 挑战已破解（nonce=%d）" % candidate)
                return resp.text or ""
        return html

    def _build(self, video_id, item):
        video = item.get("video") or {}
        play = (video.get("play_addr") or {}).get("url_list") or []
        urls = [u.replace("playwm", "play") for u in play if u]
        if not urls:
            raise ValueError("详情无播放地址")
        author = (item.get("author") or {}).get("nickname", "")
        duration = video.get("duration") or 0
        return {
            "ok": True,
            "video_id": video_id,
            "title": item.get("desc") or ("douyin_%s" % video_id),
            "author": author,
            "duration": duration // 1000 if duration > 1000 else duration,
            "urls": urls,
        }


def extract_router_data(html):
    """括号配对提取 window._ROUTER_DATA = {...}（容忍字符串内转义/花括号）。"""
    marker = "window._ROUTER_DATA = "
    start = html.find(marker)
    if start < 0:
        return {}
    idx = start + len(marker)
    while idx < len(html) and html[idx].isspace():
        idx += 1
    if idx >= len(html) or html[idx] != "{":
        return {}
    depth, in_str, escaped = 0, False, False
    for cursor in range(idx, len(html)):
        ch = html[cursor]
        if in_str:
            if escaped:
                escaped = False
            elif ch == "\\":
                escaped = True
            elif ch == '"':
                in_str = False
            continue
        if ch == '"':
            in_str = True
        elif ch == "{":
            depth += 1
        elif ch == "}":
            depth -= 1
            if depth == 0:
                try:
                    return json.loads(html[idx: cursor + 1])
                except ValueError:
                    return {}
    return {}


def find_item(data):
    for node in ((data or {}).get("loaderData") or {}).values():
        if not isinstance(node, dict):
            continue
        items = ((node.get("videoInfoRes") or {}).get("item_list")) or []
        if items and isinstance(items[0], dict):
            return items[0]
    return None


def _b64(value):
    normalized = value.replace("-", "+").replace("_", "/")
    normalized += "=" * (-len(normalized) % 4)
    return base64.b64decode(normalized)


def main():
    try:
        req = json.loads(sys.stdin.read() or "{}")
        result = Resolver().resolve(req.get("url", ""))
    except Exception as e:
        result = {"ok": False, "error": "%s: %s" % (type(e).__name__, e)}
    sys.stdout.write(json.dumps(result, ensure_ascii=False) + "\n")
    sys.stdout.flush()


if __name__ == "__main__":
    main()
