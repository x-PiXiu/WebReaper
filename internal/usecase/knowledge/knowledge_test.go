package knowledge

import (
	"context"
	"errors"
	"strings"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- mock 组件 ----

// mockCrawler 可控爬虫工具（search/static 共用；content 即 Execute 返回的 DataItem.Content）。
type mockCrawler struct {
	name      string
	content   string
	contentBy map[string]string // 可选：按 URL 返回不同正文（测试长度过滤用）
	err       error
	failURL   string // 可选：argsJSON 包含该 URL 时报错（模拟单页抓取失败）
	calls     int
}

func (m *mockCrawler) Name() string                  { return m.name }
func (m *mockCrawler) Description() string           { return "mock " + m.name }
func (m *mockCrawler) ToolDeclaration() port.ToolDecl { return port.ToolDecl{Name: m.name} }

func (m *mockCrawler) Execute(_ context.Context, argsJSON string) (entity.DataItem, error) {
	m.calls++
	if m.err != nil {
		return entity.DataItem{}, m.err
	}
	if m.failURL != "" && strings.Contains(argsJSON, m.failURL) {
		return entity.DataItem{}, errors.New("mock fetch failed")
	}
	if m.contentBy != nil {
		for u, c := range m.contentBy {
			if strings.Contains(argsJSON, u) {
				return entity.DataItem{Content: c, Title: "mock"}, nil
			}
		}
	}
	return entity.DataItem{Content: m.content, Title: "mock"}, nil
}

var _ port.CrawlerTool = (*mockCrawler)(nil)

// mockKbRepo 内存素材仓储（记录 Save 的素材；按 ID upsert，ListByIndustry 支持分页过滤）。
type mockKbRepo struct {
	saved    []entity.KnowledgeMaterial
	existsFP map[string]bool
	err      error
}

func newMockKbRepo() *mockKbRepo {
	return &mockKbRepo{existsFP: map[string]bool{}}
}

func (m *mockKbRepo) Save(_ context.Context, mat *entity.KnowledgeMaterial) error {
	if m.err != nil {
		return m.err
	}
	for i := range m.saved {
		if m.saved[i].ID == mat.ID {
			m.saved[i] = *mat // 按 ID 覆盖（重建向量场景：同素材多次保存）
			m.existsFP[mat.URLFingerprint] = true
			return nil
		}
	}
	m.saved = append(m.saved, *mat)
	m.existsFP[mat.URLFingerprint] = true
	return nil
}
func (m *mockKbRepo) ExistsByFingerprint(_ context.Context, fp string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.existsFP[fp], nil
}
func (m *mockKbRepo) SearchSimilar(context.Context, string, string, []float32, int) ([]entity.MaterialRef, error) {
	return nil, nil
}
func (m *mockKbRepo) Count(context.Context, string) (int64, error) { return 0, nil }
func (m *mockKbRepo) ListByIndustry(_ context.Context, industry string, limit, offset int) ([]entity.KnowledgeMaterial, error) {
	if m.err != nil {
		return nil, m.err
	}
	filtered := make([]entity.KnowledgeMaterial, 0, len(m.saved))
	for _, mat := range m.saved {
		if industry != "" && mat.Industry != industry {
			continue
		}
		filtered = append(filtered, mat)
	}
	if offset >= len(filtered) {
		return nil, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end], nil
}
func (m *mockKbRepo) CountByBrand(context.Context, string) (int64, error) { return 0, nil }
func (m *mockKbRepo) ListByBrand(context.Context, string, string, int, int) ([]entity.KnowledgeMaterial, error) {
	return nil, nil
}
func (m *mockKbRepo) DeleteByBrand(context.Context, string, string, string) error { return nil }
func (m *mockKbRepo) Delete(_ context.Context, id string) error {
	for i := range m.saved {
		if m.saved[i].ID == id {
			m.saved = append(m.saved[:i], m.saved[i+1:]...)
			return nil
		}
	}
	return nil
}

var _ port.KnowledgeMaterialRepository = (*mockKbRepo)(nil)

// mockSetting 内存设置仓储。
type mockSetting struct {
	value string
}

func (m *mockSetting) Get(_ context.Context, _ string) (entity.SystemSetting, error) {
	return entity.SystemSetting{Key: entity.SettingKeyKnowledgeCrawl, Value: m.value}, nil
}
func (m *mockSetting) Save(_ context.Context, s entity.SystemSetting) error {
	m.value = s.Value
	return nil
}

var _ port.SystemSettingRepository = (*mockSetting)(nil)

// mockEmbedder 固定返回查询向量（3 维）。
type mockEmbedder struct {
	err error
}

func (m *mockEmbedder) Embed(context.Context, string) ([]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []float32{1, 0, 0}, nil
}
func (m *mockEmbedder) EmbedBatch(context.Context, []string) ([][]float32, error) { return nil, nil }
func (m *mockEmbedder) Dimension() int                                            { return 3 }

var _ port.Embedder = (*mockEmbedder)(nil)

// 有效正文（>200 字）。
const longContent = "这是一段足够长的正文内容。" +
	"餐饮行业数字化转型是当下热点，越来越多商家开始利用短视频平台获客。" +
	"根据行业报告，超过60%的本地餐饮商户正在尝试新媒体营销渠道。" +
	"数据显示，短视频带来的新客占比逐年上升，2025年已接近四成。" +
	"商户需要关注内容质量、门店信息完整度与线上评价管理。" +
	"同时，本地生活服务平台也在不断优化推荐算法，给优质内容更多曝光。" +
	"对于中小餐饮品牌而言，坚持输出真实、具体的门店内容至关重要。" +
	"本文将从获客渠道、内容策略、效果追踪三个维度展开分析。" +
	"希望这些信息能帮助餐饮从业者更好地理解线上营销的底层逻辑。" +
	"数据来源：行业白皮书与平台公开报告，具体数据以官方发布为准。"

// searchJSON 构造 SearchCrawler 返回的候选 JSON。
func searchJSON(results ...string) string {
	// results 交替 title,url
	s := `{"query":"q","results":[`
	for i := 0; i+1 < len(results); i += 2 {
		s += `{"title":"` + results[i] + `","url":"` + results[i+1] + `","snippet":"s"},`
	}
	s = s[:len(s)-1] + `]}`
	return s
}

// ---- 测试 ----

// TestCrawlKeyword_Pipeline 全链路：候选 → 去重 → 抓取 → 过滤 → 入库（带来源+向量）。
func TestCrawlKeyword_Pipeline(t *testing.T) {
	search := &mockCrawler{name: "search", content: searchJSON("标题A", "https://a.com/1", "标题B", "https://a.com/2")}
	static := &mockCrawler{name: "static", content: longContent}
	repo := newMockKbRepo()
	uc := NewKnowledgeUseCase(repo, &mockSetting{value: `[{"industry":"餐饮","keywords":["餐饮营销"]}]`},
		search, static, &mockEmbedder{}, port.NopLogger{})

	if err := uc.CrawlIndustries(context.Background()); err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if len(repo.saved) != 2 {
		t.Fatalf("应入库 2 条，实际 %d", len(repo.saved))
	}
	m := repo.saved[0]
	if m.Industry != "餐饮" || m.SourceURL != "https://a.com/1" {
		t.Errorf("行业/来源错误: %+v", m)
	}
	if m.URLFingerprint != entity.FingerprintURL("https://a.com/1") {
		t.Errorf("指纹错误: %s", m.URLFingerprint)
	}
	if len(m.Embedding) == 0 {
		t.Error("入库应带向量")
	}
	if m.Summary == "" || m.Status != entity.MaterialStatusActive {
		t.Errorf("摘要/状态错误: %+v", m)
	}

	// 第二轮采集：全部已去重 → 0 新增
	repo.saved = nil
	if err := uc.CrawlIndustries(context.Background()); err != nil {
		t.Fatalf("第二轮采集失败: %v", err)
	}
	if len(repo.saved) != 0 {
		t.Errorf("第二轮应全部去重: %d", len(repo.saved))
	}
}

// TestCrawlKeyword_Filters 低质正文丢弃 / 抓取失败不阻断 / 配额截断。
func TestCrawlKeyword_Filters(t *testing.T) {
	search := &mockCrawler{name: "search", content: searchJSON(
		"短内容", "https://a.com/short", // 正文不足 200 字 → 丢弃
		"失败页", "https://a.com/fail", // 抓取失败 → 记 failed 不阻断
		"正常A", "https://a.com/ok1",
		"正常B", "https://a.com/ok2",
	)}
	// static 对 /fail 报错、对 /short 返回短正文、其余长正文
	static := &mockCrawler{
		content: longContent,
		failURL: "https://a.com/fail",
		contentBy: map[string]string{"https://a.com/short": "太短"},
	}
	repo := newMockKbRepo()
	uc := NewKnowledgeUseCase(repo, &mockSetting{value: `[{"industry":"餐饮","keywords":["kw"],"per_round":1}]`},
		search, static, &mockEmbedder{}, port.NopLogger{})

	if err := uc.CrawlIndustries(context.Background()); err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if len(repo.saved) != 1 {
		t.Errorf("per_round=1 应只入库 1 条（配额），实际 %d", len(repo.saved))
	}
	// 失败页不阻断：正常A 仍入库
	if len(repo.saved) == 1 && repo.saved[0].SourceURL != "https://a.com/ok1" {
		t.Errorf("应入库 正常A: %+v", repo.saved)
	}
}

// TestCrawlIndustries_EmptyConfig 未配置行业 → 空转（零调用）。
func TestCrawlIndustries_EmptyConfig(t *testing.T) {
	search := &mockCrawler{name: "search"}
	uc := NewKnowledgeUseCase(newMockKbRepo(), &mockSetting{value: ""},
		search, &mockCrawler{name: "static"}, nil, port.NopLogger{})
	if err := uc.CrawlIndustries(context.Background()); err != nil {
		t.Fatalf("空配置应空转: %v", err)
	}
	if search.calls != 0 {
		t.Errorf("空配置不应调用爬虫: %d", search.calls)
	}
}

// TestCrawlKeyword_EmbeddingFailure 向量化失败不阻断入库（保留原文可检索兜底）。
func TestCrawlKeyword_EmbeddingFailure(t *testing.T) {
	search := &mockCrawler{name: "search", content: searchJSON("A", "https://a.com/1")}
	repo := newMockKbRepo()
	uc := NewKnowledgeUseCase(repo, &mockSetting{value: `[{"industry":"餐饮","keywords":["kw"]}]`},
		search, &mockCrawler{name: "static", content: longContent},
		&mockEmbedder{err: context.DeadlineExceeded}, port.NopLogger{})

	if err := uc.CrawlIndustries(context.Background()); err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if len(repo.saved) != 1 || len(repo.saved[0].Embedding) != 0 {
		t.Errorf("embedding 失败应仍入库且无向量: %+v", repo.saved)
	}
}

// TestPipelineUtils 纯函数：候选解析 / URL 规范化 / 截断。
func TestPipelineUtils(t *testing.T) {
	// 候选解析
	cands, err := parseSearchResults(`{"query":"q","results":[{"title":"t","url":"https://a.com/x"}]}`)
	if err != nil || len(cands) != 1 || cands[0].URL != "https://a.com/x" {
		t.Errorf("候选解析错误: %+v %v", cands, err)
	}
	// 非法 JSON → 错误
	if _, err := parseSearchResults(`not-json`); err == nil {
		t.Error("非法 JSON 应报错")
	}

	// URL 规范化：去 utm + fragment
	got := normalizeURL("https://a.com/x?utm_source=wechat&id=1#section")
	if got != "https://a.com/x?id=1" {
		t.Errorf("规范化错误: %s", got)
	}
	// 非 URL → 空
	if normalizeURL("not a url") != "" {
		t.Error("非法 URL 应返回空")
	}

	// rune 截断（中文）
	if runeLen("中文abc") != 5 {
		t.Errorf("rune 长度错误: %d", runeLen("中文abc"))
	}
	if got := truncateRunes("你好世界", 2); got != "你好" {
		t.Errorf("截断错误: %s", got)
	}
}

// TestReindexMaterials 向量重建：全量 / 增量（仅无向量）/ embedder 未配置报错。
func TestReindexMaterials(t *testing.T) {
	repo := newMockKbRepo()
	// 预置：m1 无向量、m2 有向量（旧模型）、m3 无向量
	_ = repo.Save(context.Background(), &entity.KnowledgeMaterial{
		ID: "m1", Industry: "餐饮", SourceURL: "https://a.com/1", URLFingerprint: "fp1", Status: entity.MaterialStatusActive,
	})
	_ = repo.Save(context.Background(), &entity.KnowledgeMaterial{
		ID: "m2", Industry: "餐饮", SourceURL: "https://a.com/2", URLFingerprint: "fp2",
		Status: entity.MaterialStatusActive, Embedding: []float32{9, 9, 9}, // 旧模型向量
	})
	_ = repo.Save(context.Background(), &entity.KnowledgeMaterial{
		ID: "m3", Industry: "美业", SourceURL: "https://b.com/3", URLFingerprint: "fp3", Status: entity.MaterialStatusActive,
	})
	uc := NewKnowledgeUseCase(repo, &mockSetting{}, nil, nil, &mockEmbedder{}, port.NopLogger{})
	ctx := context.Background()

	// 按行业过滤 + 增量（预置状态：m1 无向量、m2 有旧向量）：只重建餐饮，m1 重建、m2 跳过
	processed, updated, _, _ := uc.ReindexMaterials(ctx, "餐饮", true)
	if processed != 2 || updated != 1 {
		t.Errorf("行业过滤重建: processed=%d updated=%d", processed, updated)
	}

	// 全量重建（全部行业）：3 条都重算
	processed, updated, failed, err := uc.ReindexMaterials(ctx, "", false)
	if err != nil || processed != 3 || updated != 3 || failed != 0 {
		t.Errorf("全量重建: processed=%d updated=%d failed=%d err=%v", processed, updated, failed, err)
	}

	// 增量（全量重建后全部已有向量）：全部跳过
	processed, updated, failed, err = uc.ReindexMaterials(ctx, "", true)
	if err != nil || processed != 3 || updated != 0 || failed != 0 {
		t.Errorf("增量重建: processed=%d updated=%d failed=%d err=%v", processed, updated, failed, err)
	}

	// embedder 未配置 → 明确报错
	ucNoEmb := NewKnowledgeUseCase(repo, &mockSetting{}, nil, nil, nil, port.NopLogger{})
	if _, _, _, err := ucNoEmb.ReindexMaterials(ctx, "", false); err == nil {
		t.Error("embedder 未配置应报错")
	}
}

// TestSaveIndustryConfigs_UTF8Guard 非法 UTF-8 输入拒绝（乱码会污染知识库）。
func TestSaveIndustryConfigs_UTF8Guard(t *testing.T) {
	setting := &mockSetting{}
	uc := NewKnowledgeUseCase(newMockKbRepo(), setting, nil, nil, nil, port.NopLogger{})
	ctx := context.Background()

	// 合法配置保存成功
	valid := []entity.IndustryCrawlConfig{{Industry: "餐饮", Keywords: []string{"餐饮营销"}, PerRound: 5}}
	if err := uc.SaveIndustryConfigs(ctx, valid); err != nil {
		t.Fatalf("合法配置应成功: %v", err)
	}
	// 非法 UTF-8 行业（GBK 字节）→ 拒绝
	if err := uc.SaveIndustryConfigs(ctx, []entity.IndustryCrawlConfig{
		{Industry: "\xcd\xf8\xca\xb3", Keywords: []string{"x"}}, // "美食" 的 GBK 编码
	}); err == nil {
		t.Error("乱码行业应拒绝")
	}
	// 非法 UTF-8 关键词 → 拒绝
	if err := uc.SaveIndustryConfigs(ctx, []entity.IndustryCrawlConfig{
		{Industry: "餐饮", Keywords: []string{"\xcd\xf8\xca\xb3"}},
	}); err == nil {
		t.Error("乱码关键词应拒绝")
	}
	// 空行业 → 拒绝
	if err := uc.SaveIndustryConfigs(ctx, []entity.IndustryCrawlConfig{
		{Industry: "", Keywords: []string{"x"}},
	}); err == nil {
		t.Error("空行业应拒绝")
	}
}

// TestCrawlInterval 采集间隔：默认值 / 保存读回 / 越界拒绝。
func TestCrawlInterval(t *testing.T) {
	setting := &mockSetting{}
	uc := NewKnowledgeUseCase(newMockKbRepo(), setting, nil, nil, nil, port.NopLogger{})
	ctx := context.Background()

	// 未配置 → 默认 6h
	n, err := uc.GetCrawlIntervalMinutes(ctx)
	if err != nil || n != entity.DefaultCrawlIntervalMinutes {
		t.Errorf("未配置应返回默认 %d: n=%d err=%v", entity.DefaultCrawlIntervalMinutes, n, err)
	}
	// 保存 120 → 读回 120
	if err := uc.SaveCrawlIntervalMinutes(ctx, 120); err != nil {
		t.Fatalf("保存 120 分钟失败: %v", err)
	}
	if n, _ := uc.GetCrawlIntervalMinutes(ctx); n != 120 {
		t.Errorf("读回应为 120: %d", n)
	}
	// 越界拒绝（30-1440）
	for _, bad := range []int{0, 10, 29, 1441, 10000} {
		if err := uc.SaveCrawlIntervalMinutes(ctx, bad); err == nil {
			t.Errorf("间隔 %d 应拒绝", bad)
		}
	}
}

// TestEmbeddingConfig 向量配置读写 + 校验语义。
func TestEmbeddingConfig(t *testing.T) {
	repo := newMockKbRepo()
	setting := &mockSetting{}
	uc := NewKnowledgeUseCase(repo, setting, nil, nil, nil, port.NopLogger{})
	ctx := context.Background()

	// 未配置 → 零值（env 兜底生效）
	cfg, err := uc.GetEmbeddingConfig(ctx)
	if err != nil || cfg.IsConfigured() {
		t.Errorf("未配置应返回零值: %+v %v", cfg, err)
	}

	// 合法保存 → 读回一致
	valid := entity.EmbeddingRuntimeConfig{
		Model: "embedding-1", BaseURL: "https://api.test/v1", APIKey: "k",
		VectorDB: entity.VectorDBMySQL,
	}
	if err := uc.SaveEmbeddingConfig(ctx, valid); err != nil {
		t.Fatalf("合法配置保存失败: %v", err)
	}
	got, _ := uc.GetEmbeddingConfig(ctx)
	if got.Model != "embedding-1" || got.EffectiveVectorDB() != entity.VectorDBMySQL {
		t.Errorf("读回不一致: %+v", got)
	}

	// 凭据不齐 → 拒绝
	if err := uc.SaveEmbeddingConfig(ctx, entity.EmbeddingRuntimeConfig{Model: "m"}); err == nil {
		t.Error("凭据不齐应拒绝")
	}
	// milvus 缺 host → 拒绝
	if err := uc.SaveEmbeddingConfig(ctx, entity.EmbeddingRuntimeConfig{
		Model: "m", BaseURL: "b", APIKey: "k", VectorDB: entity.VectorDBMilvus,
	}); err == nil {
		t.Error("milvus 缺 host 应拒绝")
	}
	// 未知 vector_db → 拒绝
	if err := uc.SaveEmbeddingConfig(ctx, entity.EmbeddingRuntimeConfig{
		Model: "m", BaseURL: "b", APIKey: "k", VectorDB: "pgvector",
	}); err == nil {
		t.Error("未知 vector_db 应拒绝")
	}
}
