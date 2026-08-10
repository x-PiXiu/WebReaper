# 10-新域开发：SaaS 平台业务开发文档

> 📁 本文档目录记录 WebReaper 从"GEO 内容引擎"向"全功能 SaaS 平台"演进的新域开发设计。
> 所有设计遵循整洁架构：domain ← usecase ← adapter ← main（依赖方向始终向内）。

## 🧭 目录

| 文件 | 内容 |
|---|---|
| `README.md`（本文件） | 需求全景盘点 + 五步法需求推导 + 已有/新增/增强矩阵 |
| `01-商户端业务域设计.md` | 数据驾驶舱 / 品牌管理 / 关键词工作台 / 内容工作台 / 账号池与发布 |
| `02-视频生成与登录地址域设计.md` | Vidu 视频生成流水线 / 配音合成 / 登录地址（新域） |
| `03-管理后台统一管理域设计.md` | 平台总览 / 品牌·内容·用户统一管理 / 绝对控制清单 |

---

## 一、需求全景盘点（用户原话 → 能力映射）

### 商户端（用户端 /m）

| # | 用户需求（原话提炼） | 现状 | 差距 |
|---|---|---|---|
| 1 | **数据驾驶舱**：可视化必要数据 | ❌ 无（Dashboard 仅管理后台） | **新增** /m 驾驶舱 |
| 2 | **品牌管理**：一个商户多品牌；每个品牌有名称/定位/核心卖点/竞品，新建时输入 | ✅ Brand 实体已有 `Positioning / CoreSelling / Competitors` 字段 | 前端新建表单补齐输入即可 |
| 3 | **关键词生成**：为每个品牌生成关键词，**LLM 输出需结构化约束**（避免随机化） | ✅ keyword_distill 已存在（文本蒸馏/种子拓展） | **增强**：LLM 结构化输出（JSON Schema 约束）+ 文件读取/网络获取入口 |
| 4 | **内容工作台**：选品牌 → 选关键词（**可组合**）→ 选目标平台（豆包等）→ 生成内容；或按关键词**优化用户输入内容**；**输出不含思考内容** | ✅ Content 页已有生成（按品牌+关键词） | **增强**：关键词多选组合、目标平台维度、优化模式、think 标签零泄漏（已做 StripThinkTags） |
| 5 | **账号管理**：多平台（知乎/小红书，后续抖音/快手），多账号，各平台独立账号池 | ✅ Accounts + AccountPool 已存在 | ✅ 已满足（抖音/快手 = 新增 PublishChannel 适配器） |
| 6 | **发布**：选品牌 → 半自动/全自动 → 账号**可选或随机** | ✅ Publish 已存在（含账号池调度） | 前端补"随机"选项 UI |
| 7 | **视频生成**：对接 Vidu，上传素材或随机文本生成视频 → 平台内配音 → 平台内合成 → 发布抖音等视频平台 | ❌ 无 | **全新域**（见 `02`） |
| 8 | **登录地址**：获取用户登录地址，发布内容时带上（对实体经济有用） | ❌ 无 | **全新能力**（见 `02`） |

### 管理后台（/admin）

| # | 用户需求 | 现状 | 差距 |
|---|---|---|---|
| 1 | **平台总览**：用户数/品牌数/关键词/优化内容等 | ✅ 刚完成（8 项规模指标 + 图表） | 已满足 |
| 2 | **统一管理页**：用户、品牌、优化内容、已发布公开内容等，管理后台**绝对控制** | ⚠️ users 有；品牌/内容管理页无 | **新增** admin 品牌/内容管理页 |
| 3 | **系统控制**：收录管理等 | ✅ Indexing 三件套已有 | 已满足 |

---

## 二、五步法需求推导（architect-handbook 需求推导引擎）

### 信号提取

| 域 | 核心要求（动词→方法） | 变化因子 | 多态信号 | 隔离壁 |
|---|---|---|---|---|
| 关键词生成 | `Distill(source) → []Keyword` | 获取方式会增（蒸馏/文件/网络/API） | 按 source 类型选生成器 | LLM 输出格式随机化 |
| 内容生成 | `Generate(brand, keywords[], platform) → Content` | 目标平台会增（豆包/文心/DeepSeek...） | 按平台调 prompt 模板 | 思考内容泄漏 |
| 视频生成 | `Generate(material, style) → Task` | 视频模型会变（Vidu/Sora/可灵...） | 按模型选适配器 | 第三方 API 差异 |
| 发布 | `Publish(job, account?) → externalURL` | 平台会增（知乎/小红书/抖音/快手） | 按平台选通道 | 浏览器自动化细节 |
| 登录地址 | `RecordLogin(ip) → GeoLocation` | IP 库会换（免费 API→付费库） | 按配置选解析器 | 第三方服务不稳定 |

### 推导过程

```
因为"获取方式会横向新增"        → 关键词：策略模式 + 工厂（KeywordSource 接口 + Registry）
因为"LLM 输出格式必须稳定"      → 结构化输出：JSON Schema 约束 + 解析校验（防随机化）
因为"目标平台会横向新增"        → 内容：策略模式（PlatformPrompt 模板注册表）
因为"输出不可含思考内容"        → 生成链路统一 StripThinkTags（单一出口，已实现）
因为"视频模型会变"              → 视频：策略/适配器（VideoProvider 接口 + Factory）
因为"生成流程固定（素材→视频→配音→合成）" → 视频：模板方法 + 状态机（任务生命周期）
因为"发布平台会增"              → 发布：已有 PublishChannel Registry（组合已存在）
因为"账号可选或随机"            → 账号池：已有 AccountPool（策略式调度，随机=新增策略）
因为"IP 库可替换"               → 登录地址：策略模式（GeoResolver 接口）
因为"后台要绝对控制"            → 管理：仓储直管用例（admin 全域 CRUD，多租户旁路）
```

### 模式选型汇总

| 域 | 主模式 | 组合模式 | 代码落点 |
|---|---|---|---|
| 关键词获取 | 策略 + 工厂 | 注册表 | `port.KeywordSource` → `text_distill / file_import / web_harvest / llm_generate` |
| 关键词结构化输出 | 模板方法 | 校验器 | `LLMStructuredExtractor`（JSON Schema 约束 + 重试校验） |
| 内容生成 | 策略 | 注册表 | `port.ContentGenerator` → 平台 prompt 模板 |
| 视频生成 | 适配器 + 状态机 | 工厂 | `port.VideoProvider` → `vidu_adapter`（Vidu/可灵...） |
| 发布 | 策略 + 注册表 | ✅ 已有 | `PublishChannelRegistry`（抖音=新适配器） |
| 登录地址 | 策略 | 工厂 | `port.GeoResolver` → `ipapi_resolver / mmdb_resolver` |

---

## 三、分层落点总览（服务端）

```
domain/entity/
├── geo.go        品牌/关键词/监测/优化内容（已有，品牌字段已齐）
├── video.go      【新增】VideoTask（视频生成任务）+ VideoJob（发布任务）
└── login_trace.go【新增】LoginTrace（登录地址记录）

usecase/
├── geo/          已有（品牌/关键词/内容/监测/蒸馏）
├── video/        【新增】VideoUseCase（生成→配音→合成 状态机编排）
├── account/      已有（账号池/发布）
└── geotrace/     【新增】LoginTraceUseCase（登录 IP → 地址解析入库）

port/
├── video_port.go      【新增】VideoProvider / VoiceSynthesizer / VideoComposer / VideoPublisher
├── geo_resolver.go    【新增】GeoResolver（IP → 城市/经纬度）
└── keyword_source.go  【新增】KeywordSource 策略接口

adapter/
├── video/          【新增】vidu.go（Vidu API）/ voice.go（配音）/ compose.go（ffmpeg 合成）
├── geotrace/       【新增】ipapi.go（免费 IP 解析）/ mmdb.go（本地库）
└── keyword/        【新增】file_source.go / web_source.go
```

---

## 四、客户端重写范围（高级感 SaaS）

设计语言（ui-design-system 指导）：深空科技风（#6C63FF 主色 + 玻璃拟态 + 渐变强调），详见 `web/src/index.css` 设计 token 与各页面实现。

| 页面 | 域 | 状态 |
|---|---|---|
| `/m` 数据驾驶舱 | 商户端 | 全新 |
| `/m/brands` 品牌管理 | 商户端 | 重写（多品牌卡片 + 定位/卖点/竞品表单） |
| `/m/keywords` 关键词工作台 | 商户端 | 重写（蒸馏/文件/网络 + 结构化生成） |
| `/m/content` 内容工作台 | 商户端 | 重写（品牌→关键词组合→平台→生成/优化） |
| `/m/accounts` 账号池 | 商户端 | 重写 |
| `/m/publish` 发布中心 | 商户端 | 重写（账号可选/随机） |
| `/m/video` 视频工作台 | 商户端 | 全新（Vidu 生成/配音/合成/发布） |
| `/admin` 平台总览 | 管理端 | ✅ 已重构 |
| `/admin/brands` 品牌统一管理 | 管理端 | 全新 |
| `/admin/contents` 内容统一管理 | 管理端 | 全新 |
