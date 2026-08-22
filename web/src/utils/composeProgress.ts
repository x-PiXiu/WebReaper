import type { ComposeDraft, ComposeTrack } from '../store/composeDraft'
import { GRAPHIC_FLOW_STEPS, VIDEO_FLOW_STEPS } from '../config/product'

export function getComposeBody(draft: ComposeDraft) {
  return (draft.rewritten || draft.script || draft.transcript || '').trim()
}

export function hasComposeDraft(draft: ComposeDraft) {
  return !!getComposeBody(draft)
}

/** 根据草稿字段推断应落在哪一步（0-based） */
export function inferComposeStepIndex(draft: ComposeDraft, track: ComposeTrack): number {
  if (!getComposeBody(draft)) return 0

  if (track === 'video') {
    if (draft.editedVideoUrl || draft.avatarVideoUrl) return 2
    const hasAssets =
      !!(draft.voiceUrl || draft.avatarTaskId || draft.voiceTaskId || draft.coverUrl || draft.coverTaskId)
    if (hasAssets) return 1
    return Math.min(Math.max(draft.stepIndex ?? 1, 1), 2)
  }

  const images = (draft.imageUrls || []).filter(Boolean)
  if (images.length > 0 && (draft.coverUrl || images.length >= 1)) {
    return draft.coverUrl || images.length >= 2 ? 2 : 1
  }
  if (images.length > 0 || draft.coverUrl || (draft.imageTaskIds || []).length > 0) return 1
  return Math.min(Math.max(draft.stepIndex ?? 1, 1), 2)
}

export function resolveComposeStepIndex(draft: ComposeDraft, track: ComposeTrack) {
  const saved = draft.stepIndex
  const inferred = inferComposeStepIndex(draft, track)
  if (saved == null) return inferred
  return Math.max(saved, inferred)
}

export function composeSteps(track: ComposeTrack) {
  return track === 'video' ? VIDEO_FLOW_STEPS : GRAPHIC_FLOW_STEPS
}

export function composeProgressLabel(draft: ComposeDraft, track: ComposeTrack) {
  const steps = composeSteps(track)
  const idx = inferComposeStepIndex(draft, track)
  const parts = steps.map((s, i) => {
    if (i < idx) return `${s.label} ✓`
    if (i === idx) return `${s.label}中`
    return s.label
  })
  return parts.join(' · ')
}

export function validateComposeStep(
  draft: ComposeDraft,
  track: ComposeTrack,
  stepIndex: number,
): { ok: boolean; hint?: string } {
  if (stepIndex === 0 && !getComposeBody(draft)) {
    return { ok: false, hint: track === 'video' ? '请先写好口播文案' : '请先写好图文文案' }
  }
  if (stepIndex === 1 && track === 'video') {
    const hasVoice = !!(draft.voiceUrl || draft.voiceTaskId)
    const hasAvatar = !!(draft.avatarVideoUrl || draft.avatarTaskId)
    if (!hasVoice && !hasAvatar) {
      return { ok: false, hint: '请至少生成配音或提交数字人口播' }
    }
  }
  if (stepIndex === 1 && track === 'graphic') {
    const images = (draft.imageUrls || []).filter(Boolean)
    if (images.length === 0 && !(draft.imageTaskIds || []).length) {
      return { ok: false, hint: '请至少上传或生成一张配图' }
    }
  }
  return { ok: true }
}
