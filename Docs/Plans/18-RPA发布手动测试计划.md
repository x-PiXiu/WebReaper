# 18 — RPA 发布手动测试计划

> **用途**：手动测试各平台 RPA 发布链路，验证 chromedp 浏览器自动化 + Cookie 注入 + 内容填充 + 发布提交的完整流程。
> **关联文档**：[17-服务端核心业务API完整测试计划](./17-服务端核心业务API完整测试计划.md)、[14-视频图文文章发布客户端详细设计](./14-视频图文文章发布客户端详细设计.md)
> **创建日期**：2026-08-26
> **前置条件**：需要用户手机扫码登录各平台获取 Cookie

---

## 一、测试环境

| 项目 | 值 |
|---|---|
| 后端地址 | `http://localhost:8082` |
| 前端地址 | `http://localhost:5173` |
| 管理员账号 | `admin` / `admin123` |
| 浏览器模式 | `QR_LOGIN_HEADED=true`（可见窗口，调试用） |
| 测试品牌 | 蜀香居川菜馆（`brand-1786511295011197900`） |
| 存储模式 | OSS（`https://media.zhichen.chat`） |

---

## 二、客户端组件需求说明

### 2.1 发布流程总览（用户视角）

```
用户操作流程：
  ① 选择内容 → ② 选择发布平台 → ③ 绑定账号（首次） → ④ 编辑发布信息 → ⑤ 确认发布

客户端组件：
  内容选择器 → 平台选择器 → 账号绑定（QR扫码） → 发布编辑器 → 发布确认卡
```

### 2.2 发布前：用户需要完成的操作

#### 2.2.1 选择要发布的内容

**客户端组件**：内容列表页 / 工作台 / 我的作品

用户需要先有可发布的内容来源：
- **已生成的 GEO 内容**（草稿/已发布状态）
- **已生成的视频**（生成任务 success 后的产物）
- **手动输入的内容**（直接在发布编辑器中编写）

**客户端需要做的**：
- 展示可发布内容列表（标题、状态、预览）
- 支持勾选/点击选择要发布的内容
- 将 `content_id` 传递给发布流程

**API 调用前**：
```javascript
// 客户端需要收集的数据
{
  content_id: "oc-xxx",        // 已有内容 ID（可选）
  title: "发布标题",           // 用户编辑后的标题
  content: "发布正文...",      // 用户编辑后的内容
  media_urls: ["https://..."], // 媒体文件 URL 列表
}
```

#### 2.2.2 选择发布平台

**客户端组件**：平台选择器（发布向导 Step 1）

用户需要选择目标发布平台（可多选）：
- 抖音（视频）
- 小红书（图文/视频）
- 知乎（文章）
- B站（视频）
- 快手（视频）
- 微信视频号（视频/图文）

**客户端需要做的**：
- 展示平台列表 + 图标
- 标注每个平台支持的内容类型（视频/图文/文章）
- 检查账号绑定状态（已绑定/未绑定）
- 禁用不支持当前内容类型的平台
- 返回用户选择的 `platform` 列表

**前端组件示例**：
```jsx
// 平台选择器需要展示的信息
const platforms = [
  { id: 'douyin', name: '抖音', types: ['video'], bound: true },
  { id: 'xiaohongshu', name: '小红书', types: ['image', 'video'], bound: false },
  { id: 'zhihu', name: '知乎', types: ['article'], bound: true },
  { id: 'bilibili', name: 'B站', types: ['video'], bound: false },
  { id: 'kuaishou', name: '快手', types: ['video'], bound: false },
  { id: 'weixin', name: '微信视频号', types: ['video', 'image'], bound: false },
]
```

#### 2.2.3 绑定平台账号（首次发布）

**客户端组件**：QR 扫码弹窗

用户首次发布到某平台时，需要扫码绑定账号。

**客户端需要做的**：
1. 调用 `POST /api/v1/merchant/accounts/qr-login` 启动扫码
2. 展示返回的二维码图片（base64 PNG）
3. 轮询 `GET /api/v1/merchant/accounts/qr-login/:sessionId` 等待扫码结果
4. 扫码成功后更新账号绑定状态
5. 扫码失败/超时提示用户重试

**前端交互流程**：
```
用户点击「绑定抖音账号」
  → 前端调用 POST /merchant/accounts/qr-login {platform: "douyin"}
  → 后端返回 {session_id, qr_image}
  → 前端显示二维码弹窗
  → 用户手机扫码
  → 前端轮询 GET /merchant/accounts/qr-login/:sessionId
  → 后端返回 {status: "success"} 或 {status: "expired"}
  → 前端关闭弹窗，更新绑定状态
```

**可能问题**：
- 二维码过期（120 秒）→ 前端需要倒计时 + 重新获取按钮
- 扫码失败 → 前端需要错误提示 + 重试按钮
- 网络超时 → 前端需要超时处理

#### 2.2.4 编辑发布信息

**客户端组件**：发布编辑器（发布向导 Step 2）

用户需要编辑/确认发布信息：
- **标题**：自动填充内容标题，用户可修改
- **正文**：自动填充内容正文，用户可修改
- **标签**：自动提取或用户手动添加
- **封面**：自动选择或用户上传
- **媒体文件**：自动关联或用户替换

**客户端需要做的**：
- 自动填充标题/正文（来自选中的内容）
- 支持富文本编辑（加粗、列表、链接等）
- 支持 emoji 插入（小红书/抖音需要）
- 支持标签管理（添加/删除/搜索）
- 支持封面图选择/上传
- 支持媒体文件替换
- 平台特定适配（字数限制、格式要求）

**平台特定编辑器差异**：

| 平台 | 标题 | 正文 | 标签 | 封面 | 特殊要求 |
|---|---|---|---|---|---|
| 抖音 | ≤55 字 | ≤1000 字 | ≤5 个 | 自动/手动 | 视频必填 |
| 小红书 | ≤20 字 | ≤1000 字 | ≤10 个 | 首图 | emoji 必需 |
| 知乎 | ≤100 字 | 无限制 | 无 | 无 | Markdown |
| B站 | ≤80 字 | ≤2000 字 | ≤10 个 | 自动 | 分区选择 |
| 快手 | ≤30 字 | 无限制 | 无 | 自动 | — |
| 微信 | — | ≤1000 字 | 无 | 自动 | — |

**前端校验**（提交前）：
```javascript
// 客户端需要做的校验
function validatePublishInput(platform, data) {
  const errors = [];

  // 标题校验
  if (!data.title || data.title.trim() === '') {
    errors.push('标题不能为空');
  }
  if (platform === 'xiaohongshu' && data.title.length > 20) {
    errors.push('小红书标题不能超过20字');
  }
  if (platform === 'douyin' && data.title.length > 55) {
    errors.push('抖音标题不能超过55字');
  }

  // 正文校验
  if (platform === 'zhihu' && data.content.length < 100) {
    errors.push('知乎文章建议至少100字');
  }

  // 媒体文件校验
  if (['douyin', 'bilibili', 'kuaishou', 'weixin'].includes(platform)) {
    if (!data.media_urls || data.media_urls.length === 0) {
      errors.push('视频平台需要上传视频文件');
    }
  }
  if (platform === 'xiaohongshu') {
    if (!data.media_urls || data.media_urls.length === 0) {
      errors.push('小红书需要至少1张图片');
    }
    if (data.media_urls && data.media_urls.length > 9) {
      errors.push('小红书最多9张图片');
    }
  }

  // 标签校验
  if (data.tags && data.tags.length > 10) {
    errors.push('标签不能超过10个');
  }

  return errors;
}
```

#### 2.2.5 确认发布

**客户端组件**：发布确认卡（发布向导 Step 3）

用户最终确认发布前，客户端需要展示：
- 平台名称 + 图标
- 标题预览
- 正文预览（截断）
- 媒体文件预览（缩略图）
- 标签列表
- 发布模式（自动/手动）
- 预计发布时间

**客户端需要做的**：
- 展示发布摘要
- 确认按钮
- 发布中状态（loading）
- 发布结果展示（成功/失败）

### 2.3 发布后：客户端需要处理的结果

#### 2.3.1 自动模式（RPA）

```javascript
// 客户端调用发布 API
const result = await publishAPI.submit({
  brand_id: brandId,
  platform: 'douyin',
  title: '标题',
  content: '内容',
  media_urls: ['https://...'],
  mode: 'auto'
});

// 客户端需要处理的响应
if (result.code === 0) {
  // 成功：轮询任务状态
  pollJobStatus(result.data.id);
} else {
  // 失败：显示错误信息
  showError(result.msg);
}
```

**轮询任务状态**：
```javascript
async function pollJobStatus(jobId) {
  const maxAttempts = 60; // 最多轮询 5 分钟
  for (let i = 0; i < maxAttempts; i++) {
    const res = await fetch(`/api/v1/merchant/publish-jobs/${jobId}/status`);
    const data = await res.json();

    if (data.data.status === 'success') {
      showSuccess('发布成功！');
      return;
    }
    if (data.data.status === 'failed') {
      showError(`发布失败: ${data.data.error_msg}`);
      return;
    }

    // 更新进度提示
    updateProgress(`发布中... (${i * 5}秒)`);
    await sleep(5000);
  }
  showError('发布超时，请稍后查看');
}
```

#### 2.3.2 手动模式（link 降级）

```javascript
// 当自动模式失败，降级到手动模式
if (result.data.mode === 'semi-auto') {
  // 显示手动发布提示
  showManualPublishDialog({
    platform: 'douyin',
    url: result.data.external_url,  // 平台发布页 URL
    title: '标题',
    content: '内容',
  });
}
```

**手动发布弹窗需要**：
- 显示平台发布页链接（可点击打开）
- 复制标题/正文按钮
- 「我已完成发布」确认按钮
- 「取消发布」按钮

### 2.4 客户端需要的 API 列表

| API | 用途 | 客户端调用时机 |
|---|---|---|
| `GET /merchant/accounts` | 查询已绑定账号 | 发布向导加载时 |
| `POST /merchant/accounts/qr-login` | 启动 QR 扫码 | 用户点击「绑定账号」 |
| `GET /merchant/accounts/qr-login/:id` | 轮询扫码状态 | 扫码弹窗打开期间 |
| `DELETE /merchant/accounts/qr-login/:id` | 取消扫码 | 用户关闭弹窗 |
| `POST /media/assets` | 上传媒体文件 | 用户选择文件后 |
| `GET /publish/channels` | 查询发布渠道能力 | 发布向导加载时 |
| `POST /publish` | 提交发布任务 | 用户确认发布 |
| `GET /publish-jobs` | 查询发布任务列表 | 发布管理页 |
| `GET /publish-jobs/:id/status` | 查询任务状态 | 发布后轮询 |
| `POST /publish/adapt-preview` | 内容适配预览 | 用户切换平台时 |
| `PUT /publish/draft` | 保存草稿 | 用户编辑时自动保存 |
| `GET /publish/draft` | 读取草稿 | 用户进入发布编辑器 |
| `GET /merchant/brands/:id/publish-config` | 查询品牌发布配置 | 发布向导加载时 |

### 2.5 客户端状态管理

```typescript
// 发布向导状态
interface PublishWizardState {
  // Step 1: 内容选择
  contentId: string | null;
  title: string;
  content: string;
  mediaUrls: string[];

  // Step 2: 平台选择
  selectedPlatforms: string[];

  // Step 3: 账号绑定
  accountBindings: Record<string, {
    bound: boolean;
    accountId: string;
    accountName: string;
  }>;

  // Step 4: 发布编辑
  platformConfigs: Record<string, {
    title: string;         // 平台特定标题（可覆盖）
    content: string;       // 平台特定正文（可覆盖）
    tags: string[];        // 平台特定标签
    coverUrl: string;      // 封面图
  }>;

  // Step 5: 发布状态
  publishJobs: Record<string, {
    jobId: string;
    status: 'pending' | 'processing' | 'success' | 'failed';
    errorMsg: string;
    externalUrl: string;
  }>;
}
```

### 2.6 客户端需要处理的边界情况

| 场景 | 客户端处理 |
|---|---|
| 内容过长 | 自动截断 + 提示用户 |
| 媒体文件过大 | 上传前检查 + 压缩提示 |
| 平台不支持当前内容类型 | 禁用该平台选项 |
| 账号未绑定 | 引导扫码绑定 |
| 二维码过期 | 倒计时 + 重新获取 |
| 发布超时 | 超时提示 + 查看详情 |
| 发布失败 | 错误信息 + 重试按钮 |
| link 降级 | 手动发布弹窗 |
| 多平台发布 | 并行发布 + 独立状态 |
| 网络断开 | 离线草稿保存 |

---

## 三、前置准备

### 步骤 1：启动服务

```bash
cd E:\workspace\Demo\goDemo\WebReaper
go run ./cmd/server
```

**预期**：终端输出 `WebReaper 启动中`，无报错。

**可能问题**：
- `8082 端口被占用` → 先停止旧进程：`taskkill /F /PID <PID>`
- `OSS 上传失败: i/o timeout` → 检查 `.env` 中 `OSS_INTERNAL_ENDPOINT` 是否为空（本地开发不能用内网端点）
- `数据库连接失败` → 检查 MySQL 是否启动

### 步骤 2：登录管理后台

1. 浏览器打开 `http://localhost:5173`
2. 登录 `admin` / `admin123`
3. 自动跳转到管理后台「平台总览」页面

**预期**：页面正常加载，显示 SaaS 运营指标。

**可能问题**：
- `invalid username or password` → 检查数据库中 admin 用户是否存在
- 页面白屏 → 检查前端是否启动（`cd web && npm run dev`）

### 步骤 3：绑定平台账号（QR 扫码）

每个平台需要单独扫码绑定。进入「平台方账号」页面。

#### 3.1 抖音扫码绑定

1. 点击「扫码添加」按钮
2. 选择平台：「抖音」
3. 点击「开始扫码」
4. **chromedp 启动浏览器窗口**（headed 模式可见）
5. 浏览器自动打开 `creator.douyin.com` 登录页
6. 页面显示抖音二维码
7. 打开手机抖音 APP → 左上角扫码图标 → 扫描二维码
8. 手机端确认授权
9. chromedp 检测到 Cookie 变化（日志显示 `当前 cookie（N 个）`）
10. 页面显示「绑定成功」

**预期**：
- 浏览器窗口正常弹出
- 二维码清晰可见
- 扫码后 5 秒内检测到登录成功
- 账号列表新增一条抖音记录，状态 `active`

**可能问题**：
- `二维码过期` → 二维码有效期约 120 秒，过期需重新点击「开始扫码」
- `chromedp 窗口不弹出` → 检查 `QR_LOGIN_HEADED=true` 是否生效
- `扫码后无反应` → 检查服务端日志是否有 `当前 cookie` 输出
- `Cookie 为空` → 平台可能更新了登录页结构，需检查 chromedp 选择器
- `网络超时` → 检查是否能访问 `creator.douyin.com`

#### 3.2 小红书扫码绑定

同抖音流程，选择「小红书」平台。

**可能问题**：
- 小红书登录页可能需要滑块验证 → chromedp 无法自动完成，需手动操作
- 小红书可能要求手机验证码 → 需手动输入

#### 3.3 B站扫码绑定

同抖音流程，选择「B站」平台。

**可能问题**：
- B站登录可能触发风控 → 需要验证码
- B站可能要求绑定手机号

#### 3.4 快手扫码绑定

同抖音流程，选择「快手」平台。

#### 3.5 知乎扫码绑定

同抖音流程，选择「知乎」平台。

**可能问题**：
- 知乎登录页可能需要图形验证码

#### 3.6 微信视频号扫码绑定

同抖音流程，选择「微信」平台。

**可能问题**：
- 微信扫码需要微信 APP 授权
- 视频号可能需要额外的创作者认证

---

## 四、测试用例

### TC-1：抖音 RPA 视频发布

**前置**：已扫码绑定抖音账号，Cookie 有效

#### 详细步骤

**步骤 1：准备测试视频**

```bash
# 使用之前生成的视频（8 秒，720p，H.264）
# 或准备一个新的 MP4 视频
# 要求：≤200MB，H.264 编码，1-600 秒
```

**步骤 2：上传视频素材到 OSS**

```bash
TOKEN=$(curl -s -X POST http://localhost:8082/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['token'])")

curl -s -X POST http://localhost:8082/api/v1/media/assets \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@E:/workspace/Demo/goDemo/WebReaper/data/test_video.mp4" \
  -F "owner_type=material"
```

**预期**：返回 `code=0`，`url` 以 `https://media.zhichen.chat/` 开头，`type=video`。

**可能问题**：
- `OSS 上传失败: i/o timeout` → OSS 配置问题，检查 `.env`
- `type` 为空 → MIME 检测失败，检查文件扩展名

**步骤 3：调用发布 API**

```bash
VIDEO_URL="<步骤2返回的url>"

curl -s -X POST http://localhost:8082/api/v1/merchant/publish \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "brand_id": "brand-1786511295011197900",
    "platform": "douyin",
    "title": "蜀香居川菜馆｜20年老店正宗川味",
    "content": "招牌水煮鱼麻辣鲜香，春熙路步行街18号 #成都美食 #川菜推荐",
    "media_urls": ["'"$VIDEO_URL"'"],
    "mode": "auto"
  }'
```

**预期**：返回 `code=0`，`data.id` 非空，`data.mode=auto`。

**可能问题**：
- `40002: Key: 'BrandID' Error:Field validation` → 缺少 `brand_id` 字段
- `发布服务未配置` → 管理后台未配置发布通道
- `无可用账号` → 未扫码绑定抖音账号

**步骤 4：观察 chromedp 窗口**

打开的浏览器窗口应自动执行：
1. 打开 `creator.douyin.com` 发布页
2. 注入 Cookie（页面显示已登录状态）
3. 上传视频文件
4. 填写标题：「蜀香居川菜馆｜20年老店正宗川味」
5. 填写描述：「招牌水煮鱼麻辣鲜香...」
6. 添加标签：「成都美食」「川菜推荐」
7. 点击「发布」按钮

**预期**：
- 浏览器窗口操作流畅，无卡顿
- 每步操作有合理延迟（模拟人类）
- 标题/描述/标签正确填充
- 发布成功后页面跳转到作品管理

**可能问题**：
- `Cookie 已过期` → 页面跳转到登录页，需重新扫码
- `视频上传失败` → 视频格式不支持或文件过大
- `标题/描述填充错误` → 页面结构变化，选择器失效
- `标签添加失败` → 标签输入框选择器变化
- `点击发布无反应` → 发布按钮选择器变化
- `触发风控` → 平台检测到自动化操作，弹出验证码

**步骤 5：检查发布结果**

```bash
# 查询发布任务状态
curl -s http://localhost:8082/api/v1/merchant/publish-jobs \
  -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys,json
d=json.load(sys.stdin)
jobs=d.get('data',{}).get('jobs',[])
for j in jobs[:3]:
    print(f'id={j.get(\"id\",\"\")} platform={j.get(\"platform\",\"\")} status={j.get(\"status\",\"\")} error={j.get(\"error_msg\",\"\")}')
"
```

**预期**：任务状态变为 `success`，无 `error_msg`。

**可能问题**：
- `status=failed` → 检查 `error_msg` 字段
- `status=pending` → 任务未执行，检查发布服务是否启动
- `status=processing` → 仍在执行中，等待

**步骤 6：验证抖音端**

打开抖音 APP → 我的 → 作品，确认视频已发布。

---

### TC-2：小红书 RPA 图文发布

**前置**：已扫码绑定小红书账号

#### 详细步骤

**步骤 1：准备测试图片（1-9 张）**

```bash
# 使用素材库中已有的图片
# 或准备新的 JPG/PNG 图片
# 要求：≤20MB/张，JPG/PNG/WebP 格式
```

**步骤 2：上传图片素材**

```bash
curl -s -X POST http://localhost:8082/api/v1/media/assets \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@/path/to/image1.jpg" \
  -F "owner_type=material"
```

**步骤 3：调用发布 API**

```bash
curl -s -X POST http://localhost:8082/api/v1/merchant/publish \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "brand_id": "brand-1786511295011197900",
    "platform": "xiaohongshu",
    "title": "😋成都本地人私藏20年老字号！春熙路这家川菜馆真的绝了！",
    "content": "招牌水煮鱼麻辣鲜香，人均80+就能吃到正宗川味！\n\n📍地址：春熙路步行街18号\n⏰营业时间：10:00-22:00\n💰人均：80-120元\n\n#成都美食 #川菜推荐 #春熙路美食 #探店",
    "media_urls": ["图片URL1", "图片URL2"],
    "mode": "auto"
  }'
```

**预期**：返回 `code=0`。

**步骤 4：观察 chromedp 窗口**

1. 打开 `creator.xiaohongshu.com` 发布页
2. 注入 Cookie
3. 上传图片
4. 填写标题（含 emoji）
5. 填写正文（含 emoji + 换行）
6. 添加标签（#话题）
7. 点击发布

**可能问题**：
- `emoji 显示异常` → 小红书编辑器对 emoji 的处理方式
- `标签未自动添加` → 小红书标签输入框需要特定交互方式
- `图片顺序错误` → 上传顺序与预期不符

**步骤 5：验证小红书端**

打开小红书 APP → 我的，确认笔记已发布。

---

### TC-3：知乎 RPA 文章发布

**前置**：已扫码绑定知乎账号

#### 详细步骤

**步骤 1：准备文章内容**

知乎支持 Markdown 格式，准备一篇 800-1500 字的文章。

**步骤 2：调用发布 API**

```bash
curl -s -X POST http://localhost:8082/api/v1/merchant/publish \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "brand_id": "brand-1786511295011197900",
    "platform": "zhihu",
    "title": "成都春熙路川菜馆推荐：20年老店蜀香居深度评测",
    "content": "## 为什么推荐这家店\n\n作为一个在成都生活了10年的吃货...\n\n## 招牌菜推荐\n\n1. **水煮鱼** - 麻辣鲜香，鱼肉嫩滑\n2. **回锅肉** - 肥而不腻，入口即化\n\n## 人均消费\n\n80-120元，性价比很高。",
    "mode": "auto"
  }'
```

**预期**：返回 `code=0`。

**步骤 3：观察 chromedp 窗口**

1. 打开知乎专栏发布页
2. 注入 Cookie
3. 填写标题
4. 填写正文（Markdown 格式）
5. 点击发布

**可能问题**：
- `知乎发布页 CSRF 限制` → 知乎发布页可能有 CSRF token 验证
- `Markdown 渲染异常` → 知乎编辑器可能不完全支持 Markdown
- `文章审核` → 知乎可能需要人工审核

**步骤 4：验证知乎端**

打开知乎 → 我的创作 → 文章，确认文章已发布。

---

### TC-4：B站 RPA 视频发布

**前置**：已扫码绑定 B站账号

#### 详细步骤

**步骤 1：准备视频**

B站要求：
- 格式：MP4
- 大小：≤ 8GB
- 时长：无限制
- 编码：H.264 推荐

**步骤 2：调用发布 API**

```bash
curl -s -X POST http://localhost:8082/api/v1/merchant/publish \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "brand_id": "brand-1786511295011197900",
    "platform": "bilibili",
    "title": "【探店】成都春熙路20年老店！正宗川味水煮鱼",
    "content": "今天带大家探访一家开了20年的川菜馆...",
    "tags": ["美食", "探店", "成都", "川菜"],
    "media_urls": ["视频URL"],
    "mode": "auto"
  }'
```

**步骤 3：观察 chromedp 窗口**

1. 打开 `member.bilibili.com` 投稿页
2. 注入 Cookie
3. 上传视频（可能需要较长时间）
4. 填写标题、描述、标签
5. 选择分区（生活 → 美食）
6. 点击投稿

**可能问题**：
- `视频转码中` → B站上传后需要转码，不会立即发布
- `分区选择失败` → 分区下拉框选择器变化
- `封面生成` → B站可能需要手动选择封面
- `投稿审核` → B站可能需要人工审核

---

### TC-5：快手 RPA 视频发布

**前置**：已扫码绑定快手账号

#### 详细步骤

```bash
curl -s -X POST http://localhost:8082/api/v1/merchant/publish \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "brand_id": "brand-1786511295011197900",
    "platform": "kuaishou",
    "title": "成都春熙路川菜馆探店",
    "content": "20年老店，正宗川味 #成都美食 #川菜",
    "media_urls": ["视频URL"],
    "mode": "auto"
  }'
```

**可能问题**：
- `快手发布页结构变化` → 快手可能频繁更新页面
- `视频大小限制` → 快手可能有更严格的大小限制

---

### TC-6：微信视频号 RPA 发布

**前置**：已扫码绑定微信视频号

#### 详细步骤

```bash
curl -s -X POST http://localhost:8082/api/v1/merchant/publish \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "brand_id": "brand-1786511295011197900",
    "platform": "weixin",
    "title": "成都春熙路川菜馆推荐",
    "content": "20年老店，正宗川味 #成都美食",
    "media_urls": ["视频URL"],
    "mode": "auto"
  }'
```

**可能问题**：
- `微信扫码登录复杂` → 微信可能需要多次扫码
- `视频号发布限制` → 可能需要创作者认证
- `内容审核严格` → 微信审核可能更严格

---

## 五、异常场景测试

### TC-7：Cookie 过期

**测试步骤**：
1. 等待 Cookie 自然过期（通常 7-30 天）
2. 或手动在数据库中清除 Cookie：`UPDATE accounts SET cookie_encrypted='' WHERE platform='douyin'`
3. 调用发布 API

**预期结果**：
- 任务状态变为 `failed`
- 错误信息包含「Cookie 过期」或「登录失效」
- 前端提示「请重新扫码登录」

**可能问题**：
- 错误信息不明确 → 需要改进错误翻译

---

### TC-8：平台页面改版

**测试步骤**：
1. 平台更新发布页 DOM 结构
2. 调用发布 API
3. 观察 chromedp 窗口

**预期结果**：
- RPA 失败，错误信息包含「选择器失效」或「元素未找到」
- 自动降级到 link 模式（返回平台发布页 URL）
- 前端提示「自动发布失败，请手动发布」

**可能问题**：
- 降级逻辑未触发 → 检查 `ErrTransportDegradable` 处理

---

### TC-9：link 模式降级

**测试步骤**：
1. 不绑定任何账号（无 Cookie）
2. 调用发布 API（`mode=auto`）

**预期结果**：
- 自动降级到 link 模式
- 返回平台发布页 URL（如 `https://creator.douyin.com/creator-micro/content/publish-video`）
- 任务状态为 `pending`（等待用户手动完成）

**API 响应示例**：
```json
{
  "code": 0,
  "data": {
    "mode": "semi-auto",
    "external_url": "https://creator.douyin.com/...",
    "status": "pending"
  }
}
```

---

### TC-10：DRY_RUN 模式

**测试步骤**：
1. 设置环境变量 `PUBLISH_DRY_RUN=true`
2. 重启服务
3. 调用发布 API

**预期结果**：
- chromedp 填充表单后截图返回
- 不实际点击发布
- 返回诊断截图
- 任务状态为 `pending`

**用途**：验证 RPA 选择器是否正确，不实际发布。

---

### TC-11：Cookie 滚动更新

**测试步骤**：
1. 正常发布一个视频
2. 检查数据库中 Cookie 是否更新

**预期结果**：
- 发布成功后，chromedp 读取浏览器最新 Cookie
- Cookie 加密回写到数据库
- Cookie 过期时间延长

**验证 SQL**：
```sql
SELECT platform, account_name, updated_at, LENGTH(cookie_encrypted) as cookie_len
FROM accounts WHERE platform='douyin';
```

---

### TC-12：并发发布

**测试步骤**：
1. 同时调用 2 个发布 API（不同平台）
2. 观察 chromedp 行为

**预期结果**：
- 每个平台使用独立的 chromedp 实例
- 互不干扰
- 都能成功发布

**可能问题**：
- chromedp 实例冲突 → 检查是否有并发锁
- Cookie 串台 → 检查 Cookie 注入是否按平台隔离

---

## 六、平台特定约束

### 抖音

| 约束 | 值 |
|---|---|
| 视频格式 | MP4, MOV |
| 视频大小 | ≤ 200MB |
| 视频时长 | 1-600 秒 |
| 标题长度 | ≤ 55 字符 |
| 描述长度 | ≤ 1000 字符 |
| 标签数量 | ≤ 5 个 |
| 编码要求 | H.264 推荐 |

### 小红书

| 约束 | 值 |
|---|---|
| 图片数量 | 1-9 张 |
| 图片大小 | ≤ 20MB/张 |
| 标题长度 | ≤ 20 字符 |
| 正文长度 | ≤ 1000 字符 |
| 标签数量 | ≤ 10 个 |

### 知乎

| 约束 | 值 |
|---|---|
| 文章标题 | ≤ 100 字符 |
| 文章正文 | 无限制 |
| 格式 | Markdown |

### B站

| 约束 | 值 |
|---|---|
| 视频格式 | MP4, FLV |
| 视频大小 | ≤ 8GB |
| 标题长度 | ≤ 80 字符 |
| 描述长度 | ≤ 2000 字符 |
| 标签数量 | ≤ 10 个 |

### 快手

| 约束 | 值 |
|---|---|
| 视频格式 | MP4 |
| 视频大小 | ≤ 100MB |
| 标题长度 | ≤ 30 字符 |

### 微信视频号

| 约束 | 值 |
|---|---|
| 视频格式 | MP4 |
| 视频大小 | ≤ 2GB |
| 描述长度 | ≤ 1000 字符 |

---

## 七、测试结果记录

| TC | 平台 | 场景 | 结果 | 耗时 | 备注 |
|---|---|---|---|---|---|
| TC-1 | 抖音 | 视频发布 | | | |
| TC-2 | 小红书 | 图文发布 | | | |
| TC-3 | 知乎 | 文章发布 | | | |
| TC-4 | B站 | 视频发布 | | | |
| TC-5 | 快手 | 视频发布 | | | |
| TC-6 | 微信 | 视频号发布 | | | |
| TC-7 | — | Cookie 过期 | | | |
| TC-8 | — | 页面改版 | | | |
| TC-9 | — | link 降级 | | | |
| TC-10 | — | DRY_RUN | | | |
| TC-11 | — | Cookie 滚动更新 | | | |
| TC-12 | — | 并发发布 | | | |

---

## 八、关键监控指标

| 指标 | 说明 | 查看方式 | 正常值 |
|---|---|---|---|
| chromedp 启动时间 | 浏览器从启动到页面加载完成 | 服务端日志 | < 10 秒 |
| Cookie 注入成功率 | Cookie 注入后页面是否保持登录 | 服务端日志 | 100% |
| 表单填充耗时 | 从开始填充到所有字段就绪 | 服务端日志 | < 30 秒 |
| 发布成功率 | 点击发布后平台是否接受 | 任务状态 | > 90% |
| Cookie 滚动更新 | 发布后 Cookie 是否回写更新 | 数据库查询 | 每次发布 |
| 降级触发率 | RPA 失败后降级到 link 的比例 | 任务统计 | < 10% |

---

## 九、常见问题排查

### 问题 1：chromedp 窗口不弹出

**原因**：`QR_LOGIN_HEADED` 未设置为 `true`

**解决**：
```bash
# .env
QR_LOGIN_HEADED=true
```

### 问题 2：扫码后无反应

**原因**：Cookie 检测间隔过长或选择器失效

**排查**：
1. 查看服务端日志是否有 `当前 cookie` 输出
2. 检查 chromedp 是否检测到页面变化
3. 检查平台登录页是否更新了 DOM 结构

### 问题 3：发布失败：Cookie 过期

**原因**：Cookie 已过期或被平台清除

**解决**：重新扫码绑定账号

### 问题 4：发布失败：选择器失效

**原因**：平台更新了发布页 DOM 结构

**解决**：
1. 使用 `DRY_RUN=true` 模式查看实际页面
2. 更新对应平台的 RPA 选择器代码
3. 重新测试

### 问题 5：OSS 上传失败

**原因**：OSS 配置错误或网络问题

**排查**：
```bash
# 检查 OSS 配置
cat configs/.env | grep OSS

# 测试 OSS 连通性
curl -s -o /dev/null -w "HTTP %{http_code}" https://media.zhichen.chat
```

### 问题 6：发布任务一直 pending

**原因**：发布服务未启动或任务队列阻塞

**排查**：
1. 检查服务端日志是否有 `scheduled-publish` 任务执行
2. 检查任务队列是否有积压
3. 手动触发发布任务

### 问题 7：并发发布冲突

**原因**：多个 chromedp 实例共享资源

**排查**：
1. 检查是否有进程锁
2. 检查 Cookie 是否按平台隔离
3. 检查 chromedp 实例是否独立

---

## 十、测试完成标准

| 标准 | 说明 |
|---|---|
| 所有平台至少成功发布 1 次 | TC-1 到 TC-6 全部通过 |
| Cookie 滚动更新正常 | TC-11 通过 |
| link 降级正常 | TC-9 通过 |
| 无严重 Bug | 无 P0/P1 问题 |
| 错误信息可读 | 所有失败场景有明确错误提示 |
