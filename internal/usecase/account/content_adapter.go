package account

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"unicode/utf8"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// PlatformContentConfig 平台内容配置
type PlatformContentConfig struct {
	Platform           string
	MaxTitleLength     int
	MaxDescriptionLength int
	MaxTagCount        int
	AllowEmoji         bool
	EmojiDensity       float64 // Emoji 密度限制（0-1）
	MaxNewLines        int     // 换行限制（0=不限）
	RequireCTA         bool    // 是否需要行动号召
	DefaultTags        []string
	CTATemplates       []string
}

// DefaultPlatformConfigs 默认平台内容配置
var DefaultPlatformConfigs = map[string]PlatformContentConfig{
	"douyin": {
		Platform:           "douyin",
		MaxTitleLength:     30,
		MaxDescriptionLength: 2000,
		MaxTagCount:        3,
		AllowEmoji:         true,
		EmojiDensity:       0.05,
		MaxNewLines:        5,
		RequireCTA:         true,
		DefaultTags:        []string{"#推荐", "#种草", "#好物"},
		CTATemplates:       []string{"\n\n👍 觉得有用点个赞吧", "\n\n❤️ 喜欢的话关注我", "\n\n💬 评论区见"},
	},
	"kuaishou": {
		Platform:           "kuaishou",
		MaxTitleLength:     20,
		MaxDescriptionLength: 1500,
		MaxTagCount:        2,
		AllowEmoji:         false,
		MaxNewLines:        3,
		RequireCTA:         true,
		DefaultTags:        []string{"#推荐", "#好物"},
		CTATemplates:       []string{"\n\n觉得有用点个赞吧", "\n\n喜欢的话关注我"},
	},
	"xiaohongshu": {
		Platform:           "xiaohongshu",
		MaxTitleLength:     20,
		MaxDescriptionLength: 1000,
		MaxTagCount:        0, // 小红书不需要手动加标签
		AllowEmoji:         true,
		EmojiDensity:       0.15,
		MaxNewLines:        10,
		RequireCTA:         false,
		DefaultTags:        []string{"#推荐", "#种草", "#好物"},
	},
	"weixin": {
		Platform:           "weixin",
		MaxTitleLength:     16, // panda 实测：视频号标题 UI 上限 16 字且仅汉字（RPA 层同步清洗）
		MaxDescriptionLength: 50000,
		MaxTagCount:        0,
		AllowEmoji:         false,
		MaxNewLines:        0, // 不限
		RequireCTA:         true,
		DefaultTags:        []string{},
		CTATemplates:       []string{"\n\n觉得有用点个赞吧", "\n\n喜欢的话关注我"},
	},
	"bilibili": {
		Platform:           "bilibili",
		MaxTitleLength:     50,
		MaxDescriptionLength: 5000,
		MaxTagCount:        3,
		AllowEmoji:         true,
		EmojiDensity:       0.05,
		MaxNewLines:        0,
		RequireCTA:         true,
		DefaultTags:        []string{"#推荐", "#好看", "#视频"},
		CTATemplates:       []string{"\n\n👍 觉得有用点个赞吧", "\n\n❤️ 喜欢的话关注我"},
	},
	"youtube": {
		Platform:           "youtube",
		MaxTitleLength:     100, // YouTube 标题上限 100 字符
		MaxDescriptionLength: 5000,
		MaxTagCount:        0, // 标签融进描述（# 前缀无话题语义）
		AllowEmoji:         true,
		EmojiDensity:       0.03,
		MaxNewLines:        0,
		RequireCTA:         false, // 海外平台语境不适配中文 CTA 模板
		DefaultTags:        []string{},
	},
	"zhihu": {
		Platform:           "zhihu",
		MaxTitleLength:     100, // 专栏标题上限（实测）
		MaxDescriptionLength: 100000,
		MaxTagCount:        5, // 话题可选
		AllowEmoji:         true,
		EmojiDensity:       0.02,
		MaxNewLines:        0,
		RequireCTA:         false, // 长文场景不强推 CTA
		DefaultTags:        []string{},
		CTATemplates:       []string{},
	},
}

// DefaultContentAdapter 默认内容适配器实现
type DefaultContentAdapter struct {
	config PlatformContentConfig
}

// NewDefaultContentAdapter 创建默认内容适配器
func NewDefaultContentAdapter(config PlatformContentConfig) *DefaultContentAdapter {
	return &DefaultContentAdapter{config: config}
}

func (a *DefaultContentAdapter) Platform() string {
	return a.config.Platform
}

func (a *DefaultContentAdapter) Adapt(ctx context.Context, req port.AdaptRequest) (*entity.AdaptedContent, error) {
	title := req.Title
	description := req.Description
	tags := req.Tags

	// 1. 适配标题
	title = a.adaptTitle(title)

	// 2. 适配描述
	description = a.adaptDescription(description)

	// 3. 适配标签
	tags = a.adaptTags(tags, description)

	// 4. 添加行动号召
	cta := ""
	if a.config.RequireCTA {
		cta = a.getRandomCTA()
		description = description + cta
	}

	return &entity.AdaptedContent{
		Title:       title,
		Description: description,
		Tags:        tags,
		CTA:         cta,
	}, nil
}

// adaptTitle 适配标题
func (a *DefaultContentAdapter) adaptTitle(title string) string {
	if a.config.MaxTitleLength <= 0 {
		return title
	}
	if utf8.RuneCountInString(title) <= a.config.MaxTitleLength {
		return title
	}
	// 截断并添加省略号
	runes := []rune(title)
	return string(runes[:a.config.MaxTitleLength-1]) + "…"
}

// adaptDescription 适配描述
func (a *DefaultContentAdapter) adaptDescription(description string) string {
	// 长度限制
	if a.config.MaxDescriptionLength > 0 && utf8.RuneCountInString(description) > a.config.MaxDescriptionLength {
		runes := []rune(description)
		description = string(runes[:a.config.MaxDescriptionLength-3]) + "..."
	}

	// 换行限制
	if a.config.MaxNewLines > 0 {
		lines := strings.Split(description, "\n")
		if len(lines) > a.config.MaxNewLines {
			description = strings.Join(lines[:a.config.MaxNewLines], "\n")
		}
	}

	// Emoji 处理
	if !a.config.AllowEmoji {
		description = removeEmoji(description)
	}

	return description
}

// adaptTags 适配标签
func (a *DefaultContentAdapter) adaptTags(tags []string, description string) []string {
	if a.config.MaxTagCount <= 0 {
		return tags
	}

	// 从描述中提取标签
	extractedTags := extractTags(description)

	// 合并标签
	allTags := append(tags, extractedTags...)

	// 去重
	uniqueTags := deduplicateTags(allTags)

	// 如果标签不足，补充默认标签
	if len(uniqueTags) < a.config.MaxTagCount {
		for _, defaultTag := range a.config.DefaultTags {
			if len(uniqueTags) >= a.config.MaxTagCount {
				break
			}
			if !containsTag(uniqueTags, defaultTag) {
				uniqueTags = append(uniqueTags, defaultTag)
			}
		}
	}

	// 截断到最大数量
	if len(uniqueTags) > a.config.MaxTagCount {
		uniqueTags = uniqueTags[:a.config.MaxTagCount]
	}

	return uniqueTags
}

// getRandomCTA 获取随机行动号召
func (a *DefaultContentAdapter) getRandomCTA() string {
	if len(a.config.CTATemplates) == 0 {
		return ""
	}
	return a.config.CTATemplates[rand.Intn(len(a.config.CTATemplates))]
}

// removeEmoji 移除 Emoji
func removeEmoji(s string) string {
	var result strings.Builder
	for _, r := range s {
		if r < 0x1F600 || r > 0x1F64F { // 简化处理：移除常见 Emoji 范围
			result.WriteRune(r)
		}
	}
	return result.String()
}

// extractTags 从文本中提取 #xxx 标签
func extractTags(text string) []string {
	var tags []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		words := strings.Fields(line)
		for _, word := range words {
			if strings.HasPrefix(word, "#") && len(word) > 1 {
				tags = append(tags, word)
			}
		}
	}
	return tags
}

// deduplicateTags 标签去重
func deduplicateTags(tags []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, tag := range tags {
		if !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}
	return result
}

// containsTag 检查标签列表是否包含指定标签
func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// ContentAdapterRegistryImpl 内容适配器注册表实现
type ContentAdapterRegistryImpl struct {
	adapters map[string]port.ContentAdapter
}

// NewContentAdapterRegistryImpl 创建内容适配器注册表
func NewContentAdapterRegistryImpl() *ContentAdapterRegistryImpl {
	return &ContentAdapterRegistryImpl{
		adapters: make(map[string]port.ContentAdapter),
	}
}

// Register 注册适配器
func (r *ContentAdapterRegistryImpl) Register(adapter port.ContentAdapter) {
	r.adapters[adapter.Platform()] = adapter
}

// Get 获取指定平台的适配器
func (r *ContentAdapterRegistryImpl) Get(platform string) (port.ContentAdapter, error) {
	adapter, ok := r.adapters[platform]
	if !ok {
		return nil, fmt.Errorf("平台 %s 的内容适配器未注册", platform)
	}
	return adapter, nil
}

// List 列出所有注册的适配器
func (r *ContentAdapterRegistryImpl) List() []port.ContentAdapter {
	var adapters []port.ContentAdapter
	for _, adapter := range r.adapters {
		adapters = append(adapters, adapter)
	}
	return adapters
}
