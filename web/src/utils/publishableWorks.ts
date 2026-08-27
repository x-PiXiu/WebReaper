import type { GenerationTask, WorkItem } from '../types/api'

/** 素材库生成类 sub_type——仅进素材库，不进「我的作品」 */
const MATERIAL_SUB_TYPES = new Set([
  'text2image',
  'tts',
  'text2audio',
  'sound_effect',
  'voice_clone',
  'subject',
  'text2video',
  'img2video',
  'start_end2video',
  'multiframe',
])

/** 工作台成片类 sub_type——可发布成品 */
const DELIVERABLE_SUB_TYPES = new Set([
  'lip_sync',
  'reference2video',
  'digital_human',
])

function taskFlag(task: GenerationTask, key: string): boolean {
  const v = task.params?.[key]
  return v === true || v === 'true'
}

/** 生成任务是否为可发布成片（非素材库中间产物） */
export function isDeliverableGenerationTask(task: GenerationTask): boolean {
  if (taskFlag(task, 'deliverable') || taskFlag(task, 'work_product')) return true
  const sub = (task.sub_type || '').toLowerCase()
  if (DELIVERABLE_SUB_TYPES.has(sub)) return true
  if (MATERIAL_SUB_TYPES.has(sub)) return false
  return false
}

/** 作品库条目是否应展示在「我的作品」（文章 + 成片；排除素材库 AI 产物） */
export function isPublishableWorkItem(
  w: WorkItem,
  taskById?: Map<string, GenerationTask>,
): boolean {
  if (w.kind === 'article' || w.id.startsWith('c-')) return true
  if (!w.id.startsWith('g-')) return true
  if (w.status === 'published') return true

  const taskId = w.id.slice(2)
  const task = taskById?.get(taskId)
  if (!task) {
    if (w.kind === 'image' || w.kind === 'audio') return false
    return false
  }
  return isDeliverableGenerationTask(task)
}

export function filterPublishableWorks(
  works: WorkItem[],
  tasks?: GenerationTask[],
): WorkItem[] {
  const taskById = tasks?.length
    ? new Map(tasks.map((t) => [t.id, t]))
    : undefined
  return works.filter((w) => isPublishableWorkItem(w, taskById))
}
