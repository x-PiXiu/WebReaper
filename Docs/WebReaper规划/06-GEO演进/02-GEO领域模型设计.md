# GEO 领域模型设计

> 📌 围绕 `Brand → Keyword → MonitoringResult → OptimizedContent` 组织领域模型，叠加在 WebReaper 现有架构之上。实体层是纯 struct + 领域规则，用例层声明 port 接口，适配器层实现。

## 一、领域全景：两个聚合根群

GEO 领域分为两个独立的聚合根群，分别对应"监测"和"内容生产"：

```
┌─ 监测域（闭环的"眼睛"）─────────────────┐    ┌─ 内容域（闭环的"手脚"）─────────────────┐
│                                          │    │                                          │
│  Brand（品牌资产）                        │    │  OptimizedContent（优化后的内容）         │
│   ├─ 1:N Keyword（关键词）                │    │   ├─ 指向 Brand                          │
│   ├─ 1:N Competitor（竞品）               │    │   ├─ 指向 Keyword                        │
│   └─ 1:N MonitoringResult（监测快照）     │    │   ├─ N:1 OriginalContent（原始内容）      │
│                                          │    │   └─ GEOScore（评分快照）                 │
│  MonitoringResult（监测快照）              │←──→│                                          │
│   ├─ 指向 Keyword                         │ 驱动│  PublishRecord（发布记录）               │
│   ├─ 指向 AI 引擎（LLMConfig）            │ 优化│   ├─ 指向 OptimizedContent               │
│   ├─ 提及位置/次数/情感                    │    │   └─ 指向 Account                        │
│   └─ 采样次数/置信度                       │    │                                          │
└──────────────────────────────────────────┘    └──────────────────────────────────────────┘

┌─ 账号域（发布的基础设施）────────────────────────────────────────────────────┐
│                                                                                │
│  Tenant（商户）                                                                 │
│   └─ 1:N Account（平台账号 = 一个登录态）                                        │
│        ├─ platform（知乎/抖音/小红书…）                                          │
│        ├─ cookie（加密存储）                                                     │
│        └─ health（健康状态：可用/过期/封号）                                      │
│                                                                                │
└────────────────────────────────────────────────────────────────────────────────┘
```

## 二、实体层（最内层，零框架依赖）

### 2.1 租户与品牌资产

```go
// Tenant 是 SaaS 的商户（多租户隔离的根）。
// 所有业务数据都必须挂在一个 Tenant 下。
type Tenant struct {
    ID        string
    Name      string    // 商户名
    Plan      string    // 套餐：free/pro/enterprise（决定配额）
    CreatedAt time.Time
}

// Brand 是商户的品牌资产（聚合根）。
// 一个商户可有多品牌（如装修公司有"家装""工装"两个品牌线）。
// 所有 GEO 活动围绕 Brand 展开。
type Brand struct {
    ID          string
    TenantID    string    // 租户隔离
    Name        string    // 品牌名（如"某装修公司"）
    Positioning string    // 品牌定位（用于生成内容的提示词语料）
    CoreSelling []string  // 核心卖点（用于优化内容的权威性维度）
    CreatedAt   time.Time
}

// IsValid 领域规则：品牌必须有 ID、TenantID、名称。
func (b Brand) IsValid() bool {
    return b.ID != "" && b.TenantID != "" && b.Name != ""
}
```

### 2.2 关键词与竞品

```go
// Keyword 是商户要监测/优化的搜索词。
// 一个品牌有多个关键词（如"北京装修公司""装修报价""旧房翻新"）。
type Keyword struct {
    ID        string
    BrandID   string    // 所属品牌
    TenantID  string    // 冗余 TenantID，便于仓储层直接过滤（避免 join）
    Term      string    // 关键词（如"北京装修公司哪家好"）
    Intent    string    // 搜索意图：informational/transactional/local
    CreatedAt time.Time
}

// Competitor 是竞品（用于基准对比）。
// 监测时同时记录 AI 回答里提到的竞品，形成行业排名。
type Competitor struct {
    ID       string
    BrandID  string    // 属于哪个品牌的竞品清单
    Name     string    // 竞品名
    Aliases  []string  // 别名（AI 可能用简称/全称）
}
```

### 2.3 监测结果（最核心的数据资产）

```go
// MonitoringResult 是一次 AI 引擎探测的快照。
// 这是 GEO 系统最有价值的数据——品牌在 AI 里的"心电图"。
//
// 设计要点（采样降噪）：
//   - AI 回答有随机性，单次不可信。一次监测任务对同一引擎采样多次。
//   - 提及情况用"提及率"（mentionRate）而非布尔值。
type MonitoringResult struct {
    ID            string
    TenantID      string
    BrandID       string
    KeywordID     string
    EngineName    string    // 探测的 AI 引擎（对应 LLMConfig.Name）
    // —— 提及统计（采样降噪）——
    SampleCount   int       // 采样次数（如 5 次）
    MentionCount  int       // 提到品牌的次数（如 3 次）
    MentionRate   float64   // 提及率 = MentionCount/SampleCount（0.6 = 60%）
    AvgPosition   int       // 平均排名（AI 回答里品牌出现的位置，1=最靠前）
    // —— 上下文分析 ——
    Sentiment     string    // 情感倾向：positive/neutral/negative
    Competitors   []string  // 同次回答里提到的竞品名
    // —— 置信度 ——
    Confidence    float64   // 置信度（采样次数少则低）
    ProbedAt      time.Time // 探测时间
    RawAnswers    []string  // 原始回答（留证，便于复核）
}

// MentionRateLabel 领域规则：把提及率映射为可读等级。
// 提及率 >=80% = 强势，50-80% = 稳定，20-50% = 偶尔，<20% = 缺席。
// 这是纯函数，零依赖，可独立单测。
func (m MonitoringResult) MentionRateLabel() string {
    r := m.MentionRate
    switch {
    case r >= 0.8: return "强势"
    case r >= 0.5: return "稳定"
    case r >= 0.2: return "偶尔"
    default:       return "缺席"
    }
}
```

> **架构警示**：`MonitoringResult` 的"采样降噪"是实体层业务规则，不能漏。如果某个适配器只采样 1 次就返回，置信度计算必须让它"显式不可信"——这是防止监测数据误导决策的安全阀。

### 2.4 GEO 评分（核心算法，纯函数）

```go
// GEOScore 是内容的 GEO 可见度评分（核心领域逻辑）。
// 输入(内容, 关键词)→输出(总分 + 各维度分)。零框架依赖。
//
// 五个维度（来自 GEO 实践，与具体 AI 引擎无关）：
//   1. Authority    权威性：有无数据/案例/资质支撑
//   2. Specificity  具体性：数字、细节、可验证信息
//   3. Structure    结构化：标题层级、列表、FAQ
//   4. Uniqueness   独特性：与全网内容的差异化
//   5. Recency      时效性：信息是否最新
type GEOScore struct {
    Total        float64  // 加权总分（0-100）
    Authority    float64  // 权威性（0-100）
    Specificity  float64
    Structure    float64
    Uniqueness   float64
    Recency      float64
    DiagnosedAt  time.Time
}

// Score 是评分纯函数（实体层最核心的算法）。
// 实现可结合规则匹配 + LLM 评估，但函数签名保持纯：
//   入参是内容和关键词，出参是评分，无副作用。
//   具体的"规则引擎/LLM 调用"放在用例层，实体层只定义评分的数据结构和领域规则。
func (s GEOScore) Level() string {
    switch {
    case s.Total >= 80: return "A"  // 优秀：高度可能被引用
    case s.Total >= 65: return "B"  // 良好
    case s.Total >= 50: return "C"  // 及格
    default:             return "D" // 待优化
    }
}
```

### 2.5 优化内容与发布记录

```go
// OptimizedContent 是优化后的内容（带版本，支持 A/B 对比）。
type OptimizedContent struct {
    ID            string
    TenantID      string
    BrandID       string
    KeywordID     string
    OriginalText  string    // 原始内容（商户提供的素材）
    OptimizedText string    // AI 优化后的内容
    Version       int       // 版本号（v1/v2，用于 A/B）
    Score         GEOScore  // 本版本的 GEO 评分
    Status        string    // draft/approved/published
    CreatedAt     time.Time
}

// Account 是商户绑定的平台账号（一个登录态）。
// 一个商户可有多个账号（账号池），分散发布风控压力。
type Account struct {
    ID         string
    TenantID   string
    Platform   string    // zhihu/douyin/xiaohongshu/baijiahao
    DisplayName string    // 账号显示名（如"@某装修公司官方"）
    CookieRef  string    // cookie 的加密存储引用（不存明文）
    Health     string    // active/expired/banned
    BoundAt    time.Time
    LastUsedAt time.Time
}

// PublishRecord 是一次发布的记录。
type PublishRecord struct {
    ID              string
    TenantID        string
    ContentID       string    // 指向 OptimizedContent
    AccountID       string    // 用哪个账号发的
    Platform        string
    Status          string    // pending/published/failed
    ExternalURL     string    // 发布后的文章链接
    Mode            string    // auto（API/RPA）/ semi-auto（人工确认）
    PublishedAt     time.Time
    ErrorMsg        string
}
```

## 三、用例层（应用级业务规则）

每个用例对应闭环的一段。用例只依赖 port 接口，不依赖任何适配器实现。

### 3.1 监测用例（闭环起点）

```go
// MonitorUseCase 编排一次 AI 引擎监测。
// 对每个关键词 × 每个 AI 引擎 × 多次采样，聚合出 MonitoringResult。
type MonitorUseCase struct {
    llmGen    port.AIGenerator       // 复用 WebReaper 现有接口（调 LLM）
    probe     port.AIEngineProbe     // 新增：解析 AI 回答里的品牌提及
    resultRepo port.MonitoringResultRepository
    keywordRepo port.KeywordRepository
    brandRepo  port.BrandRepository
}

// MonitorInput 监测的输入。
type MonitorInput struct {
    TenantID   string
    BrandID    string
    EngineName string  // 探测哪个 AI 引擎（对应 LLMConfig.Name）
    SampleSize int     // 采样次数（默认 5）
}

// Monitor 执行监测：取关键词 → 逐个问 AI → 解析提及 → 存快照。
func (uc *MonitorUseCase) Monitor(ctx context.Context, in MonitorInput) error
```

### 3.2 评分与诊断用例

```go
// DiagnoseUseCase 给内容打分并出诊断报告。
type DiagnoseUseCase struct {
    scorer port.GEOScorer  // 评分器（规则+LLM 混合）
    llmGen port.AIGenerator // 诊断 Agent（深度分析为什么没被引用）
}
```

### 3.3 内容生成与优化用例

```go
// OptimizeUseCase 编排内容优化（可复用 graph_orchestrator 图编排）。
// 流程：读诊断 → LLM 改写 → 自评 → 不达标再改 → 输出新版本。
type OptimizeUseCase struct {
    llmGen        port.AIGenerator
    scorer        port.GEOScorer
    contentRepo   port.OptimizedContentRepository
}
```

### 3.4 排行榜用例

```go
// RankUseCase 聚合监测结果，算出行业排名。
type RankUseCase struct {
    resultRepo port.MonitoringResultRepository
}

// BrandRank 品牌在某关键词下的排名。
type BrandRank struct {
    BrandName   string
    MentionRate float64
    AvgPosition int
    Rank        int  // 行业第几
    Trend       string  // up/down/stable（与上期比）
}
```

### 3.5 账号池与发布用例

```go
// AccountPoolUseCase 管理账号池调度。
type AccountPoolUseCase struct {
    accountRepo port.AccountRepository
    channelReg  port.PublishChannelRegistry  // 发布通道注册表
}

// PublishUseCase 编排一次发布。
type PublishUseCase struct {
    pool       port.AccountPool
    channelReg port.PublishChannelRegistry
    recordRepo port.PublishRecordRepository
}

// PublishInput 发布输入。
type PublishInput struct {
    TenantID  string
    ContentID string
    Platform  string  // 发到哪个平台
    Mode      string  // auto / semi-auto
}
```

## 四、Port 接口（用例层声明，适配器实现）

按整洁架构 § 四"依赖倒置"——接口归用例层所有，实现在适配器层：

```go
// ---- 监测相关 ----
type AIEngineProbe interface {
    // Probe 对一个关键词问 AI 引擎，采样 N 次，解析品牌提及情况。
    Probe(ctx context.Context, in ProbeInput) (ProbeResult, error)
}

type ProbeInput struct {
    Keyword     string
    EngineName  string  // LLMConfig.Name
    BrandName   string
    Competitors []string
    SampleSize  int
}

type ProbeResult struct {
    SampleCount  int
    MentionCount int
    AvgPosition  int
    Sentiment    string
    Competitors  []string
    RawAnswers   []string
}

// ---- 评分相关 ----
type GEOScorer interface {
    // Score 给内容打 GEO 分（规则匹配 + LLM 评估混合）。
    Score(ctx context.Context, content string, keyword string) (entity.GEOScore, error)
}

// ---- 发布相关 ----
type PublishChannel interface {
    // Publish 发布内容到该平台。
    Publish(ctx context.Context, content string, account entity.Account) (PublishResult, error)
    // SupportedMediaType 该平台支持的媒体类型。
    SupportedMediaType() []MediaType  // {Text} / {Video, Text}
    // Platform 标识。
    Platform() string  // "zhihu" / "douyin"
}

type PublishChannelRegistry interface {
    Get(platform string) (PublishChannel, error)
    List() []string
}

type AccountPool interface {
    // Acquire 从某平台的账号池借一个健康账号。
    Acquire(ctx context.Context, tenantID, platform string) (entity.Account, error)
    // Release 归还账号（更新使用时间）。
    Release(ctx context.Context, account entity.Account) error
    // CheckHealth 检查账号健康（cookie 是否过期/封号）。
    CheckHealth(ctx context.Context, account entity.Account) (health string, err error)
}
```

> **关键纪律**：用例层**绝对不 import** Playwright、stealth、任何平台 SDK。所有平台细节全关在适配器层的 `PublishChannel` 实现里。换平台实现 = 重写一个适配器文件，业务零影响。

## 五、适配器层（各实现，互不相干）

### 5.1 AI 引擎监测适配器

```go
// AILLMEngineProbe 是 port.AIEngineProbe 的实现。
// 它复用 port.AIGenerator（调 LLM），但把"问问题"包装成"探测"。
// 解析逻辑：用正则 + LLM 二次解析，提取品牌提及。
type AILLMEngineProbe struct {
    llmGen port.AIGenerator
}
```

> **复用 WebReaper 现有资产**：`AIEngineProbe` 内部就是调 `port.AIGenerator.ChatStream`，把关键词当问题问 AI，再解析回答。WebReaper 的多 `LLMConfig` 能力让"同时探测豆包/文心/Kimi"成为一行配置的事。

### 5.2 发布通道适配器（策略模式，每平台一个实现）

```go
// ZhihuPublishChannel 知乎发布（半自动或 RPA）。
type ZhihuPublishChannel struct {
    browserPool *BrowserPool
    mode        string  // "semi-auto" / "rpa"
}

// OfficialAccountChannel 公众号发布（有官方 API，全自动）。
type OfficialAccountChannel struct {
    apiToken string
}
// —— 这个能全自动，因为公众号有正式的草稿+发布 API ——
```

### 5.3 浏览器池适配器（发布层的心脏）

```go
// BrowserPool 管理无头浏览器实例的生命周期。
// 这是整个系统最不稳定的部分，必须设计成独立可替换的模块。
type BrowserPool struct {
    instances chan *Browser  // 池化的 Chromium 实例
    stealth   bool           // 是否启用反检测
}

// Browser 是一个包装了登录态的无头浏览器实例。
type Browser struct {
    playwright *Playwright
    context    *BrowserContext  // 预加载了某账号的 cookie
    accountID  string
}
```

## 六、数据库迁移（多租户）

所有表强制带 `tenant_id`，建立索引：

```sql
-- 品牌表
CREATE TABLE brands (
    id          VARCHAR(64) PRIMARY KEY,
    tenant_id   VARCHAR(64) NOT NULL,
    name        VARCHAR(128) NOT NULL,
    positioning TEXT,
    core_selling JSON,
    created_at  DATETIME(3) NOT NULL,
    INDEX idx_tenant (tenant_id)
);

-- 监测结果表（核心数据资产，按租户+品牌+关键词查询）
CREATE TABLE monitoring_results (
    id            VARCHAR(64) PRIMARY KEY,
    tenant_id     VARCHAR(64) NOT NULL,
    brand_id      VARCHAR(64) NOT NULL,
    keyword_id    VARCHAR(64) NOT NULL,
    engine_name   VARCHAR(64) NOT NULL,
    sample_count  INT NOT NULL,
    mention_count INT NOT NULL,
    mention_rate  DECIMAL(4,3) NOT NULL,
    avg_position  INT,
    sentiment     VARCHAR(16),
    competitors   JSON,
    confidence    DECIMAL(4,3),
    probed_at     DATETIME(3) NOT NULL,
    raw_answers   JSON,
    INDEX idx_tenant_brand_keyword (tenant_id, brand_id, keyword_id),
    INDEX idx_probed_at (probed_at)
);

-- 账号表（cookie 加密存储）
CREATE TABLE accounts (
    id           VARCHAR(64) PRIMARY KEY,
    tenant_id    VARCHAR(64) NOT NULL,
    platform     VARCHAR(32) NOT NULL,
    display_name VARCHAR(128),
    cookie_ref   VARCHAR(256) NOT NULL,  -- 加密后的引用，不存明文
    health       VARCHAR(16) NOT NULL DEFAULT 'active',
    bound_at     DATETIME(3) NOT NULL,
    last_used_at DATETIME(3),
    INDEX idx_tenant_platform (tenant_id, platform)
);
```

## 七、整洁架构自检

按 § 7.6E"一键体检"过一遍 GEO 领域设计：

| 体检项 | 是否满足 | 说明 |
|---|---|---|
| 换 AI 引擎，业务核心要不要改？ | 否 | `AIEngineProbe` 接口隔离，换引擎 = 换 LLMConfig |
| 换发布平台，业务核心要不要改？ | 否 | `PublishChannel` 接口隔离，加平台 = 加适配器 |
| 能否脱离浏览器对业务逻辑做单测？ | 能 | 用例依赖接口，mock `PublishChannel` 即可测 |
| 采样降噪逻辑可独立单测？ | 能 | `MentionRateLabel` 是纯函数 |
| 租户隔离可验证？ | 能 | 仓储强制 `tenant_id`，单测覆盖跨租户拒绝 |

---

> 📎 **关联文档**：[01-GEO演进战略与可行性分析](01-GEO演进战略与可行性分析.md) | [03-GEO多平台发布架构](03-GEO多平台发布架构.md) | [../02-架构设计/02-模块边界与分层设计](../02-架构设计/02-模块边界与分层设计.md)
