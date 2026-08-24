package crawler

import (
	_ "embed"
	"fmt"
	"os/exec"
	"strings"
)

// 抖音 a_bogus 签名实现（参考 MediaCrawler douyin/help.py）。
//
// 算法：SM3 哈希 + RC4 加密 + 自定义 base64
// 实现方式：通过 Node.js 执行 douyin.js
//
// 注意：搜索端点 /v1/web/general/search 不需要 a_bogus 签名
// 只有详情、评论等端点需要

//go:embed douyin_sign.js
var douyinSignJS []byte

// DouyinSigner 抖音签名器。
type DouyinSigner struct {
	nodeAvailable bool
}

// NewDouyinSigner 创建抖音签名器。
func NewDouyinSigner() *DouyinSigner {
	s := &DouyinSigner{}
	// 检测 Node.js 是否可用
	if _, err := exec.LookPath("node"); err == nil {
		s.nodeAvailable = true
	}
	return s
}

// Sign 生成 a_bogus 签名参数。
//
// 参数：
//   - url: API 端点路径（如 /aweme/v1/web/aweme/detail/）
//   - queryString: URL 查询字符串（已编码的参数）
//   - userAgent: User-Agent 字符串
//
// 返回：a_bogus 签名值
func (s *DouyinSigner) Sign(url, queryString, userAgent string) (string, error) {
	if !s.nodeAvailable {
		return "", fmt.Errorf("Node.js 未安装，无法生成 a_bogus 签名")
	}

	// 选择签名函数
	signFunc := "sign_datail"
	if strings.Contains(url, "/reply") {
		signFunc = "sign_reply"
	}

	// 构造 Node.js 执行脚本
	script := fmt.Sprintf(`
		%s
		console.log(%s(%q, %q));
	`, string(douyinSignJS), signFunc, queryString, userAgent)

	// 执行 Node.js
	cmd := exec.Command("node", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Node.js 执行失败: %w", err)
	}

	signature := strings.TrimSpace(string(output))
	if signature == "" {
		return "", fmt.Errorf("a_bogus 签名结果为空")
	}

	return signature, nil
}

// IsAvailable 检查签名器是否可用（Node.js 是否安装）。
func (s *DouyinSigner) IsAvailable() bool {
	return s.nodeAvailable
}

// NeedSignature 检查指定端点是否需要签名。
// 搜索端点 /v1/web/general/search 不需要签名（参考 MediaCrawler douyin/client.py 第 119 行）。
func NeedSignature(endpoint string) bool {
	return !strings.Contains(endpoint, "/v1/web/general/search")
}
