import type { GenerationTask, MediaAsset } from '../types/api'

const DONE = new Set(['success', 'failed', 'cancelled'])

export function isTaskDone(state: string) {
  return DONE.has(state)
}

export function isTaskSuccess(state: string) {
  return state === 'success'
}

/** 将相对 /media/ 路径转为可播放的绝对 URL */
export function resolveMediaUrl(url: string): string {
  const s = (url || '').trim()
  if (!s) return s
  if (/^(https?:|blob:|data:)/i.test(s)) return s
  if (s.startsWith('/')) return `${window.location.origin}${s}`
  return s
}

/** 优先取转存后的永久 URL */
export function taskPrimaryUrl(task: GenerationTask): string | null {
  const c = task.creations?.[0]
  if (!c) return null
  return c.stored_url || c.url || null
}

export function taskCoverUrl(task: GenerationTask): string | null {
  const c = task.creations?.[0]
  if (!c) return null
  return c.cover_url || null
}

const AUDIO_SUB_TYPES = new Set(['tts', 'text2audio', 'sound_effect', 'voice_clone'])

const VIDEO_SUB_TYPES = new Set([
  'text2video', 'img2video', 'start_end2video', 'reference2video',
  'multiframe', 'lip_sync', 'digital_human',
])

/** 列表展示：识别视频素材（含 mime 异常但 URL 为视频扩展名） */
export function isVideoMedia(mime: string, url: string, type?: string): boolean {
  if (type === 'video') return true
  if (mime.startsWith('video/')) return true
  return /\.(mp4|webm|mov|m4v)(\?|#|$)/i.test(url)
}

/** 推断素材类型（优先 type 字段，其次 mime / URL） */
export function inferMediaKind(mime: string, url: string, type?: string): 'image' | 'video' | 'audio' {
  if (type === 'image' || type === 'video' || type === 'audio') return type
  if (isImageMedia(mime, url, type)) return 'image'
  if (isVideoMedia(mime, url, type)) return 'video'
  if (isAudioMedia(mime, url, type)) return 'audio'
  return 'image'
}

/** 列表展示：识别图片素材（含 mime 异常但 URL 为图片扩展名） */
export function isImageMedia(mime: string, url: string, type?: string): boolean {
  if (type === 'image') return true
  if (mime.startsWith('image/')) return true
  return /\.(png|jpe?g|webp|gif|bmp)(\?|#|$)/i.test(url)
}

/** 列表展示：识别音频素材（含转存为 application/octet-stream 的 .mp3） */
export function isAudioMedia(mime: string, url: string, type?: string): boolean {
  if (type === 'audio') return true
  if (mime.startsWith('audio/')) return true
  return /\.(mp3|wav|m4a|aac|ogg)(\?|#|$)/i.test(url)
}

function guessImageMime(url: string): string {
  const ext = url.split('?')[0].split('.').pop()?.toLowerCase()
  if (ext === 'png') return 'image/png'
  if (ext === 'webp') return 'image/webp'
  if (ext === 'gif') return 'image/gif'
  return 'image/jpeg'
}

function guessAudioMime(url: string): string {
  const ext = url.split('?')[0].split('.').pop()?.toLowerCase()
  if (ext === 'wav') return 'audio/wav'
  if (ext === 'm4a') return 'audio/mp4'
  if (ext === 'ogg') return 'audio/ogg'
  return 'audio/mpeg'
}

/**
 * 生成任务 → 虚拟 MediaAsset（TTS 同步成功但转存未入素材索引时的兜底展示）。
 */
export function mediaAssetsFromGenerationTasks(
  tasks: GenerationTask[],
  kind: 'audio' | 'image' | 'video',
): MediaAsset[] {
  const out: MediaAsset[] = []
  for (const t of tasks) {
    if (t.state !== 'success') continue
    const match =
      kind === 'audio'
        ? t.type === 'audio' || AUDIO_SUB_TYPES.has(t.sub_type)
        : kind === 'image'
          ? t.type === 'image' || t.sub_type === 'text2image'
          : t.type === 'video' || t.type === 'digital_human' || VIDEO_SUB_TYPES.has(t.sub_type)
    if (!match) continue
    for (const c of t.creations || []) {
      const url = c.stored_url || c.url
      if (!url) continue
      const mime = kind === 'audio' ? guessAudioMime(url)
        : kind === 'image' ? guessImageMime(url)
        : 'video/mp4'
      const label = t.sub_type === 'tts' ? 'AI 配音'
        : t.sub_type === 'text2image' ? 'AI 配图'
        : t.sub_type === 'voice_clone' ? '克隆试听'
        : t.sub_type === 'sound_effect' ? 'AI 音效'
        : 'AI 产物'
      out.push({
        id: `gen-task:${t.id}:${c.id || '0'}`,
        tenant_id: t.tenant_id,
        brand_id: t.brand_id,
        owner_type: 'creation',
        type: kind,
        name: `${label} ${t.id.slice(-8)}`,
        url,
        cover_url: c.cover_url || undefined,
        mime,
        size_bytes: 0,
        width: 0,
        height: 0,
        duration: 0,
        created_at: t.finished_at || t.created_at,
      })
    }
  }
  return out
}

/** 合并素材库与任务产物，按 URL 去重（优先保留有 size 的索引项） */
export function mergeMediaByUrl(primary: MediaAsset[], extra: MediaAsset[]): MediaAsset[] {
  const map = new Map<string, MediaAsset>()
  for (const a of [...extra, ...primary]) {
    if (!a.url) continue
    const prev = map.get(a.url)
    if (!prev || (a.size_bytes > 0 && prev.size_bytes === 0)) {
      map.set(a.url, a)
    }
  }
  return Array.from(map.values()).sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
  )
}
