// prompt_placeholder.go 统一占位符翻译系统（28号计划）。
//
// 设计动机：
//   - Vidu 参考生视频支持 @主体name 引用已注册主体
//   - 未来其他厂商也可能有类似占位符机制
//   - 当前 translateRefs 只处理素材引用，未处理主体引用
//
// 占位符类型：
//   - @名称（素材引用）：@图片/@音频/@视频 → 翻译为端点参数
//   - @主体name（主体引用）：@主体A → 翻译为 subjects 数组 + 保留 @name
//
// 厂商差异：
//   - Vidu：保留 @name 在 prompt 中，添加 subjects 数组
//   - 其他厂商：移除 @name，只保留纯文本
package generation

import (
	"context"
	"regexp"
	"strings"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// placeholderRegexp 匹配 @名称 占位符（@后跟非空白非@字符）。
var placeholderRegexp = regexp.MustCompile(`@([^\s@]+)`)

// SubjectRef 主体引用（翻译后的中间结构）。
type SubjectRef struct {
	Name     string
	ServerID string // 已注册主体的 ID（从资产表查询）
}

// ProviderPlaceholderRule 厂商占位符规则。
type ProviderPlaceholderRule struct {
	// KeepSubjectRef 是否在 prompt 中保留 @name（Vidu=true，其他厂商可能=false）。
	KeepSubjectRef bool
}

// PlaceholderTranslator 占位符翻译器（用例层组件）。
//
// 整洁架构定位：
//   - 用例层组件，不依赖具体厂商实现
//   - 通过 ProviderPlaceholderRule 注册表支持厂商差异
//   - 依赖 port.SubjectAssetRepository 查询已注册主体
type PlaceholderTranslator struct {
	subjectAssetRepo port.SubjectAssetRepository
	providerRules    map[string]ProviderPlaceholderRule
}

// NewPlaceholderTranslator 创建占位符翻译器。
func NewPlaceholderTranslator(repo port.SubjectAssetRepository) *PlaceholderTranslator {
	return &PlaceholderTranslator{
		subjectAssetRepo: repo,
		providerRules:    defaultProviderRules(),
	}
}

// defaultProviderRules 默认厂商规则。
func defaultProviderRules() map[string]ProviderPlaceholderRule {
	return map[string]ProviderPlaceholderRule{
		"vidu": {KeepSubjectRef: true}, // Vidu 需要 @name 引用
		// 新厂商在此注册规则
	}
}

// RegisterRule 注册厂商占位符规则（运行时扩展）。
func (t *PlaceholderTranslator) RegisterRule(provider string, rule ProviderPlaceholderRule) {
	if t.providerRules == nil {
		t.providerRules = make(map[string]ProviderPlaceholderRule)
	}
	t.providerRules[provider] = rule
}

// SetSubjectAssetRepo 注入主体资产仓储（延迟注入）。
func (t *PlaceholderTranslator) SetSubjectAssetRepo(repo port.SubjectAssetRepository) {
	t.subjectAssetRepo = repo
}

// Translate 翻译占位符（核心方法）。
//
// 流程：
//   ① 解析 prompt 中的 @名称 占位符
//   ② 分类：素材引用（refs 中有对应）vs 主体引用（refs 中无对应）
//   ③ 素材引用：调用 translateRefs 翻译为端点参数
//   ④ 主体引用：按厂商规则处理（Vidu 保留 @name + subjects 数组，其他移除 @name）
//
// 参数：
//   - provider: 厂商名称（vidu/xiaomi-mimo/...）
//   - subType: 端点类型（reference2video/lip_sync/...）
//   - params: 生成参数（含 prompt）
//   - refs: 素材引用清单（@图片/@音频/@视频）
//   - tenantID: 租户ID（查询已注册主体用）
func (t *PlaceholderTranslator) Translate(
	ctx context.Context,
	provider string,
	subType string,
	params entity.GenerationParams,
	refs []entity.PromptRef,
	tenantID string,
) (entity.GenerationParams, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return params, nil
	}

	// ① 解析 @名称 占位符
	matches := placeholderRegexp.FindAllStringSubmatch(prompt, -1)
	if len(matches) == 0 && len(refs) == 0 {
		return params, nil
	}

	// ② 收集占位符名称
	placeholderNames := make(map[string]bool)
	for _, m := range matches {
		if len(m) > 1 {
			placeholderNames[m[1]] = true
		}
	}

	// ③ 分类：素材引用 vs 主体引用
	var materialRefs []entity.PromptRef
	var subjectNames []string

	// 从 refs 中匹配素材引用
	for _, r := range refs {
		if placeholderNames[r.Name] {
			materialRefs = append(materialRefs, r)
			delete(placeholderNames, r.Name)
		}
	}

	// 剩余的占位符可能是主体引用
	for name := range placeholderNames {
		subjectNames = append(subjectNames, name)
	}

	// ④ 处理素材引用（复用现有 translateRefs）
	if len(materialRefs) > 0 {
		// 获取端点能力（用于翻译规则）
		var cap entity.ModelCapability
		if t.subjectAssetRepo != nil {
			// 尝试从 registry 获取能力（如果有）
		}
		params, _ = translateRefs(subType, cap, params, materialRefs)
	}

	// ⑤ 处理主体引用（按厂商规则）
	if len(subjectNames) > 0 {
		rule := t.providerRules[provider]

		if rule.KeepSubjectRef {
			// Vidu 模式：保留 @name，添加 subjects 数组
			subjects := t.buildSubjectsArray(ctx, tenantID, subjectNames)
			if len(subjects) > 0 {
				params["subjects"] = subjects
			}
			// prompt 中的 @name 保留不变（Vidu 内部处理）
		} else {
			// 其他厂商模式：移除 @name，只保留纯文本
			currentPrompt, _ := params["prompt"].(string)
			for _, name := range subjectNames {
				currentPrompt = strings.ReplaceAll(currentPrompt, "@"+name, name)
			}
			params["prompt"] = currentPrompt
		}
	}

	return params, nil
}

// buildSubjectsArray 构建 subjects 数组（查询已注册主体的 server_id）。
func (t *PlaceholderTranslator) buildSubjectsArray(ctx context.Context, tenantID string, names []string) []any {
	if t.subjectAssetRepo == nil {
		return nil
	}

	// 查询该租户的所有主体资产
	assets, _, err := t.subjectAssetRepo.ListByTenant(ctx, tenantID, "", "", 100, 0)
	if err != nil {
		return nil
	}

	// 建立 name → server_id 映射
	nameToServerID := make(map[string]string)
	for _, a := range assets {
		if a.Name != "" && a.ServerID != "" {
			nameToServerID[a.Name] = a.ServerID
		}
	}

	// 构建 subjects 数组
	var subjects []any
	for _, name := range names {
		serverID := nameToServerID[name]
		if serverID == "" {
			// 未找到已注册主体，跳过（或可以报错）
			continue
		}
		subjects = append(subjects, map[string]any{
			"name":      name,
			"server_id": serverID,
		})
	}

	return subjects
}

// ParseSubjectNamesFromPrompt 从 prompt 中解析主体名称（工具函数）。
func ParseSubjectNamesFromPrompt(prompt string) []string {
	matches := placeholderRegexp.FindAllStringSubmatch(prompt, -1)
	var names []string
	for _, m := range matches {
		if len(m) > 1 {
			names = append(names, m[1])
		}
	}
	return names
}
