package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webreaper/internal/usecase/port"
)

// ---- P1 输入提示 / 逆地理编码 / P2 距离测量 / 静态地图 测试 ----

func TestAmapInputTipper_InputTips(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("datatype") != "poi" {
			t.Errorf("datatype = %q, want poi", r.URL.Query().Get("datatype"))
		}
		w.Write([]byte(`{"status":"1","info":"OK","count":"2","tips":[
			{"id":"B0001","name":"望京西园","district":"北京市朝阳区","adcode":"110105","location":"116.47,39.99","address":"阜通东大街6号"},
			{"id":"B0002","name":"望京SOHO","district":"北京市朝阳区","adcode":"110105","location":"116.48,39.99","address":"望京街10号"}
		]}`))
	}))
	defer srv.Close()

	tp := &AmapInputTipper{apiKey: "test-key", baseURL: srv.URL}
	tips, err := tp.InputTips(context.Background(), "望京", "110105", "")
	if err != nil {
		t.Fatalf("InputTips: %v", err)
	}
	if len(tips) != 2 {
		t.Fatalf("tips 数 = %d, want 2", len(tips))
	}
	if tips[0].Name != "望京西园" || tips[0].POIID != "B0001" || tips[0].Adcode != "110105" {
		t.Errorf("tip 解析错误: %+v", tips[0])
	}
	if tips[1].Location != "116.48,39.99" {
		t.Errorf("location 解析错误: %+v", tips[1])
	}
}

func TestAmapGeoCoder_ReverseGeocode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"1","info":"OK","regeocode":{
			"addressComponent":{"province":"北京市","city":"","district":"朝阳区","adcode":"110105",
				"township":"望京街道","streetNumber":{"street":"阜通东大街","number":"6号"}},
			"businessAreas":[{"name":"望京"},{"name":"酒仙桥"}]
		}}`))
	}))
	defer srv.Close()

	g := &AmapGeoCoder{apiKey: "k", baseURL: srv.URL, regeoURL: srv.URL}
	res, err := g.ReverseGeocode(context.Background(), 116.47, 39.99)
	if err != nil {
		t.Fatalf("ReverseGeocode: %v", err)
	}
	if res.BusinessArea != "望京" {
		t.Errorf("商圈 = %q, want 望京（取第一个）", res.BusinessArea)
	}
	if res.District != "朝阳区" || res.Township != "望京街道" || res.Street != "阜通东大街" {
		t.Errorf("地址元素解析错误: %+v", res)
	}
	// 直辖市 city 为空（官方行为）——不应报错
	if res.City != "" {
		t.Errorf("直辖市 city 应为空: %q", res.City)
	}
}

func TestAmapDistanceMeasurer_MeasureDistances(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 断言批量参数（3 起点 + type=1 驾车）
		q := r.URL.Query()
		if q.Get("type") != "1" {
			t.Errorf("type = %q, want 1", q.Get("type"))
		}
		origins := q.Get("origins")
		if strings.Count(origins, "|") != 2 {
			t.Errorf("origins 应 3 个坐标: %q", origins)
		}
		w.Write([]byte(`{"status":"1","info":"OK","results":[
			{"origin_id":"1","distance":"1200","duration":"300"},
			{"origin_id":"2","distance":"2400","duration":"600"},
			{"origin_id":"3","distance":"","duration":"","info":"起点不在中国境内"}
		]}`))
	}))
	defer srv.Close()

	m := &AmapDistanceMeasurer{apiKey: "k", baseURL: srv.URL}
	results, err := m.MeasureDistances(context.Background(),
		[]port.Location{{Lat: 39.99, Lng: 116.47}, {Lat: 39.98, Lng: 116.46}, {Lat: 1, Lng: 1}},
		port.Location{Lat: 39.99, Lng: 116.47}, 1)
	if err != nil {
		t.Fatalf("MeasureDistances: %v", err)
	}
	// 失败条目（info 非空）跳过——只剩 2 条
	if len(results) != 2 {
		t.Fatalf("results 数 = %d, want 2（失败条目跳过）", len(results))
	}
	// OriginIdx 从高德 1 起始 → 0 起始 映射
	if results[0].OriginIdx != 0 || results[0].DistanceM != 1200 || results[0].DurationSec != 300 {
		t.Errorf("第一条映射错误: %+v", results[0])
	}
	if results[1].OriginIdx != 1 {
		t.Errorf("第二条映射错误: %+v", results[1])
	}
}

func TestStaticMapURL(t *testing.T) {
	u := StaticMapURL("test-key", 39.99, 116.47, "望京川菜馆", "400x300", 15)
	if !strings.Contains(u, "restapi.amap.com/v3/staticmap") {
		t.Errorf("URL 错误: %s", u)
	}
	if !strings.Contains(u, "key=test-key") {
		t.Error("缺少 key")
	}
	if !strings.Contains(u, "location=116.470000,39.990000") {
		t.Errorf("location 错误: %s", u)
	}
	if !strings.Contains(u, "size=400*300") {
		t.Errorf("size 应转星号格式: %s", u)
	}
	if !strings.Contains(u, "zoom=15") || !strings.Contains(u, "markers=") {
		t.Errorf("zoom/markers 缺失: %s", u)
	}
	// 标签（URL 编码后的中文 + 15 字符截断）
	if !strings.Contains(u, "labels=") {
		t.Error("label 缺失")
	}
	// zoom 越界 clamp
	if !strings.Contains(StaticMapURL("k", 1, 1, "", "", 99), "zoom=17") {
		t.Error("zoom 应 clamp 到 17")
	}
	// 长标签截断 15 字符
	u2 := StaticMapURL("k", 1, 1, "这是一个超过十五个字符的超级长的门店名称标签", "", 10)
	if strings.Contains(u2, "超级长的门店名称标签") {
		t.Errorf("标签应截断 15 字符: %s", u2)
	}
}

func TestNormalizeSize(t *testing.T) {
	if got := normalizeSize("400x300"); got != "400*300" {
		t.Errorf("x 未转换: %s", got)
	}
	if got := normalizeSize("400*300"); got != "400*300" {
		t.Errorf("星号应保留: %s", got)
	}
}
