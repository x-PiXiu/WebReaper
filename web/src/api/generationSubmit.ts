import { apiClient } from './client'
import type { GenerationTask, PromptRef } from '../types/api'

/**
 * 与 Docs/API/统一生成API文档.md 对齐的统一提交载荷。
 *
 * 核心契约（傻瓜式）：brand_id + text + materials + type → 后端选端点/模型。
 * 扩展字段：
 *   - params.model / voice_setting_*：高级透传（服务端白名单合并）
 *   - sub_type：仅 subject 等显式覆盖场景
 *   - watermark / off_peak：可选计费开关
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
  /** 高级参数；文生图建议带 model=viduq2（规避 BE 默认落到需参考图的 viduq1） */
  params?: Record<string, unknown>
  sub_type?: string
  watermark?: boolean
  off_peak?: boolean
}

/** 文生图默认模型（支持 0 张参考图的纯文生图） */
export const DEFAULT_TEXT2IMAGE_MODEL = 'viduq2'

/** 合并统一提交高级参数（model / seed / voice_setting_* 等——服务端 params 白名单合并） */
export function mergeSubmitParams(
  ...parts: Array<Record<string, unknown> | undefined | null>
): Record<string, unknown> | undefined {
  const merged: Record<string, unknown> = {}
  for (const p of parts) {
    if (!p) continue
    for (const [k, v] of Object.entries(p)) {
      if (v === undefined || v === null || v === '') continue
      if (Array.isArray(v) && v.length === 0) continue
      merged[k] = v
    }
  }
  return Object.keys(merged).length > 0 ? merged : undefined
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
  // Creation 等高级表单在顶层传 model / off_peak / watermark
  if (data.model && !adv.model) adv.model = data.model
  if (data.off_peak && adv.off_peak === undefined) adv.off_peak = data.off_peak
  if (data.watermark && adv.watermark === undefined) adv.watermark = data.watermark

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

  // 文生图：默认 viduq2（0 张参考图可用）；参考图只传可访问的 http(s)/media URL，禁止 data: 撑爆 params_json
  if (sub === 'text2image') {
    if (!adv.model) adv.model = DEFAULT_TEXT2IMAGE_MODEL
    const imgUrls: string[] = []
    const pushUrl = (u: string) => {
      const s = u.trim()
      if (!s || s.startsWith('data:') || s.startsWith('blob:')) return
      imgUrls.push(s)
    }
    if (Array.isArray(images)) {
      for (const u of images) {
        if (typeof u === 'string') pushUrl(u)
      }
    }
    for (const r of data.refs || []) {
      if ((!r.kind || r.kind === 'image') && r.url) pushUrl(r.url)
    }
    if (imgUrls.length) adv.images = imgUrls
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
    watermark: data.watermark,
    off_peak: data.off_peak,
  }
}

/** 统一提交（POST /api/v1/generation/submit）——傻瓜式主路径 */
export async function submitUnified(payload: UnifiedSubmitPayload): Promise<GenerationTask> {
  if (!payload.brand_id) {
    throw new Error('请先选择人设/品牌后再生成')
  }
  // 文生图兜底模型：后端若未读 params.model，仍尽量降低落到 viduq1 的概率（管理后台默认配置对齐前）
  let params = payload.params
  if (payload.type === 'image') {
    params = mergeSubmitParams({ model: DEFAULT_TEXT2IMAGE_MODEL }, params)
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
    params,
    sub_type: payload.sub_type,
    watermark: payload.watermark,
    off_peak: payload.off_peak,
  })
}

/** 兼容旧调用：内部映射到 POST /generation/submit */
export async function submitGenerationTaskCompat(data: LegacyGenerationSubmit): Promise<GenerationTask> {
  const unified = await mapLegacyToUnified(data)
  return submitUnified(unified)
}
