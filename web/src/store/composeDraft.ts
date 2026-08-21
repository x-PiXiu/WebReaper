import { create } from 'zustand'
import { persist } from 'zustand/middleware'

/** 爆款获客双轨：发视频 / 发图文 */
export type ComposeTrack = 'video' | 'graphic'

/**
 * 多模块共享草稿（非向导）：各模块读写同一份状态。
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
}

export const useComposeDraft = create<ComposeDraftState>()(
  persist(
    (set) => ({
      ...empty,
      patch: (p) => set((s) => ({ ...s, ...p })),
      setTrack: (track) => set((s) => ({ ...s, track })),
      reset: () => set({ ...empty }),
    }),
    { name: 'compose-draft-v2' },
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
  { key: 'benchmark', path: '/m/compose/benchmark', label: '爆款对标', desc: '链接或素材 → 沉淀可改写原文', status: 'partial', track: 'shared' },
  { key: 'copy', path: '/m/compose/copy', label: '文案工作室', desc: '编辑 + AI 差异化改写（口播/图文）', status: 'ready', track: 'shared' },
  { key: 'titles', path: '/m/compose/titles', label: '标题话题', desc: '标题与话题标签', status: 'ready', track: 'shared' },
  // 发视频专属
  { key: 'voice', path: '/m/compose/voice', label: '爆款配音', desc: '口播文案转语音', status: 'partial', track: 'video' },
  { key: 'avatar', path: '/m/compose/avatar', label: '口播数字人', desc: '形象 + 文案生成口播视频', status: 'partial', track: 'video' },
  { key: 'edit', path: '/m/compose/edit', label: '智能剪辑', desc: '字幕、节奏与成片', status: 'soon', track: 'video' },
  { key: 'cover', path: '/m/compose/cover', label: '视频封面', desc: '短视频封面标题卡', status: 'partial', track: 'video' },
  { key: 'publish-video', path: '/m/distribution?contentType=video', label: '发布视频', desc: '发到抖音等短视频平台', status: 'ready', track: 'video' },
  // 发图文专属
  { key: 'images', path: '/m/compose/images', label: '图文配图', desc: '配图生成与素材挑选', status: 'partial', track: 'graphic' },
  { key: 'cover-graphic', path: '/m/compose/cover?track=graphic', label: '图文封面', desc: '笔记/帖子封面图', status: 'partial', track: 'graphic' },
  { key: 'publish-graphic', path: '/m/distribution?contentType=article', label: '发布图文', desc: '发小红书/公众号等图文渠道', status: 'ready', track: 'graphic' },
]

export function modulesForTrack(track: ComposeTrack) {
  return COMPOSE_MODULES.filter((m) => m.track === 'shared' || m.track === track)
}
