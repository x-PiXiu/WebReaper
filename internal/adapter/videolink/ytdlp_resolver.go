// ytdlp_resolver.go yt-dlp 通用解析器（多平台兜底——YouTube/微博/西瓜/X 等 1800+ 站点）。
//
// 定位（与自研通道互补，非替代）：B站走 biliweb 公开 API（快），抖音走 douyinweb
// Python sidecar + chromedp（协议自研）；其余平台统一由 yt-dlp 兜底——
// 它是社区维护的站点适配百科，但抖音恰好要 cookie（实测），正好避开。
//
// 调用形态：子进程 `-J --no-playlist` 拿 info_dict JSON → 自筛候选：
// 只取「音视频合一」（progressive：acodec/vcodec 均非 none）且 http(s) 协议的
// format，按分辨率降序全量返回——文案提取场景低清即够，且下载层单文件可直接下；
// 分离流（bv+ba）需要 ffmpeg 合并，超出 safeDownload 契约，不取。
//
// 部署：YT_DLP_CMD 可配（空格分词，如 "python -m yt_dlp"）；默认自动探测
// yt-dlp → yt-dlp.exe → python -m yt_dlp。缺失时返回错误由 composite 降级 og。
package videolink

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

// YtDlpResolver yt-dlp 子进程封装（port.VideoLinkResolver 的通用兜底实现）。
type YtDlpResolver struct {
	// cmdArgs yt-dlp 调用形态（如 ["yt-dlp"] 或 ["python","-m","yt_dlp"]）
	cmdArgs []string
	once    sync.Once
}

// NewYtDlpResolver 创建；cmdOverride 对应环境变量 YT_DLP_CMD（空=自动探测）。
func NewYtDlpResolver(cmdOverride string) *YtDlpResolver {
	r := &YtDlpResolver{}
	if cmdOverride != "" {
		r.cmdArgs = strings.Fields(cmdOverride)
	}
	return r
}

var _ interface {
	SupportedPlatforms() []string
	Resolve(ctx context.Context, tenantID, rawURL string) ([]string, string, string, string, error)
} = (*YtDlpResolver)(nil)

// SupportedPlatforms 通用兜底（不列具体站点——组合链末位挂载）。
func (r *YtDlpResolver) SupportedPlatforms() []string { return []string{"ytdlp-generic"} }

// ytDlpInfo info_dict 关键字段（yt-dlp 文档化契约的子集）。
type ytDlpInfo struct {
	Title     string `json:"title"`
	Extractor string `json:"extractor"`
	URL       string `json:"url"`
	Formats   []struct {
		URL      string `json:"url"`
		Ext      string `json:"ext"`
		Height   int    `json:"height"`
		VCodec   string `json:"vcodec"`
		ACodec   string `json:"acodec"`
		TBR      float64 `json:"tbr"`
		Protocol string `json:"protocol"`
	} `json:"formats"`
}

// ytdlpSem yt-dlp 并发闸门：每个子进程 ~50MB 内存 + CPU（Python 启动 + 解析），
// 多用户同时提交长尾平台链接时无闸门会进程堆积（90s 超时内最多同屏 N 个 python）。
var ytdlpSem = make(chan struct{}, 2)

// Resolve 链接 → 候选播放直链列表（progressive 格式按分辨率降序）。
func (r *YtDlpResolver) Resolve(ctx context.Context, tenantID, rawURL string) ([]string, string, string, string, error) {
	if err := r.ensureCmd(); err != nil {
		return nil, "", "", "", err
	}
	// 并发闸门（满载时排队等位，ctx 取消则让位）
	select {
	case ytdlpSem <- struct{}{}:
		defer func() { <-ytdlpSem }()
	case <-ctx.Done():
		return nil, "", "", "", fmt.Errorf("yt-dlp 等待队列超时/取消")
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	args := append(append([]string{}, r.cmdArgs...),
		"-J", "--no-warnings", "--no-playlist", rawURL)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stderr = os.Stderr // yt-dlp 诊断透传服务日志
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, "", "", "", fmt.Errorf("yt-dlp 执行超时（>90s）")
		}
		return nil, "", "", "", fmt.Errorf("yt-dlp 解析失败: %w", err)
	}

	var info ytDlpInfo
	if jErr := json.Unmarshal(out, &info); jErr != nil {
		return nil, "", "", "", fmt.Errorf("yt-dlp 输出解析失败: %v | 片段: %.200s", jErr, out)
	}

	urls := progressiveURLs(info)
	if len(urls) == 0 && info.URL != "" {
		urls = []string{info.URL} // 兜底：顶层选中格式直链
	}
	if len(urls) == 0 {
		return nil, "", "", "", fmt.Errorf("yt-dlp 未返回可下载的音视频合格式（可能仅分离流或需要登录）")
	}
	title := info.Title
	if title == "" {
		title = "ytdlp_video"
	}
	return urls, title, "ytdlp-generic", "", nil
}

// progressiveURLs 筛「音视频合一 + http(s)」格式，按分辨率/码率降序去重。
func progressiveURLs(info ytDlpInfo) []string {
	type cand struct {
		url    string
		height int
		tbr    float64
	}
	var list []cand
	seen := map[string]bool{}
	for _, f := range info.Formats {
		if f.URL == "" || seen[f.URL] {
			continue
		}
		if f.VCodec == "" || f.VCodec == "none" || f.ACodec == "" || f.ACodec == "none" {
			continue // 纯视频/纯音频流——需 ffmpeg 合并，超出下载层契约
		}
		if f.Protocol != "" && f.Protocol != "https" && f.Protocol != "http" {
			continue // m3u8/dash 分段流同理
		}
		seen[f.URL] = true
		list = append(list, cand{f.URL, f.Height, f.TBR})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].height != list[j].height {
			return list[i].height > list[j].height
		}
		return list[i].tbr > list[j].tbr
	})
	urls := make([]string, len(list))
	for i, c := range list {
		urls[i] = c.url
	}
	return urls
}

// ensureCmd 惰性确定 yt-dlp 调用形态（探测一次后缓存）。
func (r *YtDlpResolver) ensureCmd() error {
	if len(r.cmdArgs) > 0 {
		return nil
	}
	var probeErr error
	r.once.Do(func() {
		for _, candidate := range [][]string{
			{"yt-dlp"}, {"yt-dlp.exe"}, {"python", "-m", "yt_dlp"}, {"python3", "-m", "yt_dlp"},
		} {
			probeCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			cmd := exec.CommandContext(probeCtx, candidate[0], append(candidate[1:], "--version")...)
			if err := cmd.Run(); err == nil {
				r.cmdArgs = candidate
				log.Printf("[ytdlp] 使用调用形态: %s", strings.Join(candidate, " "))
				cancel()
				return
			}
			cancel()
		}
		probeErr = fmt.Errorf("yt-dlp 不可用（未找到 yt-dlp 命令；可安装官方 exe 或 pip install yt-dlp，或用 YT_DLP_CMD 指定调用形态）")
		log.Printf("[ytdlp] 探测失败：%v", probeErr)
	})
	return probeErr
}
