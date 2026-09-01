import { create } from 'zustand'
import { persist } from 'zustand/middleware'

/** 爆款获客双轨：发视频 / 发图文 */
export type ComposeTrack = 'video' | 'graphic' | 'lipsync'

/**
 * 多模块共享草稿（含向导）：各模块读写同一份状态。
 *
 * 08 计划 R7：向导链路的步骤游标与各步 taskId 挂在 draft 里——
 * 刷新/关闭页面后可恢复进度（TTS→参考生→对口型各自的 taskId + 当前步骤）。
 */
export type ComposeDraft = {
  /** 当前创作轨道 */
  track: ComposeTrack
  brandId?: string
  sourceUrl?: string
  refTitle?: string
  hotPoint?: string
  transcript?: string
  script?: string
  rewritten?: string
  titles: string[]
  topics: string[]
  selectedTitle?: string
  voiceUrl?: string
  voiceTaskId?: string
  avatarVideoUrl?: string
  avatarTaskId?: string
  editedVideoUrl?: string
  coverUrl?: string
  coverAccent?: string
  /** 图文配图 URL 列表 */
  imageUrls: string[]
  /** 当前步骤（0-based，持久化续写） */
  stepIndex?: number
  /** 关联 OptimizedContent ID（AI 生成/润色后落作品库） */
  contentId?: string
  /** 封面生成任务 ID */
  coverTaskId?: string
  /** 配图生成任务 ID 列表（待回填） */
  imageTaskIds: string[]
  lastUpdatedAt?: string

  // ---- 向导链路状态（08 R7——刷新不丢）----
  wizardStep?: number          // 当前步骤游标（0-4）
  wizardPresence?: 'real' | 'avatar'
  wizardTopic?: string         // 一句话主题
  wizardScript?: string        // 编辑框文案
  wizardCleanText?: string     // 清洗版原文
  wizardVoiceId?: string       // 选定音色
  wizardRealVideoUrl?: string  // 真人出镜视频 URL
  wizardSubjectId?: string     // 分身 server_id
  wizardIntent?: string        // 分身场景意图
  wizardVideoTaskId?: string   // 成片视频任务 ID（29 号单步：lip_sync / reference2video）
  wizardResultUrl?: string     // 成片 URL
  wizardAudioSource?: 'text' | 'upload' // 音频模式（01 号业务线：text=文本驱动默认 / upload=上传音频）
  wizardSchema?: number      // 向导步骤 schema 版本（4=四步式；缺省视为旧 5 步，读取时迁移一次）
  wizardUploadedAudioUrl?: string                 // 路径 C：已录音频素材 URL
  wizardEnvSubjectId?: string                     // 25 号 §6.5：出镜环境主体 server_id（空=棚拍）
  wizardBrollSegments?: Array<{ sentence_index: number; media_url: string }> // 29 号单阶段：生成前 B-Roll 配置
}

type ComposeDraftState = ComposeDraft & {
  patch: (p: Partial<ComposeDraft>) => void
  setTrack: (t: ComposeTrack) => void
  reset: () => void
}

const empty: ComposeDraft = {
  track: 'video',
  titles: [],
  topics: [],
  imageUrls: [],
  imageTaskIds: [],
}

export const useComposeDraft = create<ComposeDraftState>()(
  persist(
    (set) => ({
      ...empty,
      patch: (p) => set((s) => ({ ...s, ...p })),
      setTrack: (track) => set((s) => ({ ...s, track })),
      reset: () => set({ ...empty }),
    }),
    {
      name: 'compose-draft-v2',
      merge: (persisted, current) => ({
        ...current,
        ...(persisted as object),
        imageTaskIds: (persisted as ComposeDraft)?.imageTaskIds ?? [],
      }),
    },
  ),
)

export type ComposeModuleDef = {
  key: string
  path: string
  label: string
  desc: string
  status: 'ready' | 'partial' | 'soon'
  /** 所属轨道；shared = 两轨都可用 */
  track: ComposeTrack | 'shared'
}

export const COMPOSE_MODULES: ComposeModuleDef[] = [
  { key: 'benchmark', path: '/m/compose/benchmark', label: '爆款对标', desc: '链接或素材 → 自动转写原文', status: 'ready', track: 'shared' },
  { key: 'copy', path: '/m/compose/copy', label: '文案工作室', desc: '编辑 + AI 差异化改写（口播/图文）', status: 'ready', track: 'shared' },
  { key: 'titles', path: '/m/compose/titles', label: '标题话题', desc: '标题与话题标签', status: 'ready', track: 'shared' },
  // 发视频专属
  { key: 'voice', path: '/m/compose/voice', label: '爆款配音', desc: '口播文案转语音', status: 'ready', track: 'video' },
  { key: 'avatar', path: '/m/compose/avatar', label: '口播数字人', desc: '形象 + 文案生成口播视频', status: 'ready', track: 'video' },
  { key: 'edit', path: '/m/compose/edit', label: '成片确认', desc: '确认成片地址，再去封面发布', status: 'ready', track: 'video' },
  { key: 'cover', path: '/m/compose/cover', label: '视频封面', desc: '短视频封面标题卡', status: 'ready', track: 'video' },
  { key: 'publish-video', path: '/m/distribution?contentType=video', label: '发布视频', desc: '发到抖音等短视频平台', status: 'ready', track: 'video' },
  // 发图文专属
  { key: 'images', path: '/m/compose/images', label: '图文配图', desc: '配图生成与素材挑选', status: 'ready', track: 'graphic' },
  { key: 'cover-graphic', path: '/m/compose/cover?track=graphic', label: '图文封面', desc: '笔记/帖子封面图', status: 'ready', track: 'graphic' },
  { key: 'publish-graphic', path: '/m/distribution?contentType=image', label: '发布图文', desc: '发小红书等图文渠道', status: 'ready', track: 'graphic' },
]

export function modulesForTrack(track: ComposeTrack) {
  return COMPOSE_MODULES.filter((m) => m.track === 'shared' || m.track === track)
}
