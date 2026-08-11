package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"webreaper/internal/usecase/port"
)

// 用 httptest 服务器验证高德响应解析（真实 HTTP 形状 + 降级行为）。

func TestAmapGeoCoder_Geocode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("address") == "北京市朝阳区望京街10号" {
			w.Write([]byte(`{"status":"1","info":"OK","geocodes":[{"location":"116.47,39.99","province":"北京市","city":"北京市","district":"朝阳区","adcode":"110105","formatted_address":"北京市朝阳区望京街10号"}]}`))
			return
		}
		w.Write([]byte(`{"status":"1","info":"OK","geocodes":[]}`))
	}))
	defer srv.Close()

	g := &AmapGeoCoder{apiKey: "test-key", baseURL: srv.URL}
	loc, err := g.Geocode(context.Background(), "北京市朝阳区望京街10号")
	if err != nil {
		t.Fatalf("Geocode error: %v", err)
	}
	if loc.Lat != 39.99 || loc.Lng != 116.47 {
		t.Errorf("坐标解析错误: lat=%f lng=%f", loc.Lat, loc.Lng)
	}
	if loc.City != "北京市" || loc.District != "朝阳区" || loc.Adcode != "110105" {
		t.Errorf("区划解析错误: %+v", loc)
	}

	if _, err := g.Geocode(context.Background(), "不存在的地址"); err == nil {
		t.Error("未命中时应报错")
	}
}

func TestAmapPOISearcher_SearchNearby(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"1","info":"OK","pois":[
			{"name":"辣婆婆(望京店)","address":"望京街8号","distance":"300","type":"餐饮服务;中餐厅","location":"116.47,39.99","rating":"4.8"},
			{"name":"老张川菜","address":"阜通东大街","distance":"800","type":"餐饮服务;中餐厅","location":"116.48,39.98","biz_ext":{"rating":"4.5"}}
		]}`))
	}))
	defer srv.Close()

	p := &AmapPOISearcher{apiKey: "test-key", baseURL: srv.URL}
	pois, err := p.SearchNearby(context.Background(), port.Location{Lat: 39.99, Lng: 116.47}, "川菜", 5000)
	if err != nil {
		t.Fatalf("SearchNearby error: %v", err)
	}
	if len(pois) != 2 {
		t.Fatalf("POI 数 = %d, want 2", len(pois))
	}
	first := pois[0]
	if first.Name != "辣婆婆(望京店)" || first.Distance != 300 || first.Rating != 4.8 {
		t.Errorf("第一条解析错误: %+v", first)
	}
	// biz_ext.rating 兜底解析
	if pois[1].Rating != 4.5 {
		t.Errorf("biz_ext.rating 未解析: %+v", pois[1])
	}
}

func TestMockDegrade(t *testing.T) {
	// 未配置 key → mock 降级：返回 ErrGeoNotConfigured（业务降级信号）
	if _, err := NewAmapGeoCoder("").Geocode(context.Background(), "任何地址"); err != port.ErrGeoNotConfigured {
		t.Errorf("空 key 地理编码应返回 ErrGeoNotConfigured, got %v", err)
	}
	if _, err := NewAmapPOISearcher("", "v5").SearchNearby(context.Background(), port.Location{Lat: 1, Lng: 1}, "x", 0); err != port.ErrGeoNotConfigured {
		t.Errorf("空 key 周边搜索应返回 ErrGeoNotConfigured, got %v", err)
	}
}

func TestParseLngLat(t *testing.T) {
	lng, lat, err := parseLngLat("116.47,39.99")
	if err != nil || lng != 116.47 || lat != 39.99 {
		t.Errorf("parseLngLat 失败: lng=%f lat=%f err=%v", lng, lat, err)
	}
	if _, _, err := parseLngLat("invalid"); err == nil {
		t.Error("非法坐标应报错")
	}
}

// ---- v5 迁移测试（搜索 POI 2.0）----

func TestAmapV5POISearcher_SearchNearby(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 断言请求参数（v5 形态）
		q := r.URL.Query()
		if q.Get("show_fields") != "business,navi" {
			t.Errorf("show_fields = %q, want business,navi", q.Get("show_fields"))
		}
		if q.Get("page_size") != "25" {
			t.Errorf("page_size = %q, want 25", q.Get("page_size"))
		}
		w.Write([]byte(`{"status":"1","info":"ok","infocode":"10000","count":"2","pois":[
			{"name":"辣婆婆(望京店)","id":"B0001","address":"望京街8号","distance":"300",
			 "type":"餐饮服务;中餐厅","location":"116.47,39.99","cityname":"北京市","adname":"朝阳区",
			 "business":{"business_area":"望京","opentime_today":"10:00-22:00","tel":"010-88886666",
			             "tag":"烤鱼,水煮鱼","rating":"4.8","cost":"¥120/人"},
			 "navi":{"entr_location":"116.4701,39.9901"},"photos":[{"title":"门头","url":"https://img.example.com/1.jpg"}]},
			{"name":"老张川菜","id":"B0002","address":"阜通东大街","distance":"800",
			 "type":"餐饮服务;中餐厅","location":"116.48,39.98","cityname":"北京市","adname":"朝阳区",
			 "business":{"business_area":"望京","opentime_today":"","tel":"","tag":"","rating":"","cost":""},
			 "navi":{},"photos":[]}
		]}`))
	}))
	defer srv.Close()

	p := &AmapV5POISearcher{apiKey: "test-key", baseURL: srv.URL}
	pois, err := p.SearchNearby(context.Background(), port.Location{Lat: 39.99, Lng: 116.47}, "川菜馆", 5000)
	if err != nil {
		t.Fatalf("SearchNearby error: %v", err)
	}
	if len(pois) != 2 {
		t.Fatalf("POI 数 = %d, want 2", len(pois))
	}
	first := pois[0]
	// 门店卡字段（v5 business/navi 组）全量断言
	if first.Rating != 4.8 {
		t.Errorf("rating = %v, want 4.8", first.Rating)
	}
	if first.Cost != "¥120/人" || first.BusinessArea != "望京" || first.OpenTimeToday != "10:00-22:00" {
		t.Errorf("business 组解析错误: %+v", first)
	}
	if first.Tag != "烤鱼,水煮鱼" || first.Tel != "010-88886666" {
		t.Errorf("tag/tel 解析错误: %+v", first)
	}
	if first.EntrLocation != "116.4701,39.9901" {
		t.Errorf("entr_location 解析错误: %s", first.EntrLocation)
	}
	if first.PhotoURL != "https://img.example.com/1.jpg" {
		t.Errorf("photos 解析错误: %s", first.PhotoURL)
	}
	if first.CityName != "北京市" || first.AdName != "朝阳区" {
		t.Errorf("city/adname 解析错误: %+v", first)
	}
	// 缺字段条目（第二家评分/人均/特色菜为空）不应报错——空字段留空、有字段正常解析
	second := pois[1]
	if second.Rating != 0 || second.Cost != "" || second.OpenTimeToday != "" || second.Tag != "" {
		t.Errorf("缺字段条目应留空: %+v", second)
	}
	if second.BusinessArea != "望京" {
		t.Errorf("business_area 有数据应正常解析: %s", second.BusinessArea)
	}
}

func TestAmapV5POISearcher_RadiusClamp(t *testing.T) {
	// >50000 显式 clamp 到 50000（文档：超限按默认值——避免意外）
	var gotRadius string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRadius = r.URL.Query().Get("radius")
		w.Write([]byte(`{"status":"1","info":"ok","infocode":"10000","count":"0","pois":[]}`))
	}))
	defer srv.Close()

	p := &AmapV5POISearcher{apiKey: "k", baseURL: srv.URL}
	if _, err := p.SearchNearby(context.Background(), port.Location{Lat: 1, Lng: 1}, "x", 99999); err != nil {
		t.Fatalf("SearchNearby: %v", err)
	}
	if gotRadius != "50000" {
		t.Errorf("radius = %s, want 50000（clamp）", gotRadius)
	}
}

func TestNewAmapPOISearcher_VersionFactory(t *testing.T) {
	// 版本工厂：v3/v5/默认/mock 四态
	if _, ok := NewAmapPOISearcher("key", "v3").(*AmapPOISearcher); !ok {
		t.Error("v3 应返回 AmapPOISearcher")
	}
	if _, ok := NewAmapPOISearcher("key", "v5").(*AmapV5POISearcher); !ok {
		t.Error("v5 应返回 AmapV5POISearcher")
	}
	if _, ok := NewAmapPOISearcher("key", "").(*AmapV5POISearcher); !ok {
		t.Error("空版本应默认 v5")
	}
	if _, ok := NewAmapPOISearcher("", "v5").(MockPOISearcher); !ok {
		t.Error("空 key 应返回 mock 降级")
	}
}
