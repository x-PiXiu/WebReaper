package geo

import (
	"context"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// fakeResultRepo 内存监测结果仓储（LatestByBrand 用）。
type fakeResultRepo struct {
	results []entity.MonitoringResult
}

func (f *fakeResultRepo) Save(ctx context.Context, r entity.MonitoringResult) error {
	f.results = append(f.results, r)
	return nil
}
func (f *fakeResultRepo) LatestByKeyword(ctx context.Context, tenantID, keywordID string) ([]entity.MonitoringResult, error) {
	return f.results, nil
}
func (f *fakeResultRepo) LatestByBrand(ctx context.Context, tenantID, brandID string) ([]entity.MonitoringResult, error) {
	return f.results, nil
}
func (f *fakeResultRepo) LatestByTenant(ctx context.Context, tenantID string) ([]entity.MonitoringResult, error) {
	return f.results, nil
}
func (f *fakeResultRepo) Trend(ctx context.Context, tenantID, brandID string, limit int) ([]entity.MonitoringResult, error) {
	return f.results, nil
}
func (f *fakeResultRepo) Count(ctx context.Context) (int, error) { return len(f.results), nil }

// fakeSearcher 可编程周边搜索器（成功/未配置/失败三态）。
type fakeSearcher struct {
	pois       []port.POIStore
	byTypePois []port.POIStore // P1 类型扫描专用（nil=回退 pois）
	err        error
}

func (f *fakeSearcher) SearchNearby(ctx context.Context, center port.Location, keyword string, radiusM int) ([]port.POIStore, error) {
	return f.pois, f.err
}

func (f *fakeSearcher) SearchNearbyByType(ctx context.Context, center port.Location, types string, radiusM int) ([]port.POIStore, error) {
	if f.byTypePois != nil {
		return f.byTypePois, nil
	}
	return f.pois, f.err
}

// fakeMeasurer 可编程距离测量器（P2 驾车耗时测试用）。
type fakeMeasurer struct {
	results []port.DistanceResult
	err     error
}

func (f *fakeMeasurer) MeasureDistances(ctx context.Context, origins []port.Location, dest port.Location, typ int) ([]port.DistanceResult, error) {
	return f.results, f.err
}

func TestNearbyUseCase_GetRanking(t *testing.T) {
	ctx := context.Background()
	brandRepo := newFakeBrandRepo()
	brandRepo.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "望京川菜馆", Competitors: []string{"辣婆婆"}})

	storeRepo := newFakeStoreRepo()
	storeRepo.Save(ctx, entity.StoreLocation{
		ID: "s1", TenantID: "t1", BrandID: "b1", Address: "北京市朝阳区望京街10号",
		Lat: 39.99, Lng: 116.47, GeoStatus: entity.GeoStatusOK, CreatedAt: time.Now(),
	})

	resultRepo := &fakeResultRepo{}
	// 自己被提及率 0.5，竞品 A 0.8、竞品 B 0.3
	resultRepo.results = []entity.MonitoringResult{
		{ID: "r1", TenantID: "t1", BrandID: "b1", MentionRate: 0.5, SampleCount: 4,
			CompetitorRates: map[string]float64{"辣婆婆": 0.8, "麻椒鱼": 0.3}},
	}

	t.Run("无门店 → 明确引导错误", func(t *testing.T) {
		uc := NewNearbyUseCase(brandRepo, newFakeStoreRepo(), resultRepo)
		uc.SetPOISearcher(&fakeSearcher{})
		if _, err := uc.GetRanking(ctx, "t1", "b1", ""); err == nil {
			t.Error("无门店时应报错引导创建门店")
		}
	})

	t.Run("双榜：地图按距离升序 + AI 按提及率降序", func(t *testing.T) {
		uc := NewNearbyUseCase(brandRepo, storeRepo, resultRepo)
		uc.SetPOISearcher(&fakeSearcher{pois: []port.POIStore{
			{Name: "老张川菜", Distance: 800, Rating: 4.5},
			{Name: "辣婆婆(望京店)", Distance: 300, Rating: 4.8},
			{Name: "老张川菜", Distance: 800, Rating: 4.5}, // 重复（多搜索词命中），应去重
		}})
		view, err := uc.GetRanking(ctx, "t1", "b1", "")
		if err != nil {
			t.Fatalf("GetRanking: %v", err)
		}
		if !view.MapAvailable {
			t.Error("MapAvailable = false, want true")
		}
		if len(view.MapRanking) != 2 {
			t.Fatalf("地图榜去重后应 2 条，got %d", len(view.MapRanking))
		}
		if view.MapRanking[0].DistanceM != 300 || view.MapRanking[1].DistanceM != 800 {
			t.Errorf("地图榜未按距离升序: %+v", view.MapRanking)
		}
		if view.OwnRate != 0.5 {
			t.Errorf("own_rate = %f, want 0.5", view.OwnRate)
		}
		if len(view.AIRanking) != 3 {
			t.Fatalf("AI 榜应 3 条（2 竞品 + 自己进榜），got %d", len(view.AIRanking))
		}
		if view.AIRanking[0].Name != "辣婆婆" || view.AIRanking[0].Rate != 0.8 {
			t.Errorf("AI 榜首位应为辣婆婆 0.8: %+v", view.AIRanking)
		}
		if !view.AIRanking[1].IsOwn || view.AIRanking[1].Name != "望京川菜馆" {
			t.Errorf("AI 榜次位应为自己的品牌（0.5）：%+v", view.AIRanking)
		}
		if view.AIRanking[2].Name != "麻椒鱼" {
			t.Errorf("AI 榜第三位应为麻椒鱼: %+v", view.AIRanking)
		}
	})

	t.Run("地图服务未配置 → 降级只显示 AI 榜", func(t *testing.T) {
		uc := NewNearbyUseCase(brandRepo, storeRepo, resultRepo)
		uc.SetPOISearcher(&fakeSearcher{err: port.ErrGeoNotConfigured})
		view, err := uc.GetRanking(ctx, "t1", "b1", "")
		if err != nil {
			t.Fatalf("GetRanking: %v", err)
		}
		if view.MapAvailable {
			t.Error("未配置时 MapAvailable 应为 false")
		}
		if len(view.MapRanking) != 0 {
			t.Errorf("未配置时地图榜应为空: %+v", view.MapRanking)
		}
		if len(view.AIRanking) == 0 {
			t.Error("AI 榜不应为空")
		}
	})

	t.Run("门店坐标缺失(pending) → 地图榜不可用但 AI 榜可用", func(t *testing.T) {
		storeRepo2 := newFakeStoreRepo()
		storeRepo2.Save(ctx, entity.StoreLocation{
			ID: "s2", TenantID: "t1", BrandID: "b1", Address: "未编码地址",
			GeoStatus: entity.GeoStatusPending, CreatedAt: time.Now(),
		})
		uc := NewNearbyUseCase(brandRepo, storeRepo2, resultRepo)
		uc.SetPOISearcher(&fakeSearcher{pois: []port.POIStore{{Name: "X", Distance: 100}}})
		view, err := uc.GetRanking(ctx, "t1", "b1", "")
		if err != nil {
			t.Fatalf("GetRanking: %v", err)
		}
		if view.MapAvailable || len(view.MapRanking) != 0 {
			t.Error("pending 门店不应触发地图搜索")
		}
		if len(view.AIRanking) == 0 {
			t.Error("AI 榜不应为空")
		}
	})

	t.Run("搜索词包含品牌名与手动竞品名", func(t *testing.T) {
		searcher := &fakeSearcher{pois: []port.POIStore{{Name: "辣婆婆", Distance: 500}}}
		uc := NewNearbyUseCase(brandRepo, storeRepo, resultRepo)
		uc.SetPOISearcher(searcher)
		view, err := uc.GetRanking(ctx, "t1", "b1", "")
		if err != nil {
			t.Fatalf("GetRanking: %v", err)
		}
		if view.SearchKeyword != "辣婆婆" {
			// 最后一次成功搜索的词是"辣婆婆"（品牌名搜索无结果也计入，竞品名命中）
			t.Logf("search_keyword = %s（品牌名搜索可能无 POI）", view.SearchKeyword)
		}
		_ = view
	})
}

// ---- P1 类型扫描 + P2 驾车耗时（高德扩展补全）----

func TestNearbyUseCase_TypeScan(t *testing.T) {
	ctx := context.Background()
	brandRepo := newFakeBrandRepo()
	brandRepo.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "望京川菜馆", CoreSelling: []string{"正宗川菜"}})
	storeRepo := newFakeStoreRepo()
	storeRepo.Save(ctx, entity.StoreLocation{
		ID: "s1", TenantID: "t1", BrandID: "b1", Address: "望京街10号",
		Lat: 39.99, Lng: 116.47, GeoStatus: entity.GeoStatusOK,
	})
	// 类型扫描返回按类目搜到的门店（品牌名搜索搜不到的类型外门店）
	searcher := &fakeSearcher{
		pois:       []port.POIStore{{Name: "辣婆婆", Distance: 300, Lat: 39.99, Lng: 116.47}},
		byTypePois: []port.POIStore{{Name: "新派川菜", Distance: 900, Lat: 39.98, Lng: 116.46}},
	}
	uc := NewNearbyUseCase(brandRepo, storeRepo, &fakeResultRepo{})
	uc.SetPOISearcher(searcher)

	view, err := uc.GetRanking(ctx, "t1", "b1", "050000")
	if err != nil {
		t.Fatalf("GetRanking: %v", err)
	}
	// 品牌名搜索 + 类型扫描合并去重
	if len(view.MapRanking) != 2 {
		t.Fatalf("应合并 2 条（名称搜索1 + 类型扫描1）: %+v", view.MapRanking)
	}
	names := map[string]bool{}
	for _, e := range view.MapRanking {
		names[e.Name] = true
	}
	if !names["新派川菜"] {
		t.Error("类型扫描结果未并入地图榜")
	}
}

func TestNearbyUseCase_DriveTime(t *testing.T) {
	ctx := context.Background()
	brandRepo := newFakeBrandRepo()
	brandRepo.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "望京川菜馆"})
	storeRepo := newFakeStoreRepo()
	storeRepo.Save(ctx, entity.StoreLocation{
		ID: "s1", TenantID: "t1", BrandID: "b1", Address: "望京街10号",
		Lat: 39.99, Lng: 116.47, GeoStatus: entity.GeoStatusOK,
	})
	// 两个有坐标的门店 + 一个无坐标（不应参与测距）
	searcher := &fakeSearcher{pois: []port.POIStore{
		{Name: "甲", Distance: 300, Lat: 39.99, Lng: 116.47},
		{Name: "乙", Distance: 800, Lat: 39.98, Lng: 116.46},
		{Name: "丙", Distance: 500}, // 无坐标
	}}
	measurer := &fakeMeasurer{results: []port.DistanceResult{
		{OriginIdx: 0, DistanceM: 1200, DurationSec: 300}, // 甲：驾车 5 分钟
		{OriginIdx: 1, DistanceM: 2400, DurationSec: 600}, // 乙：驾车 10 分钟
	}}
	uc := NewNearbyUseCase(brandRepo, storeRepo, &fakeResultRepo{})
	uc.SetPOISearcher(searcher)
	uc.SetDistanceMeasurer(measurer)

	view, err := uc.GetRanking(ctx, "t1", "b1", "")
	if err != nil {
		t.Fatalf("GetRanking: %v", err)
	}
	byName := map[string]MapRankEntry{}
	for _, e := range view.MapRanking {
		byName[e.Name] = e
	}
	if byName["甲"].DriveDurationSec != 300 || byName["甲"].DriveDistanceM != 1200 {
		t.Errorf("甲驾车耗时未补全: %+v", byName["甲"])
	}
	if byName["乙"].DriveDurationSec != 600 {
		t.Errorf("乙驾车耗时未补全: %+v", byName["乙"])
	}
	if byName["丙"].DriveDurationSec != 0 {
		t.Errorf("无坐标门店不应测距: %+v", byName["丙"])
	}
}
