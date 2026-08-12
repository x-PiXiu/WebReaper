// Package viduendpoint 提供 Vidu 端点策略（EndpointAdapter 实现）与模型能力向量。
//
// 设计（Docs/Plans/03 计划文档 §2.2）：每个端点一个策略对象——参数校验规则
// （直查能力向量）与请求体组装（图片引用/subjects 结构/payload 透传）写在这里；
// 模型差异由 ModelCapability 类型化承载（新增模型 = 加一行 struct，编译期安全），
// 管理后台可经 generation_specs 表对单模型做 JSON 覆盖。
package viduendpoint

import (
	"context"
	"fmt"

	"webreaper/internal/domain/entity"
)

// capability 便捷取模型能力（未注册返回错误——由注册表保证先注册后查询）。
func capabilityFor(caps []entity.ModelCapability, model string) (entity.ModelCapability, error) {
	for _, c := range caps {
		if c.Model == model {
			return c, nil
		}
	}
	return entity.ModelCapability{}, fmt.Errorf("模型 %q 未在该端点注册", model)
}

// ---- 参数提取辅助（端点策略共用）----

func getString(p entity.GenerationParams, key string) string {
	if v, ok := p[key].(string); ok {
		return v
	}
	return ""
}

func getInt(p entity.GenerationParams, key string) int {
	switch v := p[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}

func getBool(p entity.GenerationParams, key string) bool {
	if v, ok := p[key].(bool); ok {
		return v
	}
	return false
}

func getStrings(p entity.GenerationParams, key string) []string {
	if v, ok := p[key].([]string); ok {
		return v
	}
	if v, ok := p[key].([]any); ok {
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// validateEnum 枚举校验（allowed 为空 = 不限制）。
func validateEnum(val string, allowed []string, field string) error {
	if val == "" || len(allowed) == 0 {
		return nil
	}
	for _, a := range allowed {
		if val == a {
			return nil
		}
	}
	return fmt.Errorf("%s 可选值 %v，收到 %q", field, allowed, val)
}

// validateDuration 时长校验（cap.Durations 为 [min,max]；0 表示不支持自定义）。
func validateDuration(duration int, cap entity.ModelCapability, field string) error {
	if duration <= 0 {
		return nil // 未传（用默认值）
	}
	if cap.Durations == [2]int{} {
		return fmt.Errorf("%s 该模型不支持自定义时长", field)
	}
	if duration < cap.Durations[0] || duration > cap.Durations[1] {
		return fmt.Errorf("%s 需在 %d-%d 秒之间", field, cap.Durations[0], cap.Durations[1])
	}
	return nil
}

// validateImageSlots 图片槽位校验（0=不需要；-1=动态 1-7）。
func validateImageSlots(images []string, cap entity.ModelCapability, field string) error {
	n := len(images)
	switch {
	case cap.ImageSlots == 0 && n > 0:
		return fmt.Errorf("%s 该端点不支持图片输入", field)
	case cap.ImageSlots > 0 && n != cap.ImageSlots:
		return fmt.Errorf("%s 需要恰好 %d 张图，收到 %d 张", field, cap.ImageSlots, n)
	case cap.ImageSlots < 0 && (n < 1 || n > 7):
		return fmt.Errorf("%s 需 1-7 张图，收到 %d 张", field, n)
	}
	return nil
}

// baseValidate 公共校验（时长/分辨率/比例）。
func baseValidate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	if err := validateDuration(getInt(p, "duration"), cap, "时长 duration"); err != nil {
		return err
	}
	if err := validateEnum(getString(p, "resolution"), cap.Resolutions, "分辨率 resolution"); err != nil {
		return err
	}
	if err := validateEnum(getString(p, "aspect_ratio"), cap.AspectRatios, "比例 aspect_ratio"); err != nil {
		return err
	}
	return nil
}

// ensureStringParam 请求体组装辅助（默认值补全）。
func ensureStringParam(body map[string]any, key string, p entity.GenerationParams, fallback string) {
	v := getString(p, key)
	if v == "" {
		v = fallback
	}
	body[key] = v
}
