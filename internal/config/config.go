// Package config 提供统一的配置加载。
//
// 整洁架构定位：本包属于基础设施层，负责把外部配置（.env 文件/环境变量）
// 加载为强类型的 Config 对象，供 cmd 装配时使用。
// domain/usecase 层不依赖本包（依赖方向：config 只被 cmd 和 adapter 使用）。
//
// 设计要点：
//   - .env 文件加载（godotenv）+ 环境变量覆盖
//   - 缺失项给合理默认值，保证无 .env 也能降级启动
//   - DSN() 等方法封装格式化逻辑，避免散落各处
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"webreaper/internal/domain/entity"
)

// Config 是应用的全局配置根。
type Config struct {
	Server     ServerConfig
	DB         DBConfig
	LLM        LLMConfig
	Embedding  EmbeddingConfig
	Milvus     MilvusConfig
	Redis      RedisConfig
	JWT        JWTConfig
	Publish    PublishConfig
	AgentCore  AgentCoreConfig
	Crawler    CrawlerConfig
	Telemetry  TelemetryConfig
	Tavily     TavilyConfig
}

// TelemetryConfig 链路追踪配置（OpenTelemetry）。
//
// Enabled 控制 trace 是否启用（开发默认开、生产可关）。
// Exporter 选 stdout（控制台，开发）或 otlp（发往 Collector，生产）。
// OTLPEndpoint 仅 otlp 生效，如 localhost:4318（OTLP/HTTP 默认端口）。
type TelemetryConfig struct {
	Enabled      bool
	Exporter     string // "stdout" | "otlp"
	OTLPEndpoint string // 如 localhost:4318
}

// CrawlerConfig 爬虫限流配置（可从 .env 动态调配）。
type CrawlerConfig struct {
	RequestIntervalMs int    // 请求间隔（毫秒），默认 1000
	RequestTimeoutMs  int    // 单请求超时（毫秒），默认 30000
	UserAgent         string // 自定义 User-Agent，默认 WebReaper/1.0
}

// ToPolicy 把配置转为领域层的 CrawlPolicy 值对象。
func (c CrawlerConfig) ToPolicy() entity.CrawlPolicy {
	interval := c.RequestIntervalMs
	if interval <= 0 {
		interval = 1000
	}
	timeout := c.RequestTimeoutMs
	if timeout <= 0 {
		timeout = 30000
	}
	return entity.CrawlPolicy{
		RequestIntervalMs: interval,
		RequestTimeoutMs:  timeout,
		RespectRobots:     true,
	}
}

// AgentCoreConfig 是 AgentCore 目标系统的配置（爬虫采集 AgentCore 会话用）。
type AgentCoreConfig struct {
	BaseURL     string // AgentCore 后端地址，如 http://localhost:8081
	AdminToken  string // 采集用的认证 token（AgentCore 的 JWT 或 API Key）
}

// IsConfigured 判断 AgentCore 配置是否就绪。
func (c AgentCoreConfig) IsConfigured() bool {
	return c.BaseURL != "" && c.AdminToken != ""
}

// PublishConfig 推送目标平台配置（真实推送接线用）。
type PublishConfig struct {
	APIKey        string // 目标平台的 X-API-Key
	BaseURL       string // 如 https://agentcore.example.com
	ArticlePath   string // 文章推送路径，如 /api/v1/ingest/article
	QuestionPath  string // 面试题推送路径，如 /api/v1/ingest/question
	MaxRetries    int    // HTTP 推送失败重试次数（仅对 5xx/429/网络错误），默认 3
	CookieSecret  string // 多平台发布 cookie 加密密钥（AES-GCM，从 PUBLISH_COOKIE_SECRET 读取）
	QRLoginHeaded bool   // 扫码登录是否显示浏览器窗口（调试用，生产保持 false 走灰盒 headless）
}

// IsConfigured 判断推送平台是否已配置（API Key + BaseURL 非空）。
func (c PublishConfig) IsConfigured() bool {
	return c.APIKey != "" && c.BaseURL != ""
}

// ArticleURL 完整的文章推送 URL。
func (c PublishConfig) ArticleURL() string { return c.BaseURL + c.ArticlePath }

// QuestionURL 完整的面试题推送 URL。
func (c PublishConfig) QuestionURL() string { return c.BaseURL + c.QuestionPath }

// ServerConfig 服务配置。
type ServerConfig struct {
	Port string // 监听端口，默认 8082
	Env  string // 运行环境：development / production
	// PublicBaseURL 公开内容站的根地址（生成 sitemap/llms.txt/JSON-LD 的绝对 URL）。
	// 生产环境必须是公网可达的地址（如 https://content.example.com）。
	PublicBaseURL string
	// IndexNowKey IndexNow 收录提交密钥（可选）。
	// 配置后：内容发布为 published 时自动通知 Bing/Yandex 收录，替代人工提交 sitemap。
	// Key 文件由公开站端点 /public/indexnow-key.txt 托管（keyLocation 指向它）。
	IndexNowKey string
}

// DBConfig MySQL 数据库配置。
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// DSN 构造 GORM/MySQL 驱动所需的 DSN。
// WSL 下 MySQL 绑定 IPv6 [::1]，host 用 localhost 时驱动会自动解析。
func (c DBConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.Name)
}

// IsConfigured 判断数据库是否已配置（密码非空视为已配置）。
// 未配置时 main 会降级使用内存 mock 仓储。
func (c DBConfig) IsConfigured() bool {
	return c.Password != "" && c.Name != ""
}

// LLMConfig 大语言模型配置（OpenAI 兼容协议，支持 MiniMax/OpenAI/智谱等）。
type LLMConfig struct {
	Provider string // 厂商标识：minimax / openai / zhipu（仅用于日志展示）
	APIKey   string
	BaseURL  string // API 端点，如 https://api.minimaxi.com/v1
	Model    string // 模型名，如 MiniMax-M2.5
}

// IsConfigured 判断 LLM 是否已配置（API Key 非空）。
func (c LLMConfig) IsConfigured() bool {
	return c.APIKey != ""
}

// EmbeddingConfig 向量嵌入模型配置（本轮预留，暂未使用）。
type EmbeddingConfig struct {
	Model   string
	BaseURL string
	APIKey  string
}

// MilvusConfig 向量数据库配置。
type MilvusConfig struct {
	Host           string
	Port           string
	CollectionName string // 集合名（默认 webreaper_vectors）
}

// IsConfigured 判断 Milvus 是否已配置（Host 非空）。
func (c MilvusConfig) IsConfigured() bool {
	return c.Host != ""
}

// Addr 返回 host:port 形式的地址。
func (c MilvusConfig) Addr() string {
	port := c.Port
	if port == "" {
		port = "19530"
	}
	return c.Host + ":" + port
}

// RedisConfig Redis 配置（本轮预留）。
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// TavilyConfig Tavily 搜索 API 配置（GEO 监测的高质量搜索源）。
// Tavily 是专为 AI 设计的搜索 API，返回结构化的干净内容（不需自己抓正文）。
// 不配置则降级到 Bing（WebFetcher）。
type TavilyConfig struct {
	APIKey string
}

func (c TavilyConfig) IsConfigured() bool { return c.APIKey != "" }

// JWTConfig 认证配置（本轮预留）。
type JWTConfig struct {
	Secret     string
	Expiration int // 秒
}

// Load 从 configs/.env 加载配置，环境变量优先级更高（可覆盖 .env）。
//
// 加载策略：
//  1. 尝试加载 configs/.env（文件不存在不报错，降级为纯环境变量）
//  2. 读取各配置项，缺失项用默认值填充
//
// 这让系统既能用 .env 文件配置，也能在无文件时通过环境变量运行（如容器部署）。
func Load() Config {
	// .env 不存在不视为错误——容器化部署常通过环境变量注入而非文件
	_ = godotenv.Load("configs/.env")

	return Config{
		Server: ServerConfig{
			Port:          getenvDefault("SERVER_PORT", "8082"),
			Env:           getenvDefault("APP_ENV", "development"),
			PublicBaseURL: getenvDefault("PUBLIC_BASE_URL", "http://localhost:8082"),
			IndexNowKey:   os.Getenv("INDEXNOW_KEY"),
		},
		DB: DBConfig{
			Host:     getenvDefault("DB_HOST", "localhost"),
			Port:     getenvDefault("DB_PORT", "3306"),
			User:     getenvDefault("DB_USER", "root"),
			Password: os.Getenv("DB_PASSWORD"),
			Name:     getenvDefault("DB_NAME", "agentcore"),
			SSLMode:  getenvDefault("DB_SSLMODE", "disable"),
		},
		LLM: LLMConfig{
			Provider: getenvDefault("LLM_PROVIDER", "minimax"),
			APIKey:   os.Getenv("LLM_API_KEY"),
			BaseURL:  getenvDefault("LLM_BASE_URL", "https://api.minimaxi.com/v1"),
			Model:    getenvDefault("LLM_MODEL", "MiniMax-M2.5"),
		},
		Embedding: EmbeddingConfig{
			Model:   getenvDefault("EMBEDDING_MODEL", ""),
			BaseURL: getenvDefault("EMBEDDING_BASE_URL", ""),
			APIKey:  os.Getenv("EMBEDDING_API_KEY"),
		},
		Milvus: MilvusConfig{
			Host:           getenvDefault("MILVUS_HOST", ""),
			Port:           getenvDefault("MILVUS_PORT", "19530"),
			CollectionName: getenvDefault("MILVUS_COLLECTION", "webreaper_vectors"),
		},
		Redis: RedisConfig{
			Host:     getenvDefault("REDIS_HOST", "localhost"),
			Port:     getenvDefault("REDIS_PORT", "6379"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       getenvInt("REDIS_DB", 0),
		},
		Tavily: TavilyConfig{
			APIKey: os.Getenv("TAVILY_API_KEY"),
		},
		JWT: JWTConfig{
			Secret:     os.Getenv("JWT_SECRET"),
			Expiration: getenvInt("JWT_EXPIRATION", 3600),
		},
		Publish: PublishConfig{
			APIKey:       os.Getenv("INGEST_API_KEY"),
			BaseURL:      getenvDefault("INGEST_BASE_URL", ""),
			ArticlePath:  getenvDefault("INGEST_ARTICLE_PATH", "/api/v1/ingest/article"),
			QuestionPath: getenvDefault("INGEST_QUESTION_PATH", "/api/v1/ingest/question"),
			MaxRetries:    getenvInt("PUBLISH_MAX_RETRIES", 3),
			CookieSecret:  os.Getenv("PUBLISH_COOKIE_SECRET"),
			QRLoginHeaded: getenvBool("QR_LOGIN_HEADED", false),
		},
		AgentCore: AgentCoreConfig{
			BaseURL:    getenvDefault("AGENTCORE_BASE_URL", "http://localhost:8081"),
			AdminToken: os.Getenv("AGENTCORE_ADMIN_TOKEN"),
		},
		Crawler: CrawlerConfig{
			RequestIntervalMs: getenvInt("CRAWLER_REQUEST_INTERVAL_MS", 1000),
			RequestTimeoutMs:  getenvInt("CRAWLER_REQUEST_TIMEOUT_MS", 30000),
			UserAgent:         getenvDefault("CRAWLER_USER_AGENT", "WebReaper/1.0"),
		},
		Telemetry: TelemetryConfig{
			Enabled:      getenvBool("TELEMETRY_ENABLED", true),
			Exporter:     getenvDefault("TELEMETRY_EXPORTER", "stdout"),
			OTLPEndpoint: getenvDefault("TELEMETRY_OTLP_ENDPOINT", ""),
		},
	}
}

// getenvDefault 读取环境变量，缺失时返回默认值。
func getenvDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// getenvInt 读取整型环境变量，缺失或格式错误时返回默认值。
func getenvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

// getenvBool 读取布尔型环境变量，缺失时返回默认值。
// 接受 1/0、true/false、yes/no（大小写不敏感）。
func getenvBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return defaultVal
}
