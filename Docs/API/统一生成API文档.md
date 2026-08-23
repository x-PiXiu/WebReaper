# WebReaper 统一生成 API 文档

> 版本：v1 | 基础路径：`/api/v1` | 认证：Bearer Token (JWT)

## 目录

- [认证说明](#认证说明)
- [统一提交 API](#1-统一提交-api)
- [任务查询 API](#2-任务查询-api)
- [任务列表 API](#3-任务列表-api)
- [模板列表 API](#4-模板列表-api)
- [管理后台 API](#5-管理后台-api)
- [业务场景示例](#业务场景示例)
- [错误码参考](#错误码参考)

---

## 认证说明

所有 API 需要在请求头中携带 JWT Token：

```
Authorization: Bearer <token>
```

Token 通过登录接口获取，包含 `user_id`、`username`、`role`、`tenant_id` 信息。

未认证响应：
```json
{
  "code": 40100,
  "msg": "missing authorization header"
}
```

---

## 1. 统一提交 API

### `POST /api/v1/generation/submit`

傻瓜式统一提交端点。客户端只需提供文本和素材，系统自动选择端点、厂商、模型和参数。

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `brand_id` | string | 是 | 品牌 ID |
| `text` | string | 否 | 文本描述（TTS/文生视频需要） |
| `materials` | string[] | 否 | 素材 ID 列表（从素材库选择） |
| `template` | string | 否 | 模板 ID（使用预设模板） |
| `type` | string | 否 | 生成类型：`video` / `image` / `audio` / `voice` |
| `duration` | int | 否 | 时长（秒） |
| `quality` | string | 否 | 质量/分辨率 |
| `aspect_ratio` | string | 否 | 宽高比（如 `16:9`、`9:16`） |

#### 端点自动选择规则

当 `type` 未指定时，系统根据素材自动推断：

| 素材组合 | 自动选择端点 | 说明 |
|----------|-------------|------|
| 无素材 + 文本 | `text2video` | 文生视频 |
| 1张图片 + 文本 | `img2video` | 图生视频 |
| 2张图片 | `start_end2video` | 首尾帧视频 |
| 3-7张图片 | `reference2video` | 参考生视频 |
| 1张图片 + 音频 | `digital_human` | 数字人口播 |
| 1个视频 + 音频 | `lip_sync` | 对口型 |
| 仅有文本（无素材） | `text2video` | 文生视频 |
| 仅有音频 | `tts` | 语音合成 |

#### 厂商自动路由

系统通过能力路由自动选择厂商：

| 业务类型 | 能力 ID | 默认厂商 | 说明 |
|----------|---------|----------|------|
| 视频生成 | `video` | Vidu | text2video/img2video/... |
| 图片生成 | `image` | Vidu | text2image |
| TTS 语音合成 | `tts` | 小米 MiMo | 文本转语音 |
| 声音克隆 | `voice-clone` | 小米 MiMo | 音频样本克隆音色 |
| LLM 对话 | `llm` | MiniMax | AI 文案生成 |

厂商可在管理后台动态切换，10 秒内生效，无需重启。

---

### 场景 1：TTS 语音合成

将文本转换为语音音频（使用小米 MiMo）。

#### 请求

```bash
curl -X POST http://localhost:8082/api/v1/generation/submit \
  -H "Content-Type: application/json; charset=utf-8" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "brand_id": "my-brand",
    "text": "你好世界，这是一个TTS测试。",
    "type": "audio"
  }'
```

#### 响应（成功）

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "id": "gen-1787523939427745700",
    "brand_id": "my-brand",
    "type": "audio",
    "sub_type": "tts",
    "model": "default",
    "provider": "xiaomi-mimo",
    "provider_task_id": "mimo-7536",
    "state": "success",
    "tenant_id": "tenant-001",
    "credits": 0,
    "off_peak": false,
    "watermark": false,
    "retry_count": 0,
    "err_code": "",
    "err_msg": "",
    "params": "{\"__sub_type\":\"tts\",\"text\":\"你好世界，这是一个TTS测试。\",\"voice_setting_voice_id\":\"default\"}",
    "creations": [
      {
        "id": "audio",
        "url": "data:audio/mp3;base64,//OExAAAAAAAAAAAA...",
        "cover_url": "",
        "stored_url": "",
        "watermarked_url": ""
      }
    ],
    "created_at": "2026-08-24T06:25:39.427+08:00",
    "finished_at": "2026-08-24T06:25:41.271+08:00"
  }
}
```

#### 关键字段说明

- `provider`: `"xiaomi-mimo"` — 使用小米 MiMo TTS
- `state`: `"success"` — 同步接口，提交即完成
- `creations[0].url`: `data:audio/mp3;base64,...` — 音频以 base64 Data URL 内联返回
- `params`: 记录实际使用的参数（系统自动填充默认值）

---

### 场景 2：声音克隆

使用参考音频样本克隆音色并合成语音（小米 MiMo）。

#### 请求

```bash
curl -X POST http://localhost:8082/api/v1/generation/submit \
  -H "Content-Type: application/json; charset=utf-8" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "brand_id": "my-brand",
    "text": "这是克隆音色说出的内容。",
    "type": "voice",
    "materials": ["audio-asset-id-123"]
  }'
```

> `materials` 中需包含一个音频素材 ID（10秒-5分钟，mp3/m4a/wav）

#### 参数校验失败响应

```json
{
  "code": 40000,
  "msg": "原音频 audio_url 必填（mp3/m4a/wav，10 秒-5 分钟）"
}
```

---

### 场景 3：文生视频

根据文本描述生成视频（使用 Vidu）。

#### 请求

```bash
curl -X POST http://localhost:8082/api/v1/generation/submit \
  -H "Content-Type: application/json; charset=utf-8" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "brand_id": "my-brand",
    "text": "a cute cat walking in the park",
    "type": "video"
  }'
```

#### 响应（异步任务）

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "id": "gen-1787523770282876600",
    "type": "video",
    "sub_type": "text2video",
    "model": "viduq1",
    "provider": "vidu",
    "state": "queueing",
    "provider_task_id": "",
    "creations": [],
    "params": "{\"__sub_type\":\"text2video\",\"aspect_ratio\":\"16:9\",\"prompt\":\"a cute cat walking in the park\",\"resolution\":\"1080p\"}"
  }
}
```

> 视频生成为异步任务，`state` 初始为 `queueing`，需轮询任务状态或等待回调。

---

### 场景 4：图生视频

使用图片 + 文本描述生成视频。

#### 请求

```bash
curl -X POST http://localhost:8082/api/v1/generation/submit \
  -H "Content-Type: application/json; charset=utf-8" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "brand_id": "my-brand",
    "text": "图片中的场景动起来",
    "materials": ["image-asset-id-123"],
    "type": "video"
  }'
```

---

### 场景 5：数字人口播

使用图片 + 音频生成数字人口播视频。

#### 请求

```bash
curl -X POST http://localhost:8082/api/v1/generation/submit \
  -H "Content-Type: application/json; charset=utf-8" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "brand_id": "my-brand",
    "materials": ["image-asset-id-123", "audio-asset-id-456"]
  }'
```

> 系统自动检测：1张图片 + 1个音频 → `digital_human` 端点

---

### 场景 6：对口型

使用视频 + 音频生成对口型视频。

#### 请求

```bash
curl -X POST http://localhost:8082/api/v1/generation/submit \
  -H "Content-Type: application/json; charset=utf-8" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "brand_id": "my-brand",
    "materials": ["video-asset-id-123", "audio-asset-id-456"]
  }'
```

> 系统自动检测：1个视频 + 1个音频 → `lip_sync` 端点

---

### 场景 7：使用模板

通过预设模板快速生成，模板包含默认参数。

#### 请求

```bash
curl -X POST http://localhost:8082/api/v1/generation/submit \
  -H "Content-Type: application/json; charset=utf-8" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "brand_id": "my-brand",
    "text": "品牌宣传文案",
    "template": "brand_promo",
    "materials": ["image-asset-id-123"]
  }'
```

> 模板 `brand_promo` 预设了 `duration=4`、`resolution=720p` 等参数

---

## 2. 任务查询 API

### `GET /api/v1/generation/tasks/:id`

查询单个任务的详细状态和产物。

#### 请求

```bash
curl http://localhost:8082/api/v1/generation/tasks/gen-1787523939427745700 \
  -H "Authorization: Bearer <token>"
```

#### 响应

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "id": "gen-1787523939427745700",
    "brand_id": "test",
    "type": "audio",
    "sub_type": "tts",
    "model": "default",
    "provider": "xiaomi-mimo",
    "state": "success",
    "creations": [
      {
        "id": "audio",
        "url": "data:audio/mp3;base64,..."
      }
    ],
    "created_at": "2026-08-24T06:25:39.428+08:00",
    "finished_at": "2026-08-24T06:25:41.271+08:00"
  }
}
```

#### 任务状态说明

| state | 说明 | 后续操作 |
|-------|------|----------|
| `created` | 已创建 | 等待提交 |
| `queueing` | 排队中 | 轮询等待 |
| `success` | 成功 | 读取 `creations` 获取产物 |
| `failed` | 失败 | 读取 `err_msg` 查看原因 |
| `cancelled` | 已取消 | — |

---

## 3. 任务列表 API

### `GET /api/v1/generation/tasks`

查询当前租户的任务列表（按创建时间倒序）。

#### 请求

```bash
curl http://localhost:8082/api/v1/generation/tasks \
  -H "Authorization: Bearer <token>"
```

#### 响应

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "tasks": [
      {
        "id": "gen-1787523939427745700",
        "type": "audio",
        "sub_type": "tts",
        "provider": "xiaomi-mimo",
        "state": "success",
        "creations": [{ "id": "audio", "url": "data:audio/mp3;base64,..." }]
      },
      {
        "id": "gen-1787523770282876600",
        "type": "video",
        "sub_type": "text2video",
        "provider": "vidu",
        "state": "failed",
        "err_msg": "积分不足",
        "creations": []
      }
    ]
  }
}
```

---

## 4. 模板列表 API

### `GET /api/v1/generation/templates`

查询当前租户可用的生成模板（全局模板 + 租户私有模板）。

#### 请求

```bash
curl http://localhost:8082/api/v1/generation/templates \
  -H "Authorization: Bearer <token>"
```

#### 响应

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "templates": [
      {
        "id": "brand_promo",
        "tenant_id": "",
        "name": "品牌宣传视频",
        "description": "4秒品牌Logo动画视频，适合社交媒体宣传",
        "icon": "🎬",
        "sub_type": "img2video",
        "default_params": {
          "duration": 4,
          "resolution": "720p"
        },
        "required_materials": ["image"],
        "optional_materials": [],
        "sort_order": 1,
        "enabled": true
      },
      {
        "id": "product_intro",
        "name": "产品介绍视频",
        "description": "8秒产品展示视频，详细展示产品特点",
        "icon": "📦",
        "sub_type": "text2video",
        "default_params": { "duration": 8, "resolution": "720p" },
        "required_materials": [],
        "optional_materials": ["image"]
      },
      {
        "id": "tts_narration",
        "name": "语音旁白",
        "description": "文本转语音，适合旁白配音",
        "icon": "🎤",
        "sub_type": "tts",
        "default_params": {},
        "required_materials": [],
        "optional_materials": []
      }
    ]
  }
}
```

### `GET /api/v1/generation/templates/:id`

查询单个模板详情。

#### 请求

```bash
curl http://localhost:8082/api/v1/generation/templates/brand_promo \
  -H "Authorization: Bearer <token>"
```

---

## 5. 管理后台 API

管理后台 API 需要 `admin` 角色的 Token。

### 5.1 模板管理

#### 列表 `GET /api/v1/admin/templates`

```bash
curl http://localhost:8082/api/v1/admin/templates \
  -H "Authorization: Bearer <admin-token>"
```

#### 创建 `POST /api/v1/admin/templates`

```bash
curl -X POST http://localhost:8082/api/v1/admin/templates \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "id": "custom_template",
    "name": "自定义模板",
    "description": "自定义生成模板",
    "icon": "✨",
    "sub_type": "text2video",
    "default_params": { "duration": 6, "resolution": "1080p" },
    "required_materials": [],
    "optional_materials": ["image"],
    "sort_order": 10
  }'
```

#### 更新 `PUT /api/v1/admin/templates/:id`

```bash
curl -X PUT http://localhost:8082/api/v1/admin/templates/custom_template \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "name": "更新后的模板名",
    "enabled": false
  }'
```

#### 删除 `DELETE /api/v1/admin/templates/:id`

```bash
curl -X DELETE http://localhost:8082/api/v1/admin/templates/custom_template \
  -H "Authorization: Bearer <admin-token>"
```

### 5.2 模型配置管理

#### 列表 `GET /api/v1/admin/generation/specs`

查询所有端点/模型规格配置。

#### 更新 `PUT /api/v1/admin/generation/specs`

更新端点/模型规格（启用/禁用、设置默认模型等）。

### 5.3 厂商配置管理

#### 列表 `GET /api/v1/admin/providers`

查询所有厂商配置（API Key、端点等）。

#### 更新 `PUT /api/v1/admin/providers/:provider`

更新厂商配置（API Key 热更新，10 秒内生效）。

---

## 业务场景示例

### 完整流程：从上传素材到生成音频

```
步骤 1: 上传素材
POST /api/v1/media/upload (multipart/form-data)
→ 返回 asset_id

步骤 2: 提交生成任务
POST /api/v1/generation/submit
{
  "brand_id": "my-brand",
  "text": "欢迎来到我们的品牌世界。",
  "type": "audio"
}
→ 返回 task_id, state=success, creations[0].url (base64 音频)

步骤 3: 获取音频
creations[0].url 是 data:audio/mp3;base64,... 格式
可直接在 <audio> 标签中播放：
<audio src="data:audio/mp3;base64,..." controls></audio>
```

### 完整流程：异步视频生成

```
步骤 1: 提交任务
POST /api/v1/generation/submit
{ "brand_id": "my-brand", "text": "一只猫在公园散步", "type": "video" }
→ state=queueing

步骤 2: 轮询状态（每 20 秒）
GET /api/v1/generation/tasks/:id
→ state 仍为 queueing → 继续轮询

步骤 3: 任务完成
GET /api/v1/generation/tasks/:id
→ state=success, creations[0].url = "https://..."

步骤 4: 下载产物
GET creations[0].url
→ 视频文件
```

---

## 错误码参考

| 错误码 | HTTP 状态码 | 说明 |
|--------|------------|------|
| `0` | 200 | 成功 |
| `40000` | 400 | 参数校验失败 |
| `40100` | 401 | 缺少 Authorization 头 |
| `40101` | 401 | Authorization 格式错误 |
| `40102` | 401 | Token 无效或过期 |
| `40103` | 401 | 租户隔离升级，请重新登录 |
| `40200` | 402 | 配额超限（需充值） |
| `40300` | 403 | 权限不足（角色不符） |
| `40400` | 404 | 资源不存在 |
| `50000` | 500 | 服务器内部错误 |

### 常见业务错误

| 场景 | 错误信息 | 处理方式 |
|------|----------|----------|
| TTS 文本为空 | `text参数为空` | 检查请求体 text 字段 |
| 声音克隆缺音频 | `原音频 audio_url 必填` | 在 materials 中提供音频 ID |
| Vidu 积分不足 | `积分不足，请充值后重试` | 联系管理员充值 |
| 提示词过长 | `提示词超过 N 字符上限` | 缩短文本描述 |
| 重复提交 | 返回已有任务（幂等） | 直接使用返回的任务 ID |
