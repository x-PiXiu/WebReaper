import { describe, expect, it } from 'vitest'
import {
  emptyDraft,
  hasPrefilledMedia,
  nextWizardStep,
  resolveEntryStep,
} from './wizardModel'

describe('resolveEntryStep', () => {
  it('starts at step 1 for video with media and title', () => {
    expect(resolveEntryStep({
      contentType: 'video',
      title: '口播成片',
      mediaURLs: ['https://cdn/v.mp4'],
    })).toBe(1)
  })

  it('jumps to step 4 when accounts already selected', () => {
    expect(resolveEntryStep({
      contentType: 'video',
      title: '口播成片',
      mediaURLs: ['https://cdn/v.mp4'],
      accountIDs: ['acc-1'],
    })).toBe(4)
  })
})

describe('hasPrefilledMedia', () => {
  it('requires video type, media and title', () => {
    expect(hasPrefilledMedia(emptyDraft({
      contentType: 'video',
      title: 'x',
      mediaURLs: ['u'],
    }))).toBe(true)
    expect(hasPrefilledMedia(emptyDraft({
      contentType: 'article',
      title: 'x',
      mediaURLs: ['u'],
    }))).toBe(false)
  })
})

describe('nextWizardStep', () => {
  it('skips steps 2 and 3 when prefilled video is complete', () => {
    const draft = emptyDraft({
      contentType: 'video',
      title: '成片标题',
      mediaURLs: ['https://cdn/v.mp4'],
      accountIDs: ['a1'],
    })
    expect(nextWizardStep(1, draft, [], ['douyin'])).toBe(4)
  })
})
