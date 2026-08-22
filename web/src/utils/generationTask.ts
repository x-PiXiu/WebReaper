import type { GenerationTask } from '../types/api'

const DONE = new Set(['success', 'failed', 'cancelled'])

export function isTaskDone(state: string) {
  return DONE.has(state)
}

export function isTaskSuccess(state: string) {
  return state === 'success'
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
