package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
)

// registerMerchantBillingRoutes 经济系统——商户端（我的套餐/订阅/订单，多租户隔离）。
// 语义属商户端（仅需 JWT + billingUC），此前误嵌在 admin 装配块内——已归位。
func (r *Router) registerMerchantBillingRoutes(api *gin.RouterGroup) {
	if r.billingUC == nil {
		return
	}
	api.GET("/billing/plans", r.HandleListActivePlans)
	api.GET("/billing/usage", r.HandleGetMyUsage) // 配额余量（进度条）
	api.GET("/billing/orders", r.HandleListMyOrders)
	api.POST("/billing/orders", r.HandleCreateOrder)              // 下单购买
	api.POST("/billing/orders/:id/confirm", r.HandleConfirmOrder) // 确认支付（mock 自动/真实回调）
}

// registerAdminRoutes 管理端路由（仅 admin 角色可访问）。
// geoHandler 来自 registerGEORoutes——admin 的全平台品牌/内容管理端点复用一个 handler。
func (r *Router) registerAdminRoutes(api *gin.RouterGroup, geoHandler *GEOHandler) {
	if r.userRepo == nil {
		return
	}
	adminGroup := api.Group("/admin")
	adminGroup.Use(middleware.RequireRole("admin"))
	{
		userHandler := NewUserHandler(r.authRegister, r.userRepo)
		// F3-1 运营聚合：品牌数 + 最近活跃（最近一次监测时间）——从既有用例取数，handler 零新依赖
		if r.geoBrandUC != nil {
			userHandler.SetUsageEnrichment(
				func(ctx context.Context, tenantID string) int {
					bs, err := r.geoBrandUC.List(ctx, tenantID)
					if err != nil {
						return 0
					}
					return len(bs)
				},
				func(ctx context.Context, tenantID string) (time.Time, bool) {
					if r.geoMonitorUC == nil {
						return time.Time{}, false
					}
					rs, err := r.geoMonitorUC.GetLatestByTenant(ctx, tenantID)
					if err != nil {
						return time.Time{}, false
					}
					var latest time.Time
					for _, r := range rs {
						if r.ProbedAt.After(latest) {
							latest = r.ProbedAt
						}
					}
					return latest, !latest.IsZero()
				},
			)
		}
		adminGroup.GET("/users", userHandler.HandleListUsers)
		adminGroup.POST("/users", userHandler.HandleCreateMerchant)
		adminGroup.DELETE("/users/:id", userHandler.HandleDeleteUser)
		// 全平台资源管理（admin 旁路：显式全局查询，不走商户租户上下文）
		if geoHandler != nil {
			adminGroup.GET("/brands", geoHandler.HandleAdminListBrands)
			adminGroup.GET("/contents", geoHandler.HandleAdminListContents)
			adminGroup.DELETE("/brands/:id", geoHandler.HandleAdminDeleteBrand)
			adminGroup.POST("/contents/:id/status", geoHandler.HandleAdminSetContentStatus)
			adminGroup.DELETE("/contents/:id", geoHandler.HandleAdminDeleteContent)
			// 行业全景看板（v3 P2：跨商户聚合——行业能见度/品牌美誉度/信源域名榜）
			if geoHandler.industryUC != nil {
				adminGroup.GET("/merchant/industry-overview", geoHandler.HandleAdminIndustryOverview)
			}
			// R3 运营指标（LLM 成功率/缓存命中率/配额拒绝/锁竞争——admin 专用）
			adminGroup.GET("/debug/metrics", r.HandleDebugMetrics)
			// 系统健康总览 + 生成任务监控（Admin Tools 配套）
			if r.generationUC != nil {
				ahh := NewAdminHealthHandler(r.generationUC, r.generationVoices, r.subjectAssetRepo, r.settingRepo, r.generationProvider)
				adminGroup.GET("/system/health", ahh.HandleSystemHealth)
				adminGroup.GET("/tasks", ahh.HandleListAllTasks)
				adminGroup.POST("/tasks/:id/cancel", ahh.HandleAdminCancelTask)
				// 生成域业务配置（gen_* 键值对——UI 化散落的环境变量）
				if r.settingRepo != nil {
					adminGroup.GET("/settings/gen", ahh.HandleGetGenSettings)
					adminGroup.PUT("/settings/gen/:key", ahh.HandleSetGenSetting)
				}
			}
			// 发布通道管理（三轴重构：双链路共存的手动切换入口）
			if r.transportRegistry != nil {
				ta := NewTransportAdminHandler(r.transportRegistry, r.settingRepo)
				ta.RestoreOverrides(context.Background())
				adminGroup.GET("/publish/transports", ta.HandleList)
				adminGroup.PUT("/publish/transports/:platform", ta.HandleSet)
			}
		}
		// Tavily 搜索 API 配置（需 toolRegistry）
		if r.toolRegistry != nil {
			adminGroup.GET("/tavily-status", r.handleTavilyStatus)
			adminGroup.PUT("/tavily-key", r.handleUpdateTavilyKey)
		}
		// 平台系统设置（运行时开关）
		if r.settingsUC != nil {
			adminGroup.GET("/settings/auto-monitor", r.HandleGetAutoMonitor)
			adminGroup.PUT("/settings/auto-monitor", r.HandleSetAutoMonitor)
			// 浏览器可见性（RPA 发布/扫码登录时是否显示浏览器——动态切换无需重启）
			adminGroup.GET("/settings/browser-headed", r.HandleGetBrowserHeaded)
			adminGroup.PUT("/settings/browser-headed", r.HandleSetBrowserHeaded)
		}
		// 提示词模板管理（内容生成/优化系统提示词可管理、可热更新）
		if r.promptTemplateRepo != nil {
			adminGroup.GET("/prompt-templates", r.HandleListPromptTemplates)
			adminGroup.PUT("/prompt-templates/:key", r.HandleUpdatePromptTemplate)
		}
		// 生成模板管理（管理后台可动态配置生成模板）
		if r.templateUC != nil {
			th := NewTemplateHandler(r.templateUC)
			adminGroup.GET("/templates", th.HandleList)
			adminGroup.POST("/templates", th.HandleCreate)
			adminGroup.PUT("/templates/:id", th.HandleUpdate)
			adminGroup.DELETE("/templates/:id", th.HandleDelete)
		}
		// LLM 配置管理（含厂商 api_key 明文——仅 admin 可达；商户端引擎选择走 /geo/engines 名单端点）
		if r.llmCfgUC != nil {
			adminGroup.GET("/llm-configs", r.handleListLLMConfigs)
			adminGroup.POST("/llm-configs", r.handleCreateLLMConfig)
			adminGroup.PUT("/llm-configs/:name", r.handleUpdateLLMConfig)
			adminGroup.DELETE("/llm-configs/:name", r.handleDeleteLLMConfig)
			adminGroup.PUT("/llm-configs/:name/default", r.handleSetDefaultLLMConfig) // 设置默认模型
		}
		// 生成规格管理（Vidu 端点×模型矩阵——DB 驱动全局掌控，30s 热生效）
		if r.generationRegistry != nil && r.generationSpecRepo != nil {
			gh := NewGenerationAdminHandler(r.generationRegistry, r.generationSpecRepo)
			adminGroup.GET("/generation/specs", gh.HandleListSpecs)
			adminGroup.GET("/generation/modes", gh.HandleListModes) // 模式开关（sub_type 批量启停）
			adminGroup.PUT("/generation/modes/:subType", gh.HandleSetMode)
			adminGroup.POST("/generation/modes/apply-recommended", gh.HandleApplyRecommendedModes) // 一键收敛到推荐档位
			adminGroup.PUT("/generation/specs/:subType/:model", gh.HandleSaveSpec)
			adminGroup.DELETE("/generation/specs/:subType/:model", gh.HandleDeleteSpec)
			adminGroup.PUT("/generation/specs/:subType/:model/default", gh.HandleSetDefault) // 设置默认模型
		}
		// 官方主体管理（27 号优化——运营可管理官方主体/形象视频）
		if r.subjectAssetRepo != nil && r.generationUC != nil {
			ash := NewAdminSubjectHandler(r.generationUC, r.subjectAssetRepo, r.mediaStore)
			adminGroup.POST("/subjects", ash.HandleCreateOfficialSubject)
			adminGroup.GET("/subjects", ash.HandleListOfficialSubjects)
			adminGroup.PUT("/subjects/:id", ash.HandleUpdateOfficialSubject)
			adminGroup.DELETE("/subjects/:id", ash.HandleDeleteOfficialSubject)
		}
		// 官方音色管理（白牌化——运营可管理平台音色；Vidu 音色仅作克隆参考源）
		if r.generationVoices != nil {
			avh := NewAdminVoiceHandler(r.generationVoices, r.adminVoiceSynth, r.mediaStore)
			adminGroup.POST("/voices", avh.HandleCreateVoice)
			adminGroup.GET("/voices", avh.HandleListVoices)
			adminGroup.PUT("/voices/:id", avh.HandleUpdateVoice)
			adminGroup.DELETE("/voices/:id", avh.HandleDeleteVoice)
			// 白牌化新增：从 Vidu 音色克隆 + Vidu 参考源列表 + 设为默认
			adminGroup.POST("/voices/from-vidu", avh.HandleCreateFromVidu)
			adminGroup.GET("/voices/vidu-sources", avh.HandleListViduVoices)
			adminGroup.PUT("/voices/:id/default", avh.HandleSetDefaultVoice)
		}
		// 第三方集成中心（08 计划 D7——能力路由模型：统一视图/双分组/厂商详情/健康检查）
		if r.providerConfigUC != nil {
			ih := NewIntegrationHandler(r.providerConfigUC, r.generationSpecRepo, r.llmCfgUC, r.generationRegistry, r.generationProvider, r.integrationRepo)
			adminGroup.GET("/integrations", ih.HandleList)
			adminGroup.GET("/integrations/:id", ih.HandleDetail)
			adminGroup.GET("/integrations/:id/health", ih.HandleHealth)
			adminGroup.PUT("/integrations/vidu/preferred-model", ih.HandlePreferredModel)
			// 能力路由管理（新表 integration_vendors + integration_capabilities）
			adminGroup.GET("/integrations/vendors", ih.HandleVendors)
			adminGroup.PUT("/integrations/vendors/:id", ih.HandleSaveVendor)
			adminGroup.GET("/integrations/capabilities", ih.HandleCapabilities)
			adminGroup.PUT("/integrations/capabilities/:id/default", ih.HandleSetCapabilityDefault)
			adminGroup.PUT("/integrations/capabilities/save", ih.HandleSaveCapability)        // id 在请求体（# 在 URL 中被截断）
			adminGroup.DELETE("/integrations/capabilities/delete", ih.HandleDeleteCapability) // 同上
		}
		// 厂商配置管理（按厂商设置 API Key——保存后对已装配厂商热生效）
		if r.providerConfigUC != nil {
			pch := NewProviderConfigHandler(r.providerConfigUC, r.generationProvider)
			adminGroup.GET("/provider-configs", pch.HandleList)
			adminGroup.PUT("/provider-configs/:provider", pch.HandleSave)
			// 厂商剩余积分（排查 CreditInsufficient——Vidu GET /ent/v2/credits）
			if r.generationProvider != nil {
				adminGroup.GET("/provider-configs/:provider/credits", pch.HandleGetCredits)
			}
		}
		// 收录管理（运行时配置/提交日志/手动补提交）
		if r.indexingUC != nil {
			adminGroup.GET("/indexing/config", r.HandleGetIndexingConfig)
			adminGroup.PUT("/indexing/config", r.HandleUpdateIndexingConfig)
			adminGroup.GET("/indexing/logs", r.HandleListIndexingLogs)
			adminGroup.POST("/indexing/re-submit", r.HandleReSubmitAll)
			adminGroup.POST("/indexing/generate-key", r.HandleGenerateIndexingKey) // 自动生成密钥（IndexNow 所有权证明）
			adminGroup.GET("/indexing/verify-key", r.HandleVerifyIndexingKey)      // 验证 key 文件可访问
		}
		// 平台知识库（向量嵌入/向量库/行业采集配置——30s 生效免重启；素材管理）
		if r.knowledgeUC != nil {
			adminGroup.GET("/knowledge/embedding-config", r.HandleGetKnowledgeEmbeddingConfig)
			adminGroup.PUT("/knowledge/embedding-config", r.HandleUpdateKnowledgeEmbeddingConfig)
			adminGroup.GET("/knowledge/crawl-config", r.HandleGetKnowledgeCrawlConfig)
			adminGroup.PUT("/knowledge/crawl-config", r.HandleUpdateKnowledgeCrawlConfig)
			adminGroup.GET("/knowledge/stats", r.HandleGetKnowledgeStats)
			adminGroup.GET("/knowledge/materials", r.HandleListKnowledgeMaterials)
			adminGroup.DELETE("/knowledge/materials/:id", r.HandleDeleteKnowledgeMaterial)
			adminGroup.POST("/knowledge/reindex", r.HandleReindexKnowledgeMaterials) // 换模型后重建存量向量
			adminGroup.POST("/knowledge/crawl", r.HandleCrawlKnowledgeNow)           // 手动触发一轮采集
			adminGroup.GET("/knowledge/search", r.HandleSearchKnowledgeMaterials)    // 检索验证/调试（带来源）
			adminGroup.GET("/knowledge/crawl-interval", r.HandleGetKnowledgeCrawlInterval)
			adminGroup.PUT("/knowledge/crawl-interval", r.HandleUpdateKnowledgeCrawlInterval) // 采集间隔（30-1440 分钟）
		}
		// 经济系统——套餐/订阅/订单管理（admin）
		if r.billingUC != nil {
			adminGroup.GET("/billing/plans", r.HandleAdminListPlans)
			adminGroup.POST("/billing/plans", r.HandleAdminSavePlan)
			adminGroup.DELETE("/billing/plans/:id", r.HandleAdminDeletePlan)
			adminGroup.GET("/billing/subscriptions", r.HandleAdminListSubscriptions)
			adminGroup.PUT("/billing/subscriptions/:tenant", r.HandleAdminAssignPlan) // 手动开通（线下收款）
			adminGroup.GET("/billing/orders", r.HandleAdminListOrders)
			adminGroup.GET("/billing/revenue", r.HandleAdminRevenueReport)      // 收入概览
			adminGroup.GET("/billing/cost-analysis", r.HandleAdminCostAnalysis) // 成本分析（X-01：收入 vs 成本双报表）
			adminGroup.GET("/billing/payment-config", r.HandleGetPaymentConfig) // 支付网关配置
			adminGroup.PUT("/billing/payment-config", r.HandleSetPaymentConfig) // 保存支付配置
		}

		// 32号：作品管理与内容安全——跨租户巡查流 + 下架/恢复（处置动作与业务表解耦）。
		if r.worksUC != nil {
			adminGroup.GET("/works", r.HandleAdminWorksList)
			adminGroup.POST("/works/:key/hide", r.HandleAdminWorkHide)
			adminGroup.POST("/works/:key/restore", r.HandleAdminWorkRestore)
		}
	}
}

// handleTavilyStatus GET /api/v1/admin/tavily-status —— 查看 Tavily 搜索配置状态
func (r *Router) handleTavilyStatus(c *gin.Context) {
	enabled := false
	hasKey := false
	if t, ok := r.toolRegistry.Lookup("tavily_search"); ok {
		// 透过 RateLimitCrawler 装饰器拿到内层的 TavilyCrawler
		// RateLimitCrawler 透传了 inner，但 Lookup 返回的是包装后的实例
		// 这里通过 AllWithStatus 查启用状态
		statuses := r.toolRegistry.AllWithStatus()
		for _, s := range statuses {
			if s.Name == "tavily_search" {
				enabled = s.Enabled
			}
		}
		_ = t
		hasKey = true // 能 Lookup 到说明注册了
	}
	success(c, gin.H{
		"registered": hasKey,
		"enabled":    enabled,
	})
}

// handleUpdateTavilyKey PUT /api/v1/admin/tavily-key —— 更新 Tavily API Key
// 注意：由于 TavilyCrawler 被 RateLimitCrawler 包装，这里只更新启用状态。
// Key 本身需要在 .env 里配置（TAVILY_API_KEY），运行时改 Key 需重启。
// 这个端点主要用于启用/禁用工具。
type tavilyKeyRequest struct {
	Enabled bool   `json:"enabled"`
	APIKey  string `json:"api_key"` // 可选：传入新 Key（后续版本支持）
}

func (r *Router) handleUpdateTavilyKey(c *gin.Context) {
	var req tavilyKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	// 更新启用状态
	r.toolRegistry.SetEnabled("tavily_search", req.Enabled)
	success(c, gin.H{
		"name":    "tavily_search",
		"enabled": req.Enabled,
		"note":    "API Key 请在 .env 文件配置 TAVILY_API_KEY，修改后重启生效",
	})
}
