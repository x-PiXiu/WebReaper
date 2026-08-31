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
  /** 高级参数（model/音色/运镜等——服务端 BE-GEN-01 已修，无需前端硬编码默认模型） */
  params?: Record<string, unknown>
  sub_type?: string
  watermark?: boolean
  off_peak?: boolean
  /** B-Roll 配置（29号计划——单阶段优化，视频生成后自动插入） */
  broll_segments?: Array<{ sentence_index: number; media_url: string }>
}

/** 主体引用（reference2video 一致性——BE-SUBJ-01） */
export type SubjectRef = { name: string; server_id?: string; images?: string[] }

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

/** 标记为可发布成片（工作台合成产物——非素材库中间素材） */
export function deliverableWorkParams(
  extra?: Record<string, unknown> | null,
): Record<string, unknown> {
  return mergeSubmitParams({ deliverable: true }, extra) ?? { deliverable: true }
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

  // 参考图只传可访问的 http(s)/media URL（BE-GEN-04 已修 params_json 不再内联 base64）
  if (sub === 'text2image') {
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

  if (sub === 'subject') {
    return buildSubjectRegisterPayload({
      brand_id,
      name: text || (typeof params.name === 'string' ? params.name : ''),
      imageMaterialIds: materials.filter((m) => !/^https?:\/\//i.test(m)),
      imageUrls: Array.isArray(params.images)
        ? params.images.filter((u): u is string => typeof u === 'string' && !!u.trim())
        : materials.filter((m) => /^https?:\/\//i.test(m)),
      videoUrl: Array.isArray(params.videos) && typeof params.videos[0] === 'string'
        ? params.videos[0]
        : undefined,
      voice_id: typeof adv.voice_id === 'string' ? adv.voice_id : undefined,
    })
  }

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

/**
 * 数字分身主体注册（POST /generation/submit + sub_type=subject）。
 * materials 优先传素材库 ID；params.images/videos 作 URL 直传兜底（需 BE-SUBJ-02）。
 */
export function buildSubjectRegisterPayload(input: {
  brand_id: string
  name: string
  imageMaterialIds?: string[]
  imageUrls?: string[]
  videoUrl?: string
  voice_id?: string
  /** 23 号计划 §2.1③：可选场景图（形象视频生成的环境参考；服务端接入前忽略） */
  sceneImageUrl?: string
  /** 23 号计划 §2.1③：可选场景描述（一句话：主角在哪个场景做什么） */
  sceneDescription?: string
  /** 25 号 §6.5：资产分类——person=人物分身（默认）/ scene=环境主体 */
  kind?: 'person' | 'scene'
}): UnifiedSubmitPayload {
  const name = (input.name || '').trim()
  if (!name) throw new Error('请输入主体名称')
  const ids = (input.imageMaterialIds || []).filter(Boolean).slice(0, 3)
  const urls = (input.imageUrls || []).filter(Boolean).slice(0, 3)
  const video = (input.videoUrl || '').trim()
  if (ids.length === 0 && urls.length === 0 && !video) {
    throw new Error('请上传至少 1 张形象照或 1 个主体视频')
  }
  const params: Record<string, unknown> = { name }
  if (input.voice_id) params.voice_id = input.voice_id
  if (urls.length) params.images = urls
  if (video) params.videos = [video]
  const sceneDesc = (input.sceneDescription || '').trim()
  if (input.sceneImageUrl) params.scene_image = input.sceneImageUrl
  if (sceneDesc) params.scene_description = sceneDesc
  if (input.kind) params.kind = input.kind
  const materials = [...ids]
  if (!materials.length && urls.length) materials.push(...urls)
  if (!materials.length && video) materials.push(video)
  return {
    brand_id: input.brand_id,
    text: name,
    materials: materials.length ? materials : undefined,
    sub_type: 'subject',
    params,
  }
}

/**
 * 已注册分身 + 口播文案 → reference2video（主体 server_id 一致性）。
 * 可选附带 audio 素材 ID（服务端当前以 subjects 路径为主，音频供后续扩展）。
 */
export function buildSubjectReferencePayload(input: {
  brand_id: string
  server_id: string
  name?: string
  text: string
  audioMaterialId?: string
  /** 组合出镜（25 号 §6.5）：环境主体作为第二参考主体——分身在环境里口播 */
  envSubject?: { serverId: string; name?: string }
}): UnifiedSubmitPayload {
  const subjects: SubjectRef[] = [{
    name: (input.name || '主体').trim() || '主体',
    server_id: input.server_id.trim(),
  }]
  if (input.envSubject?.serverId) {
    subjects.push({ name: (input.envSubject.name || '环境').trim(), server_id: input.envSubject.serverId })
  }
  return {
    brand_id: input.brand_id,
    text: input.text.trim(),
    materials: input.audioMaterialId ? [input.audioMaterialId] : undefined,
    params: deliverableWorkParams({ subjects }),
  }
}

/** 统一提交（POST /api/v1/generation/submit）——傻瓜式主路径 */
export async function submitUnified(payload: UnifiedSubmitPayload): Promise<GenerationTask> {
  if (!payload.brand_id) {
    throw new Error('请先选择人设/品牌后再生成')
  }
  const params = payload.params
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
    broll_segments: payload.broll_segments, // 29号计划：B-Roll配置
  })
}

/** 兼容旧调用：内部映射到 POST /generation/submit */
export async function submitGenerationTaskCompat(data: LegacyGenerationSubmit): Promise<GenerationTask> {
  const unified = await mapLegacyToUnified(data)
  return submitUnified(unified)
}
