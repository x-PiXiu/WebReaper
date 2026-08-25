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
	"sync/atomic"

	"github.com/joho/godotenv"

	"webreaper/internal/domain/entity"
)

// browserHeaded 浏览器可见性（运行时可变——管理后台动态切换，无需重启）。
var browserHeaded atomic.Bool

// SetBrowserHeaded 设置浏览器可见性（即时生效，下次 RPA 操作用新值）。
func SetBrowserHeaded(headed bool) { browserHeaded.Store(headed) }

// IsBrowserHeaded 读取浏览器可见性（RPA allocOpts 调用）。
func IsBrowserHeaded() bool { return browserHeaded.Load() }

// Config 是应用的全局配置根。
type Config struct {
	Server    ServerConfig
	DB        DBConfig
	Redis     RedisConfig
	LLM       LLMConfig
	JWT       JWTConfig
	Publish   PublishConfig
	Crawler   CrawlerConfig
	Telemetry TelemetryConfig
	Tavily    TavilyConfig
	Baidu     BaiduConfig
	AMap      AMapConfig // 高德地图（本地生活 GEO：地理编码 + 周边 POI 搜索）
	Storage   StorageConfig
	Embedding EmbeddingConfig // 向量嵌入（知识库素材向量化；缺省复用 LLM 配置）
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
	BaseURL    string // AgentCore 后端地址，如 http://localhost:8081
	AdminToken string // 采集用的认证 token（AgentCore 的 JWT 或 API Key）
}

// IsConfigured 判断 AgentCore 配置是否就绪。
func (c AgentCoreConfig) IsConfigured() bool {
	return c.BaseURL != "" && c.AdminToken != ""
}

// PublishConfig 多平台发布相关配置。
type PublishConfig struct {
	CookieSecret  string // 多平台发布 cookie 加密密钥（AES-GCM，从 PUBLISH_COOKIE_SECRET 读取）
	QRLoginHeaded bool   // 扫码登录是否显示浏览器窗口（调试用，生产保持 false 走灰盒 headless）
	// 抖音开放平台 OAuth（官方授权绑定——API 通道，替代浏览器扫码 RPA 通道）
	DouyinClientKey    string
	DouyinClientSecret string
	DouyinOAuthCallback string // 授权回调地址（必须与开放平台控制台「授权回调地址」完全一致，HTTPS）
	// DouyinOAuthScope 申请的授权作用域（逗号分隔）。⚠️ 只能填应用已开通的 scope——
	// 含任何一个未开通的 scope（如 video.create.bind）授权页直接报「scope权限非法」。
	// 默认仅 user_info；拿到视频发布/数据权限后按控制台实际开通项扩展。
	DouyinOAuthScope string
	FrontendBaseURL  string // 前端地址（OAuth 回调完成后 302 跳回）
}

// ServerConfig 服务配置。
type ServerConfig struct {
	Port string // 监听端口，默认 8082
	Env  string // 运行环境：development / production
	// LogLevel 日志级别（debug/info/warn/error；空=按环境默认：dev=debug、prod=info）。
	// 本地排查时在 .env 设 LOG_LEVEL=debug 后重启生效。
	LogLevel string
	// APIPrefix 路由统一前缀（nginx 分流用，如 /webreaper；空=无前缀）。
	// 生产部署在宿主机 nginx 后面，通过前缀区分不同项目的请求。
	APIPrefix string
	// PublicBaseURL 公开内容站的根地址（生成 sitemap/llms.txt/JSON-LD 的绝对 URL）。
	// 生产环境必须是公网可达的地址（如 https://content.example.com）。
	PublicBaseURL string
	// IndexNowKey IndexNow 收录提交密钥（可选）。
	// 配置后：内容发布为 published 时自动通知 Bing/Yandex 收录，替代人工提交 sitemap。
	// Key 文件由公开站端点 /public/indexnow-key.txt 托管（keyLocation 指向它）。
	IndexNowKey string
	// BingAPIKey Bing 站长 API 密钥（可选，收录状态验证用）。
	// 配置后：每日收录验证任务查询已发布内容是否被真正收录并回写状态。
	BingAPIKey string
	// BingSiteURL Bing 已验证的站点地址（如 https://content.example.com）。
	BingSiteURL string
	// ViduAPIKey Vidu 视频生成 API Key（可选）。未配置时视频工作台走 mock 模拟进度。
	// 运行时以 DB 厂商配置（provider_configs.vidu）优先，env 仅作启动兜底。
	ViduAPIKey string
	// FFMPEGPath ffmpeg 二进制目录（可选，FFMPEG_PATH）。空=PATH 查找；
	// 不可用时视频文案提取走降级路径（≤25MB 直传 ASR）。
	FFMPEGPath string
	// AutoMonitorEnabled 是否启用每日自动监测（AUTO_MONITOR_ENABLED=true）。
	// 启用后调度器每天对全平台品牌执行一次监测，趋势图自动生长。
	AutoMonitorEnabled bool
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
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&collation=utf8mb4_unicode_ci&parseTime=True&loc=Local",
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
	// CostPerMTokenCents 每百万 tokens 参考成本（分；0=成本报表只报 token 不估算金额）。
	// 成本分析（admin /admin/billing/cost-analysis）用——运营按实际模型单价配置，
	// 例：MiniMax M2.5 约 ¥1/百万 tokens → 100。
	CostPerMTokenCents int
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

// RedisConfig Redis 配置（分布式锁/缓存/回调 nonce——Host 为空=未启用，全链路降级单机模式）。
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// IsConfigured Redis 是否已配置（Host 非空即尝试连接；连接失败由 main 降级并记日志）。
func (c RedisConfig) IsConfigured() bool { return c.Host != "" }

// StorageConfig 文件存储配置（双模式：local 本地 / oss 阿里云 OSS）。
// STORAGE_TYPE=local（默认，本地开发）→ LocalMediaStore（./data/media）
// STORAGE_TYPE=oss（云端部署）→ OSSMediaStore（阿里云 OSS）
type StorageConfig struct {
	Type             string // local / oss
	Endpoint         string // OSS 公网 endpoint（oss-cn-guangzhou.aliyuncs.com）
	InternalEndpoint string // OSS 内网 endpoint（云服务器内网传输，oss-cn-guangzhou-internal.aliyuncs.com）
	Bucket           string // OSS bucket 名
	AccessKey        string // OSS AccessKey ID
	SecretKey        string // OSS AccessKey Secret
	PublicDomain     string // OSS 公开访问域名（可选；空=用 https://{bucket}.{endpoint}）
}

// TavilyConfig Tavily 搜索 API 配置（GEO 监测的高质量搜索源）。
// Tavily 是专为 AI 设计的搜索 API，返回结构化的干净内容（不需自己抓正文）。
// 不配置则降级到 Bing（WebFetcher）。
type TavilyConfig struct {
	APIKey string
}

func (c TavilyConfig) IsConfigured() bool { return c.APIKey != "" }

// BaiduConfig 百度收录主动推送配置。
// 参考 https://ziyuan.baidu.com/：在搜索资源平台验证域名后获取准入 token。
// 配置后内容发布为 published 时自动推送百度（单次 ≤2000 条，日配额约 10 万）。
type BaiduConfig struct {
	Site  string // 已验证域名（如 content.example.com）
	Token string // 准入 token
}

// IsConfigured 判断百度推送是否已配置（site + token 均非空）。
func (c BaiduConfig) IsConfigured() bool {
	return c.Site != "" && c.Token != ""
}

// AMapConfig 高德地图服务配置（本地生活 GEO：地理编码 + 周边 POI 搜索）。
// 申请：https://console.amap.com/ 创建 Web 服务 Key（个人开发者日配额约 5000 次，
// SaaS 初期够用；商用场景需核实商业授权条款——见计划文档 § 合规）。
// 不配置则降级：门店照常创建（geo_status=pending），附近同行只显示 AI 榜。
type AMapConfig struct {
	APIKey string
	// APIVersion 周边搜索接口版本："v5"（默认，推荐——show_fields 精确取字段、
	// 含评分/人均/商圈/营业时间）或 "v3"（旧版，兼容保留作降级开关）。
	APIVersion string
}

// IsConfigured 判断高德地图是否已配置。
func (c AMapConfig) IsConfigured() bool {
	return c.APIKey != ""
}

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
			Port:               getenvDefault("SERVER_PORT", "8082"),
			Env:                getenvDefault("APP_ENV", "development"),
			LogLevel:           getenvDefault("LOG_LEVEL", ""),
			APIPrefix:          getenvDefault("API_PREFIX", ""),
			PublicBaseURL:      getenvDefault("PUBLIC_BASE_URL", "http://localhost:8082"),
			IndexNowKey:        os.Getenv("INDEXNOW_KEY"),
			BingAPIKey:         os.Getenv("BING_API_KEY"),
			BingSiteURL:        os.Getenv("BING_SITE_URL"),
			ViduAPIKey:         os.Getenv("VIDU_API_KEY"),
			FFMPEGPath:         os.Getenv("FFMPEG_PATH"),
			AutoMonitorEnabled: os.Getenv("AUTO_MONITOR_ENABLED") == "true",
		},
		DB: DBConfig{
			Host:     getenvDefault("DB_HOST", "localhost"),
			Port:     getenvDefault("DB_PORT", "3306"),
			User:     getenvDefault("DB_USER", "root"),
			Password: os.Getenv("DB_PASSWORD"),
			Name:     getenvDefault("DB_NAME", "agentcore"),
			SSLMode:  getenvDefault("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getenvDefault("REDIS_HOST", ""),
			Port:     getenvDefault("REDIS_PORT", "6379"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       getenvInt("REDIS_DB", 0),
		},
		LLM: LLMConfig{
			Provider:           getenvDefault("LLM_PROVIDER", "minimax"),
			APIKey:             os.Getenv("LLM_API_KEY"),
			BaseURL:            getenvDefault("LLM_BASE_URL", "https://api.minimaxi.com/v1"),
			Model:              getenvDefault("LLM_MODEL", "MiniMax-M2.5"),
			CostPerMTokenCents: getenvInt("LLM_COST_PER_MToken", 100), // 默认 ¥1/百万 tokens
		},
		Tavily: TavilyConfig{
			APIKey: os.Getenv("TAVILY_API_KEY"),
		},
		Baidu: BaiduConfig{
			Site:  os.Getenv("BAIDU_SITE"),
			Token: os.Getenv("BAIDU_TOKEN"),
		},
		AMap: AMapConfig{
			APIKey:     os.Getenv("AMAP_API_KEY"),
			APIVersion: getenvDefault("AMAP_API_VERSION", "v5"), // v5 推荐；v3 兼容保留
		},
		// 向量嵌入：EMBEDDING_* 显式配置优先，缺省复用 LLM 配置（OpenAI 兼容 /embeddings）
		Embedding: EmbeddingConfig{
			Model:   getenvDefault("EMBEDDING_MODEL", getenvDefault("LLM_MODEL", "MiniMax-M2.5")),
			BaseURL: getenvDefault("EMBEDDING_BASE_URL", getenvDefault("LLM_BASE_URL", "https://api.minimaxi.com/v1")),
			APIKey:  getenvDefault("EMBEDDING_API_KEY", os.Getenv("LLM_API_KEY")),
		},
		JWT: JWTConfig{
			Secret:     os.Getenv("JWT_SECRET"),
			Expiration: getenvInt("JWT_EXPIRATION", 3600),
		},
		Publish: PublishConfig{
			CookieSecret:  os.Getenv("PUBLISH_COOKIE_SECRET"),
			QRLoginHeaded: getenvBool("QR_LOGIN_HEADED", false),
			DouyinClientKey:     os.Getenv("DOUYIN_CLIENT_KEY"),
			DouyinClientSecret:  os.Getenv("DOUYIN_CLIENT_SECRET"),
			DouyinOAuthCallback: getenvDefault("DOUYIN_OAUTH_CALLBACK", "http://localhost:8082/api/v1/merchant/accounts/douyin/oauth/callback"),
			DouyinOAuthScope:    getenvDefault("DOUYIN_OAUTH_SCOPE", "user_info"),
			FrontendBaseURL:     getenvDefault("FRONTEND_BASE_URL", "http://localhost:5173"),
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
		Storage: StorageConfig{
			Type:             getenvDefault("STORAGE_TYPE", "local"),
			Endpoint:         getenvDefault("OSS_ENDPOINT", ""),
			InternalEndpoint: getenvDefault("OSS_INTERNAL_ENDPOINT", ""),
			Bucket:           getenvDefault("OSS_BUCKET", ""),
			AccessKey:        os.Getenv("OSS_ACCESS_KEY"),
			SecretKey:        os.Getenv("OSS_SECRET_KEY"),
			PublicDomain:     getenvDefault("OSS_PUBLIC_DOMAIN", ""),
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

// Validate 生产环境配置校验（fail-fast——缺失必填项直接报错退出，避免运行时才暴露）。
// 开发环境（APP_ENV != production）跳过校验。
func (c Config) Validate() error {
	if c.Server.Env != "production" {
		return nil // 开发环境宽容
	}
	var errs []string
	if c.JWT.Secret == "" {
		errs = append(errs, "JWT_SECRET 未配置")
	}
	if c.DB.Password == "" {
		errs = append(errs, "DB_PASSWORD 未配置")
	}
	if c.Publish.CookieSecret == "" {
		errs = append(errs, "PUBLISH_COOKIE_SECRET 未配置")
	}
	if c.LLM.APIKey == "" {
		errs = append(errs, "LLM_API_KEY 未配置（核心功能依赖 LLM）")
	}
	if len(errs) > 0 {
		return fmt.Errorf("生产环境配置校验失败: %s", strings.Join(errs, "; "))
	}
	return nil
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
