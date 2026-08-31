# 29 - 口播 B-Roll 单阶段优化与插入逻辑改进

> 日期：2026-08-31
> 定位：**业务流程优化 + 交互逻辑改进**
> 背景：当前 B-Roll 插入是两阶段操作（先生成视频，再手动插入），用户体验不佳；
> 插入逻辑中极端情况处理不够合理（截断/定格/拒绝提交）。

---

## 一、问题分析

### 1.1 当前流程（两阶段）

```
阶段1: 确定文案 → 选形象/音色 → 生成口播视频 → 拿到视频
阶段2: 打开B-Roll页面 → 定位时间轴 → 选句+上传片段 → 合成 → 拿到最终视频
```

**问题**：
- 用户需要等两次（生成视频 + 合成B-Roll）
- 中间需要手动操作（打开B-Roll页面、选择句子、上传片段）
- 用户先看到纯口播视频，再决定插入什么——决策链路长

### 1.2 插入逻辑问题

| 极端情况 | 当前处理 | 问题 |
|----------|----------|------|
| 片段比时间窗长 | `shortest=1` 截断 | 片段被截断，内容丢失 |
| 片段比时间窗短 | `tpad=clone` 定格 | 最后一帧定格，视觉突兀 |
| 图片片段 | 循环视频 | 不需要循环，静态即可 |
| 时间窗重叠 | 校验报错 | 拒绝提交，用户体验差 |

---

## 二、优化方案

### 2.1 单阶段流程

```
阶段1: 确定文案
  "大家好欢迎来到小王美容"
  "我们店在春熙路已经开了10年了"
  "主打面部护理和身体按摩"

阶段2: 选形象/选音色/选B-Roll（一次性配置完）
  - 形象：选择分身A
  - 音色：选择女声B
  - B-Roll（可选）：
    - 第1句插入店铺外观图
    - 第2句插入产品视频

阶段3: 点击"生成" → 等待 → 拿到最终视频
  （系统后台自动完成：TTS → 口播 → 定位时间轴 → 合成B-Roll）
```

**优势**：
- 用户只操作一次，等待一次
- 直接拿到最终视频（含B-Roll）
- 决策链路短（一次性配置完所有内容）

### 2.2 插入逻辑改进

**核心理念变化**：
- **当前**：时间窗是**严格边界**（片段必须在边界内显示）
- **改进**：时间窗是**起始标记**（片段从这里开始，按自身时长播放）

#### 2.2.1 片段比时间窗长 → 不截断，播完整个片段

```
时间窗：第2句 = 4.7s ~ 8.5s（3.8秒）
片段：10秒产品视频

当前：只显示 4.7s~8.5s（3.8秒），后面截断
改进：从 4.7s 开始播放，播完 10 秒

结果：
┌──────────────────────────────────────────────────────────────┐
│ 原片：│ 第0句 │ 第1句 │  第2句  │ 第3句 │ 第4句 │            │
│       │ 0~2.1 │2.3~4.5│ 4.7~8.5│ 8.5~10│10~12  │            │
├──────────────────────────────────────────────────────────────┤
│ 合成：│ 第0句 │ 第1句 │    片段（10秒）     │ 第4句 │        │
│       │ 0~2.1 │2.3~4.5│ 4.7s 开始播放      │10~12  │        │
│       │       │       │ 覆盖第2句+第3句     │       │        │
│       │       │       │ 片段结束后显示原片   │       │        │
└──────────────────────────────────────────────────────────────┘
```

**关键点**：
- 片段从第2句开始播放，但会覆盖到第3句的时间段（因为片段比时间窗长）
- 片段播完后，overlay 自动失效，原片自然显示
- 不延长视频总时长（shortest=1 保证）

#### 2.2.2 片段比时间窗短 → 播完后显示原片，不定格

```
时间窗：第2句 = 4.7s ~ 8.5s（3.8秒）
片段：2秒产品视频

当前：片段播完后，最后一帧定格到 8.5s
改进：片段播完后，直接显示原片（第2句的画面）

结果：
┌──────────────────────────────────────────────────────────────┐
│ 时间窗：│        4.7s ──────── 8.5s（3.8秒）                 │
├──────────────────────────────────────────────────────────────┤
│ 合成：  │ 片段(2s) │ 原片(1.8s)                              │
│         │ 4.7~6.7  │ 6.7~8.5                                 │
│         │ 产品视频 │ 第2句原始画面                            │
└──────────────────────────────────────────────────────────────┘
```

**关键点**：
- 片段播完后，overlay 自动失效，原片自然显示
- 不需要 `tpad=clone` 定格

#### 2.2.3 图片片段 → 静态显示到时间窗结束

```
时间窗：第2句 = 4.7s ~ 8.5s（3.8秒）
片段：产品图片

当前：图片转为 3.8 秒循环视频
改进：图片静态显示到第2句结束（8.5s），然后显示原片

结果：
┌──────────────────────────────────────────────────────────────┐
│ 时间窗：│        4.7s ──────── 8.5s（3.8秒）                 │
├──────────────────────────────────────────────────────────────┤
│ 合成：  │ 产品图片（静态显示 3.8 秒）                         │
│         │ 4.7s ~ 8.5s                                        │
│         │ 然后显示原片                                        │
└──────────────────────────────────────────────────────────────┘
```

**关键点**：
- 图片不需要循环，直接静态显示到时间窗结束
- 比循环视频更自然（不会看到重复的画面）

#### 2.2.4 时间窗重叠 → 后续片段优先

```
用户选择：
- 第1句 (2.3s~4.5s) 插入片段A
- 第2句 (4.0s~8.5s) ← 与第1句重叠

当前：校验报错，拒绝提交
改进：允许重叠，后续片段覆盖前面的

结果：
┌──────────────────────────────────────────────────────────────┐
│ 时间线：│ 2.3s ────── 4.5s                                   │
│ 第1句： │ [片段A]                                              │
├──────────────────────────────────────────────────────────────┤
│ 时间线：│         4.0s ────────────── 8.5s                    │
│ 第2句： │          [片段B]                                     │
├──────────────────────────────────────────────────────────────┤
│ 合成：  │ 2.3~4.0: 片段A │ 4.0~8.5: 片段B                    │
│         │ （片段B覆盖了片段A的重叠部分）                       │
└──────────────────────────────────────────────────────────────┘
```

**关键点**：
- 重叠区域显示后续时间线的片段（后插入的优先）
- 不报错，不拒绝提交

#### 2.2.5 同句重复插入 → 保持拒绝

```
用户提交：
- 第2句插入片段A
- 第2句插入片段B

当前：校验报错
改进：保持拒绝（同一句只能插一个片段）
```

---

## 三、技术实现方案

### 3.1 单阶段流程实现

#### 3.1.1 修改 UnifiedSubmitInput 结构

```go
// generation.go - UnifiedSubmitInput 新增字段
type UnifiedSubmitInput struct {
    // ... 现有字段 ...
    BrollSegments []BrollSegment `json:"broll_segments,omitempty"` // B-Roll配置（可选）
}

// BrollSegment B-Roll片段配置
type BrollSegment struct {
    SentenceIndex int    `json:"sentence_index"` // 插入到第几句（从0开始）
    MediaURL      string `json:"media_url"`      // 片段URL（图片或视频）
}
```

#### 3.1.2 修改 UnifiedSubmit 逻辑

```go
// generation.go - UnifiedSubmit()
func (uc *GenerationUseCase) UnifiedSubmit(ctx context.Context, in UnifiedSubmitInput) (entity.GenerationTask, error) {
    // ... 现有逻辑（选择端点、提交生成）...

    task, err := uc.Submit(ctx, SubmitInput{...})
    if err != nil {
        return task, err
    }

    // 如果携带了 B-Roll 配置，创建链式任务
    if len(in.BrollSegments) > 0 {
        go uc.chainBrollAfterGeneration(ctx, task, in.BrollSegments)
    }

    return task, nil
}
```

#### 3.1.3 新增链式 B-Roll 合成方法

```go
// generation.go - chainBrollAfterGeneration()
// 视频生成完成后自动执行B-Roll合成
func (uc *GenerationUseCase) chainBrollAfterGeneration(ctx context.Context, sourceTask entity.GenerationTask, segments []BrollSegment) {
    // ① 等待视频生成完成（轮询）
    for {
        task, _ := uc.repo.FindByID(ctx, sourceTask.TenantID, sourceTask.ID)
        if entity.IsTerminal(task.State) {
            if task.State != entity.TaskStateSuccess {
                log.Printf("[broll] 源视频生成失败，跳过B-Roll合成: %s", task.ID)
                return
            }
            break
        }
        time.Sleep(5 * time.Second)
    }

    // ② 自动定位时间轴
    _, _, err := uc.composer.LocateTimeline(ctx, sourceTask.TenantID, sourceTask.ID, false, nil)
    if err != nil {
        log.Printf("[broll] 时间轴定位失败: %v", err)
        return
    }

    // ③ 转换片段配置
    composeSegments := make([]port.ComposeSegment, len(segments))
    for i, s := range segments {
        composeSegments[i] = port.ComposeSegment{
            SentenceIndex: s.SentenceIndex,
            MediaURL:      s.MediaURL,
        }
    }

    // ④ 提交compose
    _, err = uc.composer.SubmitCompose(ctx, port.ComposeInput{
        TenantID:     sourceTask.TenantID,
        BrandID:      sourceTask.BrandID,
        SourceTaskID: sourceTask.ID,
        Segments:     composeSegments,
    })
    if err != nil {
        log.Printf("[broll] B-Roll合成失败: %v", err)
        return
    }

    log.Printf("[broll] B-Roll合成已提交: %s", sourceTask.ID)
}
```

#### 3.1.4 前端改造

```typescript
// LipSyncWizard.tsx - 提交时携带 B-Roll 配置
const handleSubmit = async () => {
    const brollSegments = selectedBroll.map(b => ({
        sentence_index: b.sentenceIndex,
        media_url: b.mediaURL
    }))

    await submitGenerationTask({
        sub_type: 'lip_sync',
        params: {
            script: script,
            voice_id: selectedVoice,
            subject_id: selectedSubject
        },
        broll_segments: brollSegments.length > 0 ? brollSegments : undefined
    })
}
```

---

### 3.2 插入逻辑改进实现

#### 3.2.1 修改 ffmpeg 滤镜链

```go
// mediaav/compose.go - ComposeInsertSegments()
// 改进：移除 shortest=1，移除 tpad，使用纯 overlay

func (t *FFmpegTool) ComposeInsertSegments(ctx context.Context, mainVideoPath string, segs []port.InsertSegmentSpec, outPath string) error {
    // ... 现有分辨率探测逻辑 ...

    args := []string{"-y", "-i", mainVideoPath}
    filterParts := make([]string, 0, len(segs)*2+1)
    prev := "[0:v]"

    for i, seg := range segs {
        inputIdx := i + 1
        args = append(args, "-i", seg.MediaPath)
        out := fmt.Sprintf("[v%d]", inputIdx)

        // 改进：只做缩放+裁剪，不加 tpad（不定格）
        filterParts = append(filterParts, fmt.Sprintf(
            "[%d:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,setsar=1[s%d]",
            inputIdx, w, h, w, h, inputIdx))

        // 改进：移除 shortest=1，让片段自然播放
        // enable='between(t,S,E)' 控制显示时间窗
        // 片段播完后 overlay 自动失效，原片自然显示
        filterParts = append(filterParts, fmt.Sprintf(
            "%s[s%d]overlay=0:0:enable='between(t,%.3f,%.3f)'%s",
            prev, inputIdx, float64(seg.StartMs)/1000, float64(seg.EndMs)/1000, out))

        prev = out
    }

    filter := strings.Join(filterParts, ";")
    args = append(args,
        "-filter_complex", filter,
        "-map", prev,
        "-map", "0:a:0",
        "-c:v", "libx264", "-preset", "veryfast",
        "-c:a", "copy",
        outPath,
    )

    cmd := exec.CommandContext(ctx, t.bin("ffmpeg"), args...)
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("ffmpeg 合成失败: %w | 输出尾: %.400s", err, out)
    }
    return nil
}
```

**关键变化**：
- 移除 `tpad=stop_mode=clone:stop_duration=3600`（不再定格）
- 移除 `shortest=1`（不再截断）
- 只保留 `enable='between(t,S,E)'` 控制显示时间窗
- 片段播完后，overlay 自动失效，原片自然显示

#### 3.2.2 修改图片处理逻辑

```go
// compose.go - execute() 图片处理
// 改进：图片只显示到时间窗结束，不循环

if isImagePath(segPath) {
    dur := float64(r.spec.EndMs-r.spec.StartMs) / 1000
    loopPath := segPath + ".loop.mp4"
    // 改进：-t 参数确保只生成到时间窗结束的时长
    if lerr := uc.av.(imageLooper).LoopImageToVideo(ctx, segPath, dur, loopPath); lerr != nil {
        setState(entity.TaskStateFailed, fmt.Sprintf("图片转视频失败: %v", lerr))
        return
    }
    segPath = loopPath
}
```

**说明**：图片转视频的时长 = 时间窗时长，确保图片只显示到时间窗结束。

#### 3.2.3 修改校验逻辑

```go
// compose.go - SubmitCompose() 校验逻辑
// 改进：移除重叠检测，保留重复检测

// ① 句号有效性检测（保留）
for _, s := range in.Segments {
    if s.SentenceIndex < 0 || s.SentenceIndex >= len(meta.Lines) {
        return port.ComposeResult{}, fmt.Errorf("句号越界: %d", s.SentenceIndex)
    }
    if s.MediaURL == "" {
        return port.ComposeResult{}, fmt.Errorf("第 %d 句的片段地址为空", s.SentenceIndex)
    }
}

// ② 重复检测（保留：同一句只能插一个片段）
for i := 0; i < len(resolved); i++ {
    for j := i + 1; j < len(resolved); j++ {
        if resolved[i].idx == resolved[j].idx {
            return port.ComposeResult{}, fmt.Errorf("第 %d 句重复配置片段", resolved[i].idx)
        }
    }
}

// ③ 重叠检测（移除：允许重叠，后续片段优先）
// 不再检测窗口重叠，直接提交
```

---

## 四、数据库变更

无需新增表结构。复用现有 `generation_tasks` 表：

```json
// params_json 新增 broll_segments 字段
{
    "script": "大家好\n我们店在春熙路\n主打面部护理",
    "voice_id": "female-shaonv",
    "subject_id": "xxx",
    "broll_segments": [
        {"sentence_index": 0, "media_url": "https://oss.example.com/store.jpg"},
        {"sentence_index": 1, "media_url": "https://oss.example.com/product.mp4"}
    ]
}
```

---

## 五、API 变更

### 5.1 统一提交 API 增强

```
POST /api/v1/generation/submit

新增字段：
{
    "broll_segments": [
        {"sentence_index": 0, "media_url": "..."},
        {"sentence_index": 2, "media_url": "..."}
    ]
}
```

**行为变化**：
- 携带 `broll_segments` 时，系统自动在视频生成后执行 B-Roll 合成
- 不携带时，行为不变（纯口播视频）

### 5.2 现有 API 不变

```
POST /api/v1/generation/tasks/:id/timeline  ← 保持（手动定位场景）
POST /api/v1/generation/submit (compose)     ← 保持（手动插入场景）
```

---

## 六、前端改造

### 6.1 口播向导改造

```
当前向导步骤：
① 确定文案 → ② 出镜与配音 → ③ 生成成片 → ④ 发布

改进后：
① 确定文案 → ② 出镜与配音 → ③ B-Roll配置（可选）→ ④ 生成成片 → ⑤ 发布
```

**步骤③ B-Roll 配置**：
- 展示时间轴预估（根据台词字数估算时长）
- 用户选择插入句 + 上传片段
- 配置保存到 `broll_segments`

### 6.2 B-Roll 页面保留

B-Roll 页面保留，用于：
- 视频生成后的手动调整
- 重新定位时间轴
- 修改插入配置

---

## 七、总结

### 7.1 单阶段流程

| 方面 | 当前（两阶段） | 改进（单阶段） |
|------|---------------|---------------|
| B-Roll选择时机 | 视频生成后再选 | 生成前就选好 |
| 用户操作次数 | 2次 | 1次 |
| 等待次数 | 2次 | 1次 |
| 中间产物 | 用户看到纯口播视频 | 直接看到最终视频 |

### 7.2 插入逻辑改进

| 极端情况 | 当前 | 改进 |
|----------|------|------|
| 片段比时间窗长 | 截断 | 不截断，播完整个片段 |
| 片段比时间窗短 | 定格 | 播完后显示原片 |
| 图片片段 | 循环视频 | 静态显示到时间窗结束 |
| 时间窗重叠 | 报错拒绝 | 后续片段优先 |
| 同句重复插入 | 报错拒绝 | 保持拒绝 |

### 7.3 技术改动点

| 文件 | 改动 |
|------|------|
| `generation.go` | UnifiedSubmitInput 新增 BrollSegments + chainBrollAfterGeneration |
| `mediaav/compose.go` | 移除 tpad + shortest，改为纯 overlay |
| `compose.go` | 移除重叠检测，保留重复检测 |
| `LipSyncWizard.tsx` | 新增 B-Roll 配置步骤 |
| `generationSubmit.ts` | 提交时携带 broll_segments |
