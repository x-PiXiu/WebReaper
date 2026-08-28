// douyinweb 抖音解析 Python sidecar（Go 子挂载）。
//
// 为什么需要（2026-08 实测）：抖音 WAF 按 TLS 指纹分流——Go crypto/tls 访问
// iesdouyin 分享页只拿 JS 壳页（0/6），Python requests（OpenSSL 指纹）能拿
// SSR 数据页（8/9）。故解析链最前端挂本 sidecar：纯 HTTP 快路径（1~2s，
// 免浏览器免账号），失败自动降级回 chromedp 通道（LinkResolver.Resolve 编排）。
//
// 形态：go:embed 内嵌 py/douyin_resolver.py（部署零拷贝），运行时落临时文件，
// 子进程 stdin/stdout 各一行 JSON 通信，context 超时强杀。
// 部署依赖：目标机 python3 + pip install requests。
package douyinweb

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

//go:embed py/douyin_resolver.py
var douyinPyScript string

// pySidecarTimeout 单次 sidecar 调用上限（脚本内部自带重试，预留充足）。
const pySidecarTimeout = 45 * time.Second

// pySidecarResult sidecar 协议输出。
type pySidecarResult struct {
	OK       bool     `json:"ok"`
	VideoID  string   `json:"video_id"`
	Title    string   `json:"title"`
	Author   string   `json:"author"`
	Duration int      `json:"duration"`
	URLs     []string `json:"urls"`
	Error    string   `json:"error"`
}

// pythonCmdNames 解释器候选（按平台排序，逐个试到存在为止）。
var pythonCmdNames = func() []string {
	if runtime.GOOS == "windows" {
		return []string{"python", "py", "python3"}
	}
	return []string{"python3", "python"}
}()

// resolveViaPython 调 Python sidecar 解析分享链 → 候选直链列表。
// 任何失败（python 缺失/requests 缺失/风控）都返回 error——由调用方降级。
func resolveViaPython(ctx context.Context, rawURL string) (*pySidecarResult, error) {
	// 脚本落临时文件（python 需要真实文件路径；embed 内容无法直接执行）
	tmp, err := os.CreateTemp("", "dy-sidecar-*.py")
	if err != nil {
		return nil, fmt.Errorf("sidecar 脚本落盘失败: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(douyinPyScript); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("sidecar 脚本写入失败: %w", err)
	}
	tmp.Close()

	ctx, cancel := context.WithTimeout(ctx, pySidecarTimeout)
	defer cancel()

	var lastErr error
	for _, python := range pythonCmdNames {
		out, err := runPython(ctx, python, tmp.Name(), rawURL)
		if err == nil {
			return out, nil
		}
		lastErr = err
		// 解释器不存在（file not found）才试下一个候选；其他错误（协议/风控）换解释器无意义
		if !isExecNotFound(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

// runPython 单解释器执行：stdin 喂请求 JSON，stdout 收协议 JSON。
func runPython(ctx context.Context, python, scriptPath, rawURL string) (*pySidecarResult, error) {
	input, _ := json.Marshal(map[string]string{"url": rawURL})
	cmd := exec.CommandContext(ctx, python, scriptPath)
	cmd.Stdin = strings.NewReader(string(input))
	cmd.Stderr = os.Stderr // sidecar 诊断日志透传到服务日志
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("sidecar 执行超时（>%s）", pySidecarTimeout)
		}
		return nil, fmt.Errorf("sidecar 执行失败（%s）: %w", python, err)
	}
	var res pySidecarResult
	if jErr := json.Unmarshal(out, &res); jErr != nil {
		return nil, fmt.Errorf("sidecar 输出解析失败: %v | 输出片段: %.200s", jErr, out)
	}
	if !res.OK {
		return nil, fmt.Errorf("sidecar 解析失败: %s", res.Error)
	}
	if len(res.URLs) == 0 {
		return nil, fmt.Errorf("sidecar 未返回播放地址")
	}
	return &res, nil
}

// isExecNotFound 解释器不存在（Windows/Unix 双形态错误）。
func isExecNotFound(err error) bool {
	if err == nil {
		return false
	}
	if ee, ok := err.(*exec.Error); ok {
		return ee.Err == exec.ErrNotFound || os.IsNotExist(ee.Err)
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "cannot find") ||
		strings.Contains(msg, "no such file") || strings.Contains(msg, "不是内部或外部命令") ||
		strings.Contains(msg, "系统找不到指定的文件")
}
