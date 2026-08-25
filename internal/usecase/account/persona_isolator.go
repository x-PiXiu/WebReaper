package account

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// contextKey 类型安全的 context key
type contextKey string

const (
	// TenantIDKey 租户 ID 的 context key
	TenantIDKey contextKey = "tenant_id"
)

// TenantIDFromContext 从 context 获取租户 ID
func TenantIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(TenantIDKey).(string); ok {
		return v
	}
	return ""
}

// DefaultPersonaIsolator 默认人设隔离器实现
type DefaultPersonaIsolator struct {
	personaRepo port.PersonaRepository
}

// NewDefaultPersonaIsolator 创建默认人设隔离器
func NewDefaultPersonaIsolator(personaRepo port.PersonaRepository) *DefaultPersonaIsolator {
	return &DefaultPersonaIsolator{
		personaRepo: personaRepo,
	}
}

func (p *DefaultPersonaIsolator) Isolate(ctx context.Context, content string, personaID string) (string, error) {
	if personaID == "" {
		return content, nil
	}

	// 从 context 获取租户 ID
	tenantID := TenantIDFromContext(ctx)

	// 获取人设配置
	persona, err := p.personaRepo.FindByID(ctx, tenantID, personaID)
	if err != nil {
		return content, err
	}
	if persona == nil {
		return content, nil
	}

	// 1. 禁用词过滤
	content = p.filterBannedWords(content, persona.BannedWords)

	// 2. 语气风格调整
	content = p.applyToneStyle(content, persona.ToneStyle)

	// 3. 追加偏好标签
	content = p.appendPreferredTags(content, persona.PreferredTags)

	return content, nil
}

func (p *DefaultPersonaIsolator) GetPersona(ctx context.Context, personaID string) (*entity.Persona, error) {
	return p.personaRepo.FindByID(ctx, "", personaID)
}

// filterBannedWords 过滤禁用词
func (p *DefaultPersonaIsolator) filterBannedWords(content string, bannedWords []string) string {
	for _, word := range bannedWords {
		content = strings.ReplaceAll(content, word, "***")
	}
	return content
}

// applyToneStyle 应用语气风格
func (p *DefaultPersonaIsolator) applyToneStyle(content string, toneStyle string) string {
	switch toneStyle {
	case entity.ToneStyleLively:
		prefixes := []string{"哈哈，", "太棒了，", "绝绝子，", "爱了爱了，"}
		return prefixes[rand.Intn(len(prefixes))] + content
	case entity.ToneStyleProfessional:
		return "从专业角度来看，" + content
	case entity.ToneStyleWarm:
		return "大家好，" + content
	case entity.ToneStyleSteady:
		return "需要注意的是，" + content
	default:
		return content
	}
}

// appendPreferredTags 追加偏好标签
func (p *DefaultPersonaIsolator) appendPreferredTags(content string, preferredTags []string) string {
	if len(preferredTags) == 0 {
		return content
	}

	// 取前2个标签
	tags := preferredTags
	if len(tags) > 2 {
		tags = tags[:2]
	}

	// 检查内容中是否已包含这些标签
	var newTags []string
	for _, tag := range tags {
		if !strings.Contains(content, tag) {
			newTags = append(newTags, tag)
		}
	}

	if len(newTags) > 0 {
		content = content + " " + strings.Join(newTags, " ")
	}

	return content
}

// PersonaRepositoryImpl 人设仓储实现（内存版本，用于测试）
type PersonaRepositoryImpl struct {
	personas map[string]*entity.Persona
}

// NewPersonaRepositoryImpl 创建人设仓储实现
func NewPersonaRepositoryImpl() *PersonaRepositoryImpl {
	return &PersonaRepositoryImpl{
		personas: make(map[string]*entity.Persona),
	}
}

func (r *PersonaRepositoryImpl) FindByID(ctx context.Context, tenantID, id string) (*entity.Persona, error) {
	persona, ok := r.personas[id]
	if !ok {
		return nil, nil
	}
	return persona, nil
}

func (r *PersonaRepositoryImpl) ListByTenant(ctx context.Context, tenantID string) ([]entity.Persona, error) {
	var personas []entity.Persona
	for _, p := range r.personas {
		personas = append(personas, *p)
	}
	return personas, nil
}

func (r *PersonaRepositoryImpl) Save(ctx context.Context, persona *entity.Persona) error {
	r.personas[persona.ID] = persona
	return nil
}

func (r *PersonaRepositoryImpl) Delete(ctx context.Context, tenantID, id string) error {
	delete(r.personas, id)
	return nil
}

// Get 获取人设（辅助方法）
func (r *PersonaRepositoryImpl) Get(id string) (*entity.Persona, error) {
	persona, ok := r.personas[id]
	if !ok {
		return nil, fmt.Errorf("人设 %s 不存在", id)
	}
	return persona, nil
}
