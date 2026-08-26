import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams, useLocation, Link } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { Alert, Button, Input, Radio, Segmented, Tag, Typography } from 'antd'
import { message } from '../../../utils/antdApp'
import {
  LinkOutlined, EditOutlined, UploadOutlined, VideoCameraOutlined, UserOutlined,
  RocketOutlined, CheckCircleOutlined, SoundOutlined, ExportOutlined,
} from '@ant-design/icons'
import { transcriptLines } from '../../../utils/transcript'
import { checkMaterialFileSize, friendlyGenerationError } from '../../../utils/generationErrors'
import { extractShareUrl, isKuaishouUrl } from '../../../utils/shareUrl'
import { businessApi } from '../../../api/business'
import type { GenerationTask } from '../../../types/api'
import VoicePicker from '../../../components/VoicePicker'
import { useComposeDraft } from '../../../store/composeDraft'
import { useBrandContext } from '../../../hooks/useBrands'
import { runLipSyncPipeline, type LipSyncPipelineStage } from '../../../hooks/useLipSyncPipeline'
import { useGenerationTasks, GENERATION_TASKS_KEY } from '../../../hooks/useGenerationTasks'
import {
  WizardShell, PhonePreview, PipelineProgress, MaterialDropzone, CapabilityBanner,
  type WizardStepDef, type PipelineStage,
} from '../../../components/wizard'

const { Text } = Typography
const { TextArea } = Input

const WIZARD_STEPS: WizardStepDef[] = [
  { key: 'source', label: '文案来源', title: '从哪里开始？', tip: '粘贴爆款链接提取说话内容，或上传音视频，也可以直接手写', nextLabel: '下一步：确认文案' },
  { key: 'script', label: '确认文案', title: '确认口播文案', tip: '可切换清洗版/改写版，编辑后进入出镜设置', nextLabel: '下一步：选谁出镜' },
  { key: 'presence', label: '出镜方式', title: '谁来出镜？', tip: '真人出镜上传不说话视频；数字分身由 AI 生成画面', nextLabel: '下一步：配音色' },
  { key: 'voice', label: '音色', title: '选择口播音色', tip: '点击试听，选中后进入成片', nextLabel: '下一步：生成成片' },
  { key: 'produce', label: '成片', title: '生成成片', tip: '系统将自动完成语音合成、画面生成与对口型', nextLabel: '去发布' },
]

/** 正常语速 ≈180 字/分钟 ≈3 字/秒（口播时长估算） */
const estSeconds = (text: string) => Math.ceil((text || '').length / 3)

/** 读取本地视频时长（秒） */
function readVideoDuration(file: File): Promise<number> {
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file)
    const v = document.createElement('video')
    v.preload = 'metadata'
    v.onloadedmetadata = () => {
      const d = v.duration
      URL.revokeObjectURL(url)
      resolve(Number.isFinite(d) ? d : 0)
    }
    v.onerror = () => {
      URL.revokeObjectURL(url)
      resolve(0)
    }
    v.src = url
  })
}

type SourceMode = 'link' | 'upload' | 'manual' | null
type ScriptVersion = 'rewrite' | 'clean'

function taskParams(t: GenerationTask): Record<string, any> {
  if (t.params && typeof t.params === 'object') return t.params as Record<string, any>
  if (typeof t.params === 'string' && t.params) {
    try { return JSON.parse(t.params) } catch { return {} }
  }
  return {}
}

/**
 * 拍同款口播视频向导（08 计划 D2 五步）：
 * ① 文案来源 → ② 文案确认 → ③ 出镜方式 → ④ 音色 → ⑤ 成片
 */
export default function LipSyncWizard() {
  const navigate = useNavigate()
  const draft = useComposeDraft()
  const queryClient = useQueryClient()
  const { brandId } = useBrandContext()
  const [searchParams] = useSearchParams()
  const location = useLocation()
  const presetSubjectId = searchParams.get('subject') || location.state?.subjectId || ''
  const presetState = location.state as { rawText?: string; title?: string; method?: string } | null

  const hasDraft = (draft.wizardStep ?? 0) > 0
  const [step, setStep] = useState(presetState?.rawText ? 1 : (draft.wizardStep || 0))
  const [maxReachableStep, setMaxReachableStep] = useState(
    Math.max(step, draft.wizardStep || 0, presetState?.rawText ? 1 : 0)
  )

  // ① 文案来源
  const [sourceMode, setSourceMode] = useState<SourceMode>(null)
  const [shareUrl, setShareUrl] = useState('')
  const [extractLineCount, setExtractLineCount] = useState(0)
  const [extracting, setExtracting] = useState(false)
  // ② 文案
  const [, setRawText] = useState(presetState?.rawText || draft.wizardCleanText || '')
  const [cleanText, setCleanText] = useState(draft.wizardCleanText || '')
  const [rewriteText, setRewriteText] = useState('')
  const [scriptVersion, setScriptVersion] = useState<ScriptVersion>('rewrite')
  const [script, setScript] = useState(presetState?.rawText || draft.wizardScript || '')
  const [topic, setTopic] = useState(draft.wizardTopic || '')
  const [rewriting, setRewriting] = useState(false)
  // ③ 出镜
  const [presence, setPresence] = useState<'real' | 'avatar'>(
    presetSubjectId ? 'avatar' : (draft.wizardPresence || 'real')
  )
  const [realVideoUrl, setRealVideoUrl] = useState(draft.wizardRealVideoUrl || '')
  const [realVideoName, setRealVideoName] = useState('')
  const [realVideoSec, setRealVideoSec] = useState(0)
  const [intent, setIntent] = useState(draft.wizardIntent || '')
  // ④ 音色
  const [voiceId, setVoiceId] = useState(draft.wizardVoiceId || '')
  // ⑤ 成片
  const [producing, setProducing] = useState(false)
  const [pipelineStage, setPipelineStage] = useState<LipSyncPipelineStage>('')
  const [failedStage, setFailedStage] = useState<LipSyncPipelineStage>('')
  const [resultUrl, setResultUrl] = useState(draft.wizardResultUrl || '')
  const [error, setError] = useState('')
  const [ttsTaskId, setTtsTaskId] = useState(draft.wizardTtsTaskId || '')
  const [refTaskId, setRefTaskId] = useState(draft.wizardRefTaskId || '')
  const [lipsyncTaskId, setLipsyncTaskId] = useState(draft.wizardLipsyncTaskId || '')
  const [subjectServerId, setSubjectServerId] = useState(presetSubjectId || draft.wizardSubjectId || '')

  const goStep = (next: number) => {
    setStep(next)
    setMaxReachableStep((m) => Math.max(m, next))
  }

  // 品牌上下文写入草稿
  useEffect(() => {
    if (brandId) draft.patch({ brandId })
  }, [brandId])

  useEffect(() => {
    draft.patch({
      track: 'lipsync',
      wizardStep: step,
      wizardPresence: presence,
      wizardTopic: topic,
      wizardScript: script,
      wizardCleanText: cleanText,
      wizardVoiceId: voiceId,
      wizardRealVideoUrl: realVideoUrl,
      wizardSubjectId: subjectServerId,
      wizardIntent: intent,
      wizardTtsTaskId: ttsTaskId,
      wizardRefTaskId: refTaskId,
      wizardLipsyncTaskId: lipsyncTaskId,
      wizardResultUrl: resultUrl,
    })
  }, [step, presence, topic, script, cleanText, voiceId, realVideoUrl, subjectServerId, intent, ttsTaskId, refTaskId, lipsyncTaskId, resultUrl])

  const [initRewriting, setInitRewriting] = useState(false)
  useEffect(() => {
    if (presetState?.rawText && !initRewriting && cleanText === '') {
      const prefilled = presetState.rawText
      setInitRewriting(true)
      businessApi.rewriteScript({ raw_text: prefilled, topic: topic || '' })
        .then(rw => {
          setCleanText(rw.clean)
          setRewriteText(rw.rewrite || rw.clean)
          setScript(rw.rewrite || rw.clean)
          setScriptVersion('rewrite')
        })
        .catch(() => { setCleanText(prefilled); setScript(prefilled) })
        .finally(() => setInitRewriting(false))
    }
  }, [])

  const { tasks } = useGenerationTasks({ refetchInterval: false })
  const myVoices = useMemo(() => {
    const ids = new Set<string>()
    for (const t of tasks) {
      if (t.sub_type !== 'voice_clone' || t.state !== 'success') continue
      const vid = taskParams(t).voice_id
      if (typeof vid === 'string' && vid) ids.add(vid)
    }
    return Array.from(ids)
  }, [tasks])
  const subjects = useMemo(() => tasks
    .filter(t => t.sub_type === 'subject' && t.state === 'success')
    .map(t => {
      const p = taskParams(t)
      const images = Array.isArray(p.images) ? p.images.filter((u: unknown) => typeof u === 'string') as string[] : []
      return {
        id: t.id,
        serverId: t.provider_task_id,
        name: p.name || t.id.slice(0, 8),
        hasVideo: Array.isArray(p.videos) && p.videos.length > 0,
        portraitUrl: images[0] || t.creations?.[0]?.url || '',
      }
    }), [tasks])

  const selectedSubject = useMemo(
    () => subjects.find((s) => s.serverId === subjectServerId),
    [subjects, subjectServerId],
  )

  const doExtract = async (payload: { share_url?: string; asset_url?: string }) => {
    setExtracting(true); setError('')
    try {
      const r = await businessApi.extractTranscript(payload)
      const lines = transcriptLines(r.raw_text, r.raw_text_lines)
      setExtractLineCount(lines.length)
      setRawText(r.raw_text)
      const rw = await businessApi.rewriteScript({ raw_text: r.raw_text, topic: topic || '口播获客' })
      setCleanText(rw.clean)
      setRewriteText(rw.rewrite || rw.clean)
      setScript(rw.rewrite || rw.clean)
      setScriptVersion('rewrite')
      message.success(`提取完成，共 ${lines.length} 句`)
      goStep(1)
    } catch (e: any) {
      const raw = e?.response?.data?.msg || e?.message || '提取失败'
      setError(friendlyGenerationError(raw))
      // 链接解析类失败：引导切到上传，避免用户反复试链接
      if (/详情解析|分享链|unexpected end of JSON|Cookie|风控/i.test(raw)) {
        message.info('链接提取暂不可用，可改用下方「上传视频」')
      }
    } finally { setExtracting(false) }
  }

  const doRewrite = async () => {
    if (!script.trim()) { message.warning('请先输入文案'); return }
    setRewriting(true)
    try {
      const rw = await businessApi.rewriteScript({ raw_text: script, topic: topic || '口播获客' })
      setRawText(script)
      setCleanText(rw.clean)
      setRewriteText(rw.rewrite || rw.clean)
      setScript(rw.rewrite || rw.clean)
      setScriptVersion('rewrite')
      message.success('已润色')
    } catch { /* 拦截器已提示 */ } finally { setRewriting(false) }
  }

  const switchScriptVersion = (v: ScriptVersion) => {
    if (v === 'clean' && cleanText) {
      if (scriptVersion === 'rewrite') setRewriteText(script)
      setScript(cleanText)
    } else if (v === 'rewrite' && rewriteText) {
      if (scriptVersion === 'clean') setCleanText(script)
      setScript(rewriteText)
    }
    setScriptVersion(v)
  }

  const resolveTaskUrl = (id: string) => {
    if (!id) return ''
    const t = tasks.find(x => x.id === id)
    if (!t || t.state !== 'success') return ''
    return t.creations?.[0]?.stored_url || t.creations?.[0]?.url || ''
  }

  const produce = async (retryFrom?: 'tts' | 'ref' | 'lipsync') => {
    if (!script.trim()) { message.warning('文案为空'); return }
    const bid = brandId || draft.brandId
    if (!bid) { message.warning('请先选择人设/品牌'); return }
    if (presence === 'avatar' && !selectedSubject?.portraitUrl && !subjectServerId) {
      message.warning('请选择带人像图的数字分身')
      return
    }
    if (presence === 'real' && !realVideoUrl) {
      message.warning('请上传出镜视频')
      return
    }
    setProducing(true); setError(''); setFailedStage('')
    if (!retryFrom) setResultUrl('')
    queryClient.invalidateQueries({ queryKey: GENERATION_TASKS_KEY })
    try {
      const result = await runLipSyncPipeline({
        brandId: bid,
        script,
        voiceId: voiceId || undefined,
        presence,
        realVideoUrl,
        portraitMaterial: selectedSubject?.portraitUrl || undefined,
        subjectServerId,
        intent,
      }, {
        onStage: setPipelineStage,
        retryFrom,
        resume: retryFrom ? {
          ttsTaskId,
          refTaskId,
          lipsyncTaskId,
          audioUrl: resolveTaskUrl(ttsTaskId),
          videoUrl: presence === 'avatar' ? resolveTaskUrl(refTaskId) : realVideoUrl,
        } : undefined,
      })
      setTtsTaskId(result.ttsTaskId)
      setRefTaskId(result.refTaskId || '')
      setLipsyncTaskId(result.lipsyncTaskId)
      setResultUrl(result.resultUrl)
      message.success('成片完成')
      queryClient.invalidateQueries({ queryKey: GENERATION_TASKS_KEY })
    } catch (e: any) {
      setError(friendlyGenerationError(e?.response?.data?.msg || e?.message || '成片失败'))
      setFailedStage(pipelineStage)
    } finally {
      setProducing(false)
      setPipelineStage('')
    }
  }

  const scriptSec = estSeconds(script)
  const scriptMin = scriptSec / 60
  const durationMismatch = presence === 'real' && realVideoSec > 0 && scriptSec > 0
    && (Math.max(realVideoSec, scriptSec) / Math.min(realVideoSec, scriptSec) > 2)
  const longAvatarScript = presence === 'avatar' && script.length > 60
  const kuaishouHint = sourceMode === 'link' && isKuaishouUrl(shareUrl)
  const stepKey = WIZARD_STEPS[step]?.key || 'source'

  const pipelineStages = useMemo((): PipelineStage[] => {
    const stages: PipelineStage[] = [
      { key: 'tts', label: '语音合成', status: 'pending' },
    ]
    if (presence === 'avatar') {
      stages.push({ key: 'ref', label: '数字分身画面', status: 'pending' })
    }
    stages.push({ key: 'lipsync', label: '对口型成片', status: 'pending' })

    const order = stages.map(s => s.key)
    if (resultUrl) {
      return stages.map(s => ({ ...s, status: 'done' as const }))
    }

    const activeIdx = pipelineStage ? order.indexOf(pipelineStage) : -1
    if (error && activeIdx >= 0) {
      return stages.map((s, i) => ({
        ...s,
        status: i < activeIdx ? 'done' : i === activeIdx ? 'error' : 'pending',
      }))
    }
    if (!producing || activeIdx < 0) return stages

    return stages.map((s, i) => ({
      ...s,
      status: i < activeIdx ? 'done' : i === activeIdx ? 'active' : 'pending',
    }))
  }, [presence, producing, pipelineStage, resultUrl, error])

  const canNext = (): boolean => {
    if (step === 0) return false
    if (step === 1) return !!script.trim()
    if (step === 2) {
      if (presence === 'real') return !!realVideoUrl
      return !!subjectServerId && !!selectedSubject?.portraitUrl
    }
    if (step === 3) return true // 音色可选；未选则后端用默认
    return false
  }

  const nextHint = (): string | undefined => {
    if (step === 1 && !script.trim()) return '请先填写口播文案'
    if (step === 2 && presence === 'real' && !realVideoUrl) return '请上传出镜视频'
    if (step === 2 && presence === 'avatar' && !subjectServerId) return '请选择数字分身'
    if (step === 2 && presence === 'avatar' && subjectServerId && !selectedSubject?.portraitUrl) {
      return '该分身缺少人像图，请换一个或去素材库补图'
    }
    return undefined
  }

  const handleNext = () => {
    if (step === 4) {
      if (resultUrl) {
        const q = new URLSearchParams()
        if (brandId) q.set('brandId', brandId)
        q.set('mediaUrls', resultUrl)
        q.set('contentType', 'video')
        if (script.trim()) q.set('content', script.trim().slice(0, 8000))
        const pubTitle = (draft.selectedTitle || topic || '').trim()
        if (pubTitle) q.set('title', pubTitle)
        navigate(`/m/distribution?${q.toString()}`)
      } else if (!producing) {
        produce()
      }
      return
    }
    if (canNext()) goStep(step + 1)
  }

  const handleBack = () => {
    if (step === 0) navigate('/m/compose')
    else setStep(step - 1)
  }

  const presetFromVideo = (location.state as { fromVideoTrack?: boolean } | null)?.fromVideoTrack

  const alerts = (
    <>
      {presetFromVideo && (
        <Alert
          type="info" showIcon closable className="wz-draft-banner"
          message="发视频已升级为口播向导——你的文案已自动带入"
        />
      )}
      {error && (
        <Alert
          type="error" showIcon className="wz-draft-banner"
          message={error}
          action={failedStage ? (
            <Button size="small" onClick={() => produce(failedStage === 'ref' ? 'ref' : failedStage === 'lipsync' ? 'lipsync' : 'tts')}>
              从失败步骤重试
            </Button>
          ) : undefined}
        />
      )}
      {hasDraft && step > 0 && !resultUrl && (
        <Alert
          type="info" showIcon closable className="wz-draft-banner"
          message={`已恢复上次进度（第 ${(draft.wizardStep || 0) + 1} 步）`}
        />
      )}
      {!brandId && (
        <Alert
          type="warning" showIcon className="wz-draft-banner"
          message={<>请先在 <Link to="/m/brands">账号人设</Link> 选择品牌，生成内容将关联到该品牌</>}
        />
      )}
      <CapabilityBanner required={['lip_sync', 'tts', 'reference2video']} />
    </>
  )

  const footerNextLabel = step === 4
    ? (resultUrl ? '去发布' : producing ? '生成中…' : '一键成片')
    : undefined

  return (
    <WizardShell
      steps={WIZARD_STEPS}
      stepIndex={step}
      maxReachableStep={maxReachableStep}
      onStepChange={setStep}
      preview={
        <PhonePreview
          script={script}
          videoUrl={realVideoUrl || undefined}
          resultUrl={resultUrl || undefined}
          presence={presence}
          stepKey={stepKey}
          estimatedSeconds={scriptSec}
          topic={topic}
        />
      }
      onBack={handleBack}
      onNext={handleNext}
      nextDisabled={(step < 4 && !canNext()) || (step === 4 && producing)}
      nextHint={nextHint()}
      nextLoading={extracting || rewriting || producing || initRewriting}
      nextLabel={footerNextLabel}
      backLabel={step === 0 ? '返回工作台' : undefined}
      alerts={alerts}
    >
      {step === 0 && (
        <div className="ip-stagger">
          <div className="wz-source-grid">
            <button
              type="button"
              className={`wz-source-card${sourceMode === 'link' ? ' is-active' : ''}`}
              onClick={() => setSourceMode('link')}
            >
              <span className="wz-source-card-icon"><LinkOutlined /></span>
              <strong>粘贴分享链接</strong>
              <span>从抖音 / B站爆款提取说话内容</span>
            </button>
            <button
              type="button"
              className={`wz-source-card${sourceMode === 'upload' ? ' is-active' : ''}`}
              onClick={() => setSourceMode('upload')}
            >
              <span className="wz-source-card-icon"><UploadOutlined /></span>
              <strong>上传音/视频</strong>
              <span>从本地文件提取文案</span>
            </button>
            <button
              type="button"
              className={`wz-source-card${sourceMode === 'manual' ? ' is-active' : ''}`}
              onClick={() => { setSourceMode('manual'); goStep(1) }}
            >
              <span className="wz-source-card-icon"><EditOutlined /></span>
              <strong>手写文案</strong>
              <span>跳过提取，直接写口播稿</span>
            </button>
          </div>

          {sourceMode === 'link' && (
            <div className="wz-source-expand">
              <Input.Search
                size="large"
                enterButton={<><LinkOutlined /> 提取文案</>}
                placeholder="粘贴抖音 / B站分享链接（其他平台请下载后上传）"
                value={shareUrl}
                onChange={e => setShareUrl(e.target.value)}
                loading={extracting}
                onSearch={() => {
                  if (!shareUrl.trim()) return
                  if (isKuaishouUrl(shareUrl)) {
                    message.info('快手暂不支持链接提取，请下载视频后用上传方式')
                    return
                  }
                  const link = extractShareUrl(shareUrl)
                  if (!link) {
                    message.warning('未识别到抖音/B站链接。请粘贴完整分享口令（需含 https://v.douyin.com/…），或改用上传视频')
                    return
                  }
                  if (link !== shareUrl.trim()) setShareUrl(link)
                  doExtract({ share_url: link })
                }}
              />
              <Text type="secondary" style={{ display: 'block', marginTop: 8, fontSize: 12 }}>
                可直接粘贴抖音「复制链接」整段口令，系统会自动抽出链接
              </Text>
              {kuaishouHint && (
                <Text type="secondary" style={{ display: 'block', marginTop: 8, fontSize: 12 }}>
                  快手暂不支持链接提取，请下载视频后用上传方式
                </Text>
              )}
            </div>
          )}

          {sourceMode === 'upload' && (
            <div className="wz-source-expand">
              <MaterialDropzone
                accept="audio/*,video/*"
                hint="支持 mp4 / mov / mp3 / wav，上传后自动提取文案"
                loading={extracting}
                onUpload={async (file) => {
                  const sizeCheck = checkMaterialFileSize(file)
                  if (!sizeCheck.ok) {
                    message.error(sizeCheck.error)
                    return
                  }
                  if (sizeCheck.warning) message.warning(sizeCheck.warning)
                  const r = await businessApi.uploadAsset(file)
                  await doExtract({ asset_url: r.url })
                }}
              />
            </div>
          )}
        </div>
      )}

      {step === 1 && (
        <div className="ip-form-stack ip-stagger">
          <label>一句话主题（AI 改写围绕它）</label>
          <Input
            placeholder="如：酸菜鱼餐馆新菜品推广"
            value={topic}
            onChange={e => setTopic(e.target.value)}
            maxLength={100}
          />
          {(cleanText || rewriteText) && (
            <Segmented
              value={scriptVersion}
              onChange={v => switchScriptVersion(v as ScriptVersion)}
              options={[
                { label: '改写版（推荐）', value: 'rewrite', disabled: !rewriteText && !cleanText },
                { label: '清洗版原文', value: 'clean', disabled: !cleanText },
              ]}
            />
          )}
          <TextArea
            rows={9}
            showCount
            value={script}
            onChange={e => setScript(e.target.value)}
            placeholder="输入或提取口播文案…"
          />
          <div className="wz-script-toolbar">
            <span className="wz-duration-ring">
              预计口播约 <strong>{scriptMin >= 1 ? `${scriptMin.toFixed(1)} 分钟` : `${scriptSec} 秒`}</strong>
              {extractLineCount > 0 && (
                <Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>提取 {extractLineCount} 句</Text>
              )}
              {scriptMin > 10 && (
                <Text type="danger" style={{ marginLeft: 8, fontSize: 12 }}>文案过长，建议精简</Text>
              )}
            </span>
            <Button loading={rewriting} onClick={doRewrite} disabled={!script.trim()}>
              AI 润色/改写
            </Button>
          </div>
        </div>
      )}

      {step === 2 && (
        <div className="ip-stagger">
          <div className="wz-presence-grid">
            <button
              type="button"
              className={`wz-presence-card${presence === 'real' ? ' is-active' : ''}`}
              onClick={() => setPresence('real')}
            >
              <span className="wz-presence-card-icon"><VideoCameraOutlined /></span>
              <strong>真人出镜</strong>
              <span>上传自己拍的不说话视频，成片里你对口型开口</span>
            </button>
            <button
              type="button"
              className={`wz-presence-card${presence === 'avatar' ? ' is-active' : ''}`}
              onClick={() => setPresence('avatar')}
            >
              <span className="wz-presence-card-icon"><UserOutlined /></span>
              <strong>数字分身</strong>
              <span>AI 生成出镜画面，再对口型合成成片</span>
            </button>
          </div>

          <div className="wz-presence-detail">
            {presence === 'real' ? (
              <>
                <Alert type="info" showIcon message="正脸、光线稳定、不说话的视频效果最好" />
                <MaterialDropzone
                  accept="video/mp4,video/quicktime,video/x-msvideo"
                  hint={`文案约 ${scriptSec} 秒，出镜视频时长最好相近`}
                  fileName={realVideoName || (realVideoUrl ? '已上传出镜视频' : undefined)}
                  onUpload={async (file) => {
                    const sizeCheck = checkMaterialFileSize(file)
                    if (!sizeCheck.ok) {
                      message.error(sizeCheck.error)
                      return
                    }
                    if (sizeCheck.warning) message.warning(sizeCheck.warning)
                    const dur = await readVideoDuration(file)
                    const r = await businessApi.uploadAsset(file)
                    setRealVideoUrl(r.url)
                    setRealVideoName(file.name)
                    setRealVideoSec(dur)
                    message.success(dur > 0 ? `出镜视频已上传（时长 ${Math.round(dur)} 秒）` : '出镜视频已上传')
                  }}
                />
                {realVideoSec > 0 && (
                  <Text type="secondary" style={{ display: 'block', marginTop: 8, fontSize: 12 }}>
                    视频时长 {Math.round(realVideoSec)} 秒 · 文案预计 {scriptSec} 秒
                  </Text>
                )}
                {durationMismatch && (
                  <Alert
                    type="warning"
                    showIcon
                    style={{ marginTop: 8 }}
                    message="视频时长与文案时长差距较大，建议匹配（可循环拍摄多段或精简文案）"
                  />
                )}
              </>
            ) : (
              <>
                <Alert type="info" showIcon message="选择资产库里的数字分身，一句话描述场景" />
                {longAvatarScript && (
                  <Alert
                    type="warning"
                    showIcon
                    style={{ marginBottom: 8 }}
                    message="文案较长，将生成长视频（等待分段功能上线前建议压缩到 60 字内获得最佳效果）"
                  />
                )}
                <div className="wz-subject-picks">
                  {subjects.length === 0 ? (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
                      <Text type="warning">还没有数字分身</Text>
                      <Button type="primary" size="small" onClick={() => navigate('/m/assets?create=subject')}>
                        去创建数字分身
                      </Button>
                    </div>
                  ) : subjects.map(s => (
                    <Radio.Button
                      key={s.id}
                      className="wz-subject-pick"
                      checked={subjectServerId === s.serverId}
                      onClick={() => setSubjectServerId(s.serverId)}
                    >
                      {s.name}{s.hasVideo ? '（视频分身）' : ''}
                    </Radio.Button>
                  ))}
                </div>
                <Input
                  placeholder="场景意图（如：在厨房边做菜边对镜头讲解）"
                  value={intent}
                  onChange={e => setIntent(e.target.value)}
                  maxLength={200}
                />
              </>
            )}
          </div>
        </div>
      )}

      {step === 3 && (
        <div className="ip-form-stack ip-stagger">
          <label><SoundOutlined /> 选择口播音色（可试听）</label>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message="音色可选——未选用系统默认音色（后端 voice_setting_voice_id 已通）"
          />
          <VoicePicker value={voiceId} onChange={setVoiceId} myVoices={myVoices} style={{ maxWidth: 480 }} />
          <Text type="secondary" style={{ fontSize: 12 }}>
            可跳过；想用自己的声音？<a href="/m/compose/tools?tab=media" target="_blank" rel="noreferrer">去声音克隆</a>
          </Text>
        </div>
      )}

      {step === 4 && (
        <div className="ip-stagger">
          <div className="wz-ready-tags">
            <Tag color="green">文案 {script.length} 字 · 约 {scriptSec} 秒</Tag>
            <Tag color="green">{presence === 'real' ? '真人出镜' : '数字分身'}</Tag>
            <Tag color={voiceId ? 'green' : 'default'}>{voiceId ? '音色已选' : '默认音色'}</Tag>
          </div>

          <PipelineProgress stages={pipelineStages} />

          {!resultUrl && !producing && (
            <div className="wz-produce-actions">
              <Button type="primary" size="large" icon={<RocketOutlined />} onClick={() => produce()}>
                一键成片
              </Button>
            </div>
          )}

          {producing && (
            <Alert type="info" showIcon style={{ marginTop: 14 }} message="生成中，请勿关闭页面…" />
          )}

          {resultUrl && (
            <>
              <Alert
                type="success" showIcon icon={<CheckCircleOutlined />}
                message="成片完成" style={{ marginTop: 14 }}
              />
              <video
                src={resultUrl}
                controls
                style={{ width: '100%', maxWidth: 420, borderRadius: 12, marginTop: 12 }}
              />
              <div className="wz-produce-actions">
                <Button href={resultUrl} target="_blank" download>下载成片</Button>
                <Button type="primary" icon={<ExportOutlined />} onClick={handleNext}>
                  去发布
                </Button>
              </div>
            </>
          )}
        </div>
      )}
    </WizardShell>
  )
}
