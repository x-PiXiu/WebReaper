// Package urlprobe 实现 port.URLProbe：标准库 HTTP 探测器。
package urlprobe

import (
	"context"
	"io"
	"net/http"
	"time"

	"webreaper/internal/usecase/port"
)

// HTTPProbe URL 探测器（8s 超时；跟随重定向——搜索引擎抓取行为一致）。
type HTTPProbe struct {
	client *http.Client
}

func New() *HTTPProbe {
	return &HTTPProbe{client: &http.Client{Timeout: 8 * time.Second}}
}

var _ port.URLProbe = (*HTTPProbe)(nil)

func (p *HTTPProbe) ProbeGET(ctx context.Context, url string, maxBytes int64) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}
