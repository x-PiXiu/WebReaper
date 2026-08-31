import type { GenerationTask } from '../types/api'

export type ViduSubject = {
  taskId: string
  state: string
  name: string
  serverId: string
  voiceId: string
  /** 资产分类（25 号 §6.5）：person=人物分身 / scene=环境主体（组合出镜用） */
  kind: 'person' | 'scene'
  hasVideo: boolean
  imageCount: number
  portraitUrl: string
  errMsg: string
  createdAt: string
  /** 链式形象视频任务 ID（25 号阶段二；空=未生成/失败） */
  avatarTaskId: string
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
  return {
    taskId: t.id,
    state: t.state,
    name: (typeof p.name === 'string' && p.name) || t.id.slice(0, 12),
    serverId: subjectServerId(t),
    voiceId: typeof p.voice_id === 'string' ? p.voice_id : '',
    kind: p.kind === 'scene' ? 'scene' : 'person',
    hasVideo: Array.isArray(p.videos) && p.videos.length > 0,
    avatarTaskId: typeof p.avatar_task_id === 'string' ? p.avatar_task_id : '',
    imageCount: images.length,
    portraitUrl: images[0] || t.creations?.[0]?.url || '',
    errMsg: t.err_msg || '',
    createdAt: t.created_at,
  }
}

export function listSubjectsFromTasks(tasks: GenerationTask[]): ViduSubject[] {
  return tasks
    .map(parseSubjectFromTask)
    .filter((s): s is ViduSubject => s !== null)
}

/** 注册成功、可用于 reference2video 的分身（人物；环境主体另用 listSceneSubjects） */
export function readySubjects(subjects: ViduSubject[]): ViduSubject[] {
  return subjects.filter((s) => s.state === 'success' && !!s.serverId && s.kind === 'person')
}

/** 环境主体（25 号 §6.5：组合出镜资产——我的店面/后厨/产品特写） */
export function listSceneSubjects(subjects: ViduSubject[]): ViduSubject[] {
  return subjects.filter((s) => s.kind === 'scene')
}
