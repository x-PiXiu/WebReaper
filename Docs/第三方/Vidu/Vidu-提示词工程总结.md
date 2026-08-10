# Vidu AI 提示词工程总结

> **更新日期**：2026-08-05
> **适用模型**：viduq3-pro/turbo/mix、viduq2-pro/turbo、viduq1、vidu2.0（视频）；viduq2/viduq1（图片）
> **来源**：Vidu 官方 API 文档（api.vidu.cn）
> **Base URL**：`https://api.vidu.cn/ent/v2`

---

## 一、模型总览

### 视频模型

| 模型 | 代次 | 音画同出 | 最大时长 | 最高分辨率 | 特色 |
|------|------|---------|---------|-----------|------|
| viduq3-pro / viduq3 | Q3 旗舰 | ✅ 默认 true | 16s | 1080p | 智能切镜，主体引用 `@name` |
| viduq3-turbo | Q3 快速 | ✅ 默认 true | 16s | 1080p | q3 的加速版 |
| viduq3-pro-fast | Q3 极速 | ✅ 默认 true | 16s | 1080p | 最快速度 |
| viduq3-mix | Q3 混合 | ✅ | 16s | 1080p | 仅非主体参考生，不支持主体调用 |
| viduq2-pro | Q2 旗舰 | ❌（audio_type 拆分） | 10s | 1080p | **独家支持视频主体参考 + 智能多帧** |
| viduq2-turbo | Q2 快速 | ❌ | 10s | 1080p | 智能多帧 |
| viduq2 | Q2 标准 | ❌ | 10s | 1080p | 支持任意宽高比 |
| viduq1 / q1-classic | Q1 上一代 | ❌ | 固定 5s | 1080p | **支持 style(anime) + movement_amplitude（2.0 也支持 style）** |
| vidu2.0 | 2.0 | ❌ | 8s | 1080p | 模板成片 |

### 图片模型

| 模型 | 文生图 | 参考生图 | 最高分辨率 | 宽高比 |
|------|--------|---------|-----------|--------|
| viduq2 | ✅（0 张图） | ✅（1-7 张图） | 4K | 16:9/9:16/1:1/3:4/4:3/21:9/2:3/3:2/auto |
| viduq1 | ❌ | ✅（1-7 张图） | 1080p | 16:9/9:16/1:1/3:4/4:3 |

---

## 二、核心特性：参考内容控制方式

### 2.1 关键结论

Vidu 支持 **prompt 内联引用语法**——在参考生视频（主体模式）的 prompt 中用 `@name` 引用 `subjects[].name`。这是 Vidu 最核心的提示词工程能力。

同时，Vidu 支持 **非主体参考模式**（images[] + prompt 自然语言描述），与 Agnes 类似。

### 2.2 参考素材字段对照表

| 场景 | 字段 | 类型 | 说明 |
|------|------|------|------|
| 图片-参考生图 | `images` | string[] | 1-7 张参考图 |
| 视频-图生视频 | `images` | string[] | **恰好 1 张**（首帧） |
| 视频-首尾帧 | `images` | string[] | **恰好 2 张**（首帧+尾帧） |
| 视频-参考生(非主体) | `images` | string[] | 1-7 张参考图 |
| 视频-参考生(非主体) | `videos` | string[] | 1-2 个参考视频（**仅 q2-pro**） |
| 视频-参考生(主体) | `subjects` | object[] | 主体对象数组（含 name/images/videos/voice_id） |
| 视频-智能多帧 | `start_image` + `image_settings[].key_image` | string | 首帧 + 关键帧序列 |

### 2.3 主体引用语法（核心）

#### 两种写法（文档中并存）

| 写法 | 出处 | 示例 |
|------|------|------|
| `@name` | 参考生视频文档 | `@your_subject1_name 和 @your_subject2_name 在一起吃火锅` |
| `[@name]` | 创建主体 API 文档 | `[@1]和[@2]在[@3]一起吃火锅` |

> ⚠️ 两份官方文档对引用写法不一致（`@name` vs `[@name]`），建议两种都支持或向 Vidu 方确认。**推荐用 `@name`**（更通用，与平台统一 @ 系统一致）。

#### 引用规则

- `@name` 中的 `name` 对应 `subjects[].name` 字段值
- `name` 必须在整个请求中唯一
- 未在 prompt 中引用的主体也会传入模型（但不保证出现）
- 主体模式（传 `subjects[]`）与非主体模式（传 `images[]`）在官方文档中分两套独立请求体，实际使用时不要同时传两组参数

#### 主体定义方式

```json
{
  "subjects": [
    {"name": "主角", "images": ["url1", "url2", "url3"]},
    {"name": "对手", "images": ["url4", "url5"]}
  ],
  "prompt": "@主角 和 @对手 在悬崖上对峙"
}
```

### 2.4 主体库（server_id）

主体可预先创建并存储：

```python
# 1. 创建主体
POST /ent/v2/subjects
{"name": "张三", "images": ["url1"]}
# 返回 {"id": "server_id_xxx", ...}

# 2. 在参考生视频中引用
POST /ent/v2/reference2video
{
  "subjects": [{"name": "张三", "server_id": "server_id_xxx"}],
  "prompt": "@张三 站在海边"
}
```

- 创建主体 5 积分/次
- 编辑、使用主体免费
- `auto_subjects: true` 启用智能主体库（自动匹配系统级主体）

### 2.5 不支持负向提示词

**Vidu 全 API 不支持 `negative_prompt`**。规避不需要的内容只能在正向 prompt 中用自然语言（如"避免出现文字"、"保持画面干净"）。

---

## 三、图片提示词工程

### 3.1 文生图（viduq2 专属）

不传 `images`，仅用 prompt：

```
[主体] + [场景/背景] + [风格] + [光照] + [构图] + [质量要求]
```

```json
{
  "model": "viduq2",
  "prompt": "一只橘猫坐在窗台上，阳光透过窗帘洒在身上，温暖柔和，胶片质感，细节丰富",
  "aspect_ratio": "1:1",
  "resolution": "2K"
}
```

### 3.2 参考生图（viduq2 / viduq1）

传入 `images[]`，prompt 描述如何融合/编辑参考图：

```
[参考图角色] + [目标场景] + [编辑/融合指令] + [风格/光照/构图]
```

```json
{
  "model": "viduq2",
  "images": ["https://.../character.png", "https://.../scene.png"],
  "prompt": "将第一张图的角色放到第二张图的场景中，保持角色面部不变，电影级光影，广角构图",
  "aspect_ratio": "16:9",
  "resolution": "4K"
}
```

### 3.3 参数控制

| 参数 | viduq2 | viduq1 |
|------|--------|--------|
| 分辨率 | 1080p / 2K / 4K | 1080p |
| 宽高比 | 16:9/9:16/1:1/3:4/4:3/21:9/2:3/3:2/auto | 16:9/9:16/1:1/3:4/4:3 |
| 参考图数 | 0-7 | 1-7 |
| prompt 上限 | 2000 字符 | 2000 字符 |

> `auto` 宽高比 = 与首张输入图同比例。

---

## 四、视频提示词工程

### 4.1 文生视频

```
[主体] + [动作] + [场景] + [镜头运动] + [光线] + [风格]
```

```json
{
  "model": "viduq3-pro",
  "prompt": "一只金毛犬在海边奔跑，夕阳西下，金色光线洒在水面上，镜头低角度跟随拍摄，电影质感",
  "duration": 8,
  "resolution": "1080p",
  "aspect_ratio": "16:9",
  "audio": true
}
```

### 4.2 图生视频

传入 `images`（恰好 1 张，作为首帧）：

```
[运动描述] + [需要稳定的元素] + [镜头运动] + [氛围]
```

```json
{
  "model": "viduq2-pro",
  "images": ["https://.../portrait.png"],
  "prompt": "人物缓缓转头看向镜头，微笑，头发在微风中轻轻飘动，背景虚化，柔和自然光",
  "duration": 5,
  "audio": false,
  "audio_type": "all",
  "voice_id": "your_voice_id"
}
```

### 4.3 参考生视频 — 主体模式（核心用法）

#### 基本主体引用

```json
{
  "model": "viduq3",
  "subjects": [
    {"name": "女孩", "images": ["https://.../girl1.png", "https://.../girl2.png"]},
    {"name": "男孩", "images": ["https://.../boy.png"]}
  ],
  "prompt": "@女孩 和 @男孩 在公园里散步，阳光明媚，两人有说有笑，镜头中景跟拍",
  "duration": 8,
  "audio": true
}
```

#### 多素材同主体

一个主体可以用多张图片（最多 3 张）增强特征：

```json
{
  "subjects": [
    {"name": "主角", "images": ["https://.../face.png", "https://.../fullbody.png", "https://.../outfit.png"]}
  ],
  "prompt": "@主角 站在舞台上演讲，自信的微笑，聚光灯照射"
}
```

#### 使用主体库（server_id）

```json
{
  "subjects": [
    {"name": "张三", "server_id": "server_id_xxx", "voice_id": "voice_yyy"}
  ],
  "prompt": "@张三 在厨房做饭，切菜的动作，温馨的家庭氛围"
}
```

#### 视频主体参考（q2-pro 独家）

```json
{
  "model": "viduq2-pro",
  "subjects": [
    {"name": "舞者", "videos": ["https://.../dance.mp4"]}
  ],
  "prompt": "@舞者 在舞台上表演街舞，动作流畅有力"
}
```

#### prompt 引用模板

```
@主体A 和 @主体B 在 <场景描述>，<动作描述>，<镜头描述>
```

**示例**：
```
@女孩 站在 @场景1 的门口，参考 @视频1 的运镜方式
@主角1 和 @主角2 面对面坐在咖啡厅里，窗外下着雨
```

### 4.4 参考生视频 — 非主体模式

用 `images[]` + `videos[]`（q2-pro），不定义主体：

```json
{
  "model": "viduq2-pro",
  "images": ["https://.../char.png", "https://.../scene.png"],
  "videos": ["https://.../ref_motion.mp4"],
  "prompt": "参考第一张图的角色和第二张图的场景，生成角色在场景中行走的视频，运镜参考视频素材",
  "duration": 5
}
```

### 4.5 首尾帧

传入 `images`（恰好 2 张：首帧+尾帧）：

```
[首帧到尾帧的过渡描述] + [运动/变化描述]
```

```json
{
  "model": "viduq2-pro",
  "images": ["https://.../start.png", "https://.../end.png"],
  "prompt": "角色从站立姿势缓慢转身，最终面向镜头微笑",
  "duration": 5
}
```

> 两图分辨率比需在 0.8~1.25 之间。

### 4.6 智能多帧（q2-turbo/pro 独家）

```json
{
  "model": "viduq2-pro",
  "start_image": "https://.../first_frame.png",
  "image_settings": [
    {"key_image": "https://.../kf1.png", "prompt": "角色走进咖啡厅", "duration": 3},
    {"key_image": "https://.../kf2.png", "prompt": "角色坐下点单", "duration": 4},
    {"key_image": "https://.../kf3.png", "prompt": "角色品尝咖啡", "duration": 3}
  ]
}
```

### 4.7 音频控制

| 参数 | 说明 | q3 | q2/q1/2.0 |
|------|------|-----|-----------|
| `audio` | 音画同出 | ✅ 默认 true | ❌ 不支持 |
| `audio_type` | 音频拆分 | ❌ 不生效 | ✅ all/speech_only/sound_effect_only |
| `voice_id` | 音色 ID | ❌ 不生效 | ✅ |
| `bgm` | 背景音乐 | ❌ | ✅（q2 duration 9-10s 不生效） |

---

## 五、模型参数差异速查

### 端点 × 模型可用性

| 端点 | q3系列 | q2系列 | q1系列 | 2.0 |
|------|--------|--------|--------|-----|
| text2video | ✅ | q2 only | ✅ | ❌ |
| img2video | ✅ | ✅ | ✅ | ✅ |
| reference2video(主体) | ✅(非mix) | ✅(pro独占视频主体) | ✅ | ✅ |
| reference2video(非主体) | ✅ | ✅ | ✅ | ✅ |
| start-end2video | ✅ | ✅ | ✅ | ✅ |
| multiframe | ❌ | pro/turbo only | ❌ | ❌ |

### 时长（duration，秒）

| 模型 | 文生 | 图生 | 参考生 | 首尾帧 |
|------|------|------|--------|--------|
| q3 | 1-16 | 1-16 | 3-16 | 1-16 |
| q2-pro | — | 1-10 | 0-10 | 1-8 |
| q2 | 1-10 | — | 1-10 | — |
| q1 | 固定5 | 固定5 | 固定5 | 固定5 |
| 2.0 | — | 4/8 | 固定4 | 4/8 |

### Q1 独有参数

| 参数 | 说明 |
|------|------|
| `style` | `general`(通用) / `anime`(动漫)，仅 q1 和 2.0 生效（q2/q3 不生效） |
| `movement_amplitude` | `auto`/`small`/`medium`/`large`，仅 q1 生效 |

---

## 六、平台适配器对接要点

### 6.1 当前适配器现状

| 维度 | 图片适配器 | 视频适配器 |
|------|-----------|-----------|
| 类名 | ViduImageProvider | ViduVideoProvider |
| 模式 | 异步 | 异步 |
| PromptRenderer | **无**（NATURAL_LANGUAGE） | **ViduPromptRenderer**（AT_SYMBOL） |

### 6.2 统一 @ 引用系统的适配

Vidu 是少数**原生支持 prompt 内联引用**的厂商。统一 @ 系统对 Vidu 的处理：

1. 用户在 prompt 里写 `@主体1` → 前端构建 mediaRefs
2. 后端 `ViduPromptRenderer`（非 NoOp）→ `@主体1` → `@subject_N`
3. 适配器从 `request.getElementIds()` 构造 `subjects[]`，name 用 `subject_N`
4. prompt 的 `@subject_1` 与 subjects[].name 一一对应

**与 Kling/Seedance/Agnes 的差异**：

| 厂商 | `@主体1` 处理 | 引用机制 |
|------|-------------|---------|
| Kling | → `@element_1` | prompt 的 @id ↔ contents[].id |
| Seedance | → `图片1` + 主体定义 | prompt 的序号 ↔ content 数组位置 |
| **Vidu** | → `@subject_N` | **prompt 的 @name ↔ subjects[].name** |
| Agnes | 保留原样 | 自然语言"第一张图" ↔ extra_body.image 顺序 |

### 6.3 referenceSyntax 配置

```
referenceSyntax = AT_SYMBOL  ← Vidu 原生支持 @ 引用
```

### 6.4 适配器 buildRequestBody 要点

**图生视频**：
```java
body.add("images", [imageUrl]);  // 恰好 1 张
body.addProperty("prompt", prompt);
```

**参考生视频（主体模式）**：
```java
JsonArray subjects = new JsonArray();
for (Long elementId : request.getElementIds()) {
    JsonObject subject = new JsonObject();
    subject.addProperty("server_id", String.valueOf(elementId));
    subject.addProperty("name", "subject_" + index);  // 与 renderer 的 @subject_N 对齐
    subjects.add(subject);
}
body.add("subjects", subjects);
body.addProperty("prompt", renderedPrompt);  // 已替换为 @subject_N
```

**参考生视频（非主体模式）**：
```java
JsonArray images = new JsonArray();
for (String url : request.getImageUrls()) images.add(url);
body.add("images", images);
body.addProperty("prompt", prompt);  // 原样（NATURAL_LANGUAGE）
```

---

## 七、常见陷阱速查

| # | 陷阱 | 正确做法 |
|---|------|---------|
| 1 | 用 `viduq3-mix` 做主体调用 | q3-mix 不支持主体，用 `viduq3` |
| 2 | 主体模式与非主体模式混用 | 不要同时传 `subjects[]` 和 `images[]`（官方分两套独立请求体） |
| 3 | `@name` 引用 server_id | 引用的是 `name`，不是 `server_id` |
| 4 | 用 `negative_prompt` | Vidu 不支持，只能正向描述规避 |
| 5 | 图生视频传多张 images | img2video 恰好 1 张 |
| 6 | 首尾帧传 1 张或 3 张 | start-end2video 恰好 2 张 |
| 7 | q3 设 `voice_id` | q3 系列 voice_id 不生效 |
| 8 | q1 设 `duration: 8` | q1 固定 5 秒 |
| 9 | 参考视频用非 q2-pro 模型 | `videos[]` 仅 q2-pro 支持 |
| 10 | 主体图片超过 3 张 | `subjects[].images` 最多 3 张 |

---

## 八、提示词检查清单

### 图片提示词

```
□ 文生图：viduq2 且不传 images？
□ 参考生图：images 1-7 张？
□ prompt ≤2000 字符？
□ 宽高比选对？（viduq2 支持 auto）
□ 分辨率选对？（viduq2 支持 4K，viduq1 仅 1080p）
```

### 视频提示词

```
□ 文生视频：prompt 有 [主体]+[动作]+[场景]+[镜头]？
□ 图生视频：images 恰好 1 张？
□ 主体引用：@name 与 subjects[].name 一致？
□ 多主体：每个 subject 有唯一 name？
□ 主体图片：每个 subject ≤3 张？
□ 首尾帧：images 恰好 2 张？分辨率比 0.8-1.25？
□ 参考视频：用了 q2-pro 模型？
□ 时长在模型范围内？（q3: 1-16s, q2: 1-10s, q1: 固定5s）
□ 音频参数选对？（q3: audio=true; q2: audio_type + voice_id）
□ 没有传 negative_prompt？
```
