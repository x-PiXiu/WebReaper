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
① 定位    POST /generation/tasks/:id/timeline（按需，一次）
          成片抽音轨 → ASR(verbose_json 时间戳) → 台词句序列对齐
          → 每句 [{index, text, start_ms, end_ms}] 缓存随任务
      ↓ （之后配置均读缓存，不再调 ASR）
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
- **按需付费**：不插入片段的用户零额外成本（ASR 只在首次配置时调一次）
- **无损链式**：每次 compose 的音频流都是直拷，链式多次合成音频不劣化

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

### 方案对比（2026-08-29 修订：主方案改为按需 ASR 定位——用户反馈分句 TTS 调用成本过高）

| 方案 | 做法 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| **C2. 按需 ASR 时间戳对齐（选定）** | 管线完全不动；用户需要插入片段时，对**成片音频**跑一次带时间戳的 ASR（OpenAI 兼容 `response_format=verbose_json` → segments[{text,start,end}]），与台词分句文本做序列对齐得出每句 [start_ms, end_ms]，结果随任务缓存 | **成本：仅按需一次 ASR 调用**；管线零改动；**双生成入口通吃**（文本直生视频 / 先 TTS 再 lipsync 的音频都能定位——定位发生在最终成片上，与音频生产方式无关）；时间轴与成片严格自洽 | whisper 类 segments 时间戳精度 ±100ms 级（满足 ±200ms 验收）；需要扩展现有 ASR 接口（当前只返回纯文本，新增 verbose_json 方法） | ✅ 主方案 |
| C1. 分句 TTS | 每句单独 TTS → 拼接 + 时间轴副产品 | 时间轴毫秒精确 | **每句一次 TTS 调用**（30 句=30 次，成本/限流高风险——用户已否决）；句间停顿固定语气不连贯；需改造管线 | ⬇️ 降为备选（句级音色/语速调优需求出现时再启用） |
| C3. 字符时长估算 | 字符数 × 平均语速 | 零成本 | 误差 ±1s 级，切换点会切在字中间 | ❌ 否决 |

### C2 实施要点

**对齐示例（一句话讲透原理）**：台词文字是已知的（生成时输入），ASR 补的只是
"每句话在音频的哪个时间段被说出来"——

| 台词句（已知） | ASR 返回段（带时间戳） | 对齐结果 |
|---|---|---|
| 第0句"大家好今天给大家介绍" | `{text:"大家好今天给大家介绍", start:0.00, end:3.12}` | `start_ms=0, end_ms=3120` |
| 第1句"我们家的酸菜鱼是现杀的" | `{text:"我们家的酸菜鱼是现杀的", start:3.40, end:6.85}` | `start_ms=3400, end_ms=6850` |

识别内容有误差不影响——找的是**已知文字**的位置（编辑距离兜底），不是在猜内容。

1. **ASR 接口扩展**：`port.SpeechTranscriber` 新增 `TranscribeTimestamped(ctx, audioPath, mime, fileSize) ([]TranscriptSegment, error)`
   （`TranscriptSegment{text, start_ms, end_ms}`）——adapter 层 OpenAI 兼容请求加
   `response_format=verbose_json` 并解析 `segments[]`（硅基流动 whisper 类通用支持；
   不支持的服务商明确报错引导配置）
2. **定位触发**：`POST /generation/tasks/:id/timeline` 按需触发（不是每任务必做）；
   成片音频抽轨（复用 MediaAVTool.ExtractAudio）→ ASR → 对齐 → `TimelineLine[]` 随任务持久化；
   同片重复配置读缓存，`force:true` 重跑
3. **对齐算法**（台词句列表与 ASR segments 的序列对齐）：
   - 文本归一化：去标点/空白/大小写（中英混合各自规则）
   - ASR segments 文本顺序连接成整串，记录每段在整串中的字符区间 → 累计时间映射表
   - 台词每句在整串中顺序查找（indexOf，找不到时编辑距离模糊匹配兜底）→ 句字符区间 →
     映射为 [start_ms, end_ms]（区间跨多个 segment 时取首段 start 与末段 end）
   - ASR 切分粒度与台词句不一致（ASR 按停顿切、台词按标点切）不影响：对齐以字符区间映射，
     不要求 segment 与句一一对应
4. **双入口适配**：文本直生视频的任务成片同样有音频轨——定位逻辑入口无关；
   无音频轨的成片（纯画面）明确报错"该视频无音频，无法定位台词"

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

// TimelineLine 台词时间轴（分句 TTS 副产品，随任务持久化）。
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
4. 片段含视频轨校验（ffprobe 预检——纯音频文件拒绝；图片走 loop 输入形态）
5. 源视频存在且已有时间轴（无时间轴 → 明确报错引导重生成，见 §4 兼容策略）

### 5.4 API 设计（挂现有统一生成体系，2026-08-29 完善为五端点）

```
① POST /api/v1/generation/tasks/:id/timeline     台词时间轴定位（按需触发）
     请求: {"force": false}                      （true=重跑 ASR 对齐，忽略缓存）
     响应: {"lines": [{"index":0,"text":"...","start_ms":0,"end_ms":3120}, ...],
            "source": "asr", "located_at": "..."}
     错误: 成片无音频轨 / ASR 服务商不支持 verbose_json → 可读错误

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
```

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

### 阶段二：ASR 台词定位（管线零改动，§4 C2）

- ASR 接口扩展：`TranscribeTimestamped`（verbose_json + segments 解析）
- 对齐算法实现 + 文本归一化（含编辑距离兜底）
- timeline 定位/读取端点（POST/GET）+ 结果随任务缓存
- **验证**：真实成片定位后，播放器按时间轴逐句跳转核对（±200ms）；
  两种生成入口（文本直生 / TTS+lipsync）各验一例；无音频成片报错路径

### 阶段三：compose 用例 + API（任务体系接入）

- videocompose 用例（校验规则 §5.3 + 时间轴换算 + 异步任务 + 产物上传）
- 统一提交 type=compose 路由
- **验证**：API 全链路——提交→轮询→产物 URL；校验规则单测（重叠/越界/无时间轴错误路径）

### 阶段四：前端交互 + 端到端

- 台词列表插入交互 + 素材选择 + 进度展示
- **验证**：完整用户路径——文案→成片→配插入→合成→播放核对（口播口型/片段切换/背景声/多片段）

依赖顺序：一、二可并行；三依赖一+二；四依赖三。

## 7. 风险与踩坑预判

| # | 风险 | 缓解 |
|---|---|---|
| 1 | **overlay 不加 shortest=1 输出被 tpad 拉长**（已实测踩坑） | 滤镜构造器固定携带；CLI 工具回归用例固化 |
| 2 | ASR 服务商不支持 verbose_json 时间戳 | 提交定位时探测能力，不支持给明确配置引导；可配置专用 whisper 端点 |
| 3 | ASR 时间戳精度/识别误差导致对齐偏差 | 文本归一化+编辑距离兜底；±100ms 级满足验收；`force` 重跑支持 |
| 4 | Vidu 成片时长与音频时长有出入（±0.5s） | 时间轴以**成片实际时长**为基准等比缩放窗口；窗口 clamp 到成片时长内 |
| 5 | 片段格式杂（webm/mov/图片） | 输入探测（ffprobe）→ 图片走 loop 输入形态；编码统一 h264+aac |
| 6 | 3 分钟视频重编码耗时 | 异步任务 + 进度日志；veryfast 预设；超时保护（10 分钟） |
| 7 | 已有整段 TTS 任务无时间轴 | 明确报错引导重新生成（分句开关默认开）；C2 ASR 方案作为后续兼容层 |

## 8. 验收标准

1. **口型**：插入片段前后（及全程）口播画面口型与音频对齐，肉眼无可感偏差
2. **切换**：片段进出发生在台词边界 ±200ms 内（时间轴来自分句 TTS 精确值）
3. **音频**：口播音频全程原样（波形逐字节一致级——`-c:a copy` 直拷）；
   片段原声不出现（音轨剥离）；无爆音/咔哒
4. **时长**：合成产物总时长与源成片一致（±1 帧）
5. **多片段**：≥3 个片段、含短片段（定格）/长片段（截断）/异分辨率（cover 适配）组合用例通过
6. **任务**：compose 提交→轮询→产物可播放；校验规则错误路径全部返回可读错误
7. **前端**：台词句插入配置→合成→作品库可见新成片，源片保留

## 9. 待确认的产品决策点

| # | 决策 | 建议 default |
|---|---|---|
| 1 | 片段原声处理 | **已定：剥离**（2026-08-29 用户需求修订）——音频只映射主视频流；后续如需混入再加 `mix_broll_audio` 开关 |
| 2 | 片段短于窗口：定格 vs 循环 | 定格（tpad，已验证） |
| 3 | 分句 TTS 句间停顿时长 | 300ms 可配置 |
| 4 | 适配模式 | cover 默认（与口播画面一致无黑边）；contain 留参数 |
| 5 | 插入时机：成片后配置 vs 生成前配置 | 成片后（作品级操作，不动生成管线语义；时间轴来自生成任务） |
