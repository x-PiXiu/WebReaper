package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// EndpointSelector 端点选择器接口（整洁架构·Port层）。
//
// 设计动机：
//   - 根据素材类型自动选择端点（subType）
//   - 将统一格式参数转换为端点特定参数
//   - 填充默认值（duration、resolution等）
//
// 使用场景：
//   - 客户端上传1张图片 + 输入文本 → Select()返回img2video端点和参数
//   - 客户端上传1张图片 + 1个音频 → Select()返回digital_human端点和参数
//   - 客户端只输入文本 → Select()返回text2video端点和参数
type EndpointSelector interface {
	// Select 根据素材自动选择端点。
	// 输入：UnifiedGenerationRequest（text + materials）
	// 输出：EndpointSelectResult（subType + params）
	Select(ctx context.Context, req entity.UnifiedGenerationRequest) (entity.EndpointSelectResult, error)
}
