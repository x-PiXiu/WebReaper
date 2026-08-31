import type { GenerationTask } from '../types/api'

export type ViduSubject = {
  taskId: string
  state: string
  name: string
  serverId: string
  voiceId: string
  hasVideo: boolean
  imageCount: number
  portraitUrl: string
  /** 形象视频 URL（链式 10s 视频上线后回写；当前可能为空） */
  videoUrl: string
  errMsg: string
  createdAt: string
}

/** 解析任务 params（后端存 JSON 字符串或对象） */
export function parseGenerationTaskParams(t: GenerationTask): Record<string, unknown> {
  if (t.params && typeof t.params === 'object') return t.params as Record<string, unknown>
  if (typeof t.params === 'string' && t.params) {
    try { return JSON.parse(t.params) as Record<string, unknown> } catch { return {} }
  }
  return {}
}

export function subjectServerId(task: GenerationTask): string {
  return task.provider_task_id || task.creations?.[0]?.id || ''
}

export function parseSubjectFromTask(t: GenerationTask): ViduSubject | null {
  if (t.sub_type !== 'subject') return null
  const p = parseGenerationTaskParams(t)
  const images = Array.isArray(p.images)
    ? p.images.filter((u): u is string => typeof u === 'string')
    : []
  const videos = Array.isArray(p.videos)
    ? p.videos.filter((u): u is string => typeof u === 'string')
    : []
  const creationUrl = t.creations?.[0]?.stored_url || t.creations?.[0]?.url || ''
  const videoUrl = videos[0] || (/\.(mp4|webm|mov)(\?|$)/i.test(creationUrl) ? creationUrl : '')
  return {
    taskId: t.id,
    state: t.state,
    name: (typeof p.name === 'string' && p.name) || t.id.slice(0, 12),
    serverId: subjectServerId(t),
    voiceId: typeof p.voice_id === 'string' ? p.voice_id : '',
    hasVideo: videos.length > 0 || !!videoUrl,
    imageCount: images.length,
    portraitUrl: images[0] || (!videoUrl ? creationUrl : '') || '',
    videoUrl,
    errMsg: t.err_msg || '',
    createdAt: t.created_at,
  }
}

export function listSubjectsFromTasks(tasks: GenerationTask[]): ViduSubject[] {
  return tasks
    .map(parseSubjectFromTask)
    .filter((s): s is ViduSubject => s !== null)
}

/** 注册成功、可用于 reference2video 的分身 */
export function readySubjects(subjects: ViduSubject[]): ViduSubject[] {
  return subjects.filter((s) => s.state === 'success' && !!s.serverId)
}
