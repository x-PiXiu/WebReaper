package geo

import (
	"context"
	"errors"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- 测试用内存 fake（本包内私有，与 adapter/mock 分离：用例层测试不依赖适配器）----

type fakeBrandRepo struct {
	brands map[string]entity.Brand
}

func newFakeBrandRepo() *fakeBrandRepo {
	return &fakeBrandRepo{brands: map[string]entity.Brand{}}
}

func (f *fakeBrandRepo) Save(ctx context.Context, b entity.Brand) error {
	f.brands[b.ID] = b
	return nil
}
func (f *fakeBrandRepo) FindByID(ctx context.Context, tenantID, id string) (entity.Brand, error) {
	b, ok := f.brands[id]
	if !ok || (tenantID != "" && b.TenantID != tenantID) {
		return entity.Brand{}, errors.New("not found")
	}
	return b, nil
}
func (f *fakeBrandRepo) FindPublishedByID(ctx context.Context, id string) (entity.Brand, error) {
	b, ok := f.brands[id]
	if !ok {
		return entity.Brand{}, errors.New("not found")
	}
	return b, nil
}
func (f *fakeBrandRepo) ListByTenant(ctx context.Context, tenantID string) ([]entity.Brand, error) { return nil, nil }
func (f *fakeBrandRepo) Delete(ctx context.Context, tenantID, id string) error                     { delete(f.brands, id); return nil }
func (f *fakeBrandRepo) Count(ctx context.Context) (int, error)                                   { return len(f.brands), nil }
func (f *fakeBrandRepo) ListAll(ctx context.Context) ([]entity.Brand, error)                      { return nil, nil }

type fakeStoreRepo struct {
	stores map[string]entity.StoreLocation
}

func newFakeStoreRepo() *fakeStoreRepo {
	return &fakeStoreRepo{stores: map[string]entity.StoreLocation{}}
}

func (f *fakeStoreRepo) Save(ctx context.Context, s entity.StoreLocation) error {
	f.stores[s.ID] = s
	return nil
}
func (f *fakeStoreRepo) FindByID(ctx context.Context, tenantID, id string) (entity.StoreLocation, error) {
	s, ok := f.stores[id]
	if !ok || (tenantID != "" && s.TenantID != tenantID) {
		return entity.StoreLocation{}, errors.New("not found")
	}
	return s, nil
}
func (f *fakeStoreRepo) ListByBrand(ctx context.Context, tenantID, brandID string) ([]entity.StoreLocation, error) {
	var out []entity.StoreLocation
	for _, s := range f.stores {
		if s.BrandID == brandID {
			out = append(out, s)
		}
	}
	return out, nil
}
func (f *fakeStoreRepo) FindPrimaryByBrand(ctx context.Context, brandID string) (entity.StoreLocation, error) {
	var best entity.StoreLocation
	found := false
	for _, s := range f.stores {
		if s.BrandID == brandID && (!found || s.CreatedAt.Before(best.CreatedAt)) {
			best = s
			found = true
		}
	}
	if !found {
		return entity.StoreLocation{}, errors.New("not found")
	}
	return best, nil
}
func (f *fakeStoreRepo) Delete(ctx context.Context, tenantID, id string) error { delete(f.stores, id); return nil }

// fakeLocator 可编程地理编码器（成功/未配置/失败三态 + 逆编码商圈）。
type fakeLocator struct {
	loc      port.Location
	err      error
	calls    int
	bizArea  string // 商圈（P1 逆编码回填；非空时 ReverseGeocode 返回）
	regeoErr error
}

func (f *fakeLocator) Geocode(ctx context.Context, address string) (port.Location, error) {
	f.calls++
	return f.loc, f.err
}

func (f *fakeLocator) ReverseGeocode(ctx context.Context, lng, lat float64) (port.ReverseGeocodeResult, error) {
	if f.regeoErr != nil {
		return port.ReverseGeocodeResult{}, f.regeoErr
	}
	return port.ReverseGeocodeResult{Location: f.loc, BusinessArea: f.bizArea}, nil
}

// ---- 用例测试 ----

func TestStoreLocationUseCase_Create(t *testing.T) {
	ctx := context.Background()
	brandRepo := newFakeBrandRepo()
	brandRepo.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "望京川菜馆"})

	t.Run("地理编码成功回填坐标与区划", func(t *testing.T) {
		uc := NewStoreLocationUseCase(newFakeStoreRepo(), brandRepo)
		uc.SetLocator(&fakeLocator{loc: port.Location{Lat: 39.99, Lng: 116.47, City: "北京市", District: "朝阳区", Adcode: "110105"}})
		loc, err := uc.Create(ctx, StoreLocationInput{TenantID: "t1", BrandID: "b1", Address: "北京市朝阳区望京街10号", Hours: "10:00-22:00"})
		if err != nil {
			t.Fatalf("Create error: %v", err)
		}
		if loc.GeoStatus != entity.GeoStatusOK {
			t.Errorf("geo_status = %s, want ok", loc.GeoStatus)
		}
		if loc.Lat != 39.99 || loc.Lng != 116.47 {
			t.Errorf("坐标未回填: lat=%f lng=%f", loc.Lat, loc.Lng)
		}
		if loc.District != "朝阳区" {
			t.Errorf("district = %s, want 朝阳区", loc.District)
		}
		if !loc.HasGeo() {
			t.Error("HasGeo() = false, want true")
		}
	})

	t.Run("未配置地图服务：门店照常创建，标记 pending", func(t *testing.T) {
		uc := NewStoreLocationUseCase(newFakeStoreRepo(), brandRepo)
		uc.SetLocator(&fakeLocator{err: port.ErrGeoNotConfigured})
		loc, err := uc.Create(ctx, StoreLocationInput{TenantID: "t1", BrandID: "b1", Address: "北京市朝阳区望京街10号"})
		if err != nil {
			t.Fatalf("Create error: %v", err)
		}
		if loc.GeoStatus != entity.GeoStatusPending {
			t.Errorf("geo_status = %s, want pending", loc.GeoStatus)
		}
		if loc.HasGeo() {
			t.Error("HasGeo() = true, want false（未编码）")
		}
	})

	t.Run("编码失败（地址无法解析）：标记 failed 不阻断", func(t *testing.T) {
		uc := NewStoreLocationUseCase(newFakeStoreRepo(), brandRepo)
		uc.SetLocator(&fakeLocator{err: errors.New("高德地理编码未命中")})
		loc, err := uc.Create(ctx, StoreLocationInput{TenantID: "t1", BrandID: "b1", Address: "不存在的地址xyz"})
		if err != nil {
			t.Fatalf("Create error: %v", err)
		}
		if loc.GeoStatus != entity.GeoStatusFailed {
			t.Errorf("geo_status = %s, want failed", loc.GeoStatus)
		}
	})

	t.Run("越权：他人品牌下创建门店被拒", func(t *testing.T) {
		uc := NewStoreLocationUseCase(newFakeStoreRepo(), brandRepo)
		if _, err := uc.Create(ctx, StoreLocationInput{TenantID: "t2", BrandID: "b1", Address: "北京市朝阳区"}); err == nil {
			t.Error("t2 不应能往 t1 的品牌下挂门店")
		}
	})

	t.Run("缺地址被拒", func(t *testing.T) {
		uc := NewStoreLocationUseCase(newFakeStoreRepo(), brandRepo)
		if _, err := uc.Create(ctx, StoreLocationInput{TenantID: "t1", BrandID: "b1"}); err == nil {
			t.Error("空地址不应通过")
		}
	})
}

func TestStoreLocationUseCase_UpdateAndReGeocode(t *testing.T) {
	ctx := context.Background()
	brandRepo := newFakeBrandRepo()
	brandRepo.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "店"})
	storeRepo := newFakeStoreRepo()
	uc := NewStoreLocationUseCase(storeRepo, brandRepo)
	uc.SetLocator(&fakeLocator{loc: port.Location{Lat: 1, Lng: 2, City: "北京"}})

	created, err := uc.Create(ctx, StoreLocationInput{TenantID: "t1", BrandID: "b1", Address: "A街1号"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.GeoStatus != entity.GeoStatusOK {
		t.Fatalf("初始编码未成功: %s", created.GeoStatus)
	}

	t.Run("改地址后坐标作废并重编码", func(t *testing.T) {
		loc, err := uc.Update(ctx, "t1", "b1", created.ID, StoreLocationInput{
			TenantID: "t1", BrandID: "b1", Address: "B街2号", Hours: "09:00-21:00",
		})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if loc.Address != "B街2号" || loc.Hours != "09:00-21:00" {
			t.Errorf("字段未更新: %+v", loc)
		}
		if loc.GeoStatus != entity.GeoStatusOK {
			t.Errorf("改地址后应重新编码: %s", loc.GeoStatus)
		}
	})

	t.Run("ReGeocode 重试 pending 门店", func(t *testing.T) {
		// 先模拟未配置（pending），再注入编码器重试
		uc2 := NewStoreLocationUseCase(storeRepo, brandRepo)
		uc2.SetLocator(&fakeLocator{err: port.ErrGeoNotConfigured})
		if _, err := uc2.ReGeocode(ctx, "t1", created.ID); err != nil {
			t.Fatalf("ReGeocode(pending): %v", err)
		}
		got, _ := storeRepo.FindByID(ctx, "t1", created.ID)
		if got.GeoStatus != entity.GeoStatusPending {
			t.Errorf("未配置时应保持 pending: %s", got.GeoStatus)
		}
		// 配置后重试 → ok
		uc2.SetLocator(&fakeLocator{loc: port.Location{Lat: 39.9, Lng: 116.4, City: "北京市"}})
		got, err = uc2.ReGeocode(ctx, "t1", created.ID)
		if err != nil {
			t.Fatalf("ReGeocode(ok): %v", err)
		}
		if got.GeoStatus != entity.GeoStatusOK || got.Lat != 39.9 {
			t.Errorf("重试后应 ok 并回填坐标: status=%s lat=%f", got.GeoStatus, got.Lat)
		}
	})

	t.Run("Delete 校验品牌归属", func(t *testing.T) {
		if err := uc.Delete(ctx, "t1", "b1", created.ID); err != nil {
			t.Errorf("删除本品牌门店失败: %v", err)
		}
		if err := uc.Delete(ctx, "t1", "b9", created.ID); err == nil {
			t.Error("用错误 brandID 删除应被拒")
		}
	})
}

// ---- 本地监测问法注入（P0 补全）----

// 带 LocalContext 的 probe 记录器（验证 Monitor 注入位置）。
type localProbe struct {
	localContext string
}

func (p *localProbe) Probe(ctx context.Context, in port.ProbeInput) (port.ProbeResult, error) {
	p.localContext = in.LocalContext
	return port.ProbeResult{SampleCount: 1, MentionRate: 0.5, Competitors: map[string]int{}}, nil
}

func TestMonitorUseCase_InjectsLocalContext(t *testing.T) {
	ctx := context.Background()
	brandRepo := newFakeBrandRepo()
	brandRepo.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "望京川菜馆"})

	// 有关联门店（已编码：区级上下文）
	storeRepo := newFakeStoreRepo()
	storeRepo.Save(ctx, entity.StoreLocation{
		ID: "s1", TenantID: "t1", BrandID: "b1", Address: "北京市朝阳区望京街10号",
		City: "北京市", District: "朝阳区", GeoStatus: entity.GeoStatusOK,
	})

	// 关键词仓储（fakeResultRepo 复用 + keyword fake）
	kwRepo := &fakeKeywordRepo{}
	kwRepo.kws = []entity.Keyword{{ID: "k1", TenantID: "t1", BrandID: "b1", Term: "川菜馆"}}

	probe := &localProbe{}
	uc := NewMonitorUseCase(brandRepo, kwRepo, &fakeResultRepo{}, probe)
	uc.SetStoreRepo(storeRepo)

	if _, err := uc.Monitor(ctx, MonitorInput{TenantID: "t1", BrandID: "b1", SampleSize: 1}); err != nil {
		t.Fatalf("Monitor: %v", err)
	}
	if probe.localContext != "朝阳区" {
		t.Errorf("localContext = %q, want 朝阳区（区级优先于城市）", probe.localContext)
	}

	t.Run("无门店 → 不注入位置", func(t *testing.T) {
		probe2 := &localProbe{}
		uc2 := NewMonitorUseCase(brandRepo, kwRepo, &fakeResultRepo{}, probe2)
		// 不注入 storeRepo
		if _, err := uc2.Monitor(ctx, MonitorInput{TenantID: "t1", BrandID: "b1", SampleSize: 1}); err != nil {
			t.Fatalf("Monitor: %v", err)
		}
		if probe2.localContext != "" {
			t.Errorf("无门店时不应注入位置: %q", probe2.localContext)
		}
	})
}

// fakeKeywordRepo 关键词仓储内存实现（monitor 测试用）。
type fakeKeywordRepo struct {
	kws []entity.Keyword
}

func (f *fakeKeywordRepo) Save(ctx context.Context, k entity.Keyword) error { return nil }
func (f *fakeKeywordRepo) FindByID(ctx context.Context, tenantID, id string) (entity.Keyword, error) {
	for _, k := range f.kws {
		if k.ID == id {
			return k, nil
		}
	}
	return entity.Keyword{}, errors.New("not found")
}
func (f *fakeKeywordRepo) ListByBrand(ctx context.Context, tenantID, brandID string) ([]entity.Keyword, error) {
	return f.kws, nil
}
func (f *fakeKeywordRepo) ListByTenant(ctx context.Context, tenantID string) ([]entity.Keyword, error) {
	return f.kws, nil
}
func (f *fakeKeywordRepo) Delete(ctx context.Context, tenantID, id string) error { return nil }
func (f *fakeKeywordRepo) Count(ctx context.Context) (int, error)               { return len(f.kws), nil }

// ---- P1 商圈补全（逆地理编码回填）----

func TestStoreLocationUseCase_BusinessAreaBackfill(t *testing.T) {
	ctx := context.Background()
	brandRepo := newFakeBrandRepo()
	brandRepo.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "望京川菜馆"})

	t.Run("编码成功且逆编码有商圈 → 回填", func(t *testing.T) {
		uc := NewStoreLocationUseCase(newFakeStoreRepo(), brandRepo)
		uc.SetLocator(&fakeLocator{
			loc:     port.Location{Lat: 39.99, Lng: 116.47, City: "北京市", District: "朝阳区"},
			bizArea: "望京",
		})
		loc, err := uc.Create(ctx, StoreLocationInput{TenantID: "t1", BrandID: "b1", Address: "望京街10号"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if loc.BusinessArea != "望京" {
			t.Errorf("商圈未回填: %q", loc.BusinessArea)
		}
	})

	t.Run("逆编码失败/无商圈 → 不阻断，商圈留空", func(t *testing.T) {
		uc := NewStoreLocationUseCase(newFakeStoreRepo(), brandRepo)
		uc.SetLocator(&fakeLocator{loc: port.Location{Lat: 39.99, Lng: 116.47}, regeoErr: errors.New("逆编码失败")})
		loc, err := uc.Create(ctx, StoreLocationInput{TenantID: "t1", BrandID: "b1", Address: "望京街10号"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if loc.GeoStatus != entity.GeoStatusOK {
			t.Errorf("逆编码失败不应影响定位状态: %s", loc.GeoStatus)
		}
		if loc.BusinessArea != "" {
			t.Errorf("逆编码失败时商圈应留空: %q", loc.BusinessArea)
		}
	})
}

// ---- P1 商圈优先：本地位置上下文（关键词生成/监测问法共用）----

func TestLocalContextFromStore_Priority(t *testing.T) {
	// 商圈 > 区 > 城市
	if got := localContextFromStore(entity.StoreLocation{BusinessArea: "望京", District: "朝阳区", City: "北京市"}); got != "望京" {
		t.Errorf("商圈应优先: %q", got)
	}
	if got := localContextFromStore(entity.StoreLocation{District: "朝阳区", City: "北京市"}); got != "朝阳区" {
		t.Errorf("无商圈应回退到区: %q", got)
	}
	if got := localContextFromStore(entity.StoreLocation{City: "北京市"}); got != "北京市" {
		t.Errorf("无商圈区应回退到城市: %q", got)
	}
	if got := localContextFromStore(entity.StoreLocation{}); got != "" {
		t.Errorf("全空应返回空: %q", got)
	}
}

func TestMonitorUseCase_BusinessAreaPriority(t *testing.T) {
	ctx := context.Background()
	brandRepo := newFakeBrandRepo()
	brandRepo.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "望京川菜馆"})

	// 门店有商圈（P1 逆编码回填）→ 问法用商圈
	storeRepo := newFakeStoreRepo()
	storeRepo.Save(ctx, entity.StoreLocation{
		ID: "s1", TenantID: "t1", BrandID: "b1", Address: "望京街10号",
		City: "北京市", District: "朝阳区", BusinessArea: "望京", GeoStatus: entity.GeoStatusOK,
	})
	kwRepo := &fakeKeywordRepo{}
	kwRepo.kws = []entity.Keyword{{ID: "k1", TenantID: "t1", BrandID: "b1", Term: "川菜馆"}}

	probe := &localProbe{}
	uc := NewMonitorUseCase(brandRepo, kwRepo, &fakeResultRepo{}, probe)
	uc.SetStoreRepo(storeRepo)
	if _, err := uc.Monitor(ctx, MonitorInput{TenantID: "t1", BrandID: "b1", SampleSize: 1}); err != nil {
		t.Fatalf("Monitor: %v", err)
	}
	if probe.localContext != "望京" {
		t.Errorf("有商圈时问法上下文应为商圈: %q", probe.localContext)
	}
}
