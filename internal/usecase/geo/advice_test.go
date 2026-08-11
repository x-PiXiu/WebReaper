package geo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
)

// containsAny 判断文本是否包含任一子串。
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// 复用 store_location_test.go 的 fakeBrandRepo/fakeStoreRepo/fakeResultRepo。

type fakeContentRepo struct {
	contents []entity.OptimizedContent
}

func (f *fakeContentRepo) Save(ctx context.Context, c entity.OptimizedContent) error {
	f.contents = append(f.contents, c)
	return nil
}
func (f *fakeContentRepo) ListByBrand(ctx context.Context, tenantID, brandID string) ([]entity.OptimizedContent, error) {
	return f.contents, nil
}
func (f *fakeContentRepo) FindByID(ctx context.Context, tenantID, id string) (entity.OptimizedContent, error) {
	for _, c := range f.contents {
		if c.ID == id {
			return c, nil
		}
	}
	return entity.OptimizedContent{}, errors.New("not found")
}
func (f *fakeContentRepo) FindMaxVersion(ctx context.Context, tenantID, brandID, keywordID string) (int, error) {
	return len(f.contents), nil
}
func (f *fakeContentRepo) FindPublishedByID(ctx context.Context, id string) (entity.OptimizedContent, error) {
	for _, c := range f.contents {
		if c.ID == id && c.Status == "published" {
			return c, nil
		}
	}
	return entity.OptimizedContent{}, errors.New("not found")
}
func (f *fakeContentRepo) ListPublished(ctx context.Context) ([]entity.OptimizedContent, error) { return nil, nil }
func (f *fakeContentRepo) Count(ctx context.Context) (int, error)                              { return len(f.contents), nil }
func (f *fakeContentRepo) CountPublished(ctx context.Context) (int, error)                     { return 0, nil }
func (f *fakeContentRepo) Delete(ctx context.Context, tenantID, id string) error               { return nil }
func (f *fakeContentRepo) ListAll(ctx context.Context, status string, limit int) ([]entity.OptimizedContent, error) {
	return f.contents, nil
}
func (f *fakeContentRepo) UpdateIndexStatus(ctx context.Context, tenantID, id, status string, indexedAt time.Time) error {
	return nil
}

func TestAdviceUseCase_GetAdvice(t *testing.T) {
	ctx := context.Background()

	newUC := func(brands *fakeBrandRepo, stores *fakeStoreRepo, results *fakeResultRepo, contents *fakeContentRepo) *AdviceUseCase {
		return NewAdviceUseCase(brands, stores, results, contents)
	}

	t.Run("空品牌：无门店 + 无监测 + 无内容", func(t *testing.T) {
		brands := newFakeBrandRepo()
		brands.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "店"})
		uc := newUC(brands, newFakeStoreRepo(), &fakeResultRepo{}, &fakeContentRepo{})
		advices, err := uc.GetAdvice(ctx, "t1", "b1")
		if err != nil {
			t.Fatalf("GetAdvice: %v", err)
		}
		if len(advices) == 0 {
			t.Fatal("应有建议")
		}
		if advices[0].Level != "high" {
			t.Errorf("首位应为 high: %+v", advices[0])
		}
	})

	t.Run("被提及但自营内容零引用 → 归因建议", func(t *testing.T) {
		brands := newFakeBrandRepo()
		brands.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "店"})
		stores := newFakeStoreRepo()
		stores.Save(ctx, entity.StoreLocation{ID: "s1", TenantID: "t1", BrandID: "b1", Address: "A街", GeoStatus: entity.GeoStatusOK, Lat: 1, Lng: 2})
		results := &fakeResultRepo{}
		results.results = []entity.MonitoringResult{
			{ID: "r1", MentionRate: 0.6, SelfSourceCount: 0, CompetitorRates: map[string]float64{}},
		}
		contents := &fakeContentRepo{}
		contents.contents = []entity.OptimizedContent{{ID: "c1", Status: "published", Score: entity.GEOScore{Total: 60}}}
		uc := newUC(brands, stores, results, contents)
		advices, _ := uc.GetAdvice(ctx, "t1", "b1")
		found := false
		for _, a := range advices {
			if a.Level == "high" && containsAny(a.Message, "引用的来源里没有你的内容") {
				found = true
			}
		}
		if !found {
			t.Errorf("应有'零引用'归因建议: %+v", advices)
		}
	})

	t.Run("自营内容被引用（SelfSourceCount>0）→ 无零引用建议", func(t *testing.T) {
		brands := newFakeBrandRepo()
		brands.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "店"})
		stores := newFakeStoreRepo()
		stores.Save(ctx, entity.StoreLocation{ID: "s1", TenantID: "t1", BrandID: "b1", Address: "A街", GeoStatus: entity.GeoStatusOK, Lat: 1, Lng: 2})
		results := &fakeResultRepo{}
		results.results = []entity.MonitoringResult{
			{ID: "r1", MentionRate: 0.8, SelfSourceCount: 2, CompetitorRates: map[string]float64{}},
		}
		uc := newUC(brands, stores, results, &fakeContentRepo{})
		advices, _ := uc.GetAdvice(ctx, "t1", "b1")
		for _, a := range advices {
			if containsAny(a.Message, "引用的来源里没有你的内容") {
				t.Error("内容已被引用时不应给出'零引用'建议")
			}
		}
	})

	t.Run("竞品提及率高于自己 → 竞品压制建议", func(t *testing.T) {
		brands := newFakeBrandRepo()
		brands.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "店"})
		stores := newFakeStoreRepo()
		stores.Save(ctx, entity.StoreLocation{ID: "s1", TenantID: "t1", BrandID: "b1", Address: "A街", GeoStatus: entity.GeoStatusOK, Lat: 1, Lng: 2})
		results := &fakeResultRepo{}
		results.results = []entity.MonitoringResult{
			{ID: "r1", MentionRate: 0.3, CompetitorRates: map[string]float64{"辣婆婆": 0.9}},
		}
		uc := newUC(brands, stores, results, &fakeContentRepo{})
		advices, _ := uc.GetAdvice(ctx, "t1", "b1")
		found := false
		for _, a := range advices {
			if containsAny(a.Message, "辣婆婆") {
				found = true
			}
		}
		if !found {
			t.Errorf("应有竞品压制建议: %+v", advices)
		}
	})

	t.Run("低分内容未发布 → 发布/优化建议", func(t *testing.T) {
		brands := newFakeBrandRepo()
		brands.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "店"})
		stores := newFakeStoreRepo()
		stores.Save(ctx, entity.StoreLocation{ID: "s1", TenantID: "t1", BrandID: "b1", Address: "A街", GeoStatus: entity.GeoStatusOK, Lat: 1, Lng: 2})
		results := &fakeResultRepo{}
		results.results = []entity.MonitoringResult{
			{ID: "r1", MentionRate: 0.8, SelfSourceCount: 1, CompetitorRates: map[string]float64{}},
		}
		contents := &fakeContentRepo{}
		contents.contents = []entity.OptimizedContent{
			{ID: "c1", Status: "draft", Score: entity.GEOScore{Total: 35}},
		}
		uc := newUC(brands, stores, results, contents)
		advices, _ := uc.GetAdvice(ctx, "t1", "b1")
		found := false
		for _, a := range advices {
			if containsAny(a.Message, "都未发布") || containsAny(a.Message, "低于 50") {
				found = true
			}
		}
		if !found {
			t.Errorf("应有发布/优化建议: %+v", advices)
		}
	})

	t.Run("门店待定位 → 定位建议", func(t *testing.T) {
		brands := newFakeBrandRepo()
		brands.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "店"})
		stores := newFakeStoreRepo()
		stores.Save(ctx, entity.StoreLocation{ID: "s1", TenantID: "t1", BrandID: "b1", Address: "A街", GeoStatus: entity.GeoStatusPending})
		uc := newUC(brands, stores, &fakeResultRepo{}, &fakeContentRepo{})
		advices, _ := uc.GetAdvice(ctx, "t1", "b1")
		found := false
		for _, a := range advices {
			if containsAny(a.Message, "定位") {
				found = true
			}
		}
		if !found {
			t.Errorf("应有定位建议: %+v", advices)
		}
	})

	t.Run("全达标 → 保持建议（low）", func(t *testing.T) {
		brands := newFakeBrandRepo()
		brands.Save(ctx, entity.Brand{ID: "b1", TenantID: "t1", Name: "店"})
		stores := newFakeStoreRepo()
		stores.Save(ctx, entity.StoreLocation{ID: "s1", TenantID: "t1", BrandID: "b1", Address: "A街", GeoStatus: entity.GeoStatusOK, Lat: 1, Lng: 2})
		results := &fakeResultRepo{}
		results.results = []entity.MonitoringResult{
			{ID: "r1", MentionRate: 0.8, SelfSourceCount: 3, CompetitorRates: map[string]float64{"竞品": 0.2}},
		}
		contents := &fakeContentRepo{}
		contents.contents = []entity.OptimizedContent{{ID: "c1", Status: "published", Score: entity.GEOScore{Total: 80}}}
		uc := newUC(brands, stores, results, contents)
		advices, _ := uc.GetAdvice(ctx, "t1", "b1")
		if len(advices) != 1 || advices[0].Level != "low" {
			t.Errorf("全达标应只有一条 low 建议: %+v", advices)
		}
	})
}
