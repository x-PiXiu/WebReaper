# 22 - 口播视频画面插入（B-Roll）功能计划

> 日期：2026-08-29
> 状态：方案设计完成，ffmpeg 核心命令已实测验证（见 §3.6）
> 关联：08-傻瓜式口播视频向导（LipSyncWizard 管线）、09-统一生成架构

---

## 1. 背景与需求

### 1.1 用户故事

用户在口播向导中生成对口型视频（数字分身/真人出镜 + TTS 台词，约 3 分钟）。
生成后（或生成前配置），用户希望在**指定台词的播放期间**插入自己指定的视频画面（素材库
素材/上传视频），产出的成片效果：

```
时间轴：  0──────口播画面──────t1─────插入片段B─────t2──────口播画面──────end
画面轨：  [口播视频画面]        [用户指定的视频画面]      [口播视频画面]
音频轨：  [──────────────口播音频连续播放（全程不断）──────────────]
                ↑───────── 音量全程不变（音频流直拷，零处理）─────────↑
                                     [片段音轨剥离，不入成片]
```

### 1.2 效果定义（验收基线）

1. **口播画面期间**：口型与音频严格对齐（现状能力，不受影响）
2. **插入片段期间**：画面完全替换为片段内容；**片段自带音轨剥离**（不进入成片）；
   **口播音频全程原样、音量不变**；不需要口型（镜头不在人脸上）
3. **片段结束回到口播画面**：口型**继续对齐**——不是"重新对齐"，是从未失去对齐（见 §2 原理）
4. 支持一段成片中**多个插入片段**（各自独立时间窗）
5. 插入片段的时长策略：**片段适配台词时间窗**（短了定格最后一帧铺满、长了截断）——时间轴总长不变

### 1.3 明确不做（本期边界）

- 不做片段内再嵌套片段
- 不做动态时间轴（插入不改变任何台词的时间）
- 不做转场特效（fade 可作为后续增强，滤镜一行的事）
- 不做片段原声混音（B-roll 自带声音），本期口播声独占音频轨；`amix` 混音留作参数化扩展

### 1.4 端到端业务流程（现状 + 修改后）

#### 现状：口播视频生成流程（本功能完全不改动这段）

```
【向导①素材】链接提取(抖音/B站) / 上传音视频 / 手写文案
      ↓ 得到台词 script（前端有分句列表 raw_text_lines）
【向导②配置】选品牌人设 + 出镜形态：
      · avatar 数字分身：选已有分身（subjectServerId / portraitUrl）
      · real 真人出镜：上传出镜视频（realVideoUrl）
【向导③成片 produce —— runLipSyncPipeline 三段管线】
      TTS 阶段   ：script ──TTS任务──> 整段音频 audioUrl（ttsTaskId）
      画面阶段   ：avatar → 分身形象生成口播画面视频（refTaskId）
                   real   → 直接用用户上传的真人视频
      合成阶段   ：audioUrl + 口播画面 ──Vidu lipsync──> 成片 resultUrl
      ↓ 成片入作品库（lipsyncTaskId）
```

双入口说明：另支持「文本直生视频」（文案+分身 → Vidu 端到端，音频由 Vidu 一体生成）——
两种入口的产物都是"带音频轨的成片"，本功能在其后接管，入口无关。

#### 修改后：追加 B-Roll 插入阶段（成片后的作品级操作）

```
【作品详情】用户查看成片 → 台词列表逐句展示
      ↓ 用户首次点某句「插入画面」
① 定位    POST /generation/tasks/:id/timeline（按需，一次，纯本地零 API）
          成片抽音轨 → volumedetect 预分析 → silencedetect 三级阈值检测句间停顿
          → 语音段与台词句顺序配对 → 每句 [{index, text, start_ms, end_ms}] 缓存随任务
      ↓ （之后配置均读缓存，不再检测）
② 配置    用户在任意多句上挂片段（素材库选择/上传，可增删改）
          前端只记录 {sentence_index, media_url}
      ↓ 用户点「合成成片」
③ 提交    POST /generation/submit type=compose
          后端从 timeline 换算时间窗 + 校验（句号有效/窗口不重叠/片段含视频轨）
          → 创建异步 compose 任务（pending）
④ 执行    compose 任务（running）：
          下载源成片+片段 → ffprobe 预检 → 构造 ffmpeg 滤镜链
          （N 片段串 overlay + tpad 定格 + shortest=1；音频 -c:a copy 直拷、
            片段音轨不映射即剥离）→ 单进程编码 → 产物上传 OSS
      ↓
⑤ 完成    任务 succeeded + 产物 URL → 新成片入作品库（源片保留）
          用户可播放核对：片段窗口画面切换 / 口播音量全程不变 /
          窗口外口型保持对齐；如需调整可再次配置（以新成片为源，链式合成）
```

#### 流程特征

- **生成管线零改动**：①~⑤ 全部发生在成片之后，TTS/lipsync 现有流程一行不动
- **按需付费**：不插入片段的用户零额外成本（静音检测纯本地，零 API 成本）
- **无损链式**：每次 compose 的音频流都是直拷，链式多次合成音频不劣化

### 1.5 上游管线演进（主体形象视频前置——已确认，与本计划的关系）

口播管线上游将进行主体架构改造（独立计划，此处仅记录与本计划相关的已确认结论）：

```
创建主体 → 自动链式生成 10s 不说话的形象视频（reference2video subjects 模式，
           duration=10 无音频）→ 形象视频 URL 存主体记录
口播成片 → lipsync(主体形象视频 + TTS 音频)——不再每次 reference2video
```

**已确认的关键事实**（2026-08-29 用户确认 + 文档查证）：

1. **Vidu lipsync 自动延长**：输入视频短于音频时 Vidu 会自动延长视频适配音频——
   10s 形象视频配任意长口播音频直接可行（本计划此前的风险项消除）
2. **duration 支持查证**：q2 参考生 1-10s（10s 正好上限）、q3 参考生 3-16s
3. **主体隔离现状**：应用层隔离（任务列表 `WHERE tenant_id` 过滤，b 用户看不到
   a 的主体）；Vidu 层无隔离（同一企业 API key 下所有主体的 server_id 互通——
   独立主体表建设时必须显式带 `tenant_id` 列，不能依赖隐式隔离）
4. **官方主体**：`GET /subjects?ownership=system` 全量拉取展示给所有用户
   （公共主体，与隔离无关）；个人主体界面只读本地存储，不查 Vidu

**对本计划（B-Roll）的影响：零**——B-Roll 定位在"成片之后的后处理"，
上游成片方式怎么变（ref 一步成片 / 形象视频+lipsync 两步），产出的都是
"带音频轨的口播成片"，插入定位与合成逻辑完全不变。

---

## 2. 核心技术原理：时间轴不动，画面替换

**为什么"回来时口型继续对齐"是自动成立的**：

Vidu 对口型成片的口型由音频驱动，音画时间戳严格绑定。插入片段的本质是
**在最终视频上做画面层的覆盖（overlay），时间轴毫秒不变**：

- 音频轨：原样直拷（`-c:a copy`，全程零处理零音量变化）——口播从头说到尾，从未中断；片段音轨不映射即剥离
- 画面轨：时间窗内用片段帧替换口播帧，窗外自动回到口播帧（口型帧本来就在那里连续排着）

不存在"跳过口播帧再续上"的问题——没有任何帧被跳过或移动。这也是为什么本方案
**零对齐成本**：不需要任何口型补偿/重同步逻辑。

## 3. ffmpeg 方案设计（已实测验证）

### 3.1 方案对比与选型

| 方案 | 做法 | 结论 |
|---|---|---|
| **A. overlay 时间窗（选定）** | 一条 filter_complex：片段缩放适配后 `overlay=enable='between(t,t1,t2)'` 盖在口播画面上；音频 `volume` 时间窗 duck | 时间轴天然不动、单进程单次编码、多片段串 overlay 即可 |
| B. 分段 + concat | 按台词切主视频，插入段单独合成（片段画面+口播音频）再拼接 | 切点音频易出缝（需全段重编码音频）、临时文件多、时间轴需手工对齐——无收益 |

### 3.2 核心滤镜链（单片段模板）

```bash
ffmpeg -i main.mp4 -i broll.mp4 -filter_complex "
  [1:v]scale=W:H:force_original_aspect_ratio=increase,
       crop=W:H,setsar=1,
       tpad=stop_mode=clone:stop_duration=3600[b1];
  [0:v][b1]overlay=0:0:enable='between(t,START,END)':shortest=1[v]
" -map "[v]" -map 0:a:0 -c:v libx264 -preset veryfast -c:a copy out.mp4
```

2026-08-29 需求修订：**不做音量 duck、剥离片段音轨**——
- 音频只映射主视频流（`-map 0:a:0`），片段的音轨**天然被丢弃**（不映射即剥离，零滤镜）
- 口播音频全程零处理，`-c:a copy` 流直拷（零重编码零音质损失，合成更快）

各环节职责：

| 环节 | 作用 | 备注 |
|---|---|---|
| `scale=increase + crop` | 片段缩放适配主视频分辨率（cover 模式：填满裁切） | 竖屏口播 1080x1920 常态；留黑边模式（decrease+pad）可参数化 |
| `tpad=stop_mode=clone:stop_duration=3600` | **片段短于时间窗时定格最后一帧铺满** | 比循环重播自然；`stop_duration` 给超大值即可 |
| `overlay=enable='between(t,START,END)'` | 时间窗内画面替换，窗外自动回主画面 | **`shortest=1` 必须加**：防止 tpad 拉长的辅输入把输出时长拖长（实测踩坑，见 §3.6） |
| `-map 0:a:0 -c:a copy` | 口播音频原样直拷；片段音轨不映射即剥离 | 音频零重编码零损耗 |

### 3.3 多片段扩展（串 overlay + 音量表达式叠加）

N 个片段 → N 路 `-i` 输入，各自预处理后串接 overlay；音量表达式用**加法合成**：

```bash
ffmpeg -i main.mp4 -i b1.mp4 -i b2.mp4 -filter_complex "
  [1:v]...预处理...[s1]; [2:v]...预处理...[s2];
  [0:v][s1]overlay=0:0:enable='between(t,S1,E1)':shortest=1[v1];
  [v1][s2]overlay=0:0:enable='between(t,S2,E2)':shortest=1[v]
" -map "[v]" -map 0:a:0 -c:v libx264 -preset veryfast -c:a copy out.mp4
```

多片段各自独立时间窗串接 overlay；窗口重叠由用例层校验拒绝（§5.3）。

### 3.4 边界情况处理矩阵

| 情况 | 处理 | 实现 |
|---|---|---|
| 片段短于时间窗 | 定格最后一帧铺满 | `tpad=stop_mode=clone`（已验证） |
| 片段长于时间窗 | 截断（窗口外帧不显示） | overlay enable 天然行为（已验证） |
| 分辨率/宽高比不一致 | cover 填满裁切（默认） | `scale=increase + crop`；containing 模式参数化预留 |
| 片段无视频轨（图片） | 转为定格视频 | `-loop 1 -t 窗口时长 -i image.jpg` 输入形态 |
| 时间窗越界（超出成片时长） | 提交时校验拒绝 | 用例层校验（§5.3） |
| 窗口重叠 | 提交时校验拒绝 | 用例层校验 |

### 3.5 性能与资源预估

- 3 分钟 1080x1920@25fps、libx264 veryfast：实测级单片段全程重编码约 1~3 分钟（开发机）——**异步任务必然**（复用统一生成任务体系轮询）
- 内存/磁盘：单进程 ffmpeg，中间无临时文件（方案 A 全在滤镜内）——产物即用即传 OSS

### 3.6 实测验证记录（2026-08-29，ffmpeg 9.0.1）

用合成素材（12s 主视频 + 3s 片段，窗口 4~9s）验证：

- ✅ 输出时长精确 12.000s（**前提：overlay 加 `shortest=1`**——不加会被 tpad 拉到 23s，此为实测踩坑第一号）
- ✅ 窗口边界帧哈希验证：t=3.9 主画面 / t=4.1 片段 / t=8.9 片段（定格帧）/ t=9.1 主画面——**切换精确到帧**
- ✅ 3s 片段定格铺满 5s 窗口（tpad 生效）
- ✅ 音量 duck 表达式执行无错（eval=frame）

---

## 4. 台词时间轴来源（本计划最大决策点）

ffmpeg 需要 `between(t, START, END)` 的秒数，产品语义是"第 N 句台词"。时间轴从哪来：

### 现状（关键事实）

- 管线：文案 → **TTS 整段合成**（一个 ttsTaskId 一个音频文件）→ 口播画面（分身 ref 视频
  / 真人上传）→ Vidu lipsync 成片
- 台词在前端有分句列表（`raw_text_lines`），但**每句在音频中的起止时间当前不知道**

### 方案：静音/音量低点检测对齐（唯一方案，已实测验证）

> 2026-08-29 确认：不做备选方案（分句 TTS / ASR 时间戳均已否决——前者调用成本高，
> 后者当前 ASR 栈 SenseVoice 无时间戳能力），不过度设计。

**原理**：TTS / Vidu 文本生成对口型的音频中，**标点符号会被转换为停顿**（用户确认的
平台特性）——句间停顿即静音/音量低点。`ffmpeg silencedetect` 检测停顿边界得到
语音段起止时间，与台词句**按顺序配对**（只靠数量与顺序，零文本识别、零 API 依赖）。

- 完全本地零 API 成本 / 零服务商依赖
- 边界精度**毫秒级**（实测 2.20475s）
- 双生成入口通吃（TTS 管线 / Vidu 文本直生的音频都来自语音合成，标点停顿特性一致）
- 带底噪的用户音频用**自适应阈值**处理（见下）

### 实施要点（三级阈值策略 + 配对规则，已实测验证）

**实测数据**（2026-08-29）：

```
纯净 TTS（data/chinese_test.mp3 5.69s，-35dB 固定阈值）：
  → silence 2.20475~2.538583（句间停顿 334ms）→ 语音段 [0,2.204] [2.538,4.989]

带噪模拟（同音频混入粉噪，自适应阈值 mean-8dB=-34dB）：
  → silence 2.162417~2.54975（停顿 387ms）→ 与纯净版边界偏差仅 ±40ms ✓
```

1. **三级阈值策略**（音量低点检测——silencedetect 检测的本就是"低于阈值"的段，
   阈值即音量低点定义）：
   - 默认 `-35dB`：TTS / Vidu 生成音频（底噪极低，已验证）
   - **自适应**：`volumedetect` 预分析得 `mean_volume` → 阈值 = `mean - 8dB`
     （clamp 到 [-45, -25]）——适配**用户上传带底噪的音频**（无真静音但有音量低点，
     已验证）
   - 重试：段句数偏差 >±30% 时阈值放宽 3dB 重检一轮，仍失败报错（源音频不适合自动定位）
2. **配对规则**（语音段数 M 与台词句数 N）：
   - `M == N`（最常见）：顺序一一配对
   - `M > N`（句中停顿被切开）：按台词句字符数比例为界合并相邻段
   - `M < N`（相邻句连读）：按字符数比例拆分段
   - **窗口边界落点语义（切换在静音中，不在语音起止瞬间）**：
     窗口起点 = 上一静音段的中点（无上一静音则取 `start_ms - 150ms` 且 clamp ≥0）；
     窗口终点 = 本句语音结束（不含尾静音）。观感：上句话音刚落画面即切入片段，
     本句说完即回口播画面
3. **解析实现**：解析 silencedetect 的 stderr（`silence_start/end` 成对），首段前
   如有语音从 0 起、尾静音截断；产出 `TimelineLine[]` 随任务缓存，`force:true` 重跑
4. **无音频轨成片**：明确报错"该视频无音频，无法定位台词"
5. **台词行来源的自动分支**（三条音频路径统一覆盖，2026-08-29 补）：
   - **a. 任务 `params.script` 非空**（A 文本+音色 / B 文本直生路径）→ 上述配对规则
   - **b. `params.script` 为空**（C 上传音频路径）→ **ASR 自动分行**：
     全文 ASR 一次（现有 SenseVoice 栈纯文本能力即可）→ 按语音段时长比例把全文
     切成行 → 行与段天然一一对应（无需配对）。设计要点：
     * 全文一次调用而非逐段 ASR（省 N 次调用；短段识别率低的问题也规避）
     * 行边界（静音处）是精确锚点，段内文字错位 ±2 字不影响插入——
       **切换点由静音决定，不由文字决定**
     * 产出标记 `script_source: "asr"`（a 路径为 `"params"`）——前端展示
       "台词来自语音识别"提示
     * 用户可修正文字：timeline POST 支持 `lines_override`（只改各行 text、
       不改时间窗——错字修正不影响画面切换点）
   - **数据流规范**（客户端对接约定）：lipsync 提交时 A/B 路径在 `params.script`
     携带台词原文（一行一句）；C 路径不传 `script`（留空即触发定位时 ASR 分行）

## 5. 架构设计（整洁架构分层）

### 5.1 分层与依赖方向

```
handler（API 端点）
   ↓
usecase/videocompose（合成用例：校验/时间轴换算/任务编排）
   ↓                          ↓
port.MediaAVTool（扩展接口）   统一任务体系（GenerationTask type=compose）
   ↓
adapter/ffmpeg（滤镜链构造 + exec）→ 产物上传 mediaStore
```

与现有代码的接点：

| 层 | 现有资产 | 本期新增/扩展 |
|---|---|---|
| port | `MediaAVTool`（ExtractSubtitle/ExtractAudio/SegmentAudio） | 新增 `ComposeInsertSegments(ctx, mainVideoPath, segs []InsertSegment, outPath string) error`——**保持 ffmpeg 细节在 adapter** |
| usecase | 统一生成 `UnifiedSubmit`（type 路由） | 新增 `videocompose` 用例 + `compose` 任务类型（异步、轮询、回调与现有任务体系一致） |
| entity | GenerationTask / 素材库 | 任务参数携带 segments JSON；时间轴 `TimelineLine[]` |
| handler | generation 统一端点 | compose 类型参数校验 + 台词时间轴查询端点 |
| 前端 | LipSyncWizard 台词列表（raw_text_lines） | 每句"插入画面"交互 + 合成进度展示 |

### 5.2 数据模型

```go
// InsertSegment 插入片段指令（任务参数 JSON 内）。
type InsertSegment struct {
    SentenceIndex int     `json:"sentence_index"` // 台词句序号（0 基）
    StartMs       int64   `json:"start_ms"`       // 由时间轴换算（提交时后端计算，客户端只传句号）
    EndMs         int64   `json:"end_ms"`
    MediaURL      string  `json:"media_url"`      // 片段/图片（素材库或上传）
    FitMode       string  `json:"fit_mode"`       // cover（默认）/ contain
    // 2026-08-29 修订：无 DuckVolume——口播音量全程不变（需求 §1.2）
}

// TimelineLine 台词时间轴（静音/音量低点检测定位产物，随任务持久化）。
type TimelineLine struct {
    Index   int    `json:"index"`
    Text    string `json:"text"`
    StartMs int64  `json:"start_ms"`
    EndMs   int64  `json:"end_ms"`
}
```

### 5.3 用例层校验规则（提交时）

1. `sentence_index` 在时间轴范围内；窗口 `[start_ms, end_ms)` 非空
2. 各片段窗口**互不重叠**（重叠拒绝，提示具体冲突句）
3. `MediaURL` 可达（HEAD 预检，复用素材库校验）
4. **片段形态校验（ffprobe 预检）**：有视频流（含 mjpeg 单帧——即图片）→ 通过；
   图片形态（无音频流 + 单帧视频流 / MIME image/*）→ 自动走 `-loop 1` 输入形态；
   纯音频文件（无视频流）→ 拒绝
5. 源视频存在且已有时间轴（无时间轴 → 报错引导先调 POST timeline 定位）
6. `segments` 数量 ≤ 20（防滥用；窗口过多应考虑内容拆分）
7. 产物命名：源作品标题 + `·含插入` 后缀（多版本可区分）

### 5.4 API 设计（挂现有统一生成体系，2026-08-29 完善为五端点）

```
① POST /api/v1/generation/tasks/:id/timeline     台词时间轴定位（按需触发）
     请求: {"force": false}                      （true=重跑静音检测，忽略缓存）
           或 {"lines_override": [{"index":0,"text":"修正后的文字"}, ...]}
                                                （只改各行 text 不改时间窗——ASR 分行
                                                 的错字修正，画面切换点不受影响）
     响应: {"lines": [{"index":0,"text":"...","start_ms":0,"end_ms":3120}, ...],
            "script_source": "params" | "asr",   （台词来源：任务参数 | 语音识别自动分行）
            "align_mode": "direct" | "estimated", （配对方式：段句直配 | 比例合并/拆分）
            "located_at": "..."}
     错误: 成片无音频轨 / 段句数差异超限（源音频不适合自动定位）→ 可读错误

② GET  /api/v1/generation/tasks/:id/timeline     读取已定位时间轴
     未定位 → 404 + 引导语"请先调用 POST 定位"

③ POST /api/v1/generation/submit                 提交合成（type=compose，统一提交扩展）
     请求: {"type":"compose","source_task_id":"gen-xxx",
            "segments":[{"sentence_index":2,"media_url":"https://.../broll.mp4"}, ...]}
     说明: 客户端只传句号——时间窗由后端从 timeline 换算（防客户端伪造错位）；
           句号有效性/窗口重叠/片段含视频轨均在提交时校验（§5.3）
     响应: {"task_id":"gen-yyy","status":"pending"}（异步，同现有任务状态机）

④ GET  /api/v1/generation/tasks/:id              轮询（现有端点复用；compose 同构
     pending → running → succeeded/failed，产物 URL 在结果字段）

⑤ GET  /api/v1/generation/tasks/:id/preview?sentence_index=2
     （增强项，阶段四）窗口首帧截图——配置时所见即所得，前端缩略展示

⑥ POST /api/v1/generation/tasks/:id/cancel       （现有取消端点，compose 适配）
     compose 取消语义：排队中 → 信号量让位即取消（状态 cancelled）；
     编码中 → kill ffmpeg 进程（exec.Command 的 Process.Kill），任务标
     failed"已取消"（半截产物清理）。客户端对接：复用现有取消调用，无需新端点
```

**客户端对接小结**（前端不用改页面结构，只看端点约定）：
- lipsync 提交时带 `params.script`（A/B 路径一行一句文本；C 路径不传）
- timeline POST 首次定位 → 读响应渲染台词列表（`script_source=asr` 时显示
  "台词来自语音识别"提示；文字有错可带 `lines_override` 修正，时间窗不受影响）
- compose 提交只传 `{source_task_id, segments[{sentence_index, media_url}]}`
- 等待/取消复用现有任务端点

### 5.5 前端交互（LipSyncWizard / 作品详情）

1. 成片完成后（或作品库），台词列表每句 hover 显示「插入画面」
2. 点击 → 素材选择器（复用素材库）/ 上传 → 悬浮预览（起止时间只读展示）
3. 已插入的句显示片段缩略图标记，可删除/替换
4. 全部配置完 → 「合成成片」→ compose 任务进度 → 新成片入作品库（源片保留）

---

## 6. 实施步骤（四阶段，每阶段可独立验证）

### 阶段一：ffmpeg 合成器 + CLI 验证（后端地基）

- `adapter` 层实现 `ComposeInsertSegments`（滤镜链构造器：N 片段串接/边界处理/shortest 修复）
- `cmd/composedebug` CLI 工具：本地文件直接合成（对齐 dyfetchdebug 传统）
- **验证**：用真实口播成片 + 素材库片段，产出视频人工核对窗口切换/口型/音量

### 阶段二：静音/音量低点检测台词定位（管线零改动，§4）

- silencedetect 解析器：抽音轨（复用 ExtractAudio）→ volumedetect 预分析 →
  三级阈值检测 → 静音边界解析 → 语音段序列；配对规则（M==N 直配 / M>N 字符比例合并 / M<N 比例拆分）
- 台词行来源分支（§4-5）：params.script 配对 + C 路径 ASR 全文转写按时长比例自动分行
- timeline 定位/读取端点（POST/GET，含 lines_override 修正）+ 结果随任务缓存
- **验证**：真实 TTS 成片 + 带噪音频（模拟用户上传）各定位一例，播放器逐句跳转核对（±200ms）；
  三条音频路径各验一例（A script 配对 / B text 直生 / C 上传音频 ASR 分行）；
  无音频成片与段句数差异过大两条报错路径

### 阶段三：compose 用例 + API（任务体系接入）

- videocompose 用例（校验规则 §5.3 + 时间轴换算 + 异步任务 + 产物上传）
- 下载安全与资源控制（§10.1）：SSRF 校验复用 safeDownload 模式、500MB 上限、
  ffmpeg 并发信号量（2~3）、链式 timeline 继承、CRF/进度回调（§10.2）
- 统一提交 type=compose 路由
- **验证**：API 全链路——提交→轮询→产物 URL；校验规则单测（重叠/越界/无时间轴/
  内网 URL 拒绝错误路径）；链式合成 timeline 继承不重检测

### 阶段四：前端交互 + 端到端

- 台词列表插入交互 + 素材选择 + 进度展示
- **验证**：完整用户路径——文案→成片→配插入→合成→播放核对（口播口型/片段切换/背景声/多片段）

依赖顺序：一、二可并行；三依赖一+二；四依赖三。

## 7. 风险与踩坑预判

| # | 风险 | 缓解 |
|---|---|---|
| 1 | **overlay 不加 shortest=1 输出被 tpad 拉长**（已实测踩坑） | 滤镜构造器固定携带；CLI 工具回归用例固化 |
| 2 | 静音检测参数误切（句间停顿 <150ms 或句中停顿 >150ms） | d 参数可调；段/句数比例配对兜底合并拆分；差异 >±30% 放宽阈值重检一轮后报错 |
| 3 | 用户上传音频带底噪（无真静音） | 自适应阈值 `mean_volume-8dB`（已实测：噪声音频边界偏差仅 ±40ms） |
| 4 | Vidu 成片时长与音频时长有出入（±0.5s） | 时间轴以**成片实际时长**为基准等比缩放窗口；窗口 clamp 到成片时长内 |
| 5 | 片段格式杂（webm/mov/图片） | 输入探测（ffprobe）→ 图片走 loop 输入形态；编码统一 h264+aac |
| 6 | 3 分钟视频重编码耗时 | 异步任务 + 进度日志；veryfast 预设；超时保护（10 分钟） |
| 7 | 已有成片无时间轴 | 首次配置时按需定位（POST timeline），不阻塞已有作品的其他操作 |

## 8. 验收标准

1. **口型**：插入片段前后（及全程）口播画面口型与音频对齐，肉眼无可感偏差
2. **切换**（分级验收）：段句数一致（M==N 直配）时 ±200ms 内；行内多句触发
   比例合并/拆分时 ±400ms 内（静音检测时间轴，实测毫秒级；分级语义见 §10.1-5）
3. **音频**：口播音频全程原样（波形逐字节一致级——`-c:a copy` 直拷）；
   片段原声不出现（音轨剥离）；无爆音/咔哒
4. **时长**：合成产物总时长与源成片一致（±1 帧）
5. **多片段**：≥3 个片段、含短片段（定格）/长片段（截断）/异分辨率（cover 适配）组合用例通过
6. **任务**：compose 提交→轮询→产物可播放；校验规则错误路径全部返回可读错误
7. **前端**：台词句插入配置→合成→作品库可见新成片，源片保留

## 9. 已确认的决策（未确认项已删除——不做超前设计）

| # | 决策 | 状态 |
|---|---|---|
| 1 | 片段原声：**剥离**（音频只映射主视频流） | ✅ 已确认 |
| 2 | 口播音频：**全程原样、音量不变**（`-c:a copy` 直拷） | ✅ 已确认 |
| 3 | 片段短于窗口：**定格最后一帧**（tpad，已实测） | ✅ 已确认 |
| 4 | 适配模式：**cover 填满裁切**（与口播画面一致无黑边） | ✅ 已确认 |
| 5 | 插入时机：**成片后配置**（两段式——先成片后插入；生成管线零改动） | ✅ 已确认 |
| 6 | 台词定位：**静音/音量低点检测**（三级阈值；无备选方案） | ✅ 已确认 |
| 7 | 转场特效 / 片段原声混入：**本期不做** | ✅ 已确认（边界） |

## 10. 实施细节（2026-08-29 自查补充并完善为执行设计）

### 10.1 P0：安全与正确性（执行设计）

**① compose 下载的 SSRF 防护 + 大小上限**

compose 输入的源成片/片段 URL 来自用户提交，服务端拉取前必须过与
`videotranscript.safeDownload` 同级的校验（建议直接抽公共辅助避免两份漂移）：

```go
// videocompose 用例内（阶段三实现）：
// 1. url.Parse：仅 http/https
// 2. net.LookupIP(host)：逐 IP 拒绝 Loopback/Private/LinkLocalUnicast/Unspecified
// 3. HEAD 预检 Content-Length ≤ 500MB（无 HEAD 降级 GET 流式截断保护）
// 4. 源成片与每个片段 URL 都过同一套校验（片段数量少直接串行）
```

**② ffmpeg 并发闸门**

```go
// adapter 层包级信号量（参照 ytdlpSem 模式）：
var composeSem = make(chan struct{}, 2)  // 2~3 并发（依服务器核数调）
// 用例执行段：select 获取（ctx 取消让位退出）；排队时任务 running +
// 日志"排队中"（前端进度条显示等待状态而非卡死）
```

**③ TimelineLine 落库**

```sql
-- 076_generation_tasks_timeline.sql
ALTER TABLE generation_tasks ADD COLUMN timeline_json TEXT NULL COMMENT
  '台词时间轴（静音检测定位产物 JSON 数组；NULL=未定位）';
ALTER TABLE generation_tasks ADD COLUMN timeline_located_at DATETIME(3) NULL COMMENT
  '最近一次定位时间（force 重跑时更新）';
```

- entity.GenerationTask 加 `TimelineJSON string` / `TimelineLocatedAt *time.Time`，
  PO 映射双向；`NULL` 与空串都视为"未定位"
- timeline POST 端点写入、GET 端点读取、compose 提交换算——全部走任务表这两列，
  不引入新表

**④ 链式合成的时间轴继承（禁止重检测）**

```go
// submitCompose 内（提交时执行一次，运行期不再碰 timeline）：
if source.Type == "compose" && source.TimelineJSON != "" {
    task.TimelineJSON = source.TimelineJSON      // 音频直拷→时间轴必然不变
    task.Params["script"] = source.Params["script"] // 台词行列表随之继承
}
// 源为 lipsync/text2video 等首代成片且 timeline 为空 → 校验规则 §5.3-5 报错
// 引导先调 POST timeline
```

**⑤ 台词"句" = 文案行 的精度语义**

- 检测器输出随 timeline 增加 `align_mode` 字段：`"direct"`（M==N 直配）/
  `"estimated"`（触发过合并/拆分）——持久化进 timeline_json 首行元信息
- 前端配置界面：台词列表顶部固定提示**"插入点按文案行对齐，一行一句效果最佳"**；
  `estimated` 模式下相关句旁显示"边界为估算"角标
- 验收分级见 §8 第 2 条（direct ±200ms / estimated ±400ms）

### 10.2 P1：暂不处理（实施时再定，避免超前设计）

| # | 项 | 结论 |
|---|---|---|
| 6 | 计费/配额策略 | ⏸ **暂时不处理**——本地 ffmpeg 无 API 成本，首版免费不限 |
| 7 | 编码参数（CRF/进度回调） | ⏸ **暂时不处理**——用 x264 默认 CRF；进度条首版只显阶段不显百分比 |
| 8 | 帧率适配 | ✅ 确认项——overlay 跟随主输入时钟，无需处理 |

### 10.3 P2：小细节（执行规则）

**⑨ 首句 start 语义**：解析 silencedetect 输出时——若首个 `silence_start` 为 0
（片头静音），首语音段起点取首个 `silence_end`；否则首段起点为 0。
一律取实际检测值，不做任何"假设从 0 开始"。

**⑩ timeline 端点权限**：handler 用 `repo.FindByID(ctx, tenantID, taskID)`
取任务（与 Cancel/DeleteTask 同款——repo 层已带租户过滤）；非本租户任务
表现同"不存在"（404），不泄露存在性。

### 10.4 已核实无需额外工作

- **作品库收录**：作品库是三源聚合（`g-{taskID}` 直接展示），compose 任务
  succeeded 后自动出现在作品库，无需收录动作
