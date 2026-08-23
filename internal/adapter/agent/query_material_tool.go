package agent

import (
	"context"
	"fmt"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// QueryMaterialTool 查询素材工具（智能体专用）。
//
// 设计动机：
//   - 智能体需要查询素材库，了解用户已有的素材
//   - 根据素材类型自动选择端点
//   - 支持关键词搜索
type QueryMaterialTool struct {
	mediaStore port.MediaAssetStore
}

func NewQueryMaterialTool(mediaStore port.MediaAssetStore) *QueryMaterialTool {
	return &QueryMaterialTool{mediaStore: mediaStore}
}

func (t *QueryMaterialTool) Name() string {
	return "query_material"
}

func (t *QueryMaterialTool) Description() string {
	return `查询素材库中的素材。

参数：
- tenant_id（必填）：租户ID
- keyword（可选）：关键词搜索
- type（可选）：素材类型（image/video/audio）

返回：
- materials：素材列表
  - id：素材ID
  - type：素材类型
  - url：素材URL
  - name：素材名称

使用场景：
- 查找用户之前上传的素材
- 查找品牌Logo
- 查找产品图片
- 查找参考音频`
}

// Query 查询素材。
func (t *QueryMaterialTool) Query(ctx context.Context, tenantID, keyword, materialType string) ([]entity.MediaAsset, error) {
	if t.mediaStore == nil {
		return nil, fmt.Errorf("素材存储未配置")
	}

	// 查询素材库
	materials, err := t.mediaStore.List(ctx, tenantID, entity.AssetTypeMaterial)
	if err != nil {
		return nil, fmt.Errorf("查询素材失败: %w", err)
	}

	// 过滤类型
	if materialType != "" {
		var filtered []entity.MediaAsset
		for _, m := range materials {
			if m.Type == materialType {
				filtered = append(filtered, m)
			}
		}
		materials = filtered
	}

	// 关键词搜索（简单的名称匹配）
	if keyword != "" {
		var filtered []entity.MediaAsset
		for _, m := range materials {
			if containsIgnoreCase(m.Name, keyword) || containsIgnoreCase(m.SourceURL, keyword) {
				filtered = append(filtered, m)
			}
		}
		materials = filtered
	}

	return materials, nil
}

func containsIgnoreCase(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
