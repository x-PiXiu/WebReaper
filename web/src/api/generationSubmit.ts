import { apiClient } from './client'
import type { GenerationTask, PromptRef } from '../types/api'

/**
 * 与 Docs/API/统一生成API文档.md 对齐的统一提交载荷。
 * 仅传文档字段：brand_id / text / materials / template / type / duration / quality / aspect_ratio
 * （aspect_ratio 文档有、当前 handler 可能未透传——仍发送，后端补齐后零改动）
 */
export type UnifiedSubmitPayload = {
  brand_id: string
  text?: string
  materials?: string[]
  template?: string
  type?: 'video' | 'image' | 'audio' | 'voice'
  duration?: number
  quality?: string
  aspect_ratio?: string
  params?: Record<string, unknown>
  sub_type?: string
}

/** 旧高级提交形态（已删除 POST /generation/tasks）→ 映射到统一 submit */
export type LegacyGenerationSubmit = {
  brand_id?: string
  sub_type: string
  model?: string
  params?: Record<string, unknown>
  refs?: PromptRef[]
  off_peak?: boolean
  watermark?: boolean
}

const VIDEO_SUBTYPES = new Set([
  'text2video', 'img2video', 'start_end2video', 'reference2video',
  'multiframe', 'lip_sync', 'digital_human',
])

async function uploadAssetFile(file: File) {
  const form = new FormData()
  form.append('file', file)
  return apiClient.post<unknown, { id: string; url: string; mime: string; size_bytes: number; owner_type: string }>(
    '/api/v1/media/assets',
    form,
  )
}

/**
 * 将 URL / data-URL 入库为素材，返回 asset id（统一 submit 的 materials 只认素材库 ID）。
 * 文档示例写 /media/upload，实际路由为 POST /media/assets。
 */
export async function ensureMaterialId(source: string): Promise<string> {
  const s = (source || '').trim()
  if (!s) throw new Error('素材地址为空')
  if (!/^https?:\/\//i.test(s) && !s.startsWith('data:') && !s.startsWith('blob:')) {
    return s
  }
  // 本站托管的资产（/media/...）直接传原 URL——服务端按 URL 匹配素材，
  // 免去「下载回浏览器再重传」的双份流量与跨源 fetch 失败风险
  const mediaBase = window.location.origin + '/media/'
  if (s.startsWith('/media/') || s.startsWith(mediaBase)) {
    return s
  }
  const res = await fetch(s)
  if (!res.ok) throw new Error('无法拉取产物素材，请重试')
  const blob = await res.blob()
  const ext = blob.type.includes('audio') ? 'mp3'
    : blob.type.includes('video') ? 'mp4'
    : blob.type.includes('png') ? 'png'
    : blob.type.includes('jpeg') || blob.type.includes('jpg') ? 'jpg'
    : blob.type.includes('webp') ? 'webp'
    : 'bin'
  const file = new File([blob], `gen-${Date.now()}.${ext}`, { type: blob.type || 'application/octet-stream' })
  const uploaded = await uploadAssetFile(file)
  return uploaded.id
}

function pickText(params: Record<string, unknown> = {}): string {
  const t = params.text ?? params.prompt
  return typeof t === 'string' ? t : ''
}

function refIds(refs: PromptRef[] | undefined, kind?: PromptRef['kind']): string[] {
  if (!refs?.length) return []
  return refs
    .filter((r) => !kind || r.kind === kind)
    .map((r) => r.id)
    .filter(Boolean)
}

/**
 * 高级表单 → 统一 submit（设计思想：客户端只传 text + materials + type，服务端选端点）。
 *
 * | 场景 | type | materials |
 * | TTS | audio | — |
 * | 声音克隆 | voice | [音频ID] |
 * | 文/图生视频 | video | 0~N 图 |
 * | 数字人 | 不传 | [图, 音频] |
 * | 对口型 | 不传 | [视频, 音频] |
 */
export async function mapLegacyToUnified(data: LegacyGenerationSubmit): Promise<UnifiedSubmitPayload> {
  const sub = data.sub_type
  const params = data.params || {}
  const brand_id = data.brand_id || ''
  const text = pickText(params)

  if (sub === 'subject') {
    throw new Error('主体创建暂未纳入统一提交接口，请用人像图走数字人口播')
  }

  // 高级参数打包（白名单外的 key 服务端会丢弃；保留字段已被上面的显式提取消费）
  const ADVANCED_KEYS = [
    'seed', 'style', 'movement_amplitude', 'audio', 'audio_type', 'bgm',
    'watermark', 'off_peak', 'payload', 'voice_id', 'model',
    'image_settings', 'timing_prompts',
    'voice_setting_speed', 'voice_setting_volume', 'voice_setting_pitch',
    'voice_setting_voice_id', 'voice_setting_emotion',
  ] as const
  const adv: Record<string, unknown> = {}
  for (const k of ADVANCED_KEYS) {
    const v = (params as Record<string, unknown>)[k]
    if (v !== undefined && v !== null && v !== '') adv[k] = v
  }

  const materialIds = new Set<string>(refIds(data.refs))
  const urlKeys = ['audio_url', 'video_url', 'image', 'image_url'] as const
  for (const k of urlKeys) {
    const v = params[k]
    if (typeof v === 'string' && v.trim()) {
      materialIds.add(await ensureMaterialId(v.trim()))
    }
  }
  const images = params.images
  if (Array.isArray(images)) {
    for (const u of images) {
      if (typeof u === 'string' && u.trim()) materialIds.add(await ensureMaterialId(u.trim()))
    }
  }

  const materials = [...materialIds]
  const duration = typeof params.duration === 'number' ? params.duration : undefined
  const quality = typeof params.resolution === 'string' ? params.resolution
    : typeof params.quality === 'string' ? params.quality : undefined
  const aspect_ratio = typeof params.aspect_ratio === 'string' ? params.aspect_ratio : undefined

  let type: UnifiedSubmitPayload['type']
  if (sub === 'text2image') type = 'image'
  else if (sub === 'tts' || sub === 'text2audio' || sub === 'sound_effect') type = 'audio'
  else if (sub === 'voice_clone') type = 'voice'
  else if (sub === 'digital_human') {
    const hasAudioHint = !!(params.audio_url || refIds(data.refs, 'audio').length)
    // 文档：1图+1音频 → digital_human（不传 type）；仅图+文 → type=video → img2video
    type = hasAudioHint ? undefined : 'video'
  } else if (sub === 'lip_sync') {
    type = undefined
  } else if (VIDEO_SUBTYPES.has(sub)) {
    type = 'video'
  }

  return {
    brand_id,
    text: text || undefined,
    materials: materials.length ? materials : undefined,
    type,
    duration,
    quality,
    aspect_ratio,
    params: Object.keys(adv).length ? adv : undefined,
  }
}

/** 统一提交（POST /api/v1/generation/submit）——严格文档字段 */
export async function submitUnified(payload: UnifiedSubmitPayload): Promise<GenerationTask> {
  if (!payload.brand_id) {
    throw new Error('请先选择人设/品牌后再生成')
  }
  return apiClient.post<unknown, GenerationTask>('/api/v1/generation/submit', {
    brand_id: payload.brand_id,
    text: payload.text || '',
    materials: payload.materials,
    template: payload.template,
    type: payload.type,
    duration: payload.duration,
    quality: payload.quality,
    aspect_ratio: payload.aspect_ratio,
    params: payload.params,
    sub_type: payload.sub_type,
  })
}

/** 兼容旧调用：内部映射到 POST /generation/submit */
export async function submitGenerationTaskCompat(data: LegacyGenerationSubmit): Promise<GenerationTask> {
  const unified = await mapLegacyToUnified(data)
  return submitUnified(unified)
}
