# GEO 多平台发布架构

> 📌 发布层是 GEO 系统最脆弱、最不可控的部分。本文用策略模式 + 适配器模式把各平台差异隔离在最外层，让脆弱的发布实现随时可替换，不拖垮核心业务。

## 一、核心矛盾：平台不开放，自动化必脆弱

发布层设计的所有复杂性，根源是一个外部事实：

> **知乎、抖音、小红书等主流内容平台，不提供个人账号的内容发布 API。**

这决定了三件事：

1. **没有"调 API 发文章"这种简单解法**——只能模拟浏览器操作（RPA）或半人工
2. **平台主动反爬**——检测无头浏览器、设备指纹、操作节奏，违规即封号
3. **方案会随时失效**——平台升级风控，RPA 实现就要跟着改

整洁架构的态度很明确（§ 7.6C 推迟决策清单）：**把这种"会变且不可控的外部细节"推到最外层，用边界彻底隔离。** 它哪天失效了，重写这一个模块，业务零影响。

## 二、三种发布模式（按可靠性排序）

| 模式 | 做法 | 可靠性 | 维护成本 | 适用平台 |
|---|---|---|---|---|
| **① 官方 API** | 调平台开放的内容发布 API | ⭐⭐⭐⭐⭐ | 低 | 公众号、微博（企业蓝V）、抖音企业号（有限）|
| **② 半自动** | 系统生成内容+预填发布表单，人工点"发布" | ⭐⭐⭐⭐⭐ | 低 | 所有平台（MVP 起步首选）|
| **③ RPA 全自动** | Playwright 模拟浏览器操作，维护 cookie | ⭐⭐ | 极高 | 知乎/小红书/抖音（脆弱、需持续对抗）|

### 2.1 为什么强烈推荐半自动起步

1. **封号风险最低**：机器准备 + 人工确认，平台基本不拦
2. **合规底线**：人工确认 = 有审核留痕，避免全自动发营销内容被秋后算账
3. **架构不变**：半自动和全自动在架构上**完全一样**——都是实现 `PublishChannel` 接口。半自动的 `Publish` 是"打开预填好的发布页 + 推通知让人去点"；全自动的 `Publish` 是"Playwright 模拟点击"。**随时能从②升级到③，业务代码零改动。**

这就是整洁架构的价值：**推迟决策**。先用稳妥的半自动验证客户愿不愿意为"分发"付费，验证通过后再投入做脆弱的 RPA——而那时业务代码一行不改。

### 2.2 一个 SaaS 里混合三种模式是常态

不同平台用不同自动化级别，完全正常：

```
公众号/微博（有 API）  → API 全自动（合规稳定）
知乎/小红书（无 API）  → 半自动（机器生成+人工确认）
抖音（风控最强）       → 半自动起步，全自动最后做
```

客户能理解——他们要的是"省事"，不是"这个按钮必须机器点"。机器把内容生成好、表单填好、定时提醒去点，已省 90% 的活。

## 三、架构设计：可插拔的发布通道

这是教科书级的**策略模式 + 适配器模式 + 工厂模式**组合：

```
                 依赖方向 → 永远向内
┌──────────────────────────────────────────────────┐
│ 用例层（不知道任何平台的存在）                        │
│   port.PublishChannel 接口                         │  ← 用例层"拥有"接口
│     Publish(ctx, content, account) → Result       │
│     SupportedMediaType() → {Text, Image, Video}    │
│     Platform() → string                            │
└──────────────────────────────────────────────────┘
        ↑ 实现（依赖倒置：外层实现内层声明的接口）
┌──────────────────────────────────────────────────┐
│ 适配器层（各平台实现，互不相干，可单独替换）            │
│   OfficialAccountChannel  （官方 API 全自动）        │
│   ZhihuPublishChannel     （半自动 / RPA）           │
│   DouyinPublishChannel    （半自动 / RPA + 视频）    │
│   XiaohongshuChannel      （半自动 / RPA）           │
└──────────────────────────────────────────────────┘
```

**关键纪律（整洁架构 § 7.6C）**：用例层 `PublishUseCase` **绝对不能 import** Playwright、stealth 插件、任何平台 SDK。它只依赖 `port.PublishChannel` 接口。具体用哪个平台，由 `main` 装配时注入注册表。

## 四、核心抽象：port 接口定义

```go
// MediaType 媒体类型（决定内容能发到哪些平台）。
type MediaType string
const (
    MediaText  MediaType = "text"
    MediaImage MediaType = "image"
    MediaVideo MediaType = "video"
)

// PublishChannel 是发布通道的统一抽象（策略模式接口）。
// 每个平台一个实现，互不相干。用例层只依赖此接口。
type PublishChannel interface {
    // Publish 发布内容。account 提供登录态（cookie/token）。
    Publish(ctx context.Context, in PublishInput) (PublishOutput, error)
    // SupportedMediaType 该平台支持的媒体类型（视频发不了只支持文本的通道会被拦截）。
    SupportedMediaType() []MediaType
    // Platform 平台标识。
    Platform() string
}

type PublishInput struct {
    TenantID  string
    Content   string          // 文本内容（markdown 或纯文本）
    Title     string
    MediaPath string          // 视频/图片的本地路径（如有）
    Account   entity.Account  // 用哪个账号发（提供 cookie/token）
    Mode      string          // "auto" / "semi-auto"
}

type PublishOutput struct {
    ExternalURL string  // 发布后的文章链接
    PublishedAt time.Time
}

// PublishChannelRegistry 发布通道注册表（工厂模式）。
// 按平台名拿通道，新增平台 = 注册一个新适配器，业务零改动（开闭原则）。
type PublishChannelRegistry interface {
    Get(platform string) (PublishChannel, error)
    List() []string  // 列出所有可用平台
}
```

> **媒体类型校验是实体层业务规则**：发布前必须检查"内容类型 × 平台支持类型"是否匹配，不匹配直接拒绝（把视频发给只支持文本的通道）。这不能漏。

## 五、账号资产管理（多租户安全）

做分发绕不开"账号"聚合根。每个商户在多个平台有多个账号，账号的登录态（cookie/token）是**易失且敏感**的资产。

### 5.1 账号绑定：一次性扫码 + cookie 复用

```
商户首次绑定账号（关键：真人操作，一次性）
  1. 系统用 headless 浏览器渲染平台登录页
  2. 弹出二维码（知乎/抖音都是扫码登录）
  3. 商户用手机扫码（真人，这一步无法自动化）
  4. 系统抓取登录后的 cookie
  5. cookie 加密存库（CookieVault）
  6. 后续发布复用这个 cookie，直到过期
  7. 过期 → 通知商户重新扫码
```

> **纠正认知**："自动获取 cookie 登录知乎"做不到。知乎/抖音没有账号密码登录 API，登录方式是扫码或短信验证码，都需要真人。只能"一次性扫码 + cookie 复用 + 自动续期"。

### 5.2 CookieVault：加密存储

```go
// CookieVault 负责账号凭证的加密存储与读取。
// 安全红线（实体层硬规则）：
//   - cookie/token 是高敏感凭证，必须加密存储
//   - 使用租户级密钥（一租户泄露不影响其他）
//   - 最小权限访问、操作留审计
type CookieVault interface {
    Store(ctx context.Context, tenantID, accountID string, cookie []byte) error
    Load(ctx context.Context, tenantID, accountID string) ([]byte, error)
    Revoke(ctx context.Context, tenantID, accountID string) error
}
```

实现用 KMS 管理主密钥 + AES-GCM 数据加密，每条 cookie 用派生密钥（主密钥 + tenantID 派生）。

### 5.3 账号池调度（单账号高频必封）

**真相**：单账号高频发布必封。矩阵号运营要**一个商户多个账号轮换发布**，分散风控压力。

```go
// AccountPool 管理某租户在某平台的账号池。
type AccountPool interface {
    // Acquire 借一个健康账号（轮询/权重调度，避开刚用过的）。
    Acquire(ctx context.Context, tenantID, platform string) (entity.Account, error)
    // Release 归还账号（更新 last_used_at）。
    Release(ctx context.Context, account entity.Account) error
    // CheckHealth 检查账号健康（cookie 过期/封号检测）。
    CheckHealth(ctx context.Context, account entity.Account) (health string, err error)
}
```

调度策略：
- **轮询/权重**：多个账号轮流发，避免单号过载
- **冷却期**：刚发过的账号冷却 N 小时再用
- **健康检查**：定期探测 cookie 有效性，过期/封号的账号剔除
- **自动降级**：某账号发布失败（疑似风控）→ 标记不健康 → 换号重试

## 六、浏览器池（RPA 模式的心脏）

无头浏览器**很重**（一个 Chromium 实例几百 MB 内存），不能每次发布开一个。要做成池：

```
BrowserPool（适配器层，用例层不知道它的存在）
  · 预热 N 个 headless Chromium 实例
  · 每个实例加载某租户某账号的 cookie = 一个"虚拟登录用户"
  · 发布任务从池里租借实例 → 执行 → 归还
  · 实例用久了指纹会"脏"，定期回收换新
  · 反检测：stealth 插件 + 随机延迟 + 拟人鼠标轨迹
```

### 6.1 反检测对抗（灰盒，不是黑盒）

Playwright 能跑无头浏览器，但平台**主动检测**：

| 检测手段 | 对抗措施 |
|---|---|
| `navigator.webdriver` 标志位 | stealth 插件抹除 |
| 浏览器指纹（Canvas/WebGL/字体）| 指纹随机化 |
| 鼠标轨迹太规律 | 拟人化轨迹（贝塞尔曲线 + 随机抖动）|
| 点击节奏太快 | 随机延迟（1-3 秒）|
| IP 行为模式 | 代理 IP 池（按租户/账号分配）|

> **架构警示**：这是一场**猫鼠游戏**——平台升级风控，对抗措施要跟着改。所以浏览器池**必须设计成独立可替换的服务**（甚至独立微服务），用 `PublishChannel` 接口和主系统解耦。哪天 stealth 方案被破解，只重写这一个池子。

### 6.2 抖音视频上传的额外难点

视频比文本难一个量级：

- 大文件分片上传、断点续传
- 必须带**封面图**
- 标题/话题/位置/@人 等元数据
- 设备指纹、模拟真人操作节奏（太快必封）

**务实建议**：抖音这块从半自动都不一定好做，更现实的是"系统生成视频文件 → 下载到本地 → 用户手动上传抖音"。把"自动发布抖音"放到最后期。

## 七、视频内容生成（独立子系统）

视频发布的前提是**能生成视频**。这本身是个独立流水线，建议复用 WebReaper 的图编排框架（`graph_orchestrator`）：

```
脚本节点 → TTS节点（配音）→ 素材节点（图集/视频素材/AI 生成）→ 字幕节点 → 合成节点（FFmpeg）→ 发布节点
```

技术栈：
- **TTS**：在线 API（如智谱/MiniMax 的语音合成）或本地模型
- **素材**：图库 API / AI 图像生成 / 模板图集
- **合成**：FFmpeg（音视频合并、字幕烧录）
- **发布**：复用 `PublishChannel` 接口，新增 `DouyinPublishChannel`

> **媒体类型必须显式建模**：`PublishChannel.SupportedMediaType()` 声明平台支持什么。发布用例先校验"内容类型 × 平台支持"匹配，不匹配拒绝。这是实体层业务规则。

## 八、发布用例的编排逻辑

```go
// PublishUseCase 编排一次发布。
type PublishUseCase struct {
    pool       port.AccountPool          // 账号池
    channelReg port.PublishChannelRegistry // 发布通道注册表
    vault      port.CookieVault           // cookie 存储
    recordRepo port.PublishRecordRepository
    scorer     port.GEOScorer             // 发布后可重评 GEO 分
}

// Publish 编排：校验媒体类型 → 借账号 → 取 cookie → 调通道发布 → 存记录。
func (uc *PublishUseCase) Publish(ctx context.Context, in PublishInput) error {
    // 1. 拿发布通道
    channel, err := uc.channelReg.Get(in.Platform)
    if err != nil { return err }

    // 2. 媒体类型校验（实体层规则：内容类型 × 平台支持必须匹配）
    if !mediaSupported(in.MediaType, channel.SupportedMediaType()) {
        return ErrMediaNotSupported
    }

    // 3. 从账号池借一个健康账号
    account, err := uc.pool.Acquire(ctx, in.TenantID, in.Platform)
    if err != nil { return err }
    defer uc.pool.Release(ctx, account)

    // 4. 取 cookie（从加密存储）
    cookie, err := uc.vault.Load(ctx, in.TenantID, account.ID)
    if err != nil { return err }
    account.Cookie = cookie

    // 5. 调通道发布（半自动/RPA/API，用例层不关心）
    out, err := channel.Publish(ctx, in)
    if err != nil {
        // 发布失败可能是封号，标记账号不健康
        if isRiskError(err) { uc.pool.MarkUnhealthy(ctx, account) }
        return err
    }

    // 6. 存发布记录
    return uc.recordRepo.Save(ctx, PublishRecord{...})
}
```

> 用例层全程不知道 Playwright、stealth、平台 SDK 的存在——它只依赖接口。这就是整洁架构"用边界隔离变化"的落地。

## 九、务实的落地顺序

| 阶段 | 做什么 | 模式 | 风险 |
|---|---|---|---|
| **阶段 1** | `PublishChannel` 接口 + 知乎半自动适配器 | 半自动 | 极低 |
| **阶段 2** | 多平台扩展（小红书/百家号半自动）+ 账号池管理 | 半自动 | 低 |
| **阶段 3** | 知乎全自动 RPA（Playwright + cookie 维护） | RPA | 中高 |
| **阶段 4** | 视频生成流水线 + 抖音半自动 | 半自动 | 高 |
| **阶段 5** | 抖音全自动 | RPA | 极高 |

**核心原则**：接口先定义好（`PublishChannel`），实现从最稳的半自动开始。这样**永远不会被技术细节绑架**——某天某平台风控变了，只需重写那一个适配器，业务和用例一行不动。

## 十、关于"全自动无人值守"的最终结论

| 问题 | 答案 |
|---|---|
| 全自动发所有平台能实现吗？ | **长期不能**。有 API 的平台可以（公众号/微博）；没 API 的强做 = RPA 对抗，随时失效 |
| SaaS 必须全自动吗？ | **不需要**。SaaS 价值锚点是监测+生成+追踪闭环，不是发布动作 |
| 务实形态？ | **混合**：API 平台全自动 + 非 API 平台半自动 |

> **一个监测精准、内容优质、发布半自动的 GEO SaaS，远比一个监测拉胯、内容一般但发布全自动的 SaaS 有竞争力。客户为结果付费，不为"全自动"标签付费。**

把发布层当成"脆弱但可隔离的增值功能"，用整洁架构的边界关起来——这是整个 GEO SaaS 架构设计里最重要的一笔。

---

> 📎 **关联文档**：[01-GEO演进战略与可行性分析](01-GEO演进战略与可行性分析.md) | [02-GEO领域模型设计](02-GEO领域模型设计.md) | [../05-战略规划/数据采集差异化战略](../05-战略规划/数据采集差异化战略.md)
