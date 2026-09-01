# 31 号设计 v4：音色物化架构 + 口播画面复用

> 日期：2026-09-01（v4：**定案只保留"画面预生成、口播复用"一条路**——删除 v3 的"口播时现场生成静默画面"分支；对齐 01/23 号业务文档目标态）
> 目标：其他业务无隔离使用音色（样本为王）+ 唯一 ID 刚性场景（lip_sync 文本驱动）有系统保障 + 口播路径与产品设计文档一致。

---

## 一、统一模型（与 01/23 号业务文档一致）

```
口播视频 = 画面 × 语音

画面 = 分身资产，阶段0 一次性预生成（23号 §2：上传形象照 → 主体注册 →
       自动链式 10s 不说话形象视频 → 用户预览/删了重建/按需选择）
       ※ 口播时不生成任何画面任务——只保留复用一条路（2026-09-01 定案）

语音注入（唯一出口 lip_sync，复用预生成画面）：
  A 文本+音色 → tts 样本合成 → lip_sync(audio_url)   [两段；备胎道同构]
  B 文本直生  → lip_sync(text, voice_id)             [单段；voice_id 经 L2 保障，
                                                        默认=分身绑定音色（23号 B 路径）]
  C 上传音频  → lip_sync(audio_url)                  [单段；免音色]

真人模式：用户上传出镜视频（建议无人声）→ 同 B/C 路径。
之后：B-Roll 合成（compose）叠画面，音轨直拷。
```

**推论**：
1. **静默是"预生成资产"的属性**：阶段0 形象视频生成即静默（audio=false，见 §4.1）；用户直发 reference2video（创意素材）保留原生音频能力；
2. **voice_id 唯一消费点 = lip_sync 文本驱动**（B 路径），由 L2/L4 保障；
3. 分身+（文案|音频）一次提交 → 解析分身形象视频 → **直接提交 lip_sync**（单段）；形象视频缺失 → 显式报错引导回分身管理，**不回退现场生成**；
4. N1 守卫升级：开关开启时 subjects+音频放行（服务端复用路径处理）。

## 二、设计依据（全部已验证）

| # | 事实 | 验证方式 |
|---|------|---------|
| F1 | 两厂商都支持样本合成（Vidu audio-clone 收 URL / MiMo voiceclone 收 base64） | 代码+官方文档 |
| F2 | Vidu 同 ID 复注册幂等；7 天不用即删 | Vidu 文档+代码注释 |
| F3 | MiMo voiceclone 当纯 TTS：质量等价（节奏差 2.9%）、2049 字长文本成功 | 双实测（data/mimo-voice-test/） |
| F4 | Vidu lip_sync 文本驱动只收 voice_id——**唯一** ID 刚性端点 | 官方文档 |
| F5 | Vidu voice_id 可自定义（8-256 位字母开头）、账号命名空间隔离 | Vidu 文档 |
| F6 | 样本永久化三通道保障已上线 | 缺口 A/B 修复 |
| F7 | reference2video 默认模型 viduq3-turbo；voice_id 仅 Q2/Q1/2.0 生效；Q3 `AudioDefault: true` | 代码+官方参数文档 |
| F8 | 23 号产品设计明确"分身模式均无'口播画面'步骤——形象视频已在分身上"；01 号阶段3 即 `lip_sync(video_url,...)` 复用形象视频 | 业务文档（本定案的直接依据） |
| F9 | subject_assets 资产自带 avatar_video_url 与 voice_id 列（FindByServerID 可查） | 代码/表结构 |

## 三、四层架构

```
L1 资产层（不变）：generation_voices + 永久 sample_url；voice_id=我方PK=Vidu注册ID
    + subject_assets（分身画面资产：avatar_video_url + 绑定 voice_id）
L2 物化层（唯一入口 VoiceMaterializer.Ensure）：
    form=sample → 校验样本就绪 → 按厂商返回 URL/base64
    form=vidu_id → 租户校验 → 窗口检查(6天,可配) → 幂等注册/续期 → 返回 ID
    内置 singleflight 并发去重
L3 路由层（各端点音色形态声明）：
    MiMo tts        → voiceclone+样本（一律；9 预置白名单直传为优化）
    Vidu tts        → 窗口命中: tts+ID(≤10000字) / 未命中: audio-clone+样本(≤1000字,顺带注册)
    lip_sync 文本   → vidu_id 形态（L2/L4 保障）★唯一 ID 消费点
    lip_sync 音频   → 无音色
    reference2video → 仅阶段0形象视频（内部链 audio=false）与用户直发；口播路径不使用
L4 保险层：①默认克隆路由Vidu ②创建后异步预热 ③按需保障 ④故障自愈(双路径:applyStatus+Submit失败) ⑤备胎道 gen_lipsync_two_step（=A 路径）
```

## 四、画面复用与静默化

### 4.1 静默化——只有两处 reference2video 内部调用点

| 调用来源 | 音频策略 | 理由 |
|---------|---------|------|
| **阶段0 链式形象视频** maybeChainAvatarVideo | audio=false（已实施） | 形象视频=分身画面资产，必须不说话（lip_sync 输入；23号 §2"10s 不说话的形象视频"） |
| 环境主体组合出镜 | audio=false（待审计补） | 同上，画面素材 |
| **用户直发：多图参考生成（selector 情况6）** | **尊重用户意愿**：params.audio 透传，未指定保持上游默认 | 用户生成创意素材可能就是要带声画面——Vidu 原生能力，平台不代用户做主 |
| **用户直发：工具台高级表单/Agent/API** | 同上；voice_id（Q2 生效）为可选开放项（经 L2 保障） | 高级用户进阶玩法，默认收起不删能力 |

**注**：v3 的"口播链第①步 audio=false + 画面模板 prompt"已随现场生成分支一并删除——口播路径不再生成画面（F8）。`gen_lipsync_scene_prompt` 配置键已移除。

### 4.2 口播画面复用路径（gen_lipsync_auto_chain，默认关）

```
提交 subjects +（文案 | 音频）［开关开启］
  → selector 装配 __chain_* 标记（文案/音频URL/显式音色）
  → UnifiedSubmit 拦截：extractLipsyncChain 命中
  → submitReuseLipSync：
      subjects[0].server_id → subject_assets.FindByServerID
        ├─ 分身不存在        → 显式报错"分身不存在或已删除"
        ├─ 分身已下架        → 显式报错"已下架，请重新选择"
        ├─ 形象视频未就绪    → 显式报错"请回分身管理等待生成完成（或删除重建）"
        │   ※ 绝不回退现场生成——只保留复用一条路（本定案）
        └─ 可用 → 直接提交 lip_sync：
             C 音频驱动：{video_url: avatar_video_url, audio_url}
             B 文本驱动：{video_url, text, voice_id: 显式选定 || 分身绑定音色}
             （voice_id 经 Submit 内 L2 保障；B-Roll 挂本任务=链尾）
```

- **开关关闭（默认，过渡期）**：现状行为——分身+文本单步 ref2video 端内合成（音色选择不生效，待开关替代）、分身+音频 N1 拒绝引导手动两步。前端向导适配复用路径后开启，届时旧路径淘汰。
- **备胎道（gen_lipsync_two_step，默认关）**：B 路径注册类失败自动降级为 A 路径（tts 样本合成 → 音频驱动 lip_sync），零厂商 ID 依赖；用户错误（越权/停用/未就绪）不降级直接报错。已知限制：备胎道链头为 tts，暂不携带单阶段 B-Roll。

### 4.3 时长匹配（复用模式下的首要约束）

lip_sync 成片时长 ≈ 画面时长；形象视频固定 10s → **B/C 路径口播语音量约 ≤40-50 字**。更长的文案：
- 待实测 #9-B：Vidu lip_sync 对超长文本/音频的行为（截断？报错？画面延展？）——决定长文案方案；
- 候选方案：a) 阶段0 支持生成长时长形象视频（Q3 上限 16s）；b) 长文案分段成片拼接；c) 引导真人模式上传长视频。
- 在实测结论前，向导侧应提示时长匹配建议（23号真人模式已有此提示）。

### 4.4 上传画面的人声检测（待第三批）

真人模式上传视频若含人声，text 驱动 lip_sync 是二次驱动效果劣化。复用 `DetectSpeechSegments` 提交时检测提示；需 API warning 契约（单独设计）。

## 五、关键机制（继承）

- **一物两用不建映射表**：我方 PK 即 Vidu 注册 ID；第二家注册制厂商出现再升格映射表。
- **singleflight**：同音色并发到期只触发一次注册（进程内；多实例幂等兜底）。
- **租户归属校验**：clone 行仅归属租户可用（tts 改写路径与 ensure 双覆盖）。
- **错误语义表**：sample_not_ready / registration_failed / voice_not_found_on_vendor(先自愈再报错) / voice_disabled / voice_not_yours——无静默替换。
- **FAQ**：Vidu"使用"=调用打到 Vidu；MiMo 样本路径不续期 Vidu 注册——L4 按需保障不可省略。

## 六、审查缺口决议表（两轮 19 项，v4 修订）

| # | 缺口 | 决议 |
|---|------|------|
| 1 | ref2video 补传 voice_id 在 Q3 默认模型下无效 | ✅ 作废——口播路径不使用 ref2video（画面复用定案） |
| 2 | Vidu 无音色注销 API | 下架即阻断本平台；厂商侧 7 天自然过期 |
| 3 | Vidu 克隆时延未实测 | **待实测**；暂定注册超时 30s，超时→备胎道 |
| 4 | 注册调用计费归属 | 平台运维成本，不向用户计费 |
| 5 | 声音滥用零审核 | P1：克隆授权声明 + 管理端克隆音色审计 |
| 6 | 无 Feature flag | gen_voice_materializer_enabled（物化层）+ gen_lipsync_auto_chain（复用路径）+ gen_lipsync_two_step（备胎道） |
| 7 | 观测性 | 结构化日志 + 管理端"注册异常音色"筛选 |
| 8 | 样本重录缓存失效 | Upsert 天然覆盖（非 Vidu 重克隆时间戳置空） |
| 9 | 单任务降级 | 注册失败/超时 → 自动转备胎道（A 路径），结果标注 |
| 10 | TTS 窗口感知 | 已实施（Vidu ≤1000 样本化 / >1000 tts+ID 注册保障） |
| 11 | singleflight 进程内 | 文档写明：多实例幂等兜底 |
| 12 | 极端情况演练方式 | mock provider 故障注入 |
| 13 | 静默化 audio=false | v4 修订：仅阶段0形象视频+组合出镜两处内部调用点；口播链①已删 |
| 14 | prompt 语义分离 | v4 修订：随现场生成分支删除而消失——口播文案只进 lip_sync text，形象视频 prompt 在阶段0（gen_default_avatar_prompt） |
| 15 | N1 守卫升级 | 已实施（开关开启放行，复用路径处理音频） |
| 16 | 链式取消/重跑语义 | v4 修订：复用路径单段无链取消问题；备胎道链取消=终止后续 |
| 17 | compose 挂载点 | v4 修订：复用路径 B-Roll 直接挂 lip_sync 任务（链尾即本任务） |
| 18 | 长文案时长匹配 | §4.3——复用模式下升级为首要约束，待实测 |
| 19 | 上传画面含人声 | §4.4 待第三批（warning 契约） |

## 七、极端情况应对（v4 修订）

| # | 极端情况 | 承接机制 |
|---|---------|---------|
| 1 | 窗口误判"有效"但厂商已删 | 双路径自愈（applyStatus+Submit 失败均触发缓存失效）→ 重试即重建 |
| 2 | Vidu 政策变更（7天→3天） | 窗口配置化 + 自愈兜底 |
| 3 | 创建时 Vidu 故障 | 创建走 MiMo 照常；异步预热失败由按需保障兜底 |
| 4 | 口播时注册失败 | 备胎道自动降级（A 路径）或显式报错可重试 |
| 5 | 样本转存未完成 | 显式"准备中"错误；补偿任务自愈 |
| 6 | 租户盗用他人音色 ID | L2+改写路径双租户校验 |
| 7 | 音色被停用后旧草稿引用 | 状态校验显式报错 |
| 8 | voice_id 撞 Vidu 预置名 | PK+前缀+账号命名空间 |
| 9 | 超长文本 | MiMo voiceclone 实测 2049 字 OK；Vidu>1000 走 tts+ID（10000 上限） |
| 10 | 同音色高并发到期重建 | singleflight |
| 11 | Vidu 下线注册能力 | 备胎道全局切换（A 路径零 ID） |
| 12 | 第二家注册制厂商 | L2 加形态声明线性扩展 |
| 13 | MiMo voiceclone 收费 | 9 预置白名单 + 能力路由切回 Vidu |
| 14 | 用户在 Vidu 控制台手删音色 | 同 #1 自愈重建 |
| 15 | 删除克隆残留 | DeleteTask 联动清理（已实施） |
| 16 | **形象视频未生成就来口播** | 显式报错引导回分身管理（不回退现场生成） |
| 17 | **形象视频转存地址失效** | stored_url 永久化（三通道保障）；复用前 http(s) 校验 |
| 18 | **组合出镜（多主体）复用** | subjects[0] 主分身形象视频；多主体组合画面属阶段0资产范畴（后续按需扩展） |

## 八、实施清单与进度（v4）

| 步骤 | 内容 | 状态 |
|------|------|------|
| 1 | 085 迁移 + 配置键 | ✅ |
| 2 | VoiceMaterializer L2（Ensure/注册/失效/预热/singleflight） | ✅ |
| 3 | L3 路由（MiMo 一律样本化删静默降级 / Vidu 窗口感知 / lip_sync 保障） | ✅ |
| 4 | L4 保险（物化时间戳/异步预热/双路径自愈/备胎道） | ✅ |
| 5 | 静默化：阶段0形象视频 audio=false + 适配器 audio 透传 | ✅（组合出镜待审计） |
| 6 | **画面复用路径**：selector __chain 装配 + submitReuseLipSync（B/C 单段、分身绑定音色、缺失显式报错、B-Roll 挂链尾） | ✅（v4 重写，现场生成分支已删） |
| 7 | 删除克隆联动清理 | ✅ |
| 8 | 上传画面人声检测 | ⏳ 第三批（warning 契约） |
| 9 | 两项实测（Vidu 克隆时延 / lip_sync 超长文本） | ⏳ 部署环境 |
| 10 | 回归：18 条极端情况 + 全音色×全路径矩阵 + 备胎道验收 | ⏳ 随前端适配 |

**单测**（全部通过）：改写厂商感知（含 MiMo 超长/越权）+ L2 六场景 + 复用路径四场景（B 文本默认分身音色/C 音频优先/缺失报错/subjects 解析容错）+ 开关联动（标记装配/N1 放行）+ 备胎道触发判定。

**第三批待办**：前端向导适配复用路径后开启 gen_lipsync_auto_chain（淘汰旧单步路径）；§4.3 长文案方案（待实测）；§4.4 人声检测；admin 平台音色预热接线；组合出镜静默审计；错误码收紧。

## 九、与现有代码的映射

| 现有代码 | 处置 |
|---------|------|
| maybeRewriteSampleSynthesis（generation.go） | 已收编 L2/L3 厂商感知 |
| mimoVoiceID 降级分支（tts_as_provider.go） | 已删除改显式报错 |
| buildLipSyncTextParams voice_id 直传 | 已改经 L2 保障 |
| subjects 分支（endpoint_selector.go） | 开关开启→__chain 装配（复用路径）；关闭→现状过渡 |
| chainLipSyncAfterScene（v3 现场生成链） | **已整体删除**（v4 定案：只保留复用一条路） |
| maybeChainAvatarVideo（阶段0 形象视频） | 保留 + audio=false（画面资产的静默源头） |
| useLipSyncPipeline（前端 N1 两步链） | 过渡期保留；复用开关开启后由服务端单段替代，前端适配为跟踪 lip_sync 任务 |
