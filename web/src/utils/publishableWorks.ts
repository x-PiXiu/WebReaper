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

/** 工作台成片类 sub_type——可发布成品（compose=B-Roll 合成产物） */
const DELIVERABLE_SUB_TYPES = new Set([
  'lip_sync',
  'reference2video',
  'digital_human',
  'compose',
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

/** compose 任务 → 作品条目（标题取合成台词首句） */
export function composeTaskToWorkItem(t: GenerationTask): WorkItem | null {
  if ((t.sub_type || '').toLowerCase() !== 'compose' || t.state !== 'success') return null
  const url = t.creations?.[0]?.stored_url || t.creations?.[0]?.url || ''
  if (!url) return null
  const p = t.params && typeof t.params === 'object' ? t.params : {}
  const script = typeof p.script === 'string' ? p.script : ''
  const firstLine = script.split('\n').map((l) => l.trim()).find(Boolean) || ''
  return {
    id: `g-${t.id}`,
    kind: 'video',
    title: firstLine.slice(0, 40) || 'B-Roll 合成成片',
    brand_id: t.brand_id,
    status: 'ready',
    media_urls: [url],
    platforms: [],
    views: 0,
    likes: 0,
    comments: 0,
    created_at: t.created_at,
  }
}

/**
 * 补齐 compose 产物（B-Roll 合成成片）到作品列表。
 * 服务端 ListWorks 的 deliverableSubTypes 暂不含 compose（见 Docs/问题反馈.md），
 * 前端从生成任务补齐；服务端补上后按 id 自动去重。
 */
export function appendComposeWorks(works: WorkItem[], tasks?: GenerationTask[]): WorkItem[] {
  if (!tasks?.length) return works
  const existing = new Set(works.map((w) => w.id))
  const extras = tasks
    .map(composeTaskToWorkItem)
    .filter((w): w is WorkItem => w !== null && !existing.has(w.id))
  if (!extras.length) return works
  return [...works, ...extras]
}

/**
 * B-Roll 血缘标记（23 号计划 §6.2：作品卡显示 B-Roll 标记）。
 * - composeWorkIds：compose 产物的作品 id（g-<taskId>）→ 标"B-Roll"
 * - brollSourceWorkIds：被插入过画面的源片作品 id → 标"已插画面"
 */
export function brollLineage(tasks?: GenerationTask[]): {
  composeWorkIds: Set<string>
  brollSourceWorkIds: Set<string>
} {
  const composeWorkIds = new Set<string>()
  const brollSourceWorkIds = new Set<string>()
  for (const t of tasks || []) {
    if ((t.sub_type || '').toLowerCase() !== 'compose') continue
    composeWorkIds.add(`g-${t.id}`)
    const p = t.params && typeof t.params === 'object' ? t.params : {}
    const src = typeof p.source_task_id === 'string' ? p.source_task_id : ''
    if (src) brollSourceWorkIds.add(`g-${src}`)
  }
  return { composeWorkIds, brollSourceWorkIds }
}
