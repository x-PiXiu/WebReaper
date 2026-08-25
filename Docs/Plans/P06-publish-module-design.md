# P06 多平台发布模块设计文档

> **版本**：v1.2
> **创建时间**：2026-08-23
> **更新时间**：2026-08-23
> **参考项目**：KBSZR (极享AI口播智能体)
> **设计思想**：整洁架构 + 策略模式 + 工厂模式 + 模板方法
> **状态**：设计完成，可开始实现

---

## 一、需求分析

### 1.1 业务需求

用户需要将内容（视频/图文）发布到多个社交平台：
- 抖音
- 快手
- 小红书
- 视频号（微信）
- B站

### 1.2 核心要求

| 要求 | 说明 |
|------|------|
| 多平台支持 | 5+ 个平台，后续可能扩展 |
| 自动化发布 | 无需人工干预 |
| 错误重试 | 网络异常自动重试 |
| 进度反馈 | 实时显示发布状态 |
| 多账号支持 | 每个平台多个账号轮换 |
| 反检测能力 | 避免被平台识别为机器人 |
| 内容适配 | 不同平台不同文案规范 |
| 人设隔离 | 多账号内容差异化 |

---

## 二、KBSZR 发布流程分析

### 2.1 完整发布链路

```
[1] 扫码登录获取 Cookie
     ↓
[2] 内容准备（视频/图文）
     ↓
[3] 多平台文案适配（PlatformAdapter）
     ↓
[4] 多账号人设隔离（PersonaIsolator）
     ↓
[5] Cookie 注入浏览器
     ↓
[6] 导航到平台上传页面
     ↓
[7] 上传视频文件
     ↓
[8] 等待上传完成（轮询）
     ↓
[9] 填写表单（标题/描述/标签）
     ↓
[10] 点击发布按钮
     ↓
[11] 获取发布结果
```

### 2.2 各平台配置

| 平台 | 上传页面 | 标题上限 | 内容上限 | 标签数 | 允许Emoji | 换行限制 | 需要CTA |
|------|----------|----------|----------|--------|-----------|----------|---------|
| 抖音 | creator.douyin.com/creator/microapp/upload | 30字 | 2000字 | 3 | 是(5%) | 5行 | 是 |
| 快手 | cp.kuaishou.com/upload | 20字 | 1500字 | 2 | 否 | 3行 | 是 |
| 小红书 | creator.xiaohongshu.com/creator/post | 20字 | 1000字 | 0 | 是(15%) | 10行 | 否 |
| 视频号 | channels.weixin.qq.com/platform/media/upload | 40字 | 50000字 | 0 | 否 | 不限 | 是 |
| B站 | member.bilibili.com/v/video/upload | 50字 | 5000字 | 3 | 是(5%) | 默认 | 是 |

### 2.3 设计亮点

| 亮点 | 说明 |
|------|------|
| Cookie 持久化 | 扫码登录后保存到 JSON 文件 |
| 人设隔离 | 8 种人设类型，禁用词过滤 + 语气风格调整 |
| 文案适配 | 按平台规范截断/换行/Emoji/CTA |
| 重试机制 | 最多重试 2 次，重试间隔 3 秒 |
| 平台间休息 | 每个平台发布后休息 2 秒 |

### 2.4 设计缺陷（KBSZR）

| 缺陷 | 说明 | WebReaper 改进方案 |
|------|------|-------------------|
| 接口定义在外层 | 基类定义在 browser 模块 | 接口定义在 usecase/port |
| Cookie 双通道未统一 | CookieManager 和 PlaywrightService 各自独立 | 统一 Cookie 管理 |
| 人设隔离与文案适配未串联 | 缺少编排层 | UseCase 统一编排 |
| 发布功能未完全接入 | UI 层返回"功能开发中" | 完整实现 |
| 发布结果获取简单 | 仅获取 URL，未提取 ID | 网络拦截 + 元素提取 |
| 无 Cookie 过期检测 | 需要用户重新扫码 | 自动刷新机制 |
| 无账号轮换 | 仅人设隔离 | 自动账号轮换 |
| 无图文发布 | 仅视频发布 | 支持图文 |
| 无 Shadow DOM 处理 | 无法穿透组件 | JS 注入穿透 |
| 无定时发布 | 仅有延时队列 | 支持定时发布 |

---

## 三、各平台选择器配置（完整）

### 3.1 抖音

```go
"douyin": {
    Platform:  "douyin",
    UploadURL: "https://creator.douyin.com/creator/microapp/upload",
    MaxTitleLength:     30,
    MaxDescriptionLength: 2000,
    MaxTagCount:        3,
    AllowEmoji:         true,
    EmojiDensity:       0.05,
    MaxNewLines:        5,
    RequireCTA:         true,
    DefaultTags:        []string{"#推荐", "#种草", "#好物"},
    Selectors: map[string]string{
        "upload_input":     `input[type="file"]`,
        "title_input":      `input[placeholder*="标题"]`,
        "description_input": `textarea[placeholder*="描述"]`,
        "publish_button":   `button:has-text("发布")`,
        "confirm_button":   `button:has-text("确认")`,
        "upload_progress":  `.upload-progress`,
        "error_toast":      `.error-toast`,
    },
    // 发布结果获取：通过网络拦截捕获 item/create 响应
    ResultCapture: "network_intercept",
    ResultPattern: `item/create`,
    ResultField:   `aweme_id`,
},
```

### 3.2 快手

```go
"kuaishou": {
    Platform:  "kuaishou",
    UploadURL: "https://cp.kuaishou.com/upload",
    MaxTitleLength:     20,
    MaxDescriptionLength: 1500,
    MaxTagCount:        2,
    AllowEmoji:         false,
    MaxNewLines:        3,
    RequireCTA:         true,
    DefaultTags:        []string{"#推荐", "#好物"},
    Selectors: map[string]string{
        "upload_input":     `input[type="file"]`,
        "title_input":      `input[placeholder*="标题"]`,
        "description_input": `textarea[placeholder*="简介"]`,
        "publish_button":   `button:has-text("发布")`,
        "confirm_button":   `button:has-text("确认发布")`,
    },
    // 发布结果获取：从页面元素提取视频 ID
    ResultCapture: "page_element",
    ResultSelector: `.video-url`,
    ResultPattern: `video/(\w+)`,
},
```

### 3.3 小红书

```go
"xiaohongshu": {
    Platform:  "xiaohongshu",
    UploadURL: "https://creator.xiaohongshu.com/creator/post",
    MaxTitleLength:     20,
    MaxDescriptionLength: 1000,
    MaxTagCount:        0,
    AllowEmoji:         true,
    EmojiDensity:       0.15,
    MaxNewLines:        10,
    RequireCTA:         false,
    DefaultTags:        []string{"#推荐", "#种草", "#好物"},
    Selectors: map[string]string{
        "upload_input":     `input[type="file"]`,
        "title_input":      `input[placeholder*="标题"]`,
        "description_input": `.note-textarea`,
        "publish_button":   `button:has-text("发布")`,
        "confirm_button":   `button:has-text("确认")`,
    },
    // Shadow DOM 处理
    ShadowDOM: true,
    ShadowSelector: `xhs-publish-btn`,
    // 发布结果获取：从页面元素提取笔记 ID
    ResultCapture: "page_element",
    ResultSelector: `.note-url`,
    ResultPattern: `explore/(\w+)`,
},
```

### 3.4 视频号

```go
"weixin": {
    Platform:  "weixin",
    UploadURL: "https://channels.weixin.qq.com/platform/media/upload",
    MaxTitleLength:     40,
    MaxDescriptionLength: 50000,
    MaxTagCount:        0,
    AllowEmoji:         false,
    MaxNewLines:        0,  // 不限
    RequireCTA:         true,
    DefaultTags:        []string{},
    Selectors: map[string]string{
        "upload_input":     `input[type="file"]`,
        "title_input":      `input[placeholder*="标题"]`,
        "description_input": `textarea[placeholder*="描述"]`,
        "publish_button":   `button:has-text("发表")`,
    },
    // 发布结果获取：从页面元素提取视频 ID
    ResultCapture: "page_element",
    ResultSelector: `.video-url`,
    ResultPattern: `finder/feed/(\w+)`,
},
```

### 3.5 B站

```go
"bilibili": {
    Platform:  "bilibili",
    UploadURL: "https://member.bilibili.com/v/video/upload",
    MaxTitleLength:     50,
    MaxDescriptionLength: 5000,
    MaxTagCount:        3,
    AllowEmoji:         true,
    EmojiDensity:       0.05,
    MaxNewLines:        0,  // 默认
    RequireCTA:         true,
    DefaultTags:        []string{"#推荐", "#好看", "#视频"},
    Selectors: map[string]string{
        "upload_input":     `.upload-input`,  // 优先使用
        "upload_input_fallback": `input[type="file"]`,  // 回退
        "title_input":      `#video-title-input`,
        "description_input": `#video-desc-input`,
        "publish_button":   `.publish-button`,
        "confirm_button":   `.confirm-publish`,
    },
    // 发布结果获取：从页面元素提取 BV 号
    ResultCapture: "page_element",
    ResultSelector: `.video-url`,
    ResultPattern: `video/(BV\w+)`,
},
```

---

## 四、发布结果获取逻辑

### 4.1 获取方式对比

| 平台 | 获取方式 | 实现方案 | 提取字段 |
|------|----------|----------|----------|
| 抖音 | 网络拦截 | chromedp 网络监听 | aweme_id |
| 快手 | 页面元素 | CSS 选择器 + 正则 | video_id |
| 小红书 | 页面元素 | CSS 选择器 + 正则 | note_id |
| 视频号 | 页面元素 | CSS 选择器 + 正则 | finder_id |
| B站 | 页面元素 | CSS 选择器 + 正则 | BV号 |

### 4.2 网络拦截实现（抖音）

```go
// 抖音发布结果通过网络拦截获取
func (p *DouyinPublisher) captureResult(ctx context.Context, page *chromedp.Page) (string, error) {
    var awemeID string
    
    // 监听网络响应
    chromedp.ListenTarget(ctx, func(ev interface{}) {
        if resp, ok := ev.(*network.EventResponseReceived); ok {
            if strings.Contains(resp.Response.URL, "item/create") {
                // 读取响应体
                body, _ := network.GetResponseBody(resp.RequestID).Do(ctx)
                var result struct {
                    AwemeID string `json:"aweme_id"`
                }
                json.Unmarshal(body, &result)
                awemeID = result.AwemeID
            }
        }
    })
    
    // 点击发布
    chromedp.Click(`button:has-text("发布")`, chromedp.ByQuery).Do(ctx)
    
    // 等待结果
    time.Sleep(5 * time.Second)
    
    return awemeID, nil
}
```

### 4.3 页面元素获取实现（其他平台）

```go
// 从页面元素提取发布结果
func (p *Publisher) extractResult(ctx context.Context, page *chromedp.Page, config PlatformConfig) (string, error) {
    var url string
    
    // 等待结果元素出现
    chromedp.WaitVisible(config.ResultSelector, chromedp.ByQuery).Do(ctx)
    
    // 获取元素文本
    chromedp.Text(config.ResultSelector, &url, chromedp.ByQuery).Do(ctx)
    
    // 提取 ID
    re := regexp.MustCompile(config.ResultPattern)
    matches := re.FindStringSubmatch(url)
    if len(matches) > 1 {
        return matches[1], nil
    }
    
    return url, nil
}
```

---

## 五、Shadow DOM 处理

### 5.1 小红书 Shadow DOM 穿透

小红书发布页面使用 Shadow DOM 封装组件，需要特殊处理：

```go
// Shadow DOM 穿透点击
func (p *XiaohongshuPublisher) clickShadowButton(ctx context.Context, page *chromedp.Page) error {
    // 方式 1：JS 注入穿透
    js := `
        const host = document.querySelector('xhs-publish-btn');
        if (host && host.shadowRoot) {
            const button = host.shadowRoot.querySelector('button');
            if (button) {
                button.click();
                return true;
            }
        }
        return false;
    `
    var result bool
    chromedp.Evaluate(js, &result).Do(ctx)
    
    if !result {
        // 方式 2：重写 attachShadow 强制 mode:'open'
        js = `
            const originalAttachShadow = Element.prototype.attachShadow;
            Element.prototype.attachShadow = function() {
                return originalAttachShadow.call(this, {mode: 'open'});
            };
        `
        chromedp.Evaluate(js, nil).Do(ctx)
        
        // 重新尝试点击
        chromedp.Click(`xhs-publish-btn button`, chromedp.ByQuery).Do(ctx)
    }
    
    return nil
}
```

### 5.2 Shadow DOM 检测

```go
// 检测元素是否在 Shadow DOM 中
func isInShadowDOM(selector string) bool {
    // 已知使用 Shadow DOM 的平台组件
    shadowComponents := map[string][]string{
        "xiaohongshu": {"xhs-publish-btn", "xhs-upload"},
    }
    // 检查选择器是否匹配已知组件
    // ...
    return false
}
```

---

## 六、Cookie 管理

### 6.1 存储方式

WebReaper 采用数据库存储（与 KBSZR 的文件存储不同）：

```sql
-- 复用现有 crawler_accounts 表
CREATE TABLE crawler_accounts (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    platform VARCHAR(32) NOT NULL,
    account_name VARCHAR(128) NOT NULL,
    cookie_encrypted TEXT NOT NULL,  -- 加密存储
    user_agent VARCHAR(512),
    proxy_address VARCHAR(256),
    status VARCHAR(32) DEFAULT 'active',
    last_used_at DATETIME,
    last_health_check_at DATETIME,
    health_check_result VARCHAR(32),
    daily_usage_count INT DEFAULT 0,
    daily_usage_limit INT DEFAULT 50,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

### 6.2 Cookie 加密

```go
// Cookie 加密存储
func (m *CookieManager) EncryptCookie(cookies []http.Cookie) (string, error) {
    data, err := json.Marshal(cookies)
    if err != nil {
        return "", err
    }
    encrypted, err := m.vault.Encrypt(data)
    if err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(encrypted), nil
}

// Cookie 解密
func (m *CookieManager) DecryptCookie(encrypted string) ([]http.Cookie, error) {
    data, err := base64.StdEncoding.DecodeString(encrypted)
    if err != nil {
        return nil, err
    }
    decrypted, err := m.vault.Decrypt(data)
    if err != nil {
        return nil, err
    }
    var cookies []http.Cookie
    err = json.Unmarshal(decrypted, &cookies)
    return cookies, err
}
```

### 6.3 Cookie 过期检测

```go
// 检测 Cookie 是否过期
func (m *CookieManager) IsCookieExpired(cookies []http.Cookie) bool {
    now := time.Now()
    for _, cookie := range cookies {
        if cookie.Expires.After(now) {
            return false  // 至少有一个未过期
        }
    }
    return true  // 全部过期
}

// 检测 Cookie 是否有效（访问平台验证）
func (m *CookieManager) ValidateCookie(ctx context.Context, platform string, cookies []http.Cookie) (bool, error) {
    // 创建浏览器上下文
    page, err := m.browserManager.NewPage(ctx, cookies)
    if err != nil {
        return false, err
    }
    defer page.Close()
    
    // 访问平台检查登录状态
    switch platform {
    case "douyin":
        return m.validateDouyinCookie(ctx, page)
    case "kuaishou":
        return m.validateKuaishouCookie(ctx, page)
    // ...
    }
    return false, nil
}
```

### 6.4 Cookie 自动刷新

```go
// Cookie 自动刷新（通过访问平台获取新 Cookie）
func (m *CookieManager) RefreshCookie(ctx context.Context, platform string, oldCookies []http.Cookie) ([]http.Cookie, error) {
    // 创建浏览器上下文
    page, err := m.browserManager.NewPage(ctx, oldCookies)
    if err != nil {
        return nil, err
    }
    defer page.Close()
    
    // 访问平台主页（触发 Cookie 刷新）
    switch platform {
    case "douyin":
        chromedp.Navigate("https://www.douyin.com").Do(ctx)
    case "kuaishou":
        chromedp.Navigate("https://www.kuaishou.com").Do(ctx)
    // ...
    }
    
    // 等待页面加载
    chromedp.WaitReady("body").Do(ctx)
    
    // 获取新 Cookie
    newCookies, err := page.GetCookies()
    if err != nil {
        return nil, err
    }
    
    return newCookies, nil
}
```

---

## 七、账号轮换策略

### 7.1 轮换算法

```go
// AccountRotator 账号轮换器
type AccountRotator struct {
    accountRepo port.AccountRepository
    strategy    RotationStrategy
}

// RotationStrategy 轮换策略接口
type RotationStrategy interface {
    Select(accounts []entity.Account) *entity.Account
}

// RoundRobinStrategy 轮询策略
type RoundRobinStrategy struct {
    index int
}

func (s *RoundRobinStrategy) Select(accounts []entity.Account) *entity.Account {
    if len(accounts) == 0 {
        return nil
    }
    account := &accounts[s.index%len(accounts)]
    s.index++
    return account
}

// LeastUsedStrategy 最少使用策略
type LeastUsedStrategy struct{}

func (s *LeastUsedStrategy) Select(accounts []entity.Account) *entity.Account {
    if len(accounts) == 0 {
        return nil
    }
    min := accounts[0]
    for _, a := range accounts[1:] {
        if a.DailyUsageCount < min.DailyUsageCount {
            min = a
        }
    }
    return &min
}

// RandomStrategy 随机策略
type RandomStrategy struct{}

func (s *RandomStrategy) Select(accounts []entity.Account) *entity.Account {
    if len(accounts) == 0 {
        return nil
    }
    return &accounts[rand.Intn(len(accounts))]
}
```

### 7.2 账号健康检查

```go
// 检查账号是否可用
func (r *AccountRotator) IsAccountHealthy(ctx context.Context, account *entity.Account) bool {
    // 1. 检查状态
    if account.Status != "active" {
        return false
    }
    
    // 2. 检查每日使用上限
    if account.DailyUsageCount >= account.DailyUsageLimit {
        return false
    }
    
    // 3. 检查 Cookie 是否过期
    cookies, err := r.cookieManager.DecryptCookie(account.CookieEncrypted)
    if err != nil {
        return false
    }
    if r.cookieManager.IsCookieExpired(cookies) {
        return false
    }
    
    return true
}
```

### 7.3 限速控制

```go
// 限速控制器
type RateLimiter struct {
    limits map[string]*RateLimit  // platform -> limit
}

type RateLimit struct {
    MaxPerDay    int  // 每日最大发布数
    MaxPerHour   int  // 每小时最大发布数
    MinInterval  int  // 最小间隔（秒）
}

var DefaultRateLimits = map[string]*RateLimit{
    "douyin":      {MaxPerDay: 5, MaxPerHour: 2, MinInterval: 1800},
    "kuaishou":    {MaxPerDay: 5, MaxPerHour: 2, MinInterval: 1800},
    "xiaohongshu": {MaxPerDay: 3, MaxPerHour: 1, MinInterval: 3600},
    "weixin":      {MaxPerDay: 5, MaxPerHour: 2, MinInterval: 1800},
    "bilibili":    {MaxPerDay: 3, MaxPerHour: 1, MinInterval: 3600},
}
```

---

## 八、图文发布流程

### 8.1 小红书图文发布

```go
// XiaohongshuImagePublisher 小红书图文发布器
type XiaohongshuImagePublisher struct {
    browserManager *BrowserManager
    humanize       *humanize.HumanAction
    config         entity.PlatformConfig
}

func (p *XiaohongshuImagePublisher) Publish(ctx context.Context, req port.PublishRequest) (*port.PublishResult, error) {
    start := time.Now()
    
    // 1. 初始化浏览器
    page, err := p.browserManager.NewPage(ctx, req.Cookies)
    if err != nil {
        return nil, err
    }
    defer page.Close()
    
    // 2. 导航到发布页面
    chromedp.Navigate(p.config.UploadURL).Do(ctx)
    
    // 3. 上传多张图片
    for _, imagePath := range req.ImagePaths {
        chromedp.SetUploadFiles(`input[type="file"]`, []string{imagePath}).Do(ctx)
        time.Sleep(2 * time.Second)  // 等待上传
    }
    
    // 4. 等待编辑页就绪
    chromedp.WaitVisible(`input[placeholder*="标题"]`).Do(ctx)
    
    // 5. 填写标题（截断到 20 字）
    title := truncateString(req.Title, 20)
    chromedp.SendKeys(`input[placeholder*="标题"]`, title).Do(ctx)
    
    // 6. 填写正文（ProseMirror/Tiptap）
    p.fillProseMirror(ctx, page, req.Description)
    
    // 7. Shadow DOM 穿透点击发布
    p.clickShadowButton(ctx, page)
    
    // 8. 检查发布结果
    if err := p.checkPublishResult(ctx, page); err != nil {
        return nil, err
    }
    
    return &port.PublishResult{
        Success:  true,
        Platform: "xiaohongshu",
        Duration: time.Since(start).Seconds(),
    }, nil
}
```

### 8.2 抖音图文发布

```go
// DouyinImagePublisher 抖音图文发布器
type DouyinImagePublisher struct {
    browserManager *BrowserManager
    humanize       *humanize.HumanAction
    config         entity.PlatformConfig
}

func (p *DouyinImagePublisher) Publish(ctx context.Context, req port.PublishRequest) (*port.PublishResult, error) {
    start := time.Now()
    
    // 1. 初始化浏览器
    page, err := p.browserManager.NewPage(ctx, req.Cookies)
    if err != nil {
        return nil, err
    }
    defer page.Close()
    
    // 2. 导航到发布页面
    chromedp.Navigate(p.config.UploadURL).Do(ctx)
    
    // 3. 选择图文模式
    chromedp.Click(`button:has-text("图文")`).Do(ctx)
    
    // 4. 上传多张图片
    for _, imagePath := range req.ImagePaths {
        chromedp.SetUploadFiles(`input[type="file"]`, []string{imagePath}).Do(ctx)
        time.Sleep(2 * time.Second)
    }
    
    // 5. 选择音乐（可选）
    if req.MusicID != "" {
        p.selectMusic(ctx, page, req.MusicID)
    }
    
    // 6. 填写标题和描述
    chromedp.SendKeys(`input[placeholder*="标题"]`, req.Title).Do(ctx)
    chromedp.SendKeys(`textarea[placeholder*="描述"]`, req.Description).Do(ctx)
    
    // 7. 点击发布
    chromedp.Click(`button:has-text("发布")`).Do(ctx)
    
    // 8. 获取发布结果
    awemeID, err := p.captureResult(ctx, page)
    if err != nil {
        return nil, err
    }
    
    return &port.PublishResult{
        Success:  true,
        Platform: "douyin",
        VideoID:  awemeID,
        Duration: time.Since(start).Seconds(),
    }, nil
}
```

---

## 九、定时发布逻辑

### 9.1 定时发布实现

```go
// ScheduledPublisher 定时发布器
type ScheduledPublisher struct {
    publishUC   *publish.UseCase
    jobRepo     port.PublishJobRepository
    scheduler   *cron.Scheduler
}

// 添加定时发布任务
func (sp *ScheduledPublisher) Schedule(ctx context.Context, req port.PublishRequest, scheduledAt time.Time) error {
    // 1. 创建发布任务（状态为 scheduled）
    job := &entity.PublishJob{
        ID:          generateID(),
        TenantID:    req.TenantID,
        Platform:    req.Platform,
        Status:      "scheduled",
        ScheduledAt: &scheduledAt,
    }
    if err := sp.jobRepo.Create(ctx, job); err != nil {
        return err
    }
    
    // 2. 添加定时任务
    sp.scheduler.AddJob(scheduledAt, func() {
        sp.executeScheduledJob(job.ID)
    })
    
    return nil
}

// 执行定时发布任务
func (sp *ScheduledPublisher) executeScheduledJob(jobID string) {
    ctx := context.Background()
    
    // 1. 获取任务
    job, err := sp.jobRepo.FindByID(ctx, jobID)
    if err != nil {
        return
    }
    
    // 2. 检查任务状态
    if job.Status != "scheduled" {
        return
    }
    
    // 3. 更新状态为 pending
    job.Status = "pending"
    sp.jobRepo.Update(ctx, job)
    
    // 4. 执行发布
    req := port.PublishRequest{
        TenantID:  job.TenantID,
        Platform:  job.Platform,
        VideoPath: job.VideoPath,
        Title:     job.Title,
        Description: job.Description,
        Tags:      job.Tags,
    }
    result, err := sp.publishUC.Publish(ctx, req)
    if err != nil {
        job.Status = "failed"
        job.ErrorMessage = err.Error()
    } else {
        job.Status = "success"
        job.PublishedURL = result.URL
    }
    sp.jobRepo.Update(ctx, job)
}
```

### 9.2 Cron 调度器

```go
// 使用 robfig/cron 库
import "github.com/robfig/cron/v3"

type CronScheduler struct {
    cron *cron.Cron
}

func NewCronScheduler() *CronScheduler {
    return &CronScheduler{
        cron: cron.New(cron.WithLocation(time.Local)),
    }
}

func (s *CronScheduler) AddJob(at time.Time, fn func()) {
    // 计算 cron 表达式
    spec := fmt.Sprintf("%d %d %d %d *", at.Minute(), at.Hour(), at.Day(), at.Month())
    s.cron.AddFunc(spec, fn)
}

func (s *CronScheduler) Start() {
    s.cron.Start()
}
```

---

## 十、实现计划（更新）

### 阶段 1：基础设施（1-2 天）

| 任务 | 文件 | 说明 |
|------|------|------|
| 定义实体 | `domain/entity/publish_job.go` | PublishJob、PublishResult |
| 定义实体 | `domain/entity/persona.go` | Persona 配置 |
| 定义实体 | `domain/entity/platform_config.go` | 平台配置（完整选择器） |
| 定义接口 | `usecase/port/publisher.go` | Publisher、PublisherRegistry |
| 定义接口 | `usecase/port/content_adapter.go` | ContentAdapter |
| 定义接口 | `usecase/port/persona_isolator.go` | PersonaIsolator |
| 定义仓储 | `usecase/port/publish_repo.go` | PublishJobRepository |

### 阶段 2：用例层（2-3 天）

| 任务 | 文件 | 说明 |
|------|------|------|
| 发布用例 | `usecase/publish/publish_use_case.go` | 核心编排逻辑 |
| 内容适配 | `usecase/publish/content_adapter.go` | 多平台文案适配 |
| 人设隔离 | `usecase/publish/persona_isolator.go` | 人设风格注入 |
| 账号轮换 | `usecase/publish/account_rotator.go` | 轮询/最少使用/随机 |
| 限速控制 | `usecase/publish/rate_limiter.go` | 每日/每小时限制 |
| 重试策略 | `usecase/publish/retry_strategy.go` | 固定间隔/指数退避 |
| 进度跟踪 | `usecase/publish/progress_tracker.go` | 回调通知 |

### 阶段 3：适配器层（3-5 天）

| 任务 | 文件 | 说明 |
|------|------|------|
| 浏览器管理 | `adapter/publisher/browser_manager.go` | chromedp 封装 |
| Cookie 管理 | `adapter/publisher/cookie_manager.go` | 加密/解密/刷新 |
| 人类行为 | `adapter/publisher/humanize/human_action.go` | 反检测 |
| Shadow DOM | `adapter/publisher/shadow_dom.go` | JS 注入穿透 |
| 抖音发布 | `adapter/publisher/douyin_publisher.go` | 视频+图文 |
| 快手发布 | `adapter/publisher/kuaishou_publisher.go` | 视频 |
| 小红书发布 | `adapter/publisher/xiaohongshu_publisher.go` | 视频+图文 |
| 视频号发布 | `adapter/publisher/weixin_publisher.go` | 视频 |
| B站发布 | `adapter/publisher/bilibili_publisher.go` | 视频 |

### 阶段 4：仓储层（1-2 天）

| 任务 | 文件 | 说明 |
|------|------|------|
| 任务仓储 | `adapter/repository/publish_job_repo.go` | GORM 实现 |
| 数据库迁移 | `adapter/repository/migrations/xxx_publish_jobs.sql` | 表结构 |

### 阶段 5：路由层（1 天）

| 任务 | 文件 | 说明 |
|------|------|------|
| 发布路由 | `adapter/handler/publish_handler.go` | API 端点 |
| 注册路由 | `adapter/handler/router_publish.go` | 路由注册 |

### 阶段 6：测试（2-3 天）

| 任务 | 说明 |
|------|------|
| 单元测试 | Mock Publisher 接口 |
| 集成测试 | 真实浏览器测试 |
| 端到端测试 | 完整发布流程测试 |

---

## 十一、总结

### 设计文档完整性校验

| 模块 | 状态 | 说明 |
|------|------|------|
| 各平台选择器 | ✅ 已完成 | 5 个平台完整定义 |
| 图文发布流程 | ✅ 已完成 | 小红书/抖音图文发布 |
| 发布结果获取 | ✅ 已完成 | 网络拦截 + 页面元素 |
| Shadow DOM 处理 | ✅ 已完成 | JS 注入穿透 |
| Cookie 持久化 | ✅ 已完成 | 数据库加密存储 |
| Cookie 刷新 | ✅ 已完成 | 自动刷新机制 |
| 账号轮换 | ✅ 已完成 | 3 种策略 |
| 限速控制 | ✅ 已完成 | 每日/每小时限制 |
| 定时发布 | ✅ 已完成 | Cron 调度器 |

### 预计工时

| 阶段 | 工时 | 说明 |
|------|------|------|
| 阶段 1：基础设施 | 1-2 天 | 接口 + 实体 |
| 阶段 2：用例层 | 2-3 天 | 编排逻辑 |
| 阶段 3：适配器层 | 3-5 天 | 各平台实现 |
| 阶段 4：仓储层 | 1-2 天 | 数据持久化 |
| 阶段 5：路由层 | 1 天 | API 端点 |
| 阶段 6：测试 | 2-3 天 | 单元/集成/端到端 |
| **总计** | **10-16 天** | |

**状态**：设计完成，可开始实现。
