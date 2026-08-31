// Package bootstrap 将 main.go 的域初始化逻辑拆分为独立函数（27号优化）。
//
// 每个 Init* 函数负责一个业务域的装配（仓储→用例→路由），
// main.go 只保留骨架（配置加载→基础设施→各域 Init→HTTP 启动）。
package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"webreaper/internal/adapter/cache"
	"webreaper/internal/adapter/lock"
	"webreaper/internal/adapter/repository"
	"webreaper/internal/config"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/geo"
	"webreaper/internal/usecase/port"
)

// Deps 各域 Init 共享的基础设施依赖（main.go 装配后传入）。
type Deps struct {
	DB            *gorm.DB
	Redis         *redis.Client
	SettingRepo   port.SystemSettingRepository
	Logger        port.Logger
	Config        *config.Config
	TaskLock      port.TaskLock
	CacheStore    port.CacheStore
}

// GeoRepos GEO 域仓储集合。
type GeoRepos struct {
	DB      *gorm.DB
	Brand   port.BrandRepository
	Keyword port.KeywordRepository
	Result  port.MonitoringResultRepository
	Content port.OptimizedContentRepository
	Store   port.StoreLocationRepository
}

// InitGeoRepos 初始化 GEO 仓储。
func InitGeoRepos(db *gorm.DB) *GeoRepos {
	return &GeoRepos{
		DB:      db,
		Brand:   repository.NewGormBrandRepository(db),
		Keyword: repository.NewGormKeywordRepository(db),
		Result:  repository.NewGormMonitoringResultRepository(db),
		Content: repository.NewGormOptimizedContentRepository(db),
		Store:   repository.NewGormStoreLocationRepository(db),
	}
}

// AccountRepos 发布账号域仓储集合。
type AccountRepos struct {
	Account port.AccountRepository
	Job     port.PublishJobRepository
	Metric  port.VideoMetricRepository
}

// InitAccountRepos 初始化发布账号域仓储。
func InitAccountRepos(db *gorm.DB) *AccountRepos {
	return &AccountRepos{
		Account: repository.NewGormAccountRepository(db),
		Job:     repository.NewGormPublishJobRepository(db),
		Metric:  repository.NewGormVideoMetricRepository(db),
	}
}

// LoadIndexingConfig 从 DB 加载收录配置（system_settings 优先，env 兜底）。
func LoadIndexingConfig(ctx context.Context, settingRepo port.SystemSettingRepository, cfg *config.Config) entity.IndexingConfig {
	if s, sErr := settingRepo.Get(ctx, entity.SettingKeyIndexingConfig); sErr == nil {
		var c entity.IndexingConfig
		if json.Unmarshal([]byte(s.Value), &c) == nil {
			return c
		}
	}
	return entity.IndexingConfig{
		IndexNowKey: cfg.Server.IndexNowKey,
		BaiduSite:   cfg.Baidu.Site,
		BaiduToken:  cfg.Baidu.Token,
	}
}

// LoadEmbeddingConfig 从 DB 加载向量嵌入配置。
func LoadEmbeddingConfig(ctx context.Context, settingRepo port.SystemSettingRepository, cfg *config.Config) entity.EmbeddingRuntimeConfig {
	if s, sErr := settingRepo.Get(ctx, entity.SettingKeyEmbeddingConfig); sErr == nil {
		var c entity.EmbeddingRuntimeConfig
		if json.Unmarshal([]byte(s.Value), &c) == nil && c.IsConfigured() {
			return c
		}
	}
	return entity.EmbeddingRuntimeConfig{
		Model:    cfg.Embedding.Model,
		BaseURL:  cfg.Embedding.BaseURL,
		APIKey:   cfg.Embedding.APIKey,
		VectorDB: entity.VectorDBMySQL,
	}
}

// SeedPromptTemplates 首次启动写入内置默认提示词模板。
func SeedPromptTemplates(repo *repository.GormPromptTemplateRepository) error {
	ctx := context.Background()
	for _, t := range geo.DefaultPromptTemplates() {
		if _, err := repo.Get(ctx, t.Key); err == nil {
			continue
		}
		if err := repo.Save(ctx, t); err != nil {
			return fmt.Errorf("seed 提示词模板 %s: %w", t.Key, err)
		}
	}
	return nil
}

// RuntimeHeadedSyncer 把浏览器可见性同步到运行时全局内存。
type RuntimeHeadedSyncer struct{}

func (RuntimeHeadedSyncer) SyncBrowserHeaded(headed bool) { config.SetBrowserHeaded(headed) }

// ExtractResponseStyle 从 extra_json 提取 response_style 字段。
func ExtractResponseStyle(extraJSON string) string {
	if extraJSON == "" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(extraJSON), &m) == nil {
		if v, ok := m["response_style"].(string); ok {
			return v
		}
	}
	return ""
}

// ExtractExtraField 从 extra_json 提取指定字段的字符串值。
func ExtractExtraField(extraJSON, key string) string {
	if extraJSON == "" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(extraJSON), &m) == nil {
		if v, ok := m[key].(string); ok {
			return v
		}
	}
	return ""
}

// InitRedis 初始化 Redis 连接（失败降级单机模式）。
func InitRedis(cfg config.RedisConfig, logger port.Logger) (*redis.Client, port.CacheStore, port.TaskLock) {
	if !cfg.IsConfigured() {
		return nil, nil, lock.NewNoopLock()
	}
	rc := redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + cfg.Port,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 3*1000*1000*1000) // 3s
	defer pingCancel()
	if pErr := rc.Ping(pingCtx).Err(); pErr != nil {
		logger.Warn("Redis 连接失败，全链路降级单机模式",
			port.String("addr", cfg.Host+":"+cfg.Port), port.Err(pErr))
		_ = rc.Close()
		return nil, nil, lock.NewNoopLock()
	}
	logger.Info("Redis 已连接",
		port.String("addr", cfg.Host+":"+cfg.Port))
	return rc, cache.NewRedisCache(rc), lock.NewRedisLock(rc)
}

// ResolveViduKey 从 DB 或环境变量解析 Vidu API Key。
func ResolveViduKey(providerCfgRepo port.ProviderConfigRepository, envKey string) (string, bool) {
	if dbCfg, cfgErr := providerCfgRepo.Get(context.Background(), "vidu"); cfgErr == nil && dbCfg.Provider != "" {
		return dbCfg.APIKey, dbCfg.Enabled
	}
	return envKey, true
}

// ParseCallbackURL 解析公网回调地址。
func ParseCallbackURL(cfg *config.Config) string {
	callbackURL := os.Getenv("VIDU_CALLBACK_URL")
	if callbackURL == "" {
		base := strings.TrimRight(cfg.Server.PublicBaseURL, "/")
		if base != "" && !strings.Contains(base, "localhost") && !strings.Contains(base, "127.0.0.1") {
			callbackURL = base + cfg.Server.APIPrefix + "/api/v1/generation/callback"
		}
	}
	return callbackURL
}
