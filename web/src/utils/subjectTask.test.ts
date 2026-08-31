import { describe, expect, it } from 'vitest'
import type { GenerationTask } from '../types/api'
import {
  listSubjectsFromTasks,
  parseGenerationTaskParams,
  parseSubjectFromTask,
  readySubjects,
  subjectServerId,
} from './subjectTask'

function subjectTask(overrides: Partial<GenerationTask> = {}): GenerationTask {
  return {
    id: 'task-abc123456789',
    type: 'video',
    sub_type: 'subject',
    state: 'success',
    model: 'vidu',
    created_at: '2026-08-26T00:00:00Z',
    ...overrides,
  } as GenerationTask
}

describe('parseGenerationTaskParams', () => {
  it('parses object params', () => {
    const t = subjectTask({ params: { name: '小美', images: ['https://a/img.jpg'] } })
    expect(parseGenerationTaskParams(t)).toEqual({ name: '小美', images: ['https://a/img.jpg'] })
  })

  it('parses JSON string params', () => {
    const t = subjectTask({ params: '{"name":"店小二","voice_id":"v-1"}' as unknown as GenerationTask['params'] })
    expect(parseGenerationTaskParams(t).voice_id).toBe('v-1')
  })

  it('returns empty object on invalid JSON', () => {
    const t = subjectTask({ params: '{bad' as unknown as GenerationTask['params'] })
    expect(parseGenerationTaskParams(t)).toEqual({})
  })
})

describe('parseSubjectFromTask', () => {
  it('returns null for non-subject tasks', () => {
    expect(parseSubjectFromTask(subjectTask({ sub_type: 'tts' }))).toBeNull()
  })

  it('maps subject fields from provider_task_id', () => {
    const t = subjectTask({
      provider_task_id: 'srv-001',
      params: {
        name: '主播 A',
        images: ['https://cdn/p.jpg'],
        videos: ['https://cdn/v.mp4'],
        voice_id: 'voice-1',
      },
    })
    const s = parseSubjectFromTask(t)!
    expect(s.name).toBe('主播 A')
    expect(s.serverId).toBe('srv-001')
    expect(s.portraitUrl).toBe('https://cdn/p.jpg')
    expect(s.hasVideo).toBe(true)
    expect(s.videoUrl).toBe('https://cdn/v.mp4')
    expect(s.voiceId).toBe('voice-1')
  })

  it('falls back to creation id for serverId', () => {
    const t = subjectTask({
      creations: [{ id: 'cre-99', url: 'https://cdn/fallback.jpg' }],
    })
    expect(subjectServerId(t)).toBe('cre-99')
    expect(parseSubjectFromTask(t)!.portraitUrl).toBe('https://cdn/fallback.jpg')
  })
})

describe('listSubjectsFromTasks / readySubjects', () => {
  it('filters and lists ready subjects only', () => {
    const tasks = [
      subjectTask({ id: '1', provider_task_id: 'a', state: 'success' }),
      subjectTask({ id: '2', state: 'processing' }),
      subjectTask({ id: '3', sub_type: 'tts' }),
    ]
    const all = listSubjectsFromTasks(tasks)
    expect(all).toHaveLength(2)
    expect(readySubjects(all)).toHaveLength(1)
    expect(readySubjects(all)[0].serverId).toBe('a')
  })
})
