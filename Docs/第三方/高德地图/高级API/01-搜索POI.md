# 高德 Web 服务 API —— 搜索 POI

> 来源：高德开放平台官方文档（最后更新时间：2026-07-15）
> 服务地址：`https://restapi.amap.com`（HTTP/HTTPS，GET 请求）
> 归档位置：`Docs/第三方/高德地图/高级API/`
> 关联：本项目已对接**周边搜索**（`AmapPOISearcher`，`internal/adapter/geo/amap.go`）

---

## 1. 产品介绍

搜索服务 API 提供 **四种 POI 查询机制**：

| 机制 | 接口 | 场景 |
|---|---|---|
| **关键字搜索** | `v3/place/text` | 按 POI 关键字/类型搜索（如"肯德基"、"银行"） |
| **周边搜索** | `v3/place/around` | 以某坐标点为中心、设定半径搜索（✅ 本项目已实现） |
| **多边形搜索** | `v3/place/polygon` | 在多边形区域内搜索 |
| **ID 查询** | `v3/place/detail` | 通过 POI ID 查详情（建议与输入提示 API 配合） |

**重要限制**：
- **搜索不支持返回全量数据**——同参数翻页最多获取 **200 条**
- **城市精确度**：`city` 参数可接收 citycode 或 adcode——citycode 仅精确到城市，**adcode 可精确到区县**（强烈建议用 adcode）
  - 例：北京 citycode=010、adcode=110000；北京-海淀区 adcode=110108

**使用流程（三步）**：申请 Web 服务 Key → 拼接 URL（Key 必填）→ 解析返回 JSON/XML。

---

## 2. 关键字搜索（`v3/place/text`）

### 2.1 服务地址

```
https://restapi.amap.com/v3/place/text?parameters
请求方式：GET
```

### 2.2 请求参数

| 参数名 | 含义 | 规则说明 | 是否必填 | 缺省值 |
|---|---|---|---|---|
| `key` | 权限标识 | Web 服务 Key | 必填 | 无 |
| `keywords` | 查询关键字 | **只支持一个关键字**；不指定 city 且搜泛词（如"美食"）时返回城市列表及命中数 | 必填（keywords/types 二选一） | 无 |
| `types` | 查询 POI 类型 | 分类代码（六位数字）或汉字；多个用 `\|` 分隔；**指定大类则所属中类/小类都被包含**（例：010000 汽车服务 → 010100 加油站 → 010101 中国石化） | 必填（keywords/types 二选一） | 无 |
| `city` | 查询城市 | 城市中文 / 全拼 / citycode / **adcode**；填了优先返回该城市数据（不一定仅限该市，需配合 citylimit） | 可选 | 全国 |
| `citylimit` | 仅返回指定城市 | `true`/`false` | 可选 | false |
| `children` | 子 POI 层级展示 | `1`=子 POI 归类到父 POI（extensions=all 或空时生效） | 可选 | 0 |
| `offset` | 每页记录数 | **强烈建议 ≤25**，超过可能报错 | 可选 | 20 |
| `page` | 当前页数 | 翻页；总数据 ≤200 条 | 可选 | 1 |
| `langCode` | 返回语言 | zh / en（多语言为高级服务，需商务咨询） | 可选 | zh |
| `extensions` | 返回结果控制 | `base`=基本地址信息；`all`=地址信息 + 附近 POI + 道路 + 道路交叉口 | 可选 | base |
| `sig` / `callback` | 签名 / 回调 | 同其他接口 | 可选 | 无 |

### 2.3 返回参数

| 名称 | 含义 |
|---|---|
| `status` | 结果状态（0 失败 / 1 成功） |
| `info` | 状态说明 |
| `count` | 搜索方案数目 |
| `suggestion` | 城市建议列表（限定城市未命中时返回建议城市） |
| `pois[]` | 搜索 POI 信息列表（见下） |

`pois[]` 字段（重点标注 extensions=all 才返回的字段）：

| 字段 | 含义 | 说明 |
|---|---|---|
| `id` / `parent` | POI ID / 父 POI ID | parent 可能为空 |
| `name` | 名称 | — |
| `type` / `typecode` | 兴趣点类型（大类;中类;小类）/ 类型编码 | 例：餐饮服务;中餐厅;特色/地方风味餐厅 / 050118 |
| `biz_type` | 行业类型 | — |
| `address` | 地址 | 如"东四环中路189号百盛北门" |
| `location` | 经纬度 | 格式 X,Y |
| `distance` | 离中心点距离（米） | **仅周边搜索时有值** |
| `tel` | 电话 | — |
| `postcode` / `website` / `email` | 邮编 / 网址 / 邮箱 | all |
| `pcode` / `pname` / `citycode` / `cityname` / `adcode` / `adname` | 省编码/省名/城市编码/城市名/区域编码/区域名（如"朝阳区"） | all |
| `entr_location` | POI 入口经纬度 | all；可用作导航到达点 |
| `exit_location` | 出口经纬度 | 目前不返回内容 |
| `navi_poiid` | POI 导航 id | all |
| `alias` | 别名 | all |
| `tag` | POI 特色内容 | **主要出现在美食类 POI，代表特色菜**，如"烤鱼,麻辣香锅,老干妈回锅肉"；all |
| `business_area` | **所属商圈** | all |
| `indoor_map` / `indoor_data` | 室内地图标志/数据 | all |
| `cpid` / `floor` / `truefloor` | 父级 POI / 楼层索引 / 所在楼层 | — |
| `biz_ext` | 深度信息（all）： | 见下 |
| └ `rating` | **评分** | **仅餐饮、酒店、景点、影院类 POI 有** |
| └ `cost` | **人均消费** | 同上 |
| `photos[]` | 照片信息（title/url） | all |

> ⚠️ 部分返回值存在时为字符串、不存在时为数组——解析需类型兜底。

### 2.4 服务示例

```
https://restapi.amap.com/v3/place/text?keywords=北京大学&city=beijing&offset=20&page=1&key=<用户的key>&extensions=all
```

---

## 3. 周边搜索（`v3/place/around`）—— ✅ 本项目已实现

### 3.1 服务地址

```
https://restapi.amap.com/v3/place/around?parameters
请求方式：GET
```

### 3.2 请求参数（与关键字搜索共用的参数不再重复）

| 参数名 | 含义 | 规则说明 | 是否必填 | 缺省值 |
|---|---|---|---|---|
| `key` | 权限标识 | Web 服务 Key | 必填 | 无 |
| `location` | 中心点坐标 | **经度,纬度**，小数 ≤6 位 | 必填 | 无 |
| `keywords` | 查询关键字 | 只支持一个 | 可选 | 无 |
| `types` | 查询 POI 类型 | 多个用 `\|` 分隔 | 可选 | 无 |
| `city` | 查询城市 | 与经纬度冲突时：范围内有该市数据则返回，否则为空 | 可选 | 全国 |
| `radius` | 查询半径 | **0~50000，超 50000 按默认值**，单位米 | 可选 | 5000 |
| `sortrule` | 排序规则 | `distance`（距离）/ `weight`（综合）；**只传 keywords 时距离排序不生效** | 可选 | distance |
| `offset` / `page` | 分页 | offset 建议 ≤25；总 ≤200 条 | 可选 | 20 / 1 |
| `extensions` | 返回结果控制 | base / all | 可选 | base |
| `langCode` / `sig` / `callback` | 同关键字搜索 | — | 可选 | — |

**默认类型**：keywords 和 types 均为空时，默认 types = `050000`（餐饮服务）+ `070000`（生活服务）+ `120000`（商务住宅）。

**返回结果**：同关键字搜索（§ 2.3），`distance` 字段有值。

### 3.3 服务示例

```
https://restapi.amap.com/v3/place/around?key=<用户的key>&location=116.473168,39.993015&radius=10000&types=011100
```

### 3.4 ⚠️ 本项目对接发现（重要）

当前 `AmapPOISearcher`（`adapter/geo/amap.go`）调用时使用 `extensions=base`，但解析了 `biz_ext.rating`（评分）——按官方文档，**`biz_ext` 仅在 `extensions=all` 时返回**，base 模式下评分大概率拿不到。

**改进建议**：周边搜索请求参数改为 `extensions=all`，可一次拿到：
- `biz_ext.rating`（评分——地图榜核心字段，仅餐饮/酒店/景点/影院有）
- `biz_ext.cost`（人均消费——门店档案补全）
- `business_area`（商圈——本地关键词/问法精确化）
- `tag`（特色菜——餐饮内容素材）
- `tel`、`entr_location`（导航）

> 代价：all 模式响应更大、配额消耗略高——可评估按需使用（仅餐饮业态开 all）。

---

## 4. 多边形搜索（`v3/place/polygon`）

### 4.1 服务地址

```
https://restapi.amap.com/v3/place/polygon?parameters
请求方式：GET
```

### 4.2 请求参数（差异项）

| 参数名 | 含义 | 规则说明 | 是否必填 |
|---|---|---|---|
| `key` | 权限标识 | Web 服务 Key | 必填 |
| `polygon` | 经纬度坐标对 | 经度,纬度 用 `,` 分隔、坐标对用 `\|` 分隔；**矩形可传左上右下两顶点，其他情况首尾坐标需相同** | 必填 |
| `keywords` / `types` | 关键字 / 类型 | 同关键字搜索（keywords/types 均为空时默认 types=120000 商务住宅 + 150000 交通设施服务） | 可选 |
| `offset` / `page` / `extensions` / `langCode` | 同前 | 同上 | 可选 |

### 4.3 服务示例

```
https://restapi.amap.com/v3/place/polygon?polygon=116.460988,40.006919|116.48231,40.007381|116.47516,39.99713|116.472596,39.985227|116.45669,39.984989|116.460988,40.006919&keywords=kfc&key=<用户的key>
```

---

## 5. ID 查询（`v3/place/detail`）

> ⚠️ **官网目录未单列此接口**——它藏在"搜索 POI"功能文档内（2026-07-15 版文档含此章节）。如需验证可用 curl 直调测试（配真实 Key）。
> ⚠️ **JS API 中的 `poiOnAMAP` 不是数据接口**：它打开高德地图的 POI 落地详情页（跳转客户端/网页），返回页面而非数据——服务端拿评分/营业时间请用本接口或 v5 版，前端"点击 POI 打开高德详情页"才用它。
> ⚠️ **本项目大概率不需要本接口**：评分/人均/营业时间/商圈/特色菜/电话在**搜索接口返回里已有**（v5 `show_fields=business` 一次拿全，见 `02-搜索POI-2.0.md` § 7）。仅当需要搜索不返回的补充字段（如 `atag` 类目）或从输入提示拿到 id 后精确查证时才用。

### 5.1 服务地址

```
https://restapi.amap.com/v3/place/detail?parameters
请求方式：GET
```

### 5.2 请求参数

| 参数名 | 含义 | 规则 | 是否必填 |
|---|---|---|---|
| `key` | 权限标识 | Web 服务 Key | 必填 |
| `id` | POI 唯一标识 | **最多 1 个 id**，传目标 POI id | 必填 |
| `langCode` / `sig` / `callback` | 同前 | — | 可选 |

### 5.3 服务示例

```
https://restapi.amap.com/v3/place/detail?id=B0FFFAB6J2&key=<用户的key>
```

> ⚠️ 未能获取 POI 详情时，需联系商务提交工单申请高级权限。返回结果见关键字搜索（§ 2.3）。

---

## 6. AOI 边界查询（`v5/aoi/polyline`）—— 高阶服务

> **该服务属于高德开放平台高阶服务，正式使用前需通过工单联系开通权限。**

- 服务地址：`https://restapi.amap.com/v5/aoi/polyline?parameters`（GET）
- 请求参数：`key`（必填）、`id`（必填，最多 1 个）、`sig`、`callback`
- 返回参数：`status`（**成功返回 0**，与其他接口的 1 不同！）/ `info` / `aois[]`
  - `aois[]`：`name` / `id` / `location`（中心点）/ **`polyline`（边界经纬度坐标串，以 `_` 分隔）** / `type` / `typecode` / `pname` / `cityname` / `adname` / `address` / `pcode` / `citycode` / `adcode`
- **AOI 定义**：面状/区域状 POI——工业园区、学校校区、商圈、住宅小区、景区、火车站、机场等

**用途设想**：结合边界数据可做"商圈覆盖范围分析"（门店服务半径可视化）、多边形围栏业务管理（配合猎鹰轨迹服务）。

---

## 7. 关键注意事项汇总

1. **翻页上限 200 条**：多页查询也无法获取全量数据。
2. **adcode > citycode**：要精确到区县必须用 adcode（citycode 仅到城市级）。
3. **keywords 只支持一个关键字**；想按品类搜用 `types` 更精准（六位编码，大类自动包含中/小类）。
4. **offset ≤25**：超过可能访问报错。
5. **周边搜索 radius**：0~50000 米，超 50000 按默认值（5000）。
6. **sortrule=distance 仅在非纯 keywords 场景生效**。
7. **字段类型漂移**：存在时字符串 / 不存在时数组，解析需兜底。
8. **评分/人均/商圈/特色菜均在 extensions=all**：本项目地图榜要拿到这些字段需改 all。
9. **默认类型**：周边搜索空条件默认餐饮/生活服务/商务住宅——对餐饮场景天然友好。

---

## 8. 项目对接现状与扩展建议

| 功能 | 状态 | 说明 |
|---|---|---|
| 周边搜索（`place/around`） | ✅ 已实现 | `AmapPOISearcher`：品牌名+竞品名搜索 → 地图榜（距离/评分） |
| ⚠️ extensions 升级为 all | 建议改 | 评分/人均/商圈/特色菜才能返回（§ 3.4） |
| **POI 类型搜索** | P1 扩展 | 用 `types=050000`（餐饮大类）替代/补充关键字搜索——竞品扫描更全（不依赖品牌名命中） |
| **ID 查询补详情** | ⛔ 不需要 | 评分/人均/电话在**搜索接口返回里已有**（v5 `show_fields=business`）——无需单独调 detail（§ 5 说明） |
| **多边形搜索** | P2 扩展 | 商圈范围扫描（"望京商圈"边界内竞品）——与 AOI 边界数据配合 |
| 关键字搜索 | P2 扩展 | "城市+品类"全城扫描（附近同行之外的全城排名） |
| AOI 边界查询 | ⏸️ 高阶服务 | 需工单开通权限后再评估 |

> 对接骨架：`adapter/geo/amap.go` 扩展 `SearchPOIByType(ctx, center, types, radius)`（复用现有 HTTP 骨架）；返回结构复用 `port.POIStore`（新增 `Rating/Cost/BusinessArea/Tag` 字段）。
