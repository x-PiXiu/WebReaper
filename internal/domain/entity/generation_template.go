package entity

import "time"

// GenerationTemplate 生成模板（管理后台可动态配置）。
//
// 设计动机（整洁架构）：
//   - 模板存储在数据库中，管理后台可以动态增删改查，不是硬编码在代码中
//   - 用户只需要选择模板，系统自动填充默认参数
//   - 端点类型（SubType）由模板决定，用户不需要选择
//
// 使用场景：
//   - 用户选择"品牌宣传视频"模板 → 系统自动选择img2video端点，填充duration=4
//   - 用户选择"数字人口播"模板 → 系统自动选择digital_human端点
type GenerationTemplate struct {
	ID                string         `json:"id"`                 // 模板ID
	TenantID          string         `json:"tenant_id"`          // 租户ID（空=全局模板）
	Name              string         `json:"name"`               // 模板名称
	Description       string         `json:"description"`        // 模板描述
	Icon              string         `json:"icon"`               // 模板图标
	SubType           string         `json:"sub_type"`           // 端点类型
	DefaultParams     map[string]any `json:"default_params"`     // 默认参数
	RequiredMaterials []string       `json:"required_materials"` // 必需素材类型
	OptionalMaterials []string       `json:"optional_materials"` // 可选素材类型
	SortOrder         int            `json:"sort_order"`         // 排序
	Enabled           bool           `json:"enabled"`            // 是否启用
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// IsValid 领域规则：模板必须有ID、Name、SubType。
func (t GenerationTemplate) IsValid() bool {
	return t.ID != "" && t.Name != "" && t.SubType != ""
}

// HasRequiredMaterials 检查是否满足必需素材要求。
func (t GenerationTemplate) HasRequiredMaterials(materialTypes []string) bool {
	if len(t.RequiredMaterials) == 0 {
		return true
	}

	typeMap := make(map[string]bool)
	for _, mt := range materialTypes {
		typeMap[mt] = true
	}

	for _, required := range t.RequiredMaterials {
		if !typeMap[required] {
			return false
		}
	}
	return true
}
