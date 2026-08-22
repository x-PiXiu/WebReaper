# 小米 MiMo 开放平台

> **状态**：已接入（ASR 已验证通过，TTS 待集成）
> **协议**：OpenAI 兼容（`/v1/chat/completions`）+ Anthropic 兼容（`/anthropic`）
> **专属端点**：`https://token-plan-cn.xiaomimimo.com/v1`（OpenAI）/ `https://token-plan-cn.xiaomimimo.com/anthropic`（Anthropic）
> **额度**：11,000,000,000 Credits（非高峰 00:00-08:00 北京时间 0.8x 系数消耗）
> **TTS 系列限时免费**（mimo-v2.5-tts / voicedesign / voiceclone）

---

## 一、概述

小米 MiMo 提供三种音频能力，共用同一个 OpenAI 兼容端点，通过 **model 名**区分：

| 能力 | 模型 | 方向 | 说明 |
|---|---|---|---|
| 语音合成（TTS） | `mimo-v2.5-tts` | 文本 → 音频 | 9 种预置音色，返回 base64 音频 |
| 音色设计 TTS | `mimo-v2.5-tts-voicedesign` | 描述 → 音频 | 自然语言描述音色风格，自动生成播报文本 |
| 声音克隆 TTS | `mimo-v2.5-tts-voiceclone` | 音频样本 + 文本 → 音频 | 传入 base64 音频样本复刻音色 |
| 语音识别（ASR） | `mimo-v2.5-asr` | 音频 → 文本 | 识别音频内容返回文字 |

**关键设计**：所有能力共用 `/v1/chat/completions` 端点——与本项目的 `asropenai.Transcriber` 和 LLM 配置体系完全兼容，切换模型即可切换能力，零代码改动。

---

## 二、列出可用模型

```
GET https://token-plan-cn.xiaomimimo.com/v1/models
```

请求头：`api-key: $MIMO_API_KEY`

### 响应

```json
{
  "object": "list",
  "data": [
    { "id": "mimo-v2.5",              "object": "model", "owned_by": "xiaomi" },
    { "id": "mimo-v2.5-asr",          "object": "model", "owned_by": "xiaomi" },
    { "id": "mimo-v2.5-pro",          "object": "model", "owned_by": "xiaomi" },
    { "id": "mimo-v2.5-tts",          "object": "model", "owned_by": "xiaomi" },
    { "id": "mimo-v2.5-tts-voiceclone", "object": "model", "owned_by": "xiaomi" },
    { "id": "mimo-v2.5-tts-voicedesign", "object": "model", "owned_by": "xiaomi" }
  ]
}
```

### 可用模型清单

| 模型 ID | 能力 | 说明 |
|---|---|---|
| `mimo-v2.5` | LLM 文本对话 | 基础文本模型 |
| `mimo-v2.5-pro` | LLM 文本对话 | 高级文本模型 |
| `mimo-v2.5-asr` | 语音识别（ASR） | 音频→文本（mp3/wav） |
| `mimo-v2.5-tts` | 语音合成（TTS） | 文本→音频，9 种预置音色 |
| `mimo-v2.5-tts-voicedesign` | 音色设计 TTS | 自然语言描述音色风格→音频 |
| `mimo-v2.5-tts-voiceclone` | 声音克隆 TTS | 音频样本+文本→克隆音色音频 |

> 所有模型共用同一个端点 `POST /v1/chat/completions`（OpenAI 兼容），通过 `model` 字段区分能力。

---

## 三、认证

请求头（两种方式任选其一）：

```
api-key: $MIMO_API_KEY
Content-Type: application/json
```

> 注意：小米用 `api-key` 头，而非标准 OpenAI 的 `Authorization: Bearer`。本项目的 `asropenai.Transcriber` 使用 `Authorization: Bearer` 格式——接入小米时需在 ASR 配置的 extra_json 中标记 `auth_style: "api-key"`，或在 adapter 层适配。

---

## 四、语音合成（MiMo-TTS）

### 请求

```json
POST https://token-plan-cn.xiaomimimo.com/v1/chat/completions
{
  "model": "mimo-v2.5-tts",
  "messages": [
    { "role": "user", "content": "你好，欢迎使用小米语音服务" },
    { "role": "assistant", "content": "要合成的文本内容放在这里" }
  ],
  "audio": {
    "format": "wav",
    "voice": "mimo_default"
  }
}
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `model` | string | 是 | `mimo-v2.5-tts` / `mimo-v2.5-tts-voicedesign` / `mimo-v2.5-tts-voiceclone` |
| `messages` | array | 是 | 对话消息列表 |
| `messages[].role` | string | 是 | `user`（音色描述，voicedesign 模型必填）/ `assistant`（合成文本） |
| `messages[].content` | string | 是 | 消息内容 |
| `audio.format` | string | 否 | 输出格式：`wav`（默认）/ `mp3` / `pcm` / `pcm16` |
| `audio.voice` | string | 看模型 | 预置音色 ID 或音频样本 base64（见下方音色表） |
| `audio.optimize_text_preview` | boolean | 否 | 是否智能润色播报文本（仅 voicedesign 模型，默认 false） |
| `stream` | boolean | 否 | 流式输出（SSE），默认 false |

### 预置音色（mimo-v2.5-tts 模型）

| 音色 ID | 风格 |
|---|---|
| `mimo_default` | 默认音色 |
| `冰糖` | — |
| `茉莉` | — |
| `苏打` | — |
| `白桦` | — |
| `Mia` | — |
| `Chloe` | — |
| `Milo` | — |
| `Dean` | — |

### 三种模型差异

| 模型 | audio.voice | 特殊行为 |
|---|---|---|
| `mimo-v2.5-tts` | 可选，仅预置音色 ID，默认 `mimo_default` | 标准 TTS |
| `mimo-v2.5-tts-voicedesign` | 不支持该字段 | user 消息为音色描述；`optimize_text_preview=true` 可省略 assistant 消息 |
| `mimo-v2.5-tts-voiceclone` | **必填**，传入音频样本 base64（mp3/wav） | 声音克隆——用样本音色合成文本 |

### 响应（非流式）

```json
{
  "id": "6ebed286b58546f6b87fa7fa9d0e806b",
  "model": "mimo-v2.5-tts",
  "object": "chat.completion",
  "choices": [{
    "index": 0,
    "finish_reason": "stop",
    "message": {
      "role": "assistant",
      "content": "",
      "audio": {
        "id": "979a91904f9a4143928d9e1f54837b4f",
        "data": "<base64 编码音频>",
        "expires_at": null,
        "transcript": null
      }
    }
  }],
  "usage": {
    "prompt_tokens": 213,
    "completion_tokens": 97,
    "total_tokens": 310
  }
}
```

| 字段 | 说明 |
|---|---|
| `choices[0].message.audio.data` | base64 编码的音频文件（格式由 `audio.format` 决定） |
| `choices[0].message.audio.id` | 音频响应唯一标识 |
| `choices[0].final_text_preview` | 智能润色后的播报文本（仅 `optimize_text_preview=true` 时返回） |

---

## 五、语音识别（MiMo-V2.5-ASR）

### 请求

```bash
curl --location --request POST 'https://token-plan-cn.xiaomimimo.com/v1/chat/completions' \
--header "api-key: $MIMO_API_KEY" \
--header 'Content-Type: application/json' \
--data-raw '{
    "model": "mimo-v2.5-asr",
    "messages": [
        {
            "role": "user",
            "content": [
                {
                    "type": "input_audio",
                    "input_audio": {
                        "data": "data:audio/mpeg;base64,$BASE64_AUDIO"
                    }
                }
            ]
        }
    ],
    "asr_options": {
        "language": "auto"
    }
}'
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `model` | string | 是 | `mimo-v2.5-asr` |
| `messages[0].role` | string | 是 | 固定 `user` |
| `messages[0].content` | array | 是 | 多模态内容数组（仅支持单条音频输入） |
| `content[].type` | string | 是 | 固定 `input_audio` |
| `content[].input_audio.data` | string | 是 | base64 编码音频。支持两种格式：① data URL（`data:{MIME_TYPE};base64,...`，此时 format 可省略）② 纯 base64（此时 format 必填） |
| `content[].input_audio.format` | 看情况 | 纯 base64 时必填 | 音频格式：`mp3` / `wav`。使用 data URL 时可省略（MIME_TYPE 即格式） |
| `asr_options` | object | 否 | ASR 自定义配置 |
| `asr_options.language` | string | 否 | 语种：`auto`（默认，自动检测）/ `zh`（中文）/ `en`（英文）。仅支持单一语种 |
| `stream` | boolean | 否 | 流式输出（SSE），默认 false |

**支持的音频格式**：仅 `mp3`（MIME: `audio/mpeg` 或 `audio/mp3`）和 `wav`（MIME: `audio/wav`）。**不支持** m4a/flac/ogg——如需使用这些格式，先用 ffmpeg 转换为 mp3/wav。

### 响应（非流式）

```json
{
  "id": "9f51eba459dd4dfdabb31cabba0cb7dc",
  "model": "mimo-v2.5-asr",
  "object": "chat.completion",
  "choices": [{
    "index": 0,
    "finish_reason": "stop",
    "message": {
      "role": "assistant",
      "content": "Good morning. Could you tell me what the weather will be like today?"
    }
  }],
  "usage": {
    "prompt_tokens": 46,
    "completion_tokens": 20,
    "total_tokens": 66,
    "prompt_tokens_details": {
      "audio_tokens": 25,
      "cached_tokens": 45
    },
    "seconds": 4
  }
}
```

| 字段 | 说明 |
|---|---|
| `choices[0].message.content` | 识别出的文本内容 |
| `usage.seconds` | 输入音频时长（秒）——可用于计费/监控 |
| `usage.prompt_tokens_details.audio_tokens` | 音频输入消耗的 token 数 |
    }
  }],
  "usage": {
    "prompt_tokens": 55,
    "completion_tokens": 21,
    "total_tokens": 76,
    "prompt_tokens_details": { "audio_tokens": 34 }
  }
}
```

| 字段 | 说明 |
|---|---|
| `choices[0].message.content` | 识别出的文本内容 |

### 实测结果（2026-08-23）

| 测试 | 输入 | 输出 | 耗时 | 结果 |
|---|---|---|---|---|
| Edge-TTS 中文语音 | "你好，欢迎使用小米语音识别服务。这是一段测试音频。" | "你好，欢迎使用小米语音识别服务。这是一段测试音频。" | ~6s | ✅ 100% 准确 |
| 正弦波测试音频 | 440Hz sine 5秒 | "嗯。" | ~6s | ✅ 正确识别为非语音 |

---

## 六、与本项目的集成点

| 集成方式 | 说明 |
|---|---|
| **ASR**（已接入，协议已适配） | `asropenai.Transcriber` 通过 `Protocol: "openai-chat"` 分支自动切换为 JSON+base64 模式（小米），管理后台配置 provider=asr + vendor=xiaomi-mimo 即可 |
| **TTS**（待接入） | 可作为 Vidu TTS 的备选——小米 TTS 返回 base64 音频（Vidu TTS 返回任务 ID 需轮询），延迟更低；需新增 adapter 或扩展 `ttsAdapter` |
| **声音克隆**（待评估） | `mimo-v2.5-tts-voiceclone` 传入音频样本 base64 即可复刻——与 Vidu 声音克隆（需上传音频 URL）路径不同，可作为轻量替代 |
| **LLM**（可用） | `mimo-v2.5-pro` 等文本模型通过 `/v1/chat/completions` 调用，与现有 LLM 配置体系完全兼容 |

### 配置示例（管理后台）

```sql
-- ASR 配置
INSERT INTO provider_configs (provider, api_key, base_url, enabled, extra_json)
VALUES ('asr', 'tp-c55w...', 'https://token-plan-cn.xiaomimimo.com/v1/chat/completions', 1,
  '{"model":"mimo-v2.5-asr","response_style":"chat"}');

-- LLM 配置（可选——小米作为备选 LLM）
INSERT INTO llm_configs (name, provider, api_key, base_url, model, is_default)
VALUES ('xiaomi-mimo', '小米', 'tp-c55w...', 'https://token-plan-cn.xiaomimimo.com/v1', 'mimo-v2.5-pro', 0);
```

---

## 七、已知限制与注意事项

1. **认证头差异**：小米用 `api-key` 头而非 `Authorization: Bearer`——adapter 已按协议分支处理（`openai-chat` 协议用 `api-key`，`openai` 协议用 `Authorization: Bearer`）
2. **ASR 输入格式**：小米 ASR 用 `chat/completions` + `input_audio`（多模态），而非标准 OpenAI 的 `/audio/transcriptions`——adapter 通过 `Protocol: "openai-chat"` 分支处理
3. **ASR 音频格式限制**：仅支持 **mp3** 和 **wav** 两种格式（不支持 m4a/flac/ogg）——ffmpeg 抽音轨时需输出 mp3（`-codec:a libmp3lame`）而非 m4a
4. **ASR data URL 格式**：`input_audio.data` 支持两种传入方式：① data URL（`data:audio/mpeg;base64,...`，此时 `format` 可省略）② 纯 base64（此时 `format` 必填）。adapter 使用 data URL 方式
5. **ASR 语种控制**：`asr_options.language` 可指定 `auto`/`zh`/`en`，默认 auto 自动检测
6. **TTS 返回 base64**：音频直接在响应体中（`choices[0].message.audio.data`），无需轮询——比 Vidu TTS 延迟更低
7. **TTS 音频格式**：默认 wav，可选 mp3/pcm/pcm16
8. **流式输出**：TTS 和 ASR 都支持 SSE 流式（`stream: true`），适合实时播报/转写场景
9. **音色设计**：`voicedesign` 模型用自然语言描述音色风格（如 "Bright, bouncy, slightly sing-song tone"），适合个性化场景
10. **额度与计费**：总额度 11,000,000,000 Credits；**非高峰时段**（北京时间 00:00-08:00）0.8x 系数消耗；**TTS 系列限时免费**
11. **专属端点**：token-plan 用户使用 `https://token-plan-cn.xiaomimimo.com/v1`（OpenAI 兼容）或 `/anthropic`（Anthropic 兼容），非通用 `api.xiaomimimo.com`
