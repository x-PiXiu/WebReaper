import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { GRAPHIC_FLOW_STEPS, VIDEO_FLOW_STEPS } from '../../../config/product'
import { useComposeDraft, type ComposeTrack } from '../../../store/composeDraft'
import { useBrandContext } from '../../../hooks/useBrands'
import { useComposeTaskPoll } from '../../../hooks/useComposeTaskPoll'
import { useComposeWorkSync } from '../../../hooks/useComposeWorkSync'
import {
  resolveComposeStepIndex,
  validateComposeStep,
} from '../../../utils/composeProgress'
import { ComposeFlowShell } from './ComposeFlowShell'
import { ComposePreview } from './ComposePreview'
import { ScriptStep } from './steps/ScriptStep'
import { VideoAssetsStep } from './steps/VideoAssetsStep'
import { VideoProduceStep } from './steps/VideoProduceStep'
import { GraphicAssetsStep } from './steps/GraphicAssetsStep'
import { GraphicProduceStep } from './steps/GraphicProduceStep'
import { message } from '../../../utils/antdApp'

type Props = { track: ComposeTrack }

/** 单轨道步骤式工作台：左编辑 / 右预览 */
export default function ComposeTrackPage({ track }: Props) {
  const navigate = useNavigate()
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const { touchDraft } = useComposeWorkSync()
  const steps = track === 'video' ? VIDEO_FLOW_STEPS : GRAPHIC_FLOW_STEPS
  const [stepIndex, setStepIndex] = useState(() => resolveComposeStepIndex(draft, track))

  useComposeTaskPoll()

  useEffect(() => {
    draft.setTrack(track)
    setStepIndex(resolveComposeStepIndex(draft, track))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [track])

  const persistStep = (index: number) => {
    setStepIndex(index)
    draft.patch({ stepIndex: index, lastUpdatedAt: new Date().toISOString() })
  }

  const step = steps[stepIndex]
  const body = draft.rewritten || draft.script || draft.transcript || ''
  const hasScript = !!body.trim()
  const validation = validateComposeStep(draft, track, stepIndex)
  const nextDisabled = !validation.ok
  const nextHint = validation.hint

  const goPublish = () => {
    const q = new URLSearchParams()
    if (draft.contentId) q.set('contentId', draft.contentId)
    if (track === 'video') {
      q.set('contentType', 'video')
      const media = draft.editedVideoUrl || draft.avatarVideoUrl
      if (media) q.set('mediaUrls', media)
      if (draft.coverUrl) q.set('coverUrl', draft.coverUrl)
    } else {
      // 图文种草 → 平台矩阵 image（小红书图文），非长文 article
      q.set('contentType', 'image')
      const imgs = [...(draft.imageUrls || []), draft.coverUrl].filter(Boolean) as string[]
      if (imgs.length) q.set('mediaUrls', imgs.join(','))
    }
    if (draft.selectedTitle) q.set('title', draft.selectedTitle)
    const bodyText = (draft.rewritten || draft.script || draft.transcript || '').trim()
    if (bodyText) q.set('content', bodyText.slice(0, 8000))
    const bid = brandId || draft.brandId
    if (bid) q.set('brandId', bid)
    touchDraft()
    navigate(`/m/distribution?${q.toString()}`)
  }

  const onBack = () => {
    if (stepIndex === 0) {
      navigate('/m/compose')
      return
    }
    persistStep(stepIndex - 1)
  }

  const onNext = () => {
    const v = validateComposeStep(draft, track, stepIndex)
    if (!v.ok) {
      message.warning(v.hint || '请先完成当前步骤')
      return
    }
    if (stepIndex >= steps.length - 1) {
      goPublish()
      return
    }
    persistStep(stepIndex + 1)
  }

  const onStepChange = (i: number) => {
    if (i > 0 && !hasScript) {
      message.warning(track === 'video' ? '请先写好口播文案' : '请先写好图文文案')
      return
    }
    if (i > stepIndex) {
      const v = validateComposeStep(draft, track, stepIndex)
      if (!v.ok) {
        message.warning(v.hint || '请先完成当前步骤')
        return
      }
    }
    persistStep(i)
  }

  let workspace: React.ReactNode = null
  if (step.key === 'script') workspace = <ScriptStep track={track} />
  else if (track === 'video' && step.key === 'assets') workspace = <VideoAssetsStep />
  else if (track === 'video' && step.key === 'produce') workspace = <VideoProduceStep />
  else if (track === 'graphic' && step.key === 'assets') workspace = <GraphicAssetsStep />
  else if (track === 'graphic' && step.key === 'produce') workspace = <GraphicProduceStep />

  return (
    <ComposeFlowShell
      track={track}
      steps={steps}
      stepIndex={stepIndex}
      onStepChange={onStepChange}
      preview={<ComposePreview track={track} stepKey={step.key} />}
      onBack={onBack}
      onNext={onNext}
      nextDisabled={nextDisabled}
      nextHint={nextHint}
    >
      {workspace}
    </ComposeFlowShell>
  )
}
