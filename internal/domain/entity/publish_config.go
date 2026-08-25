package entity

import "time"

// BrandPublishConfig 品牌发布配置
// 每个品牌在每个平台可以有不同的发布配置（账号绑定、限速、默认标签等）
type BrandPublishConfig struct {
	ID             string
	TenantID       string
	BrandID        string
	Platform       string
	AccountIDs     []string
	RateLimit      RateLimit
	DefaultTags    []string
	DefaultPersona string
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// RateLimit 限速配置
type RateLimit struct {
	MaxPerDay   int `json:"max_per_day"`   // 每日最大发布数
	MaxPerHour  int `json:"max_per_hour"`  // 每小时最大发布数
	MinInterval int `json:"min_interval"`  // 最小间隔（秒）
}

// DefaultRateLimits 默认限速配置
var DefaultRateLimits = map[string]RateLimit{
	"douyin":      {MaxPerDay: 5, MaxPerHour: 2, MinInterval: 1800},
	"kuaishou":    {MaxPerDay: 5, MaxPerHour: 2, MinInterval: 1800},
	"xiaohongshu": {MaxPerDay: 3, MaxPerHour: 1, MinInterval: 3600},
	"weixin":      {MaxPerDay: 5, MaxPerHour: 2, MinInterval: 1800},
	"bilibili":    {MaxPerDay: 3, MaxPerHour: 1, MinInterval: 3600},
}

// AccountBrandBinding 账号品牌绑定
// 一个账号可以绑定到多个品牌，一个品牌可以绑定多个账号
type AccountBrandBinding struct {
	ID        string
	TenantID  string
	AccountID string
	BrandID   string
	Platform  string
	IsDefault bool
	CreatedAt time.Time
}

// PublishUsageStat 发布使用量统计
// 按品牌、平台、日期统计发布次数，用于限速控制
type PublishUsageStat struct {
	ID            string
	TenantID      string
	BrandID       string
	Platform      string
	PublishDate   time.Time
	UsageCount    int
	LastPublishAt *time.Time
}

// AdaptedContent 适配后的内容
type AdaptedContent struct {
	Title       string   // 适配后的标题
	Description string   // 适配后的描述
	Tags        []string // 适配后的标签
	CTA         string   // 行动号召（Call To Action）
}

// Persona 人设配置
type Persona struct {
	ID              string
	TenantID        string
	Name            string   // 人设名称
	Type            string   // 人设类型：beauty/knowledge/ecommerce/life/fitness/food/baby/travel
	ToneStyle       string   // 语气风格：lively/professional/warm/steady
	BannedWords     []string // 禁用词列表
	PreferredTags   []string // 偏好话题标签
	AvatarID        string   // 数字人形象ID
	VoiceID         string   // 音色ID
	BackgroundID    string   // 背景ID
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// PersonaType 人设类型常量
const (
	PersonaTypeBeauty     = "beauty"     // 美妆
	PersonaTypeKnowledge  = "knowledge"  // 知识
	PersonaTypeEcommerce  = "ecommerce"  // 电商
	PersonaTypeLife       = "life"       // 生活
	PersonaTypeFitness    = "fitness"    // 健身
	PersonaTypeFood       = "food"       // 美食
	PersonaTypeBaby       = "baby"       // 母婴
	PersonaTypeTravel     = "travel"     // 旅游
)

// ToneStyle 语气风格常量
const (
	ToneStyleLively     = "lively"      // 活泼
	ToneStyleProfessional = "professional" // 专业
	ToneStyleWarm       = "warm"        // 亲切
	ToneStyleSteady     = "steady"      // 沉稳
)
