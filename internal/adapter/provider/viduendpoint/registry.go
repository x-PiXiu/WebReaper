package viduendpoint

import (
	"context"
	"fmt"
	"sort"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// Registry 是 port.EndpointRegistry 的 Vidu 端点注册表实现。
//
// 设计：端点策略在 init 时自注册（注册表模式）——新增端点 = 新增策略文件 +
// 注册一行，开闭原则。能力向量默认内置于代码（类型安全），generation_specs
// 表的 JSON 覆盖优先（管理后台热更新运营策略）。
type Registry struct {
	adapters map[string]port.EndpointAdapter
	caps     map[string][]entity.ModelCapability // key: subType
	// specOverrides 管理后台覆盖（subType|model → 覆盖后的能力；注入可为 nil）
	specOverrides func(ctx context.Context, subType, model string) (*entity.ModelCapability, error)
}

// NewRegistry 创建注册表并注册内置端点策略与能力向量表。
func NewRegistry() *Registry {
	r := &Registry{adapters: map[string]port.EndpointAdapter{}, caps: map[string][]entity.ModelCapability{}}
	r.register(text2videoAdapter{})
	r.register(img2videoAdapter{})
	r.register(startEnd2videoAdapter{})
	r.register(reference2videoAdapter{})
	r.register(multiframeAdapter{})
	r.register(digitalHumanAdapter{})
	r.register(subjectAdapter{})
	// 能力向量表（数据源：Vidu端点完整参数限制.md + 各端点文档，交叉核对模型参数对照表）
	r.registerCapabilities("text2video", text2videoCaps)
	r.registerCapabilities("img2video", img2videoCaps)
	r.registerCapabilities("start_end2video", startEnd2videoCaps)
	r.registerCapabilities("reference2video", reference2videoCaps)
	r.registerCapabilities("multiframe", multiframeCaps)
	r.registerCapabilities("digital_human", digitalHumanCaps)
	return r
}

// SetSpecOverrides 注入管理后台覆盖源（可选；nil=只用代码默认能力）。
func (r *Registry) SetSpecOverrides(fn func(ctx context.Context, subType, model string) (*entity.ModelCapability, error)) {
	r.specOverrides = fn
}

func (r *Registry) register(a port.EndpointAdapter) {
	r.adapters[a.Type()] = a
}

// registerCapabilities 注册某端点的能力向量表（策略文件内调用）。
func (r *Registry) registerCapabilities(subType string, caps []entity.ModelCapability) {
	r.caps[subType] = caps
}

func (r *Registry) Get(ctx context.Context, subType string) (port.EndpointAdapter, error) {
	a, ok := r.adapters[subType]
	if !ok {
		return nil, fmt.Errorf("端点 %q 未注册", subType)
	}
	return a, nil
}

func (r *Registry) Types() []string {
	out := make([]string, 0, len(r.adapters))
	for t := range r.adapters {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// Capability 取模型能力：spec 表覆盖优先，代码默认兜底。
func (r *Registry) Capability(ctx context.Context, subType, model string) (entity.ModelCapability, error) {
	if r.specOverrides != nil {
		if ov, err := r.specOverrides(ctx, subType, model); err == nil && ov != nil {
			return *ov, nil
		}
	}
	caps := r.caps[subType]
	return capabilityFor(caps, model)
}

// Models 某端点可用模型列表（spec 覆盖后以覆盖为准；简化：取代码默认表）。
func (r *Registry) Models(ctx context.Context, subType string) ([]string, error) {
	caps := r.caps[subType]
	if len(caps) == 0 {
		return nil, fmt.Errorf("端点 %q 无能力注册", subType)
	}
	out := make([]string, 0, len(caps))
	for _, c := range caps {
		out = append(out, c.Model)
	}
	return out, nil
}

// HasModel 判断模型是否在该端点注册（防重复提交校验用）。
func (r *Registry) HasModel(ctx context.Context, subType, model string) bool {
	_, err := r.Capability(ctx, subType, model)
	return err == nil
}
