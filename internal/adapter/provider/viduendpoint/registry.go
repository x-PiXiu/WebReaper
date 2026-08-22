package viduendpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// Registry 是 port.EndpointRegistry 的 Vidu 端点注册表实现。
//
// 设计（数据库驱动·全局掌控——用户要求所有模型/端点/参数管理后台动态控制）：
//   - **端点策略**（Validate/BuildRequest 行为）在代码注册——端点是行为，不可配置化
//   - **模型与能力**（generation_specs 表）为唯一事实源：
//       ① 首次启动 seed：代码默认能力表写入 DB（出厂默认值）
//       ② 查询 = DB 优先（30s TTL 缓存——管理后台改完 30s 生效，不重启）
//       ③ DB 删除行 = 恢复代码默认（出厂值回退）
//       ④ 新增模型 = 管理后台直接插入一行（端点组装逻辑与模型名无关）
//       ⑤ Enabled=false = 模型停用（下拉隐藏 + 提交拒绝）
type Registry struct {
	adapters map[string]port.EndpointAdapter
	// 代码默认能力表（seed 源 + DB 删除行后的回退）
	defaultCaps map[string][]entity.ModelCapability // key: subType
	// specRepo DB 事实源（nil=纯代码模式，测试用）
	specRepo port.GenerationSpecRepository
	// 30s TTL 缓存
	mu        sync.Mutex
	cachedAt  time.Time
	cacheList []entity.GenerationSpec
}

// NewRegistry 创建注册表并注册内置端点策略与默认能力表。
func NewRegistry() *Registry {
	r := &Registry{adapters: map[string]port.EndpointAdapter{}, defaultCaps: map[string][]entity.ModelCapability{}}
	r.register(text2videoAdapter{})
	r.register(img2videoAdapter{})
	r.register(startEnd2videoAdapter{})
	r.register(reference2videoAdapter{})
	r.register(multiframeAdapter{})
	r.register(digitalHumanAdapter{})
	r.register(subjectAdapter{})
	r.register(lipSyncAdapter{})
	// 媒体端点（P1：图片+音频全端点）
	r.register(text2imageAdapter{})
	r.register(text2audioAdapter{})
	r.register(soundEffectAdapter{})
	r.register(ttsAdapter{})
	r.register(voiceCloneAdapter{})
	// 出厂默认能力表（seed 源——数据源：Vidu端点完整参数限制.md + 各端点文档）
	r.defaultCaps["text2video"] = text2videoCaps
	r.defaultCaps["img2video"] = img2videoCaps
	r.defaultCaps["start_end2video"] = startEnd2videoCaps
	r.defaultCaps["reference2video"] = reference2videoCaps
	r.defaultCaps["multiframe"] = multiframeCaps
	r.defaultCaps["digital_human"] = digitalHumanCaps
	r.defaultCaps["text2image"] = text2imageCaps
	r.defaultCaps["text2audio"] = text2audioCaps
	r.defaultCaps["sound_effect"] = soundEffectCaps
	r.defaultCaps["tts"] = ttsCaps
	r.defaultCaps["voice_clone"] = voiceCloneCaps
	r.defaultCaps["lip_sync"] = lipSyncCaps
	r.defaultCaps["lip_sync"] = lipSyncCaps
	// subject 端点无模型概念（Vidu 主体 API 不需要 model 参数）——注册单条默认能力，
	// model="" 时自动匹配；不传或传 "default" 均通过
	r.defaultCaps["subject"] = []entity.ModelCapability{{Model: "default"}}
	return r
}

// SetSpecRepo 注入规格仓储（数据库驱动开关；nil=纯代码模式）。
func (r *Registry) SetSpecRepo(repo port.GenerationSpecRepository) {
	r.specRepo = repo
	r.mu.Lock()
	r.cacheList = nil // 清缓存
	r.mu.Unlock()
}

// ClosedDefaultModes 收敛后默认关闭的模式（08 计划 D1：傻瓜式定位只保留
// subject/reference2video/lip_sync/tts/voice_clone 五个端点）。
// seed 写 Enabled=false；存量部署经管理后台"应用推荐档位"一键收敛
//（HandleApplyRecommendedModes）——不启动时偷偷改运营已配置的状态。
var ClosedDefaultModes = map[string]bool{
	"text2video": true, "img2video": true, "start_end2video": true, "multiframe": true,
	"digital_human": true, "text2image": true, "text2audio": true, "sound_effect": true,
}

// SeedDefaults 首次启动 seed：DB 为空时写入代码默认能力（保留运营已有修改）。
func (r *Registry) SeedDefaults(ctx context.Context) error {
	if r.specRepo == nil {
		return nil
	}
	existing, err := r.specRepo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("读取 generation_specs 失败: %w", err)
	}
	if len(existing) > 0 {
		return nil // 已有数据（运营改过）——不覆盖
	}
	// 写入代码默认（含 subject 端点——无能力约束，仅端点路径）
	var specs []entity.GenerationSpec
	for subType, caps := range r.defaultCaps {
		for _, c := range caps {
			capsJSON, _ := json.Marshal(c)
			specs = append(specs, entity.GenerationSpec{
				SubType: subType, Model: c.Model, Endpoint: endpointOf(subType),
				// D1 收敛：关闭清单内模式 seed 即 disabled（admin 可 reopen）
				Enabled: !ClosedDefaultModes[subType], CapabilitiesJSON: string(capsJSON),
			})
		}
	}
	for _, s := range specs {
		if err := r.specRepo.Upsert(ctx, s); err != nil {
			return fmt.Errorf("seed generation_specs %s/%s: %w", s.SubType, s.Model, err)
		}
	}
	return nil
}

// specs 取全量规格（DB 优先 + 30s TTL 缓存 + 代码默认回退合并）。
func (r *Registry) specs(ctx context.Context) []entity.GenerationSpec {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.specRepo != nil {
		if r.cacheList == nil || time.Since(r.cachedAt) > 30*time.Second {
			if list, err := r.specRepo.ListAll(ctx); err == nil {
				r.cacheList = list
				r.cachedAt = time.Now()
			}
		}
		if r.cacheList != nil {
			return r.cacheList
		}
	}
	// 纯代码模式：把默认表转成 spec 列表（只读）
	var out []entity.GenerationSpec
	for subType, caps := range r.defaultCaps {
		for _, c := range caps {
			out = append(out, entity.GenerationSpec{
				SubType: subType, Model: c.Model, Endpoint: endpointOf(subType), Enabled: true,
			})
		}
	}
	return out
}

func (r *Registry) register(a port.EndpointAdapter) {
	r.adapters[a.Type()] = a
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

// Capability 取模型能力：DB 覆盖行优先；DB 无该行 → 代码默认回退（出厂值）。
func (r *Registry) Capability(ctx context.Context, subType, model string) (entity.ModelCapability, error) {
	for _, s := range r.specs(ctx) {
		if s.SubType == subType && s.Model == model {
			if !s.Enabled {
				return entity.ModelCapability{}, fmt.Errorf("模型 %q 已停用（管理后台可重新启用）", model)
			}
			if s.CapabilitiesJSON != "" {
				var c entity.ModelCapability
				if json.Unmarshal([]byte(s.CapabilitiesJSON), &c) == nil && c.Model != "" {
					return c, nil
				}
			}
			// 无能力 JSON（如 subject 端点）：返回最小能力（仅模型名）
			return entity.ModelCapability{Model: model}, nil
		}
	}
	// 回退代码默认（DB 删除行 = 恢复出厂）
	for _, c := range r.defaultCaps[subType] {
		if c.Model == model {
			return c, nil
		}
	}
	// model 为空且端点只有一条默认能力（如 subject 的"default"）——自动匹配。
	// 设计：subject 等无模型概念的端点，前端不传 model 也能工作
	if model == "" && len(r.defaultCaps[subType]) == 1 {
		return r.defaultCaps[subType][0], nil
	}
	return entity.ModelCapability{}, fmt.Errorf("模型 %q 未在该端点注册（可在管理后台新增）", model)
}

// Models 某端点可用模型（DB 全量 enabled + 代码默认合并去重；subject 端点无模型约束）。
func (r *Registry) Models(ctx context.Context, subType string) ([]string, error) {
	if _, ok := r.adapters[subType]; !ok {
		return nil, fmt.Errorf("端点 %q 未注册", subType)
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range r.specs(ctx) {
		if s.SubType == subType && s.Enabled && !seen[s.Model] {
			seen[s.Model] = true
			out = append(out, s.Model)
		}
	}
	for _, c := range r.defaultCaps[subType] {
		if !seen[c.Model] {
			seen[c.Model] = true
			out = append(out, c.Model)
		}
	}
	sort.Strings(out)
	return out, nil
}

// AllSpecs 全量规格视图（管理后台：含代码默认回退条目——DB 行 + 未覆盖的出厂值）。
// 返回的 Enabled 为实际生效值；HasOverride 标记该行是否 DB 覆盖。
func (r *Registry) AllSpecs(ctx context.Context) []entity.GenerationSpec {
	dbList := r.specs(ctx)
	dbMap := map[string]entity.GenerationSpec{}
	for _, s := range dbList {
		dbMap[s.SubType+"|"+s.Model] = s
	}
	var out []entity.GenerationSpec
	for subType, caps := range r.defaultCaps {
		for _, c := range caps {
			key := subType + "|" + c.Model
			if db, ok := dbMap[key]; ok {
				out = append(out, db)
				delete(dbMap, key)
			} else {
				capsJSON, _ := json.Marshal(c)
				out = append(out, entity.GenerationSpec{
					SubType: subType, Model: c.Model, Endpoint: endpointOf(subType),
					Enabled: !ClosedDefaultModes[subType], CapabilitiesJSON: string(capsJSON),
				})
			}
		}
	}
	// DB 新增的模型（代码默认表之外的）
	for _, s := range dbMap {
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SubType != out[j].SubType {
			return out[i].SubType < out[j].SubType
		}
		return out[i].Model < out[j].Model
	})
	return out
}

// endpointOf 端点路径（与策略的 Endpoint() 一致）。
func endpointOf(subType string) string {
	if a, ok := registryAdapterPaths[subType]; ok {
		return a
	}
	return ""
}

// registryAdapterPaths 端点路径表（seed 用；与各策略 Endpoint() 方法保持一致）。
var registryAdapterPaths = map[string]string{
	"text2video":      "/ent/v2/text2video",
	"img2video":       "/ent/v2/img2video",
	"start_end2video": "/ent/v2/start-end2video",
	"reference2video": "/ent/v2/reference2video",
	"multiframe":      "/ent/v2/multiframe",
	"digital_human":   "/ent/v2/digital-human",
	"subject":         "/ent/v2/subjects",
	"text2image":      "/ent/v2/reference2image",
	"text2audio":      "/ent/v2/text2audio",
	"sound_effect":    "/ent/v2/timing2audio",
	"tts":             "/ent/v2/audio-tts",
	"voice_clone":     "/ent/v2/audio-clone",
	"lip_sync":        "/ent/v2/lip-sync",
}
