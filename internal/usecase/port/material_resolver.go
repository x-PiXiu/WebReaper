package port

import "context"

// MaterialURLResolver 素材 URL 可达性解析（整洁架构·Port 接口）。
//
// 职责：将素材 URL 转换为上游厂商可访问的形式。
//   - 公网 URL → 原样返回
//   - 私网小文件 → base64 data URI 内联
//   - 私网大文件 → 上传到公网存储 → 返回公网 URL
//
// 设计动机（SRP）：URL 可达性判断 + 数据格式转换 + 大文件上传是
// Adapter 层关注点，Use Case 层只关心"给我一个厂商能访问的 URL"。
type MaterialURLResolver interface {
	// Resolve 将素材 URL 转换为上游厂商可访问的形式。
	// 返回转换后的 URL 和是否发生了转换。
	// 公网 URL 返回 (url, false, nil)——无需转换。
	Resolve(ctx context.Context, rawURL string) (resolvedURL string, changed bool, err error)
}
