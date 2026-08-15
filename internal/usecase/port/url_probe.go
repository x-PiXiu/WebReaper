package port

import "context"

// URLProbe 单次 HTTP GET 探测（收录密钥文件可访问性验证用）。
//
// 设计动机：用例需要"以搜索引擎视角"验证 {base}/{key}.txt 可达且内容一致——
// 这是业务校验，但 HTTP 客户端是传输细节。接口归用例所有（依赖倒置），
// 适配器负责超时/代理/TLS 等细节，用例层零 net/http 依赖（可纯单测）。
type URLProbe interface {
	// ProbeGET GET url，返回状态码与响应体（最多 maxBytes 字节；超长截断）。
	ProbeGET(ctx context.Context, url string, maxBytes int64) (statusCode int, body []byte, err error)
}
