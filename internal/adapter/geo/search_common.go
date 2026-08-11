package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"webreaper/internal/usecase/port"
)

// ---- 周边搜索公共执行/解析（v3/v5 各自实现，SearchNearby/SearchNearbyByType 共用）----

// doSearch v3 响应执行与解析。
func (p *AmapPOISearcher) doSearch(ctx context.Context, u string) ([]port.POIStore, error) {
	body, err := httpGet(ctx, u, p.httpDo)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Status string `json:"status"`
		Info   string `json:"info"`
		POIs   []struct {
			Name     string `json:"name"`
			Address  string `json:"address"`
			Distance string `json:"distance"`
			Type     string `json:"type"`
			Location string `json:"location"`
			Rating   string `json:"rating"`
			BizExt   struct {
				Rating string `json:"rating"`
			} `json:"biz_ext"`
		} `json:"pois"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析高德 POI 响应失败: %w", err)
	}
	if parsed.Status != "1" {
		return nil, fmt.Errorf("高德周边搜索失败: %s", parsed.Info)
	}
	out := make([]port.POIStore, 0, len(parsed.POIs))
	for _, poi := range parsed.POIs {
		rating, _ := strconv.ParseFloat(firstNonEmpty(poi.Rating, poi.BizExt.Rating), 64)
		dist, _ := strconv.Atoi(poi.Distance)
		lng, lat, _ := parseLngLat(poi.Location)
		out = append(out, port.POIStore{
			Name: poi.Name, Address: poi.Address,
			Distance: dist, Rating: rating, Category: poi.Type,
			OpenStatus: "unknown",
			Lat: lat, Lng: lng,
		})
	}
	return out, nil
}

// doSearchV5 v5 响应执行与解析。
func (p *AmapV5POISearcher) doSearchV5(ctx context.Context, u string) ([]port.POIStore, error) {
	body, err := httpGet(ctx, u, p.httpDo)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Status  string `json:"status"`
		Info    string `json:"info"`
		Infocode string `json:"infocode"`
		POIs    []struct {
			Name     string `json:"name"`
			Address  string `json:"address"`
			Distance string `json:"distance"`
			Type     string `json:"type"`
			Location string `json:"location"`
			CityName string `json:"cityname"`
			AdName   string `json:"adname"`
			Business struct {
				BusinessArea  string `json:"business_area"`
				OpenTimeToday string `json:"opentime_today"`
				Tel           string `json:"tel"`
				Tag           string `json:"tag"`
				Rating        string `json:"rating"`
				Cost          string `json:"cost"`
			} `json:"business"`
			Navi struct {
				EntrLocation string `json:"entr_location"`
			} `json:"navi"`
			Photos []struct {
				URL string `json:"url"`
			} `json:"photos"`
		} `json:"pois"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析高德 v5 POI 响应失败: %w", err)
	}
	if parsed.Status != "1" {
		return nil, fmt.Errorf("高德 v5 周边搜索失败: %s（infocode=%s）", parsed.Info, parsed.Infocode)
	}
	out := make([]port.POIStore, 0, len(parsed.POIs))
	for _, poi := range parsed.POIs {
		rating, _ := strconv.ParseFloat(poi.Business.Rating, 64)
		dist, _ := strconv.Atoi(poi.Distance)
		lng, lat, _ := parseLngLat(poi.Location)
		photo := ""
		if len(poi.Photos) > 0 {
			photo = poi.Photos[0].URL
		}
		out = append(out, port.POIStore{
			Name: poi.Name, Address: poi.Address,
			Distance: dist, Rating: rating, Category: poi.Type,
			OpenStatus:   "unknown",
			Lat:          lat, Lng:  lng,
			CityName:      poi.CityName,
			AdName:        poi.AdName,
			Cost:          poi.Business.Cost,
			BusinessArea:  poi.Business.BusinessArea,
			OpenTimeToday: poi.Business.OpenTimeToday,
			Tag:           poi.Business.Tag,
			Tel:           poi.Business.Tel,
			EntrLocation:  poi.Navi.EntrLocation,
			PhotoURL:      photo,
		})
	}
	return out, nil
}

// httpGet 通用 GET 请求（复用超时客户端），返回响应体。
func httpGet(ctx context.Context, u string, httpDo func(*http.Request) (*http.Response, error)) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	do := httpDo
	if do == nil {
		do = (&http.Client{Timeout: amapHTTPTimeout}).Do
	}
	resp, err := do(req)
	if err != nil {
		return nil, fmt.Errorf("高德请求失败: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}
