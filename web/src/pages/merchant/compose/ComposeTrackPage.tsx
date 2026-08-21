import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { message } from 'antd'
import { GRAPHIC_FLOW_STEPS, VIDEO_FLOW_STEPS } from '../../../config/product'
import { useComposeDraft, type ComposeTrack } from '../../../store/composeDraft'
import { useBrandContext } from '../../../hooks/useBrands'
import { ComposeFlowShell } from './ComposeFlowShell'
import { ComposePreview } from './ComposePreview'
import { ScriptStep } from './steps/ScriptStep'
import { VideoAssetsStep } from './steps/VideoAssetsStep'
import { VideoProduceStep } from './steps/VideoProduceStep'
import { GraphicAssetsStep } from './steps/GraphicAssetsStep'
import { GraphicProduceStep } from './steps/GraphicProduceStep'

type Props = { track: ComposeTrack }

/** 单轨道步骤式工作台：左编辑 / 右预览 */
export default function ComposeTrackPage({ track }: Props) {
  const navigate = useNavigate()
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const steps = track === 'video' ? VIDEO_FLOW_STEPS : GRAPHIC_FLOW_STEPS
  const [stepIndex, setStepIndex] = useState(0)

  useEffect(() => {
    draft.setTrack(track)
    setStepIndex(0)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [track])

  const step = steps[stepIndex]
  const body = draft.rewritten || draft.script || draft.transcript || ''
  const hasScript = !!body.trim()

  const nextDisabled = stepIndex === 0 && !hasScript
  const nextHint = nextDisabled
    ? (track === 'video' ? '请先写好口播文案' : '请先写好图文文案')
    : undefined

  const goPublish = () => {
    const q = new URLSearchParams()
    if (track === 'video') {
      q.set('contentType', 'video')
      const media = draft.editedVideoUrl || draft.avatarVideoUrl
      if (media) q.set('mediaUrls', media)
      if (draft.coverUrl) q.set('coverUrl', draft.coverUrl)
    } else {
      q.set('contentType', 'article')
      const imgs = [...(draft.imageUrls || []), draft.coverUrl].filter(Boolean) as string[]
      if (imgs.length) q.set('mediaUrls', imgs.join(','))
    }
    if (draft.selectedTitle) q.set('title', draft.selectedTitle)
    const bid = brandId || draft.brandId
    if (bid) q.set('brandId', bid)
    navigate(`/m/distribution?${q.toString()}`)
  }

  const onBack = () => {
    if (stepIndex === 0) {
      navigate('/m/compose')
      return
    }
    setStepIndex((i) => i - 1)
  }

  const onNext = () => {
    if (stepIndex === 0 && !hasScript) {
      message.warning(nextHint || '请先完成文案')
      return
    }
    if (stepIndex >= steps.length - 1) {
      goPublish()
      return
    }
    setStepIndex((i) => i + 1)
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
      onStepChange={(i) => {
        if (i > 0 && !hasScript) {
          message.warning(track === 'video' ? '请先写好口播文案' : '请先写好图文文案')
          return
        }
        setStepIndex(i)
      }}
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
