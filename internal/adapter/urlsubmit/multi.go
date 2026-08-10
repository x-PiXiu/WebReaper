package urlsubmit

import (
	"context"
	"errors"
	"sync"

	"webreaper/internal/usecase/port"
)

// MultiSubmitter 是 port.URLSubmitter 的组合实现（组合模式）。
//
// 设计动机（多渠道并行收录通知）：
//   - 百度（国内主流量）+ IndexNow（Bing/Yandex/Naver）等渠道独立配置、独立失败。
//   - 一个渠道失败不影响其他渠道——把"失败隔离"从 usecase 下沉到组合器。
//
// 失败语义：全部渠道成功 → nil；任一失败 → 聚合错误（errors.Join），
// 调用方（SetStatus 发布副作用）已按"尽力而为"处理（失败不阻断发布）。
type MultiSubmitter struct {
	submitters []port.URLSubmitter
}

// NewMultiSubmitter 创建多通道提交器（可传 0 个——SubmitURLs 直接返回 nil）。
func NewMultiSubmitter(submitters ...port.URLSubmitter) *MultiSubmitter {
	return &MultiSubmitter{submitters: submitters}
}

// SubmitURLs 并行向所有渠道提交，失败互不影响（收集聚合错误）。
func (m *MultiSubmitter) SubmitURLs(ctx context.Context, urls []string) error {
	if len(m.submitters) == 0 || len(urls) == 0 {
		return nil
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		errs  []error
	)
	for _, s := range m.submitters {
		wg.Add(1)
		go func(sub port.URLSubmitter) {
			defer wg.Done()
			if err := sub.SubmitURLs(ctx, urls); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(s)
	}
	wg.Wait()
	return joinErrors(errs)
}

// joinErrors 聚合错误（nil 列表返回 nil；单个直接返回；多个 errors.Join）。
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return errors.Join(errs...)
}

var _ port.URLSubmitter = (*MultiSubmitter)(nil)
