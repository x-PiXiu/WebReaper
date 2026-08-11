package port

import (
	"context"
	"errors"
)

// ---- 位置服务接口（本地生活 GEO 的核心适配点）----
//
// 设计动机（策略模式 + 双实现降级，沿用项目惯例）：
//   - 地图供应商（高德/百度/腾讯）是"最易变的外部细节"，用例层只依赖本接口，
//     新增供应商 = 新适配器注册，业务零改动（开闭原则）。
//   - 未配置任何供应商时注入 mock 降级（Geocode 返回 ErrGeoNotConfigured），
//     门店可正常创建（地址先落库，geo_status=pending），后续配置后重试。
//   - 本文件只声明接口与数据契约，实现见 internal/adapter/geo/。

// ErrGeoNotConfigured 地理编码服务未配置（降级标记 pending 的信号，不阻断业务）。
var ErrGeoNotConfigured = errors.New("地理编码服务未配置（AMAP_API_KEY）")

// Location 地理编码结果（纯数据契约）。
type Location struct {
	Lat             float64 // 纬度（WGS84/GCJ-02 由供应商决定，本项目内统一使用）
	Lng             float64 // 经度
	Province        string  // 省
	City            string  // 市
	District        string  // 区/县
	Adcode          string  // 行政区划代码
	FormattedAddress string // 结构化地址（供应商标准化后的完整地址）
}

// ReverseGeocodeResult 逆地理编码结果（坐标 → 地址 + 商圈）。
type ReverseGeocodeResult struct {
	Location
	BusinessArea string // 所属商圈（如"望京"；无商圈数据为空）
	Township     string // 乡镇/街道（社区街道）
	Street       string // 街道名
	StreetNumber string // 门牌号
}

// GeoLocator 地理编码服务（地址 ↔ 经纬度双向）。
type GeoLocator interface {
	// Geocode 把详细地址解析为经纬度与行政区划。
	// 未配置地图服务时返回 ErrGeoNotConfigured（调用方标记 pending，不阻断业务）。
	Geocode(ctx context.Context, address string) (Location, error)
	// ReverseGeocode 把经纬度解析为地址与商圈（P1 商圈补全）。
	// 未配置地图服务时返回 ErrGeoNotConfigured。
	ReverseGeocode(ctx context.Context, lng, lat float64) (ReverseGeocodeResult, error)
}

// Tip 输入提示建议项（地址联想，P1）。
type Tip struct {
	Name     string // tip 名称（如"望京西园"）
	Address  string // 详细地址
	District string // 所属区域（省+市+区）
	Adcode   string // 区域编码
	Location string // 中心点坐标（"lng,lat"）
	POIID    string // POI ID（可联动 ID 查询）
}

// InputTipper 输入提示服务（门店建档地址联想）。
type InputTipper interface {
	// InputTips 按用户输入的关键词返回建议列表（datatype=poi）。
	// city 可填 citycode/adcode（空=全国）；location "lng,lat" 可做附近优先（需 city 非空）。
	// 未配置地图服务时返回 ErrGeoNotConfigured（表单退化为纯手输）。
	InputTips(ctx context.Context, keyword, city, location string) ([]Tip, error)
}

// POIStore 周边 POI 门店（附近同行对比的现实世界数据）。
type POIStore struct {
	Name       string  // 门店名
	Address    string  // 地址
	Distance   int     // 距中心点距离（米）
	Rating     float64 // 评分（0 = 无评分数据；供应商覆盖不全，前端需标"暂无"）
	Category   string  // 行业分类（如"川菜"）
	OpenStatus string  // open/closed/unknown
	Lat        float64
	Lng        float64
	// ---- 门店卡扩展字段（v5 show_fields=business,navi；v3 部分可解析，无数据留空）----
	CityName       string  // 所属城市
	AdName         string  // 所属区县
	Cost           string  // 人均消费（如"¥80/人"；仅餐饮/酒店/景点/影院类有）
	BusinessArea   string  // 所属商圈
	OpenTimeToday  string  // 今日营业时间（如"08:30-17:30"）
	Tag            string  // 特色内容（仅美食 POI：特色菜）
	Tel            string  // 联系电话
	EntrLocation   string  // 入口经纬度（导航到达点，"lng,lat"）
	PhotoURL       string  // 首张照片链接（可选）
}

// POISearcher 周边 POI 搜索（按中心点 + 关键词/类型 + 半径找同行业门店）。
type POISearcher interface {
	// SearchNearby 搜索中心点附近的 POI（按关键词，如品牌名/竞品名）。
	// 未配置地图服务时返回 ErrGeoNotConfigured（调用方降级只显示 AI 竞品榜）。
	SearchNearby(ctx context.Context, center Location, keyword string, radiusM int) ([]POIStore, error)
	// SearchNearbyByType 按 POI 类型搜索（P1：types=050000 餐饮大类——不依赖名称命中的竞品扫描）。
	// types 为六位分类编码（多个用 | 分隔，大类自动包含中/小类）。
	SearchNearbyByType(ctx context.Context, center Location, types string, radiusM int) ([]POIStore, error)
}

// DistanceResult 单条距离测量结果。
type DistanceResult struct {
	OriginIdx   int // 起点序号（对应入参 origins 下标，0 起始）
	DistanceM   int // 路径距离（米）
	DurationSec int // 预计行驶时间（秒）
}

// DistanceMeasurer 距离测量服务（P2：地图榜"驾车耗时"）。
// 支持批量：一次请求最多 100 个起点 → 1 个目的地。
type DistanceMeasurer interface {
	// MeasureDistances 批量测距。typ：0=直线 / 1=驾车（考虑路况）/ 3=步行（仅 5km 内）。
	// 单条失败（无道路/越界等）跳过不阻断——结果里只有成功条目。
	// 未配置地图服务时返回 ErrGeoNotConfigured（调用方降级只显示直线距离）。
	MeasureDistances(ctx context.Context, origins []Location, dest Location, typ int) ([]DistanceResult, error)
}
