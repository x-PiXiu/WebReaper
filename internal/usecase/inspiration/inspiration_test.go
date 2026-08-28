package inspiration

import (
	"context"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- Mock 实现 ----

type mockVideoRepo struct {
	videos   []entity.InspirationVideo
	saveErr  error
	listErr  error
}

func (m *mockVideoRepo) Update(ctx context.Context, video entity.InspirationVideo) error {
	return nil
}

func (m *mockVideoRepo) SaveBatch(ctx context.Context, videos []entity.InspirationVideo) (int, error) {
	if m.saveErr != nil {
		return 0, m.saveErr
	}
	m.videos = append(m.videos, videos...)
	return len(videos), nil
}

func (m *mockVideoRepo) List(ctx context.Context, brandID, platform, keyword, sortBy string, page, pageSize int) ([]entity.InspirationVideo, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.videos, len(m.videos), nil
}

func (m *mockVideoRepo) FindByID(ctx context.Context, id string) (entity.InspirationVideo, error) {
	for _, v := range m.videos {
		if v.ID == id {
			return v, nil
		}
	}
	return entity.InspirationVideo{}, nil
}

func (m *mockVideoRepo) UpdateMetrics(ctx context.Context, videoID string, metrics entity.MetricsUpdate) error {
	return nil
}

func (m *mockVideoRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockVideoRepo) CountByPlatform(ctx context.Context) ([]port.PlatformCount, error) {
	return nil, nil
}

func (m *mockVideoRepo) CountByBrand(ctx context.Context) ([]port.BrandCount, error) {
	return nil, nil
}

type mockBrandRepo struct {
	links []string
}

func (m *mockBrandRepo) Link(ctx context.Context, brandID, videoID, keyword string) error {
	m.links = append(m.links, brandID+":"+videoID)
	return nil
}

func (m *mockBrandRepo) Unlink(ctx context.Context, brandID, videoID string) error {
	return nil
}

func (m *mockBrandRepo) ListByBrand(ctx context.Context, brandID string) ([]string, error) {
	return m.links, nil
}

func (m *mockBrandRepo) ListByVideo(ctx context.Context, videoID string) ([]string, error) {
	return nil, nil
}

type mockConfigRepo struct {
	configs []entity.CrawlerConfig
}

func (m *mockConfigRepo) Save(ctx context.Context, config entity.CrawlerConfig) error {
	return nil
}

func (m *mockConfigRepo) FindByPlatform(ctx context.Context, platform string) (entity.CrawlerConfig, error) {
	for _, c := range m.configs {
		if c.Platform == platform {
			return c, nil
		}
	}
	return entity.CrawlerConfig{}, nil
}

func (m *mockConfigRepo) ListAll(ctx context.Context) ([]entity.CrawlerConfig, error) {
	return m.configs, nil
}

func (m *mockConfigRepo) Delete(ctx context.Context, platform string) error {
	return nil
}

func (m *mockConfigRepo) UpdateLastCrawled(ctx context.Context, platform string) error {
	return nil
}

func (m *mockConfigRepo) UpdateLastError(ctx context.Context, platform string, errMsg string) error {
	return nil
}

type mockAccountRepo struct{}

func (m *mockAccountRepo) Save(ctx context.Context, account entity.CrawlerAccount) error { return nil }
func (m *mockAccountRepo) FindByID(ctx context.Context, id int64) (entity.CrawlerAccount, error) {
	return entity.CrawlerAccount{}, nil
}
func (m *mockAccountRepo) ListByPlatform(ctx context.Context, platform string) ([]entity.CrawlerAccount, error) {
	return nil, nil
}
func (m *mockAccountRepo) ListAll(ctx context.Context) ([]entity.CrawlerAccount, error) { return nil, nil }
func (m *mockAccountRepo) Delete(ctx context.Context, id int64) error                   { return nil }
func (m *mockAccountRepo) UpdateStatus(ctx context.Context, id int64, status string) error { return nil }
func (m *mockAccountRepo) UpdateHealth(ctx context.Context, id int64, result string) error { return nil }
func (m *mockAccountRepo) IncrementUsage(ctx context.Context, id int64) error              { return nil }
func (m *mockAccountRepo) ResetDailyUsage(ctx context.Context) error                       { return nil }
func (m *mockAccountRepo) SelectAvailable(ctx context.Context, platform string) (*entity.CrawlerAccount, error) {
	return nil, nil
}

type mockCrawler struct {
	videos []entity.CrawledVideo
	alive  bool
}

func (m *mockCrawler) Platform() string { return "douyin" }
func (m *mockCrawler) Search(ctx context.Context, opts entity.SearchOptions) ([]entity.CrawledVideo, error) {
	return m.videos, nil
}
func (m *mockCrawler) GetDetail(ctx context.Context, videoID string) (*entity.CrawledVideo, error) {
	return nil, nil
}
func (m *mockCrawler) RefreshMetrics(ctx context.Context, videoIDs []string) ([]entity.MetricsUpdate, error) {
	return nil, nil
}
func (m *mockCrawler) IsAlive(ctx context.Context) bool { return m.alive }
func (m *mockCrawler) CheckAccountAlive(ctx context.Context, cookie string) (bool, string) {
	return m.alive, ""
}
func (m *mockCrawler) GetCapabilities() entity.PlatformCapabilities {
	return entity.PlatformCapabilities{SupportSearch: true}
}

// ---- 测试 ----

func TestUseCase_RegisterPlatform(t *testing.T) {
	uc := NewUseCase(&mockVideoRepo{}, &mockBrandRepo{}, &mockConfigRepo{}, &mockAccountRepo{})
	crawler := &mockCrawler{alive: true}
	uc.RegisterPlatform("douyin", crawler)

	platforms := uc.ListPlatforms()
	if len(platforms) != 1 {
		t.Errorf("ListPlatforms() returned %d, want 1", len(platforms))
	}
	if platforms[0] != "douyin" {
		t.Errorf("platforms[0] = %v, want douyin", platforms[0])
	}
}

func TestUseCase_List(t *testing.T) {
	videoRepo := &mockVideoRepo{
		videos: []entity.InspirationVideo{
			{ID: "1", Title: "视频1"},
			{ID: "2", Title: "视频2"},
		},
	}
	uc := NewUseCase(videoRepo, &mockBrandRepo{}, &mockConfigRepo{}, &mockAccountRepo{})

	videos, total, err := uc.List(context.Background(), "", "", "", "", 1, 20)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %v, want 2", total)
	}
	if len(videos) != 2 {
		t.Errorf("len(videos) = %v, want 2", len(videos))
	}
}

func TestUseCase_CrawlBrand(t *testing.T) {
	crawler := &mockCrawler{
		videos: []entity.CrawledVideo{
			{Platform: "douyin", VideoID: "1", Title: "川菜1", Author: "作者1", PlayCount: 1000, DiggCount: 100},
			{Platform: "douyin", VideoID: "2", Title: "川菜2", Author: "作者2", PlayCount: 2000, DiggCount: 200},
		},
		alive: true,
	}
	videoRepo := &mockVideoRepo{}
	brandRepo := &mockBrandRepo{}
	uc := NewUseCase(videoRepo, brandRepo, &mockConfigRepo{}, &mockAccountRepo{})
	uc.RegisterPlatform("douyin", crawler)

	result, err := uc.CrawlBrand(context.Background(), "douyin", "brand-001", []string{"川菜", "川味"})
	if err != nil {
		t.Fatalf("CrawlBrand() error = %v", err)
	}

	// 2 个关键词 × 2 个视频 = 4 个结果
	if result.VideosFound != 4 {
		t.Errorf("VideosFound = %v, want 4", result.VideosFound)
	}
	if result.VideosNew != 4 {
		t.Errorf("VideosNew = %v, want 4", result.VideosNew)
	}
	if result.DurationMs < 0 {
		t.Errorf("DurationMs = %v, want >= 0", result.DurationMs)
	}

	// 验证视频已保存
	if len(videoRepo.videos) != 4 {
		t.Errorf("saved videos = %d, want 4", len(videoRepo.videos))
	}

	// 验证品牌关联已建立
	if len(brandRepo.links) != 4 {
		t.Errorf("brand links = %d, want 4", len(brandRepo.links))
	}
}

func TestUseCase_CrawlBrand_PlatformNotRegistered(t *testing.T) {
	uc := NewUseCase(&mockVideoRepo{}, &mockBrandRepo{}, &mockConfigRepo{}, &mockAccountRepo{})
	_, err := uc.CrawlBrand(context.Background(), "unknown", "brand-001", []string{"test"})
	if err == nil {
		t.Error("CrawlBrand() should return error for unregistered platform")
	}
}

func TestUseCase_IsPlatformAlive(t *testing.T) {
	uc := NewUseCase(&mockVideoRepo{}, &mockBrandRepo{}, &mockConfigRepo{}, &mockAccountRepo{})
	uc.RegisterPlatform("douyin", &mockCrawler{alive: true})
	uc.RegisterPlatform("kuaishou", &mockCrawler{alive: false})

	if !uc.IsPlatformAlive(context.Background(), "douyin") {
		t.Error("douyin should be alive")
	}
	if uc.IsPlatformAlive(context.Background(), "kuaishou") {
		t.Error("kuaishou should not be alive")
	}
	if uc.IsPlatformAlive(context.Background(), "unknown") {
		t.Error("unknown platform should not be alive")
	}
}

func TestStaggeredScheduler_AddBrand(t *testing.T) {
	s := NewStaggeredScheduler(nil, &mockConfigRepo{}, &mockAccountRepo{}, nil)
	s.AddBrand("douyin", "brand-001", []string{"川菜"})
	s.AddBrand("douyin", "brand-002", []string{"火锅"})

	main, retry := s.QueueLen()
	if main != 2 {
		t.Errorf("main queue = %d, want 2", main)
	}
	if retry != 0 {
		t.Errorf("retry queue = %d, want 0", retry)
	}
}

func TestStaggeredScheduler_LoadBrandsFromConfig(t *testing.T) {
	configRepo := &mockConfigRepo{
		configs: []entity.CrawlerConfig{
			{Platform: "douyin", TenantID: "tenant-001", Enabled: true, SearchKeywords: []string{"川菜"}},
			{Platform: "douyin", TenantID: "tenant-002", Enabled: true, SearchKeywords: []string{"火锅"}},
			{Platform: "douyin", TenantID: "tenant-003", Enabled: false, SearchKeywords: []string{"奶茶"}},
		},
	}
	s := NewStaggeredScheduler(nil, configRepo, &mockAccountRepo{}, nil)
	if err := s.LoadBrandsFromConfig(context.Background()); err != nil {
		t.Fatalf("LoadBrandsFromConfig() error = %v", err)
	}

	main, _ := s.QueueLen()
	if main != 2 {
		t.Errorf("main queue = %d, want 2 (disabled config should be skipped)", main)
	}
}

func TestSchedulerConfig_Defaults(t *testing.T) {
	s := NewStaggeredScheduler(nil, &mockConfigRepo{}, &mockAccountRepo{}, nil)
	if s.interval != 15*time.Minute {
		t.Errorf("interval = %v, want 15m", s.interval)
	}
	if s.workerCount != 5 {
		t.Errorf("workerCount = %v, want 5", s.workerCount)
	}
}

func TestSchedulerConfig_Custom(t *testing.T) {
	cfg := &SchedulerConfig{
		Interval:    5 * time.Minute,
		WorkerCount: 3,
	}
	s := NewStaggeredScheduler(nil, &mockConfigRepo{}, &mockAccountRepo{}, cfg)
	if s.interval != 5*time.Minute {
		t.Errorf("interval = %v, want 5m", s.interval)
	}
	if s.workerCount != 3 {
		t.Errorf("workerCount = %v, want 3", s.workerCount)
	}
}
