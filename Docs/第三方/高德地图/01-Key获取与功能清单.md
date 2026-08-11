# 高德地图 Web 服务 —— Key 获取与功能清单

> 来源：https://console.amap.com（高德开放平台）
> 用途：WebReaper 本地生活 GEO 改造的位置服务（门店地理编码 + 周边同行 POI 搜索）
> 对接代码：`internal/adapter/geo/amap.go`（GeoLocator / POISearcher 适配器 + mock 降级）
> 配置项：`.env` → `AMAP_API_KEY`

---

## 1. Key 获取操作步骤

### 1.1 注册与登录

1. 打开 [高德开放平台](https://console.amap.com/)（控制台地址：https://console.amap.com/dev/key/app）
2. 右上角「登录/注册」——支持**手机号**、**支付宝**扫码快捷登录（个人开发者即可，无需企业资质）

### 1.2 实名认证（必做，否则无法创建 Key）

1. 登录后进入控制台，首页会提示「实名认证」
2. 点击「去认证」→ 选择**个人开发者**
3. 填写姓名 + 身份证号（或按提示使用支付宝/手机号辅助认证）
4. 提交后通常**即时生效**（个别情况需等待审核）

> ⚠️ 实名认证是硬门槛：未认证无法创建应用和 Key，接口调用会返回 `USER_IS_NOT_VALID`。

### 1.3 创建应用

1. 控制台顶部菜单：「应用管理」→「我的应用」→ 点击「**创建新应用**」
2. 填写：
   - 应用名称：如 `webreaper-geo`（自拟，后续可改）
   - 应用类型：选择「**出行**」或「其他」（不影响功能，仅分类）
3. 创建完成后进入应用详情页

### 1.4 添加 Key（关键步骤）

在应用详情页点击「**添加 Key**」，按下表填写：

| 字段 | 填写 | 说明 |
|---|---|---|
| 服务平台 | **Web服务** | ✅ 必须选这个——本项目用的是 REST API（地理编码/周边搜索） |
| Key 名称 | 如 `geo-server` | 自拟 |
| 绑定域名 | `*` 或你的公网域名 | 安全白名单：填 `*` 表示不限域名（开发方便，生产建议填具体域名如 `content.example.com`） |

> ⚠️ 常见错误：选了「Web端(JS API)」或「Android/iOS」平台——那种 Key 是给前端/移动端 SDK 用的，**不能**用于服务端 REST 调用（会返回 `INVALID_USER_KEY`）。

### 1.5 获取 Key 并配置

1. 添加成功后，列表会显示：
   - **Key**（形如 `a1b2c3d4e5f6...`，40 位左右的字符串）
   - 新版还有「安全密钥（Secret）」——**本项目不需要 Secret**，只需 Key
2. 复制 Key → 写入项目配置：

```bash
# configs/.env
AMAP_API_KEY=你的Key
```

3. 重启服务，日志出现：
   - `本地生活位置服务已启用（高德：地理编码 + 周边 POI 搜索）` ✅
   - 若未配置：`本地生活位置服务未配置 AMAP_API_KEY（门店暂不编码，附近同行仅 AI 榜）`（降级模式，功能不报错）

### 1.6 快速验证

```bash
# 地理编码（地址 → 经纬度）
curl "https://restapi.amap.com/v3/geocode/geo?key=你的Key&address=北京市朝阳区望京街10号"

# 周边搜索（找附近川菜馆）
curl "https://restapi.amap.com/v3/place/around?key=你的Key&location=116.47,39.99&keywords=川菜馆&radius=5000"
```

返回 `"status":"1"` 即 Key 可用。

---

## 2. 本项目已实现的功能（对接现状）

| 功能 | 高德 API | 项目落点 | 使用场景 |
|---|---|---|---|
| **地理编码** | `v3/geocode/geo`（地址→经纬度/省市区/adcode） | `AmapGeoCoder`（`adapter/geo/amap.go`） | 门店档案保存/改址/手动重试时自动定位，回填经纬度与区划——**所有本地功能的地基** |
| **周边 POI 搜索** | `v3/place/around`（中心点+关键词+半径） | `AmapPOISearcher` | 附近同行双榜的"地图榜"——以门店为中心搜品牌名+竞品名，返回距离/评分/地址 |

**降级机制**：未配置 Key 时工厂返回 mock（`MockGeoCoder`/`MockPOISearcher`），门店照常创建（`geo_status=pending`）、附近同行只显示 AI 竞品榜——业务永不因地图服务缺失而中断。

---

## 3. 高德 API 可实现的更多功能（未来扩展清单）

> 以下均为高德 Web 服务已开放的能力，结合 GEO/实体餐饮场景给出落地设想，按价值排序。

### 3.1 直接可做（服务端 REST，零前端依赖）

| 功能 | API | 设想用途 |
|---|---|---|
| **逆地理编码** | `v3/geocode/regeo`（经纬度→地址） | 老板手机授权定位后反查店铺地址，免手输；或解析"附近"提问中的坐标 |
| **关键词搜索** | `v3/place/text`（关键词搜 POI） | 竞品精细扫描（按"川菜馆"搜全城再按距离筛选）、"城市+品类"候选门店清单 |
| **POI 详情** | `v3/place/detail` | ⚠️ 官网目录未单列（藏在"搜索 POI"文档）；**本项目无需单独接入**——评分/营业时间/人均/商圈/特色菜在搜索接口已能返回（v5 `show_fields=business`），详见 `高级API/02-搜索POI-2.0.md` § 7 |
| **输入提示** | `v3/assistant/inputtips` | 门店地址表单"边输入边联想"（省地址 + 精确 POI ID） |
| **行政区划查询** | `v3/config/district` | 前端城市/区县级联选择器；关键词生成时按行政区枚举本地词 |
| **距离测量** | `v3/distance` | "距你 3.2 公里"文案；双榜里补直线距离 |
| **天气查询** | `v3/weather/weatherInfo` | 内容生成注入当日天气（"下雨天适合火锅"）——增强内容时效性与本地感 |
| **静态地图** | `v4/staticmap`（图片 URL） | 公开文章页/附近同行页嵌入门店位置静态图（无需 JS） |

### 3.2 需要前端 SDK（地图展示/交互）

| 功能 | 载体 | 设想用途 |
|---|---|---|
| **地图选点** | JS API `AMap.PlaceSearch`/`AMap.MouseTool` | 老板在地图上拖拽/点击选店址，自动回填地址+坐标（比手输地址体验好一个量级） |
| **门店地图卡片** | JS API 地图实例 | 附近同行页内嵌交互地图（点门店看详情）；公开文章页"一键导航" |
| **路径规划** | `v3/direction/driving` 等 | 生成"从哪怎么去"（驾车/公交/步行）——内容里的实用段落，AI 引用价值高 |

### 3.3 本项目的具体落地优先级建议

| 优先级 | 功能 | 说明 |
|---|---|---|
| ✅ 已实现 | **输入提示（地址联想）** | 门店建档 AutoComplete——已落地 |
| ✅ 已实现 | **POI 详情字段** | 由**搜索接口直接返回**（v5 `show_fields=business`）——无需单独接 detail 接口 |
| 🔻 降级 | **静态地图**（公开文章页） | ✅ 已实现但**标"低优先级/可裁剪"**：AI 不引用图片，JSON-LD `geo` 已覆盖结构信号；不再投入（见计划文档 § 五.5） |
| 🔒 冻结 | **天气注入**（内容生成） | 不再新增——等真实用户数据驱动 |
| 🔒 冻结 | 路径规划/行政区划/地图选点（JS API） | 不再新增——同上 |

---

## 4. 配额与商用注意事项

### 4.1 免费配额（个人开发者，参考值——以控制台「配额管理」页为准）

| API | 参考日配额 |
|---|---|
| 地理编码/逆地理编码 | 5000 次/日 |
| 周边搜索/关键词搜索/输入提示 | 1000 次/日（早期 5000，已调整） |
| 路径规划/静态地图/天气 | 数百~千次/日不等 |

> ⚠️ 高德配额政策**经常调整**（2024 年起多次收紧），务必以控制台实际显示为准。本项目每次"附近同行"搜索会按"品牌名+竞品名"各调 1 次周边搜索（5 个竞品 = 5 次/次操作），SaaS 多租户放大后需监控用量。

### 4.2 商用合规

- 个人开发者免费额度仅限**个人学习/非商业**用途；**SaaS 商用必须购买「商业授权」**（高德开放平台 → 商务合作；或转企业认证开发者 + 商用套餐）
- 禁止批量抓取 POI 数据转售/导出（服务条款红线）
- 本项目按需实时查询、聚合展示，属于合规用法；**上线商用前必须完成授权评估**

### 4.3 常见报错排查

| 报错信息 | 原因 | 处理 |
|---|---|---|
| `INVALID_USER_KEY` | Key 错误或平台类型选错（选了 JS/Android 而非 Web服务） | 检查 Key；重新创建「Web服务」Key |
| `USER_IS_NOT_VALID` | 未实名认证 | 完成实名认证 |
| `DAILY_QUERY_OVER_LIMIT` | 当日配额耗尽 | 控制台查看配额；升级套餐或减少调用 |
| `INVALID_PARAMS` | 参数格式错误（location 必须是 "lng,lat" 逗号分隔） | 对照接口文档检查 |
| `"status":"0"` + info 描述 | 业务错误 | 按 info 文案排查 |

---

## 5. 相关文档

- 高德 Web 服务 API 文档：https://lbs.amap.com/api/webservice/summary
- 地理编码：https://lbs.amap.com/api/webservice/guide/api/georegeo
- 周边搜索：https://lbs.amap.com/api/webservice/guide/api/search
- 本项目对接代码：`internal/adapter/geo/amap.go` + `amap_test.go`（httptest 模拟响应，离线可测）

---

## 6. API 文档获取清单（去高德官网下载时对照）

> 高德文档中心：https://lbs.amap.com/api/webservice/summary（Web 服务 API 总览）
> 按下表逐个打开对应文档页即可。标注 ⭐ = 本项目对接代码需要的接口（服务端 REST，用「Web服务」Key）。

### 6.1 服务端 REST（Web服务 Key）—— 本项目对接用

| 优先级 | API | 接口路径 | 本项目用途 | 文档地址 |
|---|---|---|---|---|
| ⭐ 已用 | **地理编码** | `v3/geocode/geo` | 门店地址→经纬度/区划（已实现） | https://lbs.amap.com/api/webservice/guide/api/georegeo |
| ⭐ 已用 | **周边搜索** | `v5/place/around`（默认）/ `v3/place/around`（兼容） | 附近同行地图榜（✅ v5 迁移完成：`show_fields=business,navi`，评分/人均/商圈/营业时间/特色菜/电话一次拿全；`AMAP_API_VERSION` 可切回 v3） | https://lbs.amap.com/api/webservice/guide/api/search |
| ⭐ P1 | **逆地理编码** | `v3/geocode/regeo` | 经纬度→地址（手机定位反查店铺地址） | https://lbs.amap.com/api/webservice/guide/api/georegeo（同页） |
| ⭐ P1 | **关键词搜索** | `v3/place/text` | 全城"品类"扫描、竞品候选清单 | https://lbs.amap.com/api/webservice/guide/api/search（同页） |
| ⭐ P1 | **POI 详情** | `v3/place/detail` | ⚠️ 官网目录未单列；**本项目不需要**（搜索返回已覆盖所需字段） | https://lbs.amap.com/api/webservice/guide/api/search（同页） |
| ⭐ P1 | **输入提示** | `v3/assistant/inputtips` | 地址表单联想（省地址+精确 POI） | https://lbs.amap.com/api/webservice/guide/api/inputtips |
| ⭐ P2 | **距离测量** | `v3/distance` | "距你 X 公里"、双榜距离补全 | https://lbs.amap.com/api/webservice/guide/api/distance |
| P2 | **行政区划查询** | `v3/config/district` | 城市/区县级联选择器 | https://lbs.amap.com/api/webservice/guide/api/district |
| P2 | **天气查询** | `v3/weather/weatherInfo` | 内容生成注入天气（本地感） | https://lbs.amap.com/api/webservice/guide/api/weatherinfo |
| P2 | **静态地图** | `v4/staticmap` | 文章页嵌门店位置图（无需前端） | https://lbs.amap.com/api/webservice/guide/api/staticmaps |
| P3 | **路径规划** | `v3/direction/driving` 等 | "怎么去"内容段落 | https://lbs.amap.com/api/webservice/guide/api/direction |

### 6.2 前端 JS API（需另建「Web端(JS API)」类型的 Key + 安全密钥 jscode）

> ⚠️ 与 Web 服务 Key 是**两个不同的 Key 类型**，要在地图选点/展示时才需要，届时再创建。

| 优先级 | 能力 | 本项目用途 | 文档地址 |
|---|---|---|---|
| P1 | **地图选点**（PlaceSearch/MouseTool） | 老板地图上拖点选店址，自动回填地址+坐标 | https://lbs.amap.com/api/javascript-api-v2/documentation |
| P2 | **地图展示**（门店卡片/导航按钮） | 附近同行页内嵌地图、文章页"一键导航" | 同上 |

### 6.3 获取文档时的要点提醒

1. 先拿 **6.1 打 ⭐ 的 8 个接口**（覆盖当前与近期规划），其余按需
2. 每个文档页重点看：**请求参数表 + 返回 JSON 示例 + 错误码表**（本项目对接代码已就绪，拿文档主要用于字段核对与扩展）
3. **配额页**：控制台「配额管理」页确认各接口免费额度（政策会调整）
4. JS API 文档（6.2）现在**不用拿**——用到时再取，且需要单独建 Key

---

## 8. 对接实现状态（2026-08-11 更新）

| API | 状态 | 落点 |
|---|---|---|
| 地理编码 `v3/geocode/geo` | ✅ 已实现 | `AmapGeoCoder.Geocode` |
| 逆地理编码 `v3/geocode/regeo` | ✅ 已实现 | `AmapGeoCoder.ReverseGeocode`——**商圈补全**：门店定位后自动回填 `business_area`（迁移 031） |
| 周边搜索 `v5/place/around` | ✅ 已实现（v5 默认） | `AmapV5POISearcher`（show_fields=business,navi）；v3 兼容开关 `AMAP_API_VERSION` |
| POI 类型搜索 | ✅ 已实现 | `SearchNearbyByType`（v3/v5）——附近同行页"类型编码"输入框（如 050000 餐饮） |
| 输入提示 `v3/assistant/inputtips` | ✅ 已实现 | `AmapInputTipper` + `GET /geo/location/suggest`——门店建档地址 AutoComplete 联想 |
| 距离测量 `v3/distance` | ✅ 已实现 | `AmapDistanceMeasurer`——地图榜"🚗 驾车约 N 分钟" |
| 静态地图 `v3/staticmap` | ✅ 已实现 | `StaticMapURL` 纯函数 + `GET /public/store-map/:storeId`（302，Key 不暴露）+ 文章页门店位置图 |
| POI 详情 `v3/place/detail` | ⛔ 不需要 | 搜索返回已覆盖所需字段（评分/人均/商圈/营业时间） |

**新增端点**：`GET /geo/location/suggest`（商户地址联想）、`GET /public/store-map/:storeId`（门店位置图）
**新增迁移**：`031_store_business_area.sql`（商圈补全）
**配置**：`AMAP_API_VERSION`（v5 默认 / v3 兼容）
