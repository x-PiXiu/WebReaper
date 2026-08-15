// Package knowledge 提供"平台知识库"的用例层（采集编排 + 素材管理）。
//
// 整洁架构定位：usecase 层只依赖 port 接口（仓储/设置/爬虫工具/Embedder），
// 协议细节（爬虫实现、向量 API、SQL）全在 adapter；新采集源 = 换工具注入，零改动。
//
// 核心链路（Docs/Plans/04）：
//   按行业配置关键词持续采集（保留来源 URL+原文）→ 入库（指纹去重+向量化）
//   → 生成时按"品牌行业+关键词"检索（带来源）→ 规格化 system prompt → 上游 LLM。
package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// KnowledgeUseCase 知识库用例：采集编排 + 素材管理 + 行业配置。
type KnowledgeUseCase struct {
	repo      port.KnowledgeMaterialRepository
	setting   port.SystemSettingRepository
	search    port.CrawlerTool // SearchCrawler：搜候选（标题/URL/摘要）
	static    port.CrawlerTool // StaticCrawler：抓正文（robots/合规内置）
	embedder  port.Embedder    // 可选：nil 时入库不带向量（不可检索但保留原文）
	retriever port.KnowledgeRetriever // 可选：检索验证/调试用（生成注入用同一实例）
	logger    port.Logger
}

// NewKnowledgeUseCase 创建知识库用例。
// search/static 为采集工具（可注入 mock 测试）；embedder 可为 nil（降级入库）。
func NewKnowledgeUseCase(repo port.KnowledgeMaterialRepository, setting port.SystemSettingRepository,
	search, static port.CrawlerTool, embedder port.Embedder, logger port.Logger) *KnowledgeUseCase {
	return &KnowledgeUseCase{
		repo: repo, setting: setting, search: search, static: static,
		embedder: embedder, logger: logger,
	}
}

// SetRetriever 注入检索器（可选；管理后台"检索验证"用，与生成注入同一实例）。
func (uc *KnowledgeUseCase) SetRetriever(r port.KnowledgeRetriever) {
	if r != nil {
		uc.retriever = r
	}
}

// SearchMaterials 按"行业 + 查询词"检索素材（管理后台检索验证/调试；带来源可溯源）。
// 未注入检索器返回空列表（不影响生成链路——生成注入是独立路径）。
func (uc *KnowledgeUseCase) SearchMaterials(ctx context.Context, industry, query string, num int) ([]entity.MaterialRef, error) {
	if uc.retriever == nil {
		return nil, nil
	}
	if num <= 0 {
		num = 3
	}
	return uc.retriever.Retrieve(ctx, industry, query, num)
}

// ---- 行业采集配置 ----

// GetIndustryConfigs 读取行业采集配置（system_settings JSON；未配置返回空列表）。
// 注意：返回空 slice 而非 nil——nil 序列化为 JSON null，前端数组操作会崩。
func (uc *KnowledgeUseCase) GetIndustryConfigs(ctx context.Context) ([]entity.IndustryCrawlConfig, error) {
	s, err := uc.setting.Get(ctx, entity.SettingKeyKnowledgeCrawl)
	if err != nil {
		return []entity.IndustryCrawlConfig{}, nil // 未配置 → 空列表（安全默认）
	}
	if strings.TrimSpace(s.Value) == "" {
		return []entity.IndustryCrawlConfig{}, nil // 配置为空 → 空列表（不报错）
	}
	var cfgs []entity.IndustryCrawlConfig
	if err := json.Unmarshal([]byte(s.Value), &cfgs); err != nil {
		return nil, fmt.Errorf("行业采集配置解析失败: %w", err)
	}
	if cfgs == nil {
		cfgs = []entity.IndustryCrawlConfig{} // JSON "null" 兜底为空列表
	}
	for i := range cfgs {
		cfgs[i].Normalize()
	}
	return cfgs, nil
}

// SaveIndustryConfigs 保存行业采集配置（管理后台可调，任务按新配置跑）。
// 校验：行业/关键词必须为合法 UTF-8——乱码输入会污染知识库（采集关键词→搜索词→
// 素材行业归属全链错乱，实测教训：GBK 终端直接 curl 导致 industry 乱码入库）。
// 先校验原始输入再 Normalize（Normalize 会把空行业填"通用"，必须在 Normalize 前拦截空值）。
func (uc *KnowledgeUseCase) SaveIndustryConfigs(ctx context.Context, cfgs []entity.IndustryCrawlConfig) error {
	for i := range cfgs {
		c := &cfgs[i]
		if c.Industry == "" {
			return fmt.Errorf("行业名不能为空")
		}
		if !utf8.ValidString(c.Industry) {
			return fmt.Errorf("行业名必须是合法 UTF-8（当前输入含乱码）: %q", c.Industry)
		}
		for _, kw := range c.Keywords {
			if strings.TrimSpace(kw) == "" {
				return fmt.Errorf("行业 %s 存在空关键词", c.Industry)
			}
			if !utf8.ValidString(kw) {
				return fmt.Errorf("关键词必须是合法 UTF-8（当前输入含乱码）: %q", kw)
			}
		}
	}
	for i := range cfgs {
		cfgs[i].Normalize()
	}
	data, err := json.Marshal(cfgs)
	if err != nil {
		return err
	}
	return uc.setting.Save(ctx, entity.SystemSetting{
		Key: entity.SettingKeyKnowledgeCrawl, Value: string(data), UpdatedAt: time.Now(),
	})
}

// ---- 向量嵌入 / 向量库运行时配置（管理后台可改，30s 生效）----

// GetEmbeddingConfig 读取向量配置（system_settings；未配置返回零值 = env 兜底生效）。
func (uc *KnowledgeUseCase) GetEmbeddingConfig(ctx context.Context) (entity.EmbeddingRuntimeConfig, error) {
	s, err := uc.setting.Get(ctx, entity.SettingKeyEmbeddingConfig)
	if err != nil {
		return entity.EmbeddingRuntimeConfig{}, nil // 未配置：走 env 兜底
	}
	if strings.TrimSpace(s.Value) == "" {
		return entity.EmbeddingRuntimeConfig{}, nil // 配置为空：走 env 兜底
	}
	var cfg entity.EmbeddingRuntimeConfig
	if err := json.Unmarshal([]byte(s.Value), &cfg); err != nil {
		return entity.EmbeddingRuntimeConfig{}, fmt.Errorf("向量配置解析失败: %w", err)
	}
	return cfg, nil
}

// SaveEmbeddingConfig 保存向量配置（校验 + 持久化；30s 内生效——CachedEmbedder/VectorStoreProvider TTL）。
func (uc *KnowledgeUseCase) SaveEmbeddingConfig(ctx context.Context, cfg entity.EmbeddingRuntimeConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	cfg.UpdatedAt = time.Now()
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return uc.setting.Save(ctx, entity.SystemSetting{
		Key: entity.SettingKeyEmbeddingConfig, Value: string(data), UpdatedAt: time.Now(),
	})
}

// ---- 采集编排 ----

// CrawlIndustries 按行业配置采集一轮（每行业每关键词 ≤ perRound 条入库）。
// 未配置行业 → 空转；单条失败不阻断；日志统计。
func (uc *KnowledgeUseCase) CrawlIndustries(ctx context.Context) error {
	if uc.search == nil || uc.static == nil {
		return nil // 工具未注入（测试/降级场景）
	}
	cfgs, err := uc.GetIndustryConfigs(ctx)
	if err != nil || len(cfgs) == 0 {
		return err // 空配置空转
	}
	var saved, dup, failed int
	for _, cfg := range cfgs {
		for _, kw := range cfg.Keywords {
			kwSaved, kwDup, kwFailed := uc.crawlKeyword(ctx, cfg.Industry, kw, cfg.PerRound)
			saved += kwSaved
			dup += kwDup
			failed += kwFailed
		}
	}
	uc.logger.Info("知识库采集完成",
		port.Int("saved", saved), port.Int("duplicated", dup), port.Int("failed", failed))
	return nil
}

// crawlKeyword 单个关键词的采集管线：
// 搜候选 → 指纹去重（持久化）→ 抓正文 → 长度/合规过滤 → 入库（+向量化）。
func (uc *KnowledgeUseCase) crawlKeyword(ctx context.Context, industry, keyword string, perRound int) (saved, dup, failed int) {
	if perRound <= 0 {
		perRound = 10
	}
	// ① 搜索候选（SearchCrawler：DDG，返回标题/URL/摘要 JSON）
	args, _ := json.Marshal(map[string]any{"query": keyword, "num": 10})
	item, err := uc.search.Execute(ctx, string(args))
	if err != nil {
		return 0, 0, 1
	}
	results, err := parseSearchResults(item.Content)
	if err != nil {
		return 0, 0, 1
	}

	// ② 逐候选：去重 → 抓取 → 过滤 → 入库
	for _, res := range results {
		if saved >= perRound {
			break // 本轮配额
		}
		url := normalizeURL(res.URL)
		if url == "" {
			continue
		}
		// 持久化去重（URL 指纹唯一索引兜底）
		fp := entity.FingerprintURL(url)
		exists, err := uc.repo.ExistsByFingerprint(ctx, fp)
		if err != nil {
			failed++
			continue
		}
		if exists {
			dup++
			continue
		}

		// ③ 抓正文（StaticCrawler：robots/登录/付费墙合规内置）
		pageArgs, _ := json.Marshal(map[string]string{"url": url})
		page, err := uc.static.Execute(ctx, string(pageArgs))
		if err != nil {
			failed++ // 单条失败不阻断
			continue
		}
		content := strings.TrimSpace(page.Content)
		if runeLen(content) < entity.MaterialMinContentRunes {
			dup++ // 低质/空页视为无效候选，不占配额也不占存储
			continue
		}

		// ④ 确定性清洗 + 入库（embedding 失败不阻断——保留原文，可后续补向量）
		m := &entity.KnowledgeMaterial{
			// ID 用随机 hex 而非 UnixNano——同一采集轮内连续入库（纳秒时钟分辨率下
			// 可能相同）会导致第二条覆盖第一条（upsert），素材静默丢失。
			ID:             fmt.Sprintf("kb-%s", pkg.ShortID(8)),
			Industry:       industry,
			SourceURL:      url,
			URLFingerprint: fp,
			Title:          truncateRunes(res.Title, 200),
			Content:        truncateRunes(content, entity.MaterialContentMaxRunes),
			Summary:        truncateRunes(content, entity.MaterialSummaryMaxRunes),
			CrawlKeyword:   keyword,
			Status:         entity.MaterialStatusActive,
			CreatedAt:      time.Now(),
		}
		if uc.embedder != nil {
			embText := m.Title + "\n" + m.Summary + "\n" + truncateRunes(m.Content, entity.MaterialEmbedTextMaxRunes)
			if emb, err := uc.embedder.Embed(ctx, embText); err == nil {
				m.Embedding = emb
			}
		}
		if err := uc.repo.Save(ctx, m); err != nil {
			failed++
			continue
		}
		saved++
	}
	return saved, dup, failed
}

// ---- 素材管理（管理后台透传）----

// Count 素材数（行业为空 = 全库）。
func (uc *KnowledgeUseCase) Count(ctx context.Context, industry string) (int64, error) {
	return uc.repo.Count(ctx, industry)
}

// ListByIndustry 分页列出素材。
func (uc *KnowledgeUseCase) ListByIndustry(ctx context.Context, industry string, limit, offset int) ([]entity.KnowledgeMaterial, error) {
	return uc.repo.ListByIndustry(ctx, industry, limit, offset)
}

// Delete 删除素材。
func (uc *KnowledgeUseCase) Delete(ctx context.Context, id string) error {
	return uc.repo.Delete(ctx, id)
}

// ---- 采集间隔（管理后台可改，下个周期生效）----

// GetCrawlIntervalMinutes 读取采集间隔（分钟；未配置/非法返回默认 6h）。
func (uc *KnowledgeUseCase) GetCrawlIntervalMinutes(ctx context.Context) (int, error) {
	s, err := uc.setting.Get(ctx, entity.SettingKeyCrawlIntervalMinutes)
	if err != nil || strings.TrimSpace(s.Value) == "" {
		return entity.DefaultCrawlIntervalMinutes, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(s.Value))
	if err != nil || n <= 0 {
		return entity.DefaultCrawlIntervalMinutes, nil
	}
	return n, nil
}

// SaveCrawlIntervalMinutes 保存采集间隔（分钟；30-1440 校验——防过频打爆搜索源/防素材陈旧）。
func (uc *KnowledgeUseCase) SaveCrawlIntervalMinutes(ctx context.Context, minutes int) error {
	if minutes < entity.CrawlIntervalMinMinutes || minutes > entity.CrawlIntervalMaxMinutes {
		return fmt.Errorf("%w: 采集间隔必须在 %d-%d 分钟之间（%.1fh-%dh）",
			pkg.ErrInvalidArgument,
			entity.CrawlIntervalMinMinutes, entity.CrawlIntervalMaxMinutes,
			float64(entity.CrawlIntervalMinMinutes)/60, entity.CrawlIntervalMaxMinutes/60)
	}
	return uc.setting.Save(ctx, entity.SystemSetting{
		Key: entity.SettingKeyCrawlIntervalMinutes, Value: strconv.Itoa(minutes), UpdatedAt: time.Now(),
	})
}

// ---- 向量重建 ----

// ReindexMaterials 重建素材向量（换 embedding 模型后的正确性修复）。
//
// 背景：管理后台切换模型后，存量素材的向量是旧模型产物——维度/语义空间不一致，
// 检索会错乱。本操作按当前配置重新向量化并同步向量库（仓储 Save 自动落库）。
//
// 参数：industry 空 = 全行业；onlyMissing = 仅处理无向量的素材（增量补向量）。
// 返回：processed=扫描数 / updated=重建数 / failed=失败数。
func (uc *KnowledgeUseCase) ReindexMaterials(ctx context.Context, industry string, onlyMissing bool) (processed, updated, failed int, err error) {
	if uc.embedder == nil {
		return 0, 0, 0, fmt.Errorf("向量嵌入未配置——无法重建向量（管理后台：知识库 → 向量配置）")
	}
	const pageSize = 50
	for offset := 0; ; offset += pageSize {
		list, err := uc.repo.ListByIndustry(ctx, industry, pageSize, offset)
		if err != nil {
			return processed, updated, failed, err
		}
		if len(list) == 0 {
			break
		}
		for i := range list {
			m := list[i]
			processed++
			if onlyMissing && len(m.Embedding) > 0 {
				continue // 增量模式：已有向量跳过
			}
			embText := m.Title + "\n" + m.Summary + "\n" + truncateRunes(m.Content, entity.MaterialEmbedTextMaxRunes)
			emb, err := uc.embedder.Embed(ctx, embText)
			if err != nil {
				failed++ // 单条失败不阻断
				continue
			}
			m.Embedding = emb
			if err := uc.repo.Save(ctx, &m); err != nil {
				failed++
				continue
			}
			updated++
		}
		if len(list) < pageSize {
			break
		}
	}
	return processed, updated, failed, nil
}
