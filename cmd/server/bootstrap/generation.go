// generation.go 生成域初始化辅助（从 main.go 迁移——27号优化 main.go 瘦身）。
//
// 提供生成域初始化的辅助函数，main.go 仍保留主装配逻辑。
package bootstrap

import (
	"context"
	"encoding/json"
	"os"

	"gorm.io/gorm"

	"webreaper/internal/adapter/asropenai"
	"webreaper/internal/adapter/integration"
	"webreaper/internal/adapter/provider/viduendpoint"
	"webreaper/internal/adapter/repository"
	"webreaper/internal/config"
	"webreaper/internal/usecase/port"
)

// GenerationDeps 生成域依赖集合（main.go 装配后传入）。
type GenerationDeps struct {
	DB             *gorm.DB
	Config         *config.Config
	AIGenerator    port.AIGenerator
	LLMConfigRepo  port.LLMConfigRepository
	SettingRepo    port.SystemSettingRepository
	ToolRegistry   *port.ToolRegistry
	CacheStore     port.CacheStore
}

// InitGenerationSpecs 初始化生成规格注册表。
func InitGenerationSpecs(db *gorm.DB) (*viduendpoint.Registry, *repository.GormGenerationSpecRepository) {
	genRegistry := viduendpoint.NewRegistry()
	genSpecRepo := repository.NewGormGenerationSpecRepository(db)
	genRegistry.SetSpecRepo(genSpecRepo)
	genRegistry.SeedDefaults(context.Background())
	return genRegistry, genSpecRepo
}

// InitVoiceRepo 初始化音色仓储并 seed。
func InitVoiceRepo(db *gorm.DB) *repository.GormVoiceRepository {
	voiceRepo := repository.NewGormVoiceRepository(db)
	voiceRepo.SeedIfEmpty(context.Background(), viduendpoint.DefaultVoices())
	return voiceRepo
}

// InitCapabilityResolver 初始化能力路由解析器。
func InitCapabilityResolver(db *gorm.DB, providerCfgRepo *repository.GormProviderConfigRepository, llmConfigRepo port.LLMConfigRepository) *integration.Resolver {
	integrationRepo := repository.NewGormIntegrationRepository(db)
	integrationRepo.SeedIfEmpty(context.Background(),
		integration.DefaultVendors, integration.DefaultCapabilities)
	return integration.NewResolver(integrationRepo, providerCfgRepo, llmConfigRepo)
}

// InitASRClient 初始化 ASR 客户端（能力路由优先，旧表兜底）。
func InitASRClient(capResolver *integration.Resolver, providerCfgRepo *repository.GormProviderConfigRepository) *asropenai.Transcriber {
	return asropenai.NewTranscriber(func() asropenai.ASRConfig {
		if cap, err := capResolver.Resolve(context.Background(), "asr"); err == nil && cap.APIKey != "" {
			return asropenai.ASRConfig{
				Endpoint:      cap.Endpoint,
				APIKey:        cap.APIKey,
				Model:         cap.Model,
				ResponseStyle: ExtractResponseStyle(cap.ExtraJSON),
				Protocol:      cap.Protocol,
				ASRLanguage:   ExtractExtraField(cap.ExtraJSON, "asr_options_language"),
			}
		}
		if cfgRow, cfgErr := providerCfgRepo.Get(context.Background(), "asr"); cfgErr == nil && cfgRow.APIKey != "" {
			var ac asropenai.ASRConfig
			_ = json.Unmarshal([]byte(cfgRow.ExtraJSON), &ac)
			ac.APIKey = cfgRow.APIKey
			if ac.Endpoint == "" {
				ac.Endpoint = cfgRow.BaseURL
			}
			return ac
		}
		return asropenai.ASRConfig{}
	})
}

// ResolveMimoKey 解析小米 MiMo API Key（能力路由优先，环境变量兜底）。
func ResolveMimoKey(capResolver *integration.Resolver) string {
	mimoKey := os.Getenv("MIMO_API_KEY")
	if mimoKey == "" && capResolver != nil {
		if cap, capErr := capResolver.Resolve(context.Background(), "tts"); capErr == nil && cap.VendorID == "xiaomi-mimo" && cap.APIKey != "" {
			return cap.APIKey
		}
	}
	if mimoKey == "" && capResolver != nil {
		if cap, capErr := capResolver.Resolve(context.Background(), "voice-clone"); capErr == nil && cap.VendorID == "xiaomi-mimo" && cap.APIKey != "" {
			return cap.APIKey
		}
	}
	return mimoKey
}
