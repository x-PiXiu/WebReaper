/** 账号 IP 智能体 · 前端演示假数据（第一期不接真实资产接口） */

export type PersonaAvatar = {
  id: string
  name: string
  style: string
  tags: string[]
  cover: string
  updatedAt: string
}

export type VoiceAsset = {
  id: string
  name: string
  tone: string
  lang: string
  durationSec: number
  sampleLabel: string
}

export type StoryboardClip = {
  id: string
  title: string
  scene: string
  durationSec: number
  ratio: string
}

export type CoverTemplate = {
  id: string
  name: string
  mood: string
  ratio: string
  accent: string
}

export type WorkItem = {
  id: string
  title: string
  coverAccent: string
  status: 'draft' | 'ready' | 'published'
  platform?: string
  createdAt: string
  publishedAt?: string
  durationSec?: number
  views?: number
  likes?: number
  comments?: number
  leads?: number
}

export type MetricPoint = { day: string; views: number; leads: number; engage: number }

export const MOCK_AVATARS: PersonaAvatar[] = [
  { id: 'av-1', name: '主理人·日间通勤', style: '写实半身', tags: ['商务', '亲切'], cover: 'linear-gradient(145deg,#1e293b,#0f766e)', updatedAt: '2026-08-18' },
  { id: 'av-2', name: '主理人·舞台光', style: '影棚特写', tags: ['高端', 'IP'], cover: 'linear-gradient(145deg,#292524,#b45309)', updatedAt: '2026-08-12' },
  { id: 'av-3', name: '品牌吉祥物', style: '扁平插画', tags: ['活泼', '短视频'], cover: 'linear-gradient(145deg,#312e81,#0891b2)', updatedAt: '2026-08-05' },
]

export const MOCK_VOICES: VoiceAsset[] = [
  { id: 'vc-1', name: '沉稳男声·讲述', tone: '专业可信', lang: '中文', durationSec: 18, sampleLabel: '示例 18s' },
  { id: 'vc-2', name: '轻快女声·种草', tone: '亲和活泼', lang: '中文', durationSec: 12, sampleLabel: '示例 12s' },
  { id: 'vc-3', name: '纪录片旁白', tone: '冷静克制', lang: '中文', durationSec: 24, sampleLabel: '示例 24s' },
]

export const MOCK_STORYBOARDS: StoryboardClip[] = [
  { id: 'sb-1', title: '开场钩子·痛点提问', scene: '口播特写', durationSec: 5, ratio: '9:16' },
  { id: 'sb-2', title: '产品演示三段式', scene: '分屏对比', durationSec: 12, ratio: '9:16' },
  { id: 'sb-3', title: '用户见证·字幕卡', scene: '图文混排', durationSec: 8, ratio: '9:16' },
  { id: 'sb-4', title: '行动号召收束', scene: '品牌落版', durationSec: 4, ratio: '9:16' },
]

export const MOCK_COVERS: CoverTemplate[] = [
  { id: 'cv-1', name: '聚光标题卡', mood: '舞台感', ratio: '9:16', accent: '#d4a574' },
  { id: 'cv-2', name: '杂志大字报', mood: '编辑感', ratio: '9:16', accent: '#5eead4' },
  { id: 'cv-3', name: '对比前后', mood: '转化向', ratio: '9:16', accent: '#fb7185' },
  { id: 'cv-4', name: '极简留白', mood: '高端克制', ratio: '1:1', accent: '#94a3b8' },
]

export const MOCK_WORKS_SEED: WorkItem[] = [
  {
    id: 'wk-1',
    title: '三天打造人设开场片',
    coverAccent: '#0f766e',
    status: 'published',
    platform: '抖音',
    createdAt: '2026-08-10T10:00:00+08:00',
    publishedAt: '2026-08-11T18:20:00+08:00',
    durationSec: 42,
    views: 12840,
    likes: 932,
    comments: 76,
    leads: 28,
  },
  {
    id: 'wk-2',
    title: '痛点钩子·30 秒种草',
    coverAccent: '#b45309',
    status: 'ready',
    createdAt: '2026-08-16T14:00:00+08:00',
    durationSec: 31,
    views: 0,
    likes: 0,
    comments: 0,
    leads: 0,
  },
  {
    id: 'wk-3',
    title: '口播脚本草稿·未合成',
    coverAccent: '#334155',
    status: 'draft',
    createdAt: '2026-08-19T09:30:00+08:00',
  },
]

export const MOCK_METRICS: MetricPoint[] = [
  { day: '08-14', views: 2100, leads: 4, engage: 6.2 },
  { day: '08-15', views: 2800, leads: 6, engage: 7.1 },
  { day: '08-16', views: 3200, leads: 8, engage: 6.8 },
  { day: '08-17', views: 4100, leads: 11, engage: 8.4 },
  { day: '08-18', views: 3900, leads: 9, engage: 7.9 },
  { day: '08-19', views: 5200, leads: 14, engage: 9.2 },
  { day: '08-20', views: 6100, leads: 18, engage: 9.8 },
]

export function formatDuration(sec?: number) {
  if (!sec) return '—'
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return m > 0 ? `${m}:${String(s).padStart(2, '0')}` : `${s}s`
}
