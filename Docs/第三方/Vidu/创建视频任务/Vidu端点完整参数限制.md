# Vidu 各端点完整参数限制

> **日期**：2026-08-08
> **范围**：Vidu 全部 5 个视频生成端点
> **用途**：前后端参数校验、数据库配置参考

---

## 一、文生视频 `/ent/v2/text2video`

| 参数 | 类型 | 必填 | viduq3系列 | viduq2系列 | viduq1/q1-classic | vidu2.0 |
|------|------|------|-----------|-----------|-------------------|---------|
| model | String | ✅ | viduq3-turbo/viduq3-pro | viduq2 | viduq1/viduq1-classic | vidu2.0 |
| prompt | String | ✅ | ≤5000字符 | ≤5000字符 | ≤5000字符 | ≤5000字符 |
| duration | Int | 可选 | 1-16秒，默认5 | 1-10秒，默认5 | 5秒 | 4/8秒 |
| resolution | String | 可选 | 540p/720p/1080p | 540p/720p/1080p | 1080p | 360p/720p/1080p(4s), 720p(8s) |
| seed | Int | 可选 | 随机 | 随机 | 随机 | 随机 |
| audio | Bool | 可选 | 默认true | 默认false | 默认false | 默认false |
| audio_type | String | 可选 | all/speech_only/sound_effect_only | 同左 | 同左 | 同左 |
| bgm | Bool | 可选 | 不生效 | 9/10秒时不生效 | - | - |
| callback_url | String | 可选 | 回调地址 | 同左 | 同左 | 同左 |

---

## 二、图生视频 `/ent/v2/img2video`

| 参数 | 类型 | 必填 | viduq3系列 | viduq2系列 | viduq1/q1-classic | vidu2.0 |
|------|------|------|-----------|-----------|-------------------|---------|
| model | String | ✅ | viduq3-turbo/viduq3-pro/viduq3-pro-fast | viduq2-pro/viduq2-pro-fast/viduq2-turbo | viduq1/viduq1-classic | vidu2.0 |
| images | Array[String] | ✅ | **1张** | **1张** | **1张** | **1张** |
| prompt | String | 可选 | ≤5000字符 | ≤5000字符 | ≤5000字符 | ≤5000字符 |
| duration | Int | 可选 | 1-16秒，默认5 | 1-10秒，默认5 | 5秒 | 4/8秒 |
| resolution | String | 可选 | 540p/720p/1080p | 540p/720p/1080p | 1080p | 360p/720p/1080p(4s), 720p(8s) |
| seed | Int | 可选 | 随机 | 随机 | 随机 | 随机 |
| audio | Bool | 可选 | 默认true | 默认false | 默认false | 默认false |
| audio_type | String | 可选 | all/speech_only/sound_effect_only | 同左 | 同左 | 同左 |
| bgm | Bool | 可选 | 不生效 | 9/10秒时不生效 | - | - |
| movement_amplitude | String | 可选 | 不生效 | 不生效 | auto/small/medium/large | auto/small/medium/large |
| callback_url | String | 可选 | 回调地址 | 同左 | 同左 | 同左 |

**图片限制**：
- 格式：png, jpeg, jpg, webp
- 大小：≤50MB
- 比例：1:4 到 4:1
- HTTP POST body：≤20MB

---

## 三、首尾帧 `/ent/v2/start-end2video`

| 参数 | 类型 | 必填 | viduq3系列 | viduq2系列 | viduq1/q1-classic | vidu2.0 |
|------|------|------|-----------|-----------|-------------------|---------|
| model | String | ✅ | viduq3-turbo/viduq3-pro | viduq2-pro/viduq2-pro-fast/viduq2-turbo | viduq1/viduq1-classic | vidu2.0 |
| images | Array[String] | ✅ | **2张**（首帧+尾帧） | **2张** | **2张** | **2张** |
| prompt | String | 可选 | ≤5000字符 | ≤5000字符 | ≤5000字符 | ≤5000字符 |
| duration | Int | 可选 | 1-16秒，默认5 | 1-8秒，默认5 | 5秒 | 4/8秒 |
| resolution | String | 可选 | 540p/720p/1080p | 540p/720p/1080p | 1080p | 360p/720p/1080p(4s), 720p(8s) |
| seed | Int | 可选 | 随机 | 随机 | 随机 | 随机 |
| audio | Bool | 可选 | 默认true | 默认false | 默认false | 默认false |
| bgm | Bool | 可选 | 不生效 | 9/10秒时不生效 | - | - |
| movement_amplitude | String | 可选 | 不生效 | 不生效 | auto/small/medium/large | auto/small/medium/large |
| callback_url | String | 可选 | 回调地址 | 同左 | 同左 | 同左 |

**图片限制**：
- 格式：png, jpeg, jpg, webp
- 大小：≤50MB
- 比例：1:4 到 4:1
- **首尾帧分辨率比**：0.8-1.25
- HTTP POST body：≤20MB

---

## 四、参考生视频 `/ent/v2/reference2video`

### 4.1 主体调用模式

| 参数 | 类型 | 必填 | viduq3系列 | viduq2-pro | viduq2/viduq1/vidu2.0 |
|------|------|------|-----------|------------|----------------------|
| model | String | ✅ | viduq3-turbo/viduq3 | viduq2-pro | viduq2/viduq1/vidu2.0 |
| subjects | Array | ✅ | 图片主体+文字主体 | 图片/视频/文字主体 | 图片主体+文字主体 |
| subjects[].name | String | ✅ | 主体ID | 同左 | 同左 |
| subjects[].images | Array[String] | 可选 | **≤3张** | **≤3张**（与videos共享槽位） | **≤3张** |
| subjects[].videos | Array[String] | 可选 | ❌ | **≤1个5秒视频** | ❌ |
| subjects[].voice_id | String | 可选 | 不生效 | 音色ID | 音色ID |
| subjects[].server_id | String | 可选 | 已有主体ID | 同左 | 同左 |
| prompt | String | ✅ | ≤5000字符，@主体name引用 | 同左 | 同左 |
| duration | Int | 可选 | **3-16秒**，默认5 | 1-10秒，默认5 | 5秒(vidu2.0: 4秒) |
| resolution | String | 可选 | 540p/720p/1080p | 540p/720p/1080p | 1080p(q1)/360p/720p(2.0) |
| aspect_ratio | String | 可选 | 16:9/9:16/1:1 | 16:9/9:16/1:1 | 同左 |
| seed | Int | 可选 | 随机 | 随机 | 随机 |
| audio | Bool | 可选 | 默认true | 默认false | 默认false |
| audio_type | String | 可选 | all/speech_only/sound_effect_only | 同左 | 同左 |
| callback_url | String | 可选 | 回调地址 | 同左 | 同左 |

### 4.2 非主体调用模式

| 参数 | 类型 | 必填 | viduq3系列 | viduq2-pro | viduq2 | viduq1/vidu2.0 |
|------|------|------|-----------|------------|--------|----------------|
| model | String | ✅ | viduq3-turbo/viduq3/viduq3-mix | viduq2-pro | viduq2 | viduq1/vidu2.0 |
| images | Array[String] | ✅ | **1-7张** | **1-4张**（有视频时）/1-7张 | **1-7张** | **1-7张** |
| videos | Array[String] | 可选 | ❌ | **1-2个**（1个8秒或2个5秒） | ❌ | ❌ |
| prompt | String | ✅ | ≤2000字符 | ≤2000字符 | ≤2000字符 | ≤2000字符 |
| duration | Int | 可选 | **3-16秒**，默认5 | 0-10秒（0=自动），默认5 | 1-10秒 | 5秒(vidu2.0: 4秒) |
| resolution | String | 可选 | 540p/720p/1080p | 540p/720p/1080p | 540p/720p/1080p | 1080p(q1)/360p/720p(2.0) |
| aspect_ratio | String | 可选 | 16:9/9:16/4:3/3:4/1:1 | 16:9/9:16/4:3/3:4/1:1 | 任意比例 | 16:9/9:16/1:1 |
| seed | Int | 可选 | 随机 | 随机 | 随机 | 随机 |
| audio | Bool | 可选 | 默认true(q3)/false(mix) | 默认false | 默认false | 默认false |
| audio_type | String | 可选 | all/speech_only/sound_effect_only | 同左 | 同左 | 同左 |
| movement_amplitude | String | 可选 | 不生效 | 不生效 | auto/small/medium/large | auto/small/medium/large |
| callback_url | String | 可选 | 回调地址 | 同左 | 同左 | 同左 |

**图片限制**：
- 格式：png, jpeg, jpg, webp
- 大小：≤50MB
- 比例：1:4 到 4:1
- 像素：≥128×128

**视频限制**（仅q2-pro）：
- 格式：mp4, avi, mov
- 大小：≤100MB
- 比例：1:4 到 4:1
- 像素：≥128×128
- 时长：≤5秒（单个）或 ≤8秒（单个）或 2×5秒

---

## 五、智能多帧 `/ent/v2/multiframe`

| 参数 | 类型 | 必填 | viduq2-pro/viduq2-turbo |
|------|------|------|------------------------|
| model | String | ✅ | viduq2-pro/viduq2-turbo |
| start_image | String | ✅ | **1张**首帧图 |
| image_settings | Array | ✅ | **2-9个**关键帧配置 |
| image_settings[].key_image | String | ✅ | 关键帧图片 |
| image_settings[].prompt | String | 可选 | 关键帧提示词 |
| image_settings[].duration | Int | 可选 | **2-7秒**，默认5 |
| resolution | String | 可选 | 540p/720p/1080p，默认720p |
| aspect_ratio | String | 可选 | 16:9/9:16/4:3/3:4/1:1 |
| seed | Int | 可选 | 随机 |
| callback_url | String | 可选 | 回调地址 |

**图片限制**：
- 格式：png, jpeg, jpg, webp
- 大小：≤50MB
- 比例：1:4 到 4:1
- HTTP POST body：≤10MB（start_image）/ ≤20MB（image_settings）

---

## 六、模型-端点支持矩阵

| 模型 | text2video | img2video | startEnd2video | reference2video | multiframe |
|------|:----------:|:---------:|:--------------:|:---------------:|:----------:|
| vidu2.0 | ❌ | ✅ | ✅ | ✅ | ❌ |
| viduq1 | ✅ | ✅ | ✅ | ✅ | ❌ |
| viduq1-classic | ❌ | ✅ | ✅ | ❌ | ❌ |
| viduq2 | ✅ | ❌ | ❌ | ✅ | ❌ |
| viduq2-pro | ❌ | ✅ | ✅ | ✅ | ✅ |
| viduq2-pro-fast | ❌ | ✅ | ✅ | ❌ | ❌ |
| viduq2-turbo | ❌ | ✅ | ✅ | ❌ | ✅ |
| viduq3-pro | ✅ | ✅ | ✅ | ✅ | ✅ |
| viduq3-turbo | ✅ | ✅ | ✅ | ✅ | ✅ |
| viduq3-pro-fast | ❌ | ✅ | ❌ | ❌ | ❌ |
| viduq3-mix | ❌ | ❌ | ❌ | ✅ | ❌ |

---

## 七、关键差异总结

| 差异点 | 说明 |
|--------|------|
| **duration 范围** | 参考生视频：q3系列 3-16秒，其他 1-10秒或固定值 |
| **图片数量** | 参考生视频：q3系列 1-7张，q2-pro+视频时 1-4张 |
| **视频参考** | 仅 viduq2-pro 支持，最多 2个视频 |
| **分辨率选项** | q1系列固定 1080p，2.0 有 360p，其他 540p/720p/1080p |
| **音频默认值** | q3系列默认 true，其他默认 false |
| **运动幅度** | q2/q3系列不生效，q1/2.0 生效 |

---

## 八、数据库配置参考

### parameter_schema_json 结构

```json
{
  "common": {
    "parameters": {
      "seed": { "key": "seed", "type": "INTEGER", ... },
      "callbackUrl": { "key": "callback_url", "type": "STRING", ... }
    }
  },
  "endpoints": {
    "img2video": {
      "parameters": {
        "duration": { "minValue": 1, "maxValue": 16, "defaultValue": 5, ... },
        "resolution": { "enumValues": ["540p", "720p", "1080p"], ... },
        "aspectRatio": { "enumValues": ["16:9", "9:16", "1:1"], ... }
      }
    },
    "startEnd2video": { ... },
    "reference2video": { ... },
    "multiFrame2video": { ... }
  }
}
```

### endpoints_json 结构

```json
{
  "text2video": "/ent/v2/text2video",
  "img2video": "/ent/v2/img2video",
  "startEnd2video": "/ent/v2/start-end2video",
  "reference2video": "/ent/v2/reference2video",
  "multiFrame2video": "/ent/v2/multi-frame2video"
}
```
