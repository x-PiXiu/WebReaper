import { describe, expect, it } from 'vitest'
import type { ComposeDraft } from '../store/composeDraft'
import { composeProgressLabel, composeResumeLabel, composeResumeHint, composeResumePath } from './composeProgress'

function draft(partial: Partial<ComposeDraft>): ComposeDraft {
  return partial as ComposeDraft
}

describe('composeProgressLabel', () => {
  it('shows video asset sub-progress on assets step', () => {
    const label = composeProgressLabel(
      draft({
        script: '口播文案',
        voiceUrl: 'https://audio',
        avatarTaskId: 't-1',
        stepIndex: 1,
      }),
      'video',
    )
    expect(label).toContain('配音✓')
    expect(label).toContain('成片中')
  })
})

describe('composeResumePath / composeResumeLabel', () => {
  it('routes video track through redirect', () => {
    expect(composeResumePath(draft({ track: 'video' }))).toBe('/m/compose/video')
    expect(composeResumeLabel(draft({ track: 'video' }))).toBe('发视频')
  })

  it('routes lipsync wizard draft', () => {
    expect(composeResumePath(draft({ track: 'lipsync', wizardStep: 2 }))).toBe('/m/compose/lipsync')
    expect(composeResumeLabel(draft({ wizardStep: 1 }))).toBe('拍口播')
    expect(composeResumeHint(draft({ track: 'lipsync', wizardStep: 2 }))).toBe('拍口播向导 · 第 3 步')
  })
})
