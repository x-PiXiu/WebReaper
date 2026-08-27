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
    if (i === idx) {
      if (track === 'video' && i === 1) {
        const hint = composeVideoAssetHint(draft)
        if (hint) return hint
      }
      return `${s.label}中`
    }
    return s.label
  })
  return parts.join(' · ')
}

/** 发视频轨「配素材」步的细粒度进度 */
function composeVideoAssetHint(draft: ComposeDraft): string | null {
  const bits: string[] = []
  if (draft.voiceUrl) bits.push('配音✓')
  else if (draft.voiceTaskId) bits.push('配音中')
  if (draft.avatarVideoUrl) bits.push('成片✓')
  else if (draft.avatarTaskId) bits.push('成片中')
  if (draft.coverUrl) bits.push('封面✓')
  else if (draft.coverTaskId) bits.push('封面中')
  if (bits.length === 0) return null
  return `配素材（${bits.join(' · ')}）`
}

export function composeResumeLabel(draft: ComposeDraft): string {
  if (draft.track === 'graphic') return '发图文'
  if (draft.track === 'lipsync' || (draft.wizardStep ?? 0) > 0) return '拍口播'
  if (draft.track === 'video') return '发视频'
  return '创作'
}

/** 续写草稿副标题（口播向导步数） */
export function composeResumeHint(draft: ComposeDraft): string | undefined {
  if ((draft.wizardStep ?? 0) > 0 || draft.track === 'lipsync') {
    const step = Math.max(draft.wizardStep ?? 0, 0) + 1
    return `拍口播向导 · 第 ${step} 步`
  }
  return undefined
}

export function composeResumePath(draft: ComposeDraft): string {
  if (draft.track === 'graphic') return '/m/compose/graphic'
  if (draft.track === 'video') return '/m/compose/video'
  if (draft.track === 'lipsync' || (draft.wizardStep ?? 0) > 0) return '/m/compose/lipsync'
  return '/m/compose'
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
  // 成片确认步：必须有可发布产物，避免空着手进发布向导
  if (stepIndex === 2 && track === 'video') {
    if (!(draft.editedVideoUrl || draft.avatarVideoUrl)) {
      return { ok: false, hint: '请先完成口播成片（数字人成片或手动填入视频地址）' }
    }
  }
  if (stepIndex === 2 && track === 'graphic') {
    const images = (draft.imageUrls || []).filter(Boolean)
    if (images.length === 0) {
      return { ok: false, hint: '请至少保留一张配图再去发布' }
    }
  }
  return { ok: true }
}
