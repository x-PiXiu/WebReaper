// Package geo 提供位置服务适配器（高德地图 + mock 降级）。
//
// 整洁架构落点（策略模式 + 双实现降级）：
//   - 用例层只依赖 port.GeoLocator / port.POISearcher 接口（见
//     internal/usecase/port/geo_locator.go），本包是"最易变的外部细节"——
//     地图供应商的 HTTP/密钥/限额全部隔离在最外层。
//   - 未配置 AMAP_API_KEY 时工厂返回 mock 实现：Geocode/SearchNearby 返回
//     port.ErrGeoNotConfigured，业务降级不阻断（门店先落库、双榜只显示 AI 榜）。
//   - 新增供应商（百度/腾讯）= 本包新实现 + 工厂分支，用例零改动（开闭原则）。
package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"webreaper/internal/usecase/port"
)

const (
	amapGeoBaseURL   = "https://restapi.amap.com/v3/geocode/geo"
	amapRegeoBaseURL = "https://restapi.amap.com/v3/geocode/regeo"
	amapPOIBaseURL   = "https://restapi.amap.com/v3/place/around"
	amapV5POIBaseURL = "https://restapi.amap.com/v5/place/around"
	amapTipsBaseURL  = "https://restapi.amap.com/v3/assistant/inputtips"
	amapDistBaseURL  = "https://restapi.amap.com/v3/distance"
	amapHTTPTimeout  = 8 * time.Second
)

// ---- 高德地理编码 ----

// AmapGeoCoder 高德地理编码适配器（v3/geocode/geo + regeo）。
type AmapGeoCoder struct {
	apiKey   string
	baseURL  string // 可注入（测试用）；空则用默认
	regeoURL string // 逆地理编码 URL（可注入）
	httpDo   func(*http.Request) (*http.Response, error)
}

// NewAmapGeoCoder 创建高德地理编码器（apiKey 为空时返回 mock 降级）。
func NewAmapGeoCoder(apiKey string) port.GeoLocator {
	if apiKey == "" {
		return MockGeoCoder{}
	}
	return &AmapGeoCoder{apiKey: apiKey, baseURL: amapGeoBaseURL, regeoURL: amapRegeoBaseURL}
}

// Geocode 地址 → 经纬度/行政区划。
func (g *AmapGeoCoder) Geocode(ctx context.Context, address string) (port.Location, error) {
	if strings.TrimSpace(address) == "" {
		return port.Location{}, fmt.Errorf("地址不能为空")
	}
	u := g.baseURL + "?key=" + url.QueryEscape(g.apiKey) + "&address=" + url.QueryEscape(address)
	resp, err := g.do(ctx, u)
	if err != nil {
		return port.Location{}, fmt.Errorf("高德地理编码请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return port.Location{}, fmt.Errorf("读取高德响应失败: %w", err)
	}
	var parsed struct {
		Status   string `json:"status"`
		Info     string `json:"info"`
		Geocodes []struct {
			Location        string `json:"location"` // "lng,lat"
			Province        string `json:"province"`
			City            string `json:"city"`
			District        string `json:"district"`
			Adcode          string `json:"adcode"`
			FormattedAddress string `json:"formatted_address"`
		} `json:"geocodes"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return port.Location{}, fmt.Errorf("解析高德响应失败: %w", err)
	}
	if parsed.Status != "1" || len(parsed.Geocodes) == 0 {
		return port.Location{}, fmt.Errorf("高德地理编码未命中: %s", parsed.Info)
	}
	gc := parsed.Geocodes[0]
	lng, lat, err := parseLngLat(gc.Location)
	if err != nil {
		return port.Location{}, fmt.Errorf("解析高德坐标失败: %w", err)
	}
	return port.Location{
		Lat: lat, Lng: lng,
		Province: gc.Province, City: gc.City, District: gc.District,
		Adcode: gc.Adcode, FormattedAddress: gc.FormattedAddress,
	}, nil
}

func (g *AmapGeoCoder) do(ctx context.Context, u string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	do := g.httpDo
	if do == nil {
		do = (&http.Client{Timeout: amapHTTPTimeout}).Do
	}
	return do(req)
}

// ---- 高德周边 POI 搜索 ----

// AmapPOISearcher 高德周边搜索适配器（v3/place/around，兼容保留）。
type AmapPOISearcher struct {
	apiKey  string
	baseURL string
	httpDo  func(*http.Request) (*http.Response, error)
}

// NewAmapPOISearcher 创建高德周边搜索器（apiKey 为空时返回 mock 降级）。
// version 决定接口版本（策略工厂）：
//   - "v5"（默认，推荐）：v5/place/around + show_fields=business,navi——
//     评分/人均/商圈/营业时间/特色菜/电话一次拿全，响应最小
//   - "v3"：旧版 place/around（extensions=base）——保留兼容作降级开关
func NewAmapPOISearcher(apiKey, version string) port.POISearcher {
	if apiKey == "" {
		return MockPOISearcher{}
	}
	if version == "v3" {
		return &AmapPOISearcher{apiKey: apiKey, baseURL: amapPOIBaseURL}
	}
	return &AmapV5POISearcher{apiKey: apiKey, baseURL: amapV5POIBaseURL}
}

// SearchNearby 按中心点 + 关键词 + 半径搜索 POI（v3 兼容实现）。
func (p *AmapPOISearcher) SearchNearby(ctx context.Context, center port.Location, keyword string, radiusM int) ([]port.POIStore, error) {
	if center.Lng == 0 && center.Lat == 0 {
		return nil, fmt.Errorf("中心点坐标无效")
	}
	if radiusM <= 0 {
		radiusM = 5000 // 默认 5km（本地生活合理半径）
	}
	if radiusM > 50000 {
		radiusM = 50000 // 文档：>50000 按默认值处理——显式 clamp 避免意外行为
	}
	u := fmt.Sprintf("%s?key=%s&location=%f,%f&keywords=%s&radius=%d&offset=25&extensions=base",
		p.baseURL, url.QueryEscape(p.apiKey), center.Lng, center.Lat, url.QueryEscape(keyword), radiusM)
	return p.doSearch(ctx, u)
}

// ---- 高德周边 POI 搜索 2.0（v5，推荐）----


// AmapV5POISearcher 高德周边搜索适配器（v5/place/around）。
//
// v5 vs v3（详见 Docs/第三方/高德地图/高级API/02-搜索POI-2.0.md § 2）：
//   - 字段控制：show_fields 精确筛选字段组（business/navi）——比 v3 extensions=all 响应更小
//   - 返回内容：评分/人均/商圈/营业时间/特色菜/电话/入口坐标一次拿全
//   - 输出仅 JSON；分页参数 page_size/page_num（≤25）
type AmapV5POISearcher struct {
	apiKey  string
	baseURL string
	httpDo  func(*http.Request) (*http.Response, error)
}

// SearchNearby 按中心点 + 关键词 + 半径搜索 POI（v5 + show_fields=business,navi）。
func (p *AmapV5POISearcher) SearchNearby(ctx context.Context, center port.Location, keyword string, radiusM int) ([]port.POIStore, error) {
	if center.Lng == 0 && center.Lat == 0 {
		return nil, fmt.Errorf("中心点坐标无效")
	}
	if radiusM <= 0 {
		radiusM = 5000
	}
	if radiusM > 50000 {
		radiusM = 50000 // 文档：>50000 按默认值——显式 clamp
	}
	// v5 请求参数（对照官方文档）：key/location/radius/keywords/show_fields/page_size
	// show_fields=business,navi → 评分/人均/商圈/营业时间/特色菜/电话/入口坐标
	u := fmt.Sprintf("%s?key=%s&location=%f,%f&radius=%d&keywords=%s&show_fields=business,navi&page_size=25",
		p.baseURL, url.QueryEscape(p.apiKey), center.Lng, center.Lat, radiusM, url.QueryEscape(keyword))
	return p.doSearchV5(ctx, u)
}

// ---- mock 降级（未配置 AMAP_API_KEY）----

// MockGeoCoder 降级实现：未配置地图服务时返回 ErrGeoNotConfigured
// （调用方标记 geo_status=pending，不阻断门店创建——配置后重试即可）。
type MockGeoCoder struct{}

func (MockGeoCoder) Geocode(ctx context.Context, address string) (port.Location, error) {
	return port.Location{}, port.ErrGeoNotConfigured
}

func (MockGeoCoder) ReverseGeocode(ctx context.Context, lng, lat float64) (port.ReverseGeocodeResult, error) {
	return port.ReverseGeocodeResult{}, port.ErrGeoNotConfigured
}

// MockPOISearcher 降级实现：未配置地图服务时返回 ErrGeoNotConfigured
// （调用方降级只显示 AI 竞品榜）。
type MockPOISearcher struct{}

func (MockPOISearcher) SearchNearby(ctx context.Context, center port.Location, keyword string, radiusM int) ([]port.POIStore, error) {
	return nil, port.ErrGeoNotConfigured
}

func (MockPOISearcher) SearchNearbyByType(ctx context.Context, center port.Location, types string, radiusM int) ([]port.POIStore, error) {
	return nil, port.ErrGeoNotConfigured
}

// MockInputTipper 降级实现：未配置地图服务时返回 ErrGeoNotConfigured
// （地址表单退化为纯手输，行为不变）。
type MockInputTipper struct{}

func (MockInputTipper) InputTips(ctx context.Context, keyword, city, location string) ([]port.Tip, error) {
	return nil, port.ErrGeoNotConfigured
}

// MockDistanceMeasurer 降级实现：未配置地图服务时返回 ErrGeoNotConfigured
// （地图榜只显示直线距离，行为不变）。
type MockDistanceMeasurer struct{}

func (MockDistanceMeasurer) MeasureDistances(ctx context.Context, origins []port.Location, dest port.Location, typ int) ([]port.DistanceResult, error) {
	return nil, port.ErrGeoNotConfigured
}

// ---- 工具 ----

// parseLngLat 解析高德 "lng,lat" 坐标串。
func parseLngLat(s string) (lng, lat float64, err error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("非法坐标 %q", s)
	}
	lng, err = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, err
	}
	lat, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, err
	}
	return lng, lat, nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// ---- 高德逆地理编码（P1：商圈补全）----

// ReverseGeocode 经纬度 → 地址 + 商圈（v3/geocode/regeo + extensions=all）。
// 商圈来自 regeocode.businessAreas[0].name——门店档案"商圈"字段的数据源。
func (g *AmapGeoCoder) ReverseGeocode(ctx context.Context, lng, lat float64) (port.ReverseGeocodeResult, error) {
	u := fmt.Sprintf("%s?key=%s&location=%f,%f&extensions=all&radius=1000",
		g.regeoURL, url.QueryEscape(g.apiKey), lng, lat)
	resp, err := g.do(ctx, u)
	if err != nil {
		return port.ReverseGeocodeResult{}, fmt.Errorf("高德逆地理编码请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return port.ReverseGeocodeResult{}, fmt.Errorf("读取高德响应失败: %w", err)
	}
	var parsed struct {
		Status   string `json:"status"`
		Info     string `json:"info"`
		Regeocode struct {
			AddressComponent struct {
				Province string `json:"province"`
				City     string `json:"city"` // 直辖市/省直辖县为空
				District string `json:"district"`
				Adcode   string `json:"adcode"`
				Township string `json:"township"`
				StreetNumber struct {
					Street string `json:"street"`
					Number string `json:"number"`
				} `json:"streetNumber"`
			} `json:"addressComponent"`
			BusinessAreas []struct {
				Name string `json:"name"`
			} `json:"businessAreas"`
		} `json:"regeocode"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return port.ReverseGeocodeResult{}, fmt.Errorf("解析高德逆编码响应失败: %w", err)
	}
	if parsed.Status != "1" {
		return port.ReverseGeocodeResult{}, fmt.Errorf("高德逆地理编码失败: %s", parsed.Info)
	}
	res := port.ReverseGeocodeResult{
		Location: port.Location{
			Lat: lat, Lng: lng,
			Province: parsed.Regeocode.AddressComponent.Province,
			City:     parsed.Regeocode.AddressComponent.City,
			District: parsed.Regeocode.AddressComponent.District,
			Adcode:   parsed.Regeocode.AddressComponent.Adcode,
		},
		Township:     parsed.Regeocode.AddressComponent.Township,
		Street:       parsed.Regeocode.AddressComponent.StreetNumber.Street,
		StreetNumber: parsed.Regeocode.AddressComponent.StreetNumber.Number,
	}
	if len(parsed.Regeocode.BusinessAreas) > 0 {
		res.BusinessArea = parsed.Regeocode.BusinessAreas[0].Name
	}
	return res, nil
}

// ---- 高德输入提示（P1：门店建档地址联想）----

// AmapInputTipper 高德输入提示适配器（v3/assistant/inputtips + datatype=poi）。
type AmapInputTipper struct {
	apiKey  string
	baseURL string
	httpDo  func(*http.Request) (*http.Response, error)
}

// NewAmapInputTipper 创建输入提示器（apiKey 为空时返回 mock 降级）。
func NewAmapInputTipper(apiKey string) port.InputTipper {
	if apiKey == "" {
		return MockInputTipper{}
	}
	return &AmapInputTipper{apiKey: apiKey, baseURL: amapTipsBaseURL}
}

// InputTips 按关键词返回地址建议（datatype=poi 限定 POI 类型）。
// city 可填 citycode/adcode（空=全国）；location "lng,lat" 附近优先（需 city 非空）。
func (t *AmapInputTipper) InputTips(ctx context.Context, keyword, city, location string) ([]port.Tip, error) {
	u := fmt.Sprintf("%s?key=%s&keywords=%s&datatype=poi",
		t.baseURL, url.QueryEscape(t.apiKey), url.QueryEscape(keyword))
	if city != "" {
		u += "&city=" + url.QueryEscape(city)
	}
	if location != "" {
		u += "&location=" + url.QueryEscape(location)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	do := t.httpDo
	if do == nil {
		do = (&http.Client{Timeout: amapHTTPTimeout}).Do
	}
	resp, err := do(req)
	if err != nil {
		return nil, fmt.Errorf("高德输入提示请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取高德响应失败: %w", err)
	}
	var parsed struct {
		Status string `json:"status"`
		Info   string `json:"info"`
		Tips   []struct {
			Id       string          `json:"id"`
			Name     string          `json:"name"`
			District string          `json:"district"`
			Adcode   string          `json:"adcode"`
			Location string          `json:"location"`
			Address  json.RawMessage `json:"address"` // 高德混型：字符串或数组（[] 常见）——RawMessage 兼容
		} `json:"tips"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析高德输入提示响应失败: %w", err)
	}
	if parsed.Status != "1" {
		return nil, fmt.Errorf("高德输入提示失败: %s", parsed.Info)
	}
	out := make([]port.Tip, 0, len(parsed.Tips))
	for _, tip := range parsed.Tips {
		out = append(out, port.Tip{
			Name: tip.Name, Address: rawAddressToStr(tip.Address), District: tip.District,
			Adcode: tip.Adcode, Location: tip.Location, POIID: tip.Id,
		})
	}
	return out, nil
}

// rawAddressToStr 高德 inputtips address 混型归一：字符串原样，数组 join（多数为空数组）。
func rawAddressToStr(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil {
		return strings.Join(arr, "")
	}
	return strings.Trim(string(raw), `"`)
}

// ---- 高德距离测量（P2：地图榜驾车耗时）----

// AmapDistanceMeasurer 高德距离测量适配器（v3/distance）。
type AmapDistanceMeasurer struct {
	apiKey  string
	baseURL string
	httpDo  func(*http.Request) (*http.Response, error)
}

// NewAmapDistanceMeasurer 创建距离测量器（apiKey 为空时返回 mock 降级）。
func NewAmapDistanceMeasurer(apiKey string) port.DistanceMeasurer {
	if apiKey == "" {
		return MockDistanceMeasurer{}
	}
	return &AmapDistanceMeasurer{apiKey: apiKey, baseURL: amapDistBaseURL}
}

// MeasureDistances 批量测距（≤100 起点 → 1 目的地）。
// 单条失败（results[].info 非空/无该条）跳过不阻断——调用方按 OriginIdx 映射。
func (m *AmapDistanceMeasurer) MeasureDistances(ctx context.Context, origins []port.Location, dest port.Location, typ int) ([]port.DistanceResult, error) {
	if len(origins) == 0 {
		return nil, nil
	}
	if len(origins) > 100 {
		origins = origins[:100] // 文档上限 100
	}
	parts := make([]string, 0, len(origins))
	for _, o := range origins {
		parts = append(parts, fmt.Sprintf("%f,%f", o.Lng, o.Lat))
	}
	u := fmt.Sprintf("%s?key=%s&origins=%s&destination=%f,%f&type=%d",
		m.baseURL, url.QueryEscape(m.apiKey), strings.Join(parts, "|"), dest.Lng, dest.Lat, typ)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	do := m.httpDo
	if do == nil {
		do = (&http.Client{Timeout: amapHTTPTimeout}).Do
	}
	resp, err := do(req)
	if err != nil {
		return nil, fmt.Errorf("高德距离测量请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取高德响应失败: %w", err)
	}
	var parsed struct {
		Status  string `json:"status"`
		Info    string `json:"info"`
		Results []struct {
			OriginID string `json:"origin_id"` // 1 起始
			Distance string `json:"distance"`  // 米
			Duration string `json:"duration"`  // 秒
			Info     string `json:"info"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析高德距离响应失败: %w", err)
	}
	if parsed.Status != "1" {
		return nil, fmt.Errorf("高德距离测量失败: %s", parsed.Info)
	}
	out := make([]port.DistanceResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		if r.Info != "" {
			continue // 单条失败（无道路/越界）跳过——不阻断整体
		}
		idx, _ := strconv.Atoi(r.OriginID)
		dist, _ := strconv.Atoi(r.Distance)
		dur, _ := strconv.Atoi(r.Duration)
		out = append(out, port.DistanceResult{
			OriginIdx: idx - 1, // 高德 1 起始 → 入参 0 起始
			DistanceM: dist, DurationSec: dur,
		})
	}
	return out, nil
}

// ---- 高德周边搜索按类型（P1：types=050000 竞品扫描）----

// SearchNearbyByType v3 兼容实现：types 参数替换 keywords。
func (p *AmapPOISearcher) SearchNearbyByType(ctx context.Context, center port.Location, types string, radiusM int) ([]port.POIStore, error) {
	if center.Lng == 0 && center.Lat == 0 {
		return nil, fmt.Errorf("中心点坐标无效")
	}
	if radiusM <= 0 {
		radiusM = 5000
	}
	if radiusM > 50000 {
		radiusM = 50000
	}
	u := fmt.Sprintf("%s?key=%s&location=%f,%f&types=%s&radius=%d&offset=25&extensions=base",
		p.baseURL, url.QueryEscape(p.apiKey), center.Lng, center.Lat, url.QueryEscape(types), radiusM)
	return p.doSearch(ctx, u)
}

// SearchNearbyByType v5 实现：types 参数 + show_fields=business,navi。
func (p *AmapV5POISearcher) SearchNearbyByType(ctx context.Context, center port.Location, types string, radiusM int) ([]port.POIStore, error) {
	if center.Lng == 0 && center.Lat == 0 {
		return nil, fmt.Errorf("中心点坐标无效")
	}
	if radiusM <= 0 {
		radiusM = 5000
	}
	if radiusM > 50000 {
		radiusM = 50000
	}
	u := fmt.Sprintf("%s?key=%s&location=%f,%f&types=%s&radius=%d&show_fields=business,navi&page_size=25",
		p.baseURL, url.QueryEscape(p.apiKey), center.Lng, center.Lat, url.QueryEscape(types), radiusM)
	return p.doSearchV5(ctx, u)
}
