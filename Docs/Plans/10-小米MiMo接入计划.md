# 小米MiMo接入计划

> 基于获客智能体统一生成架构方案，对接小米MiMo的LLM、TTS、声音克隆能力

## 一、概述

### 1.1 背景

根据获客智能体统一生成架构方案，系统需要支持多厂商能力路由。小米MiMo提供以下能力：

| 能力 | 模型 | 方向 | 状态 |
|------|------|------|------|
| LLM文本对话 | `mimo-v2.5` / `mimo-v2.5-pro` | 文本→文本 | 待接入 |
| 语音识别（ASR） | `mimo-v2.5-asr` | 音频→文本 | ✅ 已接入 |
| 语音合成（TTS） | `mimo-v2.5-tts` | 文本→音频 | 待接入 |
| 音色设计TTS | `mimo-v2.5-tts-voicedesign` | 描述→音频 | 待接入 |
| 声音克隆TTS | `mimo-v2.5-tts-voiceclone` | 音频+文本→音频 | 待接入 |

### 1.2 设计原则

根据架构方案的设计原则：
1. **适配器扩展**：新增厂商 = 新增适配器，不改已有逻辑
2. **能力路由**：管理后台配置默认模型，用户无感
3. **统一接口**：通过Port层接口隔离，用例层不感知厂商差异

---

## 二、架构设计

### 2.1 分层架构

```
┌─────────────────────────────────────────────────────────────┐
│                    Usecase 层                               │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │ Generation  │ │   Agent     │ │   Video     │           │
│  │   UseCase   │ │ Orchestrator│ │ Transcript  │           │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
├─────────────────────────────────────────────────────────────┤
│                    Port 层（接口定义）                       │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │ AIGenerator │ │AudioSynth.  │ │SpeechTransc.│           │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
├─────────────────────────────────────────────────────────────┤
│                    Adapter 层                               │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │ MiMo LLM   │ │ MiMo TTS   │ │ MiMo ASR    │           │
│  │  Adapter    │ │  Adapter    │ │  Adapter    │           │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
├─────────────────────────────────────────────────────────────┤
│                    小米MiMo API                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  /v1/chat/completions（OpenAI兼容）                  │    │
│  │  - mimo-v2.5 / mimo-v2.5-pro（LLM）                │    │
│  │  - mimo-v2.5-tts（TTS）                            │    │
│  │  - mimo-v2.5-tts-voiceclone（声音克隆）             │    │
│  │  - mimo-v2.5-asr（ASR）                            │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 集成点分析

| 集成点 | Port接口 | 说明 | 优先级 |
|--------|----------|------|--------|
| LLM | `AIGenerator` | 作为Vidu的备选LLM | 高 |
| TTS | `AudioSynthesizer` | 作为Vidu TTS的备选（延迟更低） | 高 |
| 声音克隆 | `AudioSynthesizer` | 轻量替代方案 | 中 |
| ASR | `SpeechTranscriber` | 已接入 | ✅ 完成 |

---

## 三、实施计划

### 阶段1：LLM接入

#### 3.1.1 任务清单

| 任务 | 说明 |
|------|------|
| 1.1 | 创建MiMo LLM适配器 |
| 1.2 | 配置能力路由 |
| 1.3 | 管理后台配置 |
| 1.4 | 测试验证 |

#### 3.1.2 MiMo LLM适配器

```go
// adapter/ai/mimo_llm.go
package ai

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    
    "webreaper/internal/usecase/port"
)

// MiMoLLMAdapter 小米MiMo LLM适配器。
//
// 设计动机：
//   - 小米MiMo使用OpenAI兼容的 /v1/chat/completions 端点
//   - 认证头使用 api-key 而非 Authorization: Bearer
//   - 支持LLM文本对话
type MiMoLLMAdapter struct {
    apiKey  string
    baseURL string
    client  *http.Client
}

var _ port.AIGenerator = (*MiMoLLMAdapter)(nil)

func NewMiMoLLMAdapter(apiKey, baseURL string) *MiMoLLMAdapter {
    return &MiMoLLMAdapter{
        apiKey:  apiKey,
        baseURL: baseURL,
        client:  &http.Client{Timeout: 60 * time.Second},
    }
}

func (a *MiMoLLMAdapter) ChatStream(ctx context.Context, conversationID, llmConfigName string, messages []port.ChatMessage, onDelta func(delta string)) (string, error) {
    // 构建请求
    req := map[string]any{
        "model":    "mimo-v2.5-pro",
        "messages": a.convertMessages(messages),
        "stream":   true,
    }
    
    // 发送请求
    resp, err := a.sendRequest(ctx, req)
    if err != nil {
        return "", err
    }
    
    // 处理流式响应
    return a.handleStreamResponse(resp, onDelta)
}

func (a *MiMoLLMAdapter) RunWithTools(ctx context.Context, conversationID, llmConfigName, task, systemPrompt string, tools []string, onEvent func(port.ToolEvent)) error {
    // 构建请求
    messages := []map[string]string{
        {"role": "system", "content": systemPrompt},
        {"role": "user", "content": task},
    }
    
    req := map[string]any{
        "model":    "mimo-v2.5-pro",
        "messages": messages,
    }
    
    // 发送请求
    resp, err := a.sendRequest(ctx, req)
    if err != nil {
        return err
    }
    
    // 处理响应
    return a.handleResponse(resp, onEvent)
}

func (a *MiMoLLMAdapter) sendRequest(ctx context.Context, req any) (*http.Response, error) {
    payload, _ := json.Marshal(req)
    
    httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/v1/chat/completions", bytes.NewReader(payload))
    if err != nil {
        return nil, err
    }
    
    // 小米MiMo使用api-key头
    httpReq.Header.Set("api-key", a.apiKey)
    httpReq.Header.Set("Content-Type", "application/json")
    
    return a.client.Do(httpReq)
}
```

#### 3.1.3 能力路由配置

```sql
-- 添加小米MiMo厂商
INSERT INTO integration_vendors (id, name, icon, desc, enabled)
VALUES ('xiaomi-mimo', '小米MiMo', '🤖', '小米MiMo大模型，支持LLM/TTS/ASR', 1);

-- 添加LLM能力
INSERT INTO integration_capabilities (id, vendor_id, capability, model, endpoint, enabled, is_default)
VALUES 
('mimo-llm', 'xiaomi-mimo', 'llm', 'mimo-v2.5-pro', '/v1/chat/completions', 1, 0),
('mimo-tts', 'xiaomi-mimo', 'tts', 'mimo-v2.5-tts', '/v1/chat/completions', 1, 0),
('mimo-asr', 'xiaomi-mimo', 'asr', 'mimo-v2.5-asr', '/v1/chat/completions', 1, 0);
```

---

### 阶段2：TTS接入

#### 3.2.1 任务清单

| 任务 | 说明 |
|------|------|
| 2.1 | 创建MiMo TTS适配器 |
| 2.2 | 实现AudioSynthesizer接口 |
| 2.3 | 配置能力路由 |
| 2.4 | 测试验证 |

#### 3.2.2 MiMo TTS适配器

```go
// adapter/ttsmimo/tts_provider.go
package ttsmimo

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "io"
    "net/http"
    
    "webreaper/internal/usecase/port"
)

// MiMoTTSProvider 小米MiMo TTS适配器。
//
// 设计动机：
//   - 小米MiMo TTS返回base64音频，无需轮询，延迟更低
//   - 支持9种预置音色
//   - 支持声音克隆
type MiMoTTSProvider struct {
    apiKey  string
    baseURL string
    client  *http.Client
}

var _ port.AudioSynthesizer = (*MiMoTTSProvider)(nil)

func NewMiMoTTSProvider(apiKey, baseURL string) *MiMoTTSProvider {
    return &MiMoTTSProvider{
        apiKey:  apiKey,
        baseURL: baseURL,
        client:  &http.Client{Timeout: 30 * time.Second},
    }
}

// Synthesize 语音合成。
func (p *MiMoTTSProvider) Synthesize(ctx context.Context, req port.TTSRequest) ([]byte, error) {
    // 构建请求
    mimoReq := map[string]any{
        "model": "mimo-v2.5-tts",
        "messages": []map[string]string{
            {"role": "assistant", "content": req.Text},
        },
        "audio": map[string]any{
            "format": "mp3",
            "voice":  req.VoiceID,
        },
    }
    
    // 发送请求
    resp, err := p.sendRequest(ctx, mimoReq)
    if err != nil {
        return nil, err
    }
    
    // 解析base64音频
    var result struct {
        Choices []struct {
            Message struct {
                Audio struct {
                    Data string `json:"data"`
                } `json:"audio"`
            } `json:"message"`
        } `json:"choices"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    if len(result.Choices) == 0 {
        return nil, fmt.Errorf("no audio data in response")
    }
    
    // 解码base64
    return base64.StdEncoding.DecodeString(result.Choices[0].Message.Audio.Data)
}

// CloneVoice 声音克隆。
func (p *MiMoTTSProvider) CloneVoice(ctx context.Context, req port.VoiceCloneRequest) ([]byte, error) {
    // 构建请求
    mimoReq := map[string]any{
        "model": "mimo-v2.5-tts-voiceclone",
        "messages": []map[string]string{
            {"role": "assistant", "content": req.Text},
        },
        "audio": map[string]any{
            "format": "mp3",
            "voice":  base64.StdEncoding.EncodeToString(req.AudioSample),
        },
    }
    
    // 发送请求
    resp, err := p.sendRequest(ctx, mimoReq)
    if err != nil {
        return nil, err
    }
    
    // 解析base64音频
    var result struct {
        Choices []struct {
            Message struct {
                Audio struct {
                    Data string `json:"data"`
                } `json:"audio"`
            } `json:"message"`
        } `json:"choices"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    if len(result.Choices) == 0 {
        return nil, fmt.Errorf("no audio data in response")
    }
    
    return base64.StdEncoding.DecodeString(result.Choices[0].Message.Audio.Data)
}

func (p *MiMoTTSProvider) sendRequest(ctx context.Context, req any) (*http.Response, error) {
    payload, _ := json.Marshal(req)
    
    httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/chat/completions", bytes.NewReader(payload))
    if err != nil {
        return nil, err
    }
    
    httpReq.Header.Set("api-key", p.apiKey)
    httpReq.Header.Set("Content-Type", "application/json")
    
    return p.client.Do(httpReq)
}
```

---

### 阶段3：智能体集成

#### 3.3.1 任务清单

| 任务 | 说明 |
|------|------|
| 3.1 | 配置小米MiMo作为智能体LLM |
| 3.2 | 更新PromptBuilder支持小米MiMo |
| 3.3 | 测试验证 |

#### 3.3.2 配置方式

```sql
-- 添加小米MiMo LLM配置
INSERT INTO llm_configs (name, provider, api_key, base_url, model, is_default)
VALUES ('xiaomi-mimo', '小米', 'your-api-key', 'https://token-plan-cn.xiaomimimo.com/v1', 'mimo-v2.5-pro', 0);
```

---

### 阶段4：管理后台配置

#### 3.4.1 任务清单

| 任务 | 说明 |
|------|------|
| 4.1 | 在集成中心添加小米MiMo |
| 4.2 | 配置API Key |
| 4.3 | 启用/禁用能力 |
| 4.4 | 设置默认模型 |

#### 4.2 配置界面

管理后台路径：`/admin/integrations`

配置项：
- 厂商名称：小米MiMo
- API Key：输入框
- 启用状态：开关
- 能力列表：
  - LLM：启用/禁用，设置默认模型
  - TTS：启用/禁用，设置默认模型
  - ASR：启用/禁用，设置默认模型

---

## 四、API调用示例

### 4.1 LLM调用

```bash
curl -X POST https://token-plan-cn.xiaomimimo.com/v1/chat/completions \
  -H "api-key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mimo-v2.5-pro",
    "messages": [
      {"role": "user", "content": "你好"}
    ]
  }'
```

### 4.2 TTS调用

```bash
curl -X POST https://token-plan-cn.xiaomimimo.com/v1/chat/completions \
  -H "api-key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mimo-v2.5-tts",
    "messages": [
      {"role": "assistant", "content": "你好，欢迎使用小米语音服务"}
    ],
    "audio": {
      "format": "mp3",
      "voice": "mimo_default"
    }
  }'
```

### 4.3 声音克隆调用

```bash
curl -X POST https://token-plan-cn.xiaomimimo.com/v1/chat/completions \
  -H "api-key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mimo-v2.5-tts-voiceclone",
    "messages": [
      {"role": "assistant", "content": "要合成的文本"}
    ],
    "audio": {
      "format": "mp3",
      "voice": "base64编码的音频样本"
    }
  }'
```

---

## 五、关键设计点

### 5.1 认证头差异

小米MiMo使用 `api-key` 头，而非标准OpenAI的 `Authorization: Bearer`。

```go
// 标准OpenAI
req.Header.Set("Authorization", "Bearer "+apiKey)

// 小米MiMo
req.Header.Set("api-key", apiKey)
```

### 5.2 TTS返回base64

小米MiMo TTS返回base64音频，无需轮询，延迟更低。

```go
// 解析响应
var result struct {
    Choices []struct {
        Message struct {
            Audio struct {
                Data string `json:"data"`
            } `json:"audio"`
        } `json:"message"`
    } `json:"choices"`
}

// 解码base64
audioData, err := base64.StdEncoding.DecodeString(result.Choices[0].Message.Audio.Data)
```

### 5.3 音色库

小米MiMo提供9种预置音色：

| 音色ID | 风格 |
|--------|------|
| `mimo_default` | 默认音色 |
| `冰糖` | — |
| `茉莉` | — |
| `苏打` | — |
| `白桦` | — |
| `Mia` | — |
| `Chloe` | — |
| `Milo` | — |
| `Dean` | — |

---

## 六、实施进度

| 阶段 | 任务 | 状态 |
|------|------|------|
| 阶段1 | LLM接入 | ⏳ 待实施 |
| 阶段2 | TTS接入 | ⏳ 待实施 |
| 阶段3 | 智能体集成 | ⏳ 待实施 |
| 阶段4 | 管理后台配置 | ⏳ 待实施 |

---

## 七、架构缺陷修复（已完成）

### 7.1 问题描述

原有架构存在一个关键缺陷：

```go
// 原有设计：GenerationUseCase 只有一个 provider
type GenerationUseCase struct {
    provider port.GenerationProvider  // 单一厂商
    // ...
}
```

**问题**：
1. `provider` 是单一的，不支持多厂商
2. 没有使用 `CapabilityResolver` 动态选择厂商
3. 不同业务（视频/TTS/图片）都用同一个厂商

### 7.2 解决方案（已实施）

**修改 GenerationUseCase，支持多厂商动态选择**：

```go
// 修改后的设计
type GenerationUseCase struct {
    providers map[string]port.GenerationProvider  // 多厂商
    resolver  port.CapabilityResolver            // 能力路由
    registry  port.EndpointRegistry
    repo      port.GenerationTaskRepository
    // ...
}

// subType → capID 映射
var subTypeToCapID = map[string]string{
    "text2video":      "video",
    "img2video":       "video",
    "tts":             "tts",
    "voice_clone":     "voice-clone",
    // ...
}

// 根据 subType 动态选择 provider
func (uc *GenerationUseCase) getProvider(ctx context.Context, subType string) (port.GenerationProvider, error) {
    // 1. 查询能力路由
    capID := subTypeToCapID[subType]
    cap, err := uc.resolver.Resolve(ctx, capID)
    if err == nil && cap.VendorID != "" {
        if provider, ok := uc.providers[cap.VendorID]; ok {
            return provider, nil
        }
    }
    
    // 2. 使用默认 provider
    return uc.providers[uc.defaultProvider], nil
}
```

### 7.3 实施进度

| 任务 | 状态 | 说明 |
|------|------|------|
| 7.3.1 | ✅ 完成 | 修改 GenerationUseCase 支持多 provider |
| 7.3.2 | ✅ 完成 | 注入 CapabilityResolver |
| 7.3.3 | ✅ 完成 | 实现 subType → capID 映射 |
| 7.3.4 | ✅ 完成 | 在 main.go 中注册多个 provider |
| 7.3.5 | ✅ 完成 | 测试验证（单元测试通过） |

### 7.4 配置示例

**能力路由配置**：
```sql
-- 音频相关能力 → 小米MiMo
INSERT INTO integration_capabilities (id, cap_id, vendor_id, endpoint, model, is_default, enabled)
VALUES 
('tts#xiaomi-mimo', 'tts', 'xiaomi-mimo', '/v1/chat/completions', 'mimo-v2.5-tts', 1, 1),
('voice-clone#xiaomi-mimo', 'voice-clone', 'xiaomi-mimo', '/v1/chat/completions', 'mimo-v2.5-tts-voiceclone', 1, 1);

-- 视频/图片能力 → Vidu
INSERT INTO integration_capabilities (id, cap_id, vendor_id, endpoint, model, is_default, enabled)
VALUES 
('video#vidu', 'video', 'vidu', '/ent/v2/text2video', 'viduq3-pro', 1, 1),
('image#vidu', 'image', 'vidu', '/ent/v2/reference2image', 'viduq2', 1, 1);
```

**环境变量配置**：
```bash
# Vidu API Key
VIDU_API_KEY=your-vidu-key

# 小米MiMo API Key（可选）
MIMO_API_KEY=your-mimo-key
```

---

*文档生成时间：2026-08-23*
*基于获客智能体统一生成架构方案*
