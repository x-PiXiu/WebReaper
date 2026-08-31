import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearchParams, useLocation, Link } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { Alert, Button, Input, Popconfirm, Popover, Segmented, Select, Space, Tag, Typography } from 'antd'
import { message } from '../../../utils/antdApp'
import {
  EditOutlined, UploadOutlined, VideoCameraOutlined, UserOutlined,
  RocketOutlined, CheckCircleOutlined, SoundOutlined, ExportOutlined, ClockCircleOutlined,
  VideoCameraAddOutlined, StopOutlined,
} from '@ant-design/icons'
import { transcriptLines } from '../../../utils/transcript'
import { checkMaterialFileSize, friendlyGenerationError } from '../../../utils/generationErrors'
import { extractShareUrl, isKuaishouUrl } from '../../../utils/shareUrl'
import { businessApi } from '../../../api/business'
import VoicePicker from '../../../components/VoicePicker'
import { useComposeDraft } from '../../../store/composeDraft'
import { useBrandContext } from '../../../hooks/useBrands'
import { runLipSyncPipeline, type LipSyncAudioSource, type LipSyncPipelineStage } from '../../../hooks/useLipSyncPipeline'
import { useGenerationTasks, GENERATION_TASKS_KEY } from '../../../hooks/useGenerationTasks'
import { useSubjectList } from '../../../hooks/useSubjectList'
import { parseGenerationTaskParams } from '../../../utils/subjectTask'
import { ScriptLinesEditor } from '../../../components/compose/ScriptLinesEditor'
import { SubjectPicker } from '../../../components/compose/SubjectPicker'
import {
  WizardShell, PhonePreview, PipelineProgress, MaterialDropzone, CapabilityBanner,
  type WizardStepDef, type PipelineStage,
} from '../../../components/wizard'

const { Text } = Typography

const WIZARD_STEPS: WizardStepDef[] = [
  { key: 'script', label: '文案', title: '第一步：搞定文案', tip: '', nextLabel: '下一步 ›' },
  { key: 'config', label: '配置', title: '第二步：配置出镜与配音', tip: '', nextLabel: '下一步 ›' },
  { key: 'produce', label: '生成', title: '第三步：生成成片', tip: '', nextLabel: '一键成片' },
  { key: 'publish', label: '完成', title: '第四步：完成', tip: '', nextLabel: '去发布' },
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

/**
 * 拍口播视频向导（23 号计划四步式）：
 * ① 确定文案（三来源提取，逐句编辑，润色显式可选）→ ② 出镜与配音（形态二选一 + 音频三选一）
 * → ③ 生成成片（阶段进度 + 断点重试 + 可取消）→ ④ 发布
 */
export default function LipSyncWizard() {
  const navigate = useNavigate()
  const draft = useComposeDraft()
  const queryClient = useQueryClient()
  const { brandId, brands, setCurrentBrand } = useBrandContext()
  const [searchParams] = useSearchParams()
  const location = useLocation()
  const presetSubjectId = searchParams.get('subject') || location.state?.subjectId || ''
  const presetState = location.state as { rawText?: string; title?: string; method?: string } | null

  // 旧版 5 步草稿（0来源 1文案 2出镜 3音频 4成片）迁移到新版 4 步（0文案 1配置 2成片 3发布）
  // wizardSchema >= 4 的新草稿不做迁移（否则新值 2/3 会被二次映射弹回旧步）
  const isLegacySchema = (draft.wizardSchema ?? 0) < 4
  const legacyStep = draft.wizardStep
  const migratedStep = legacyStep == null || !isLegacySchema
    ? (legacyStep ?? 0)
    : legacyStep >= 4 ? 2 : legacyStep >= 2 ? 1 : legacyStep
  const [step, setStep] = useState(presetState?.rawText ? 0 : migratedStep)
  const [maxReachableStep, setMaxReachableStep] = useState(Math.max(step, migratedStep))

  // ① 文案来源
  const [sourceMode, setSourceMode] = useState<SourceMode>('link')
  const [shareUrl, setShareUrl] = useState('')
  const [extracting, setExtracting] = useState(false)
  const [extractStage, setExtractStage] = useState('')
  // ② 文案（逐句形态：一行一句；润色显式可选——提取结果原样进编辑器，不自动润色）
  const [cleanText, setCleanText] = useState(draft.wizardCleanText || '')
  const [rewriteText, setRewriteText] = useState('')
  const [scriptVersion, setScriptVersion] = useState<ScriptVersion>('clean')
  const [script, setScript] = useState(presetState?.rawText || draft.wizardScript || '')
  const [topic, setTopic] = useState(draft.wizardTopic || presetState?.title || '')
  const [rewriting, setRewriting] = useState(false)
  const [rewritePopOpen, setRewritePopOpen] = useState(false)
  const [rewriteReq, setRewriteReq] = useState('')
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
  // ④ 音频来源（23 号计划三选一）：A 文本+音色（默认）/ B 文本直生 / C 上传已录音频
  const [audioSource, setAudioSource] = useState<LipSyncAudioSource>(draft.wizardAudioSource || 'tts')
  const [uploadedAudioUrl, setUploadedAudioUrl] = useState(draft.wizardUploadedAudioUrl || '')
  const [uploadingAudio, setUploadingAudio] = useState(false)
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
  // 生成中可取消（23 号计划 §4.1）：跟踪活动任务，取消调服务端真实取消端点
  const [activeTaskId, setActiveTaskId] = useState('')
  const [cancelling, setCancelling] = useState(false)
  const cancelRequested = useRef(false)

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
      wizardAudioSource: audioSource,
      wizardUploadedAudioUrl: uploadedAudioUrl,
      wizardSchema: 4,
    })
  }, [step, presence, topic, script, cleanText, voiceId, realVideoUrl, subjectServerId, intent, ttsTaskId, refTaskId, lipsyncTaskId, resultUrl, audioSource, uploadedAudioUrl])

  // 预填原文（灵感广场等入口带入）原样进编辑器——不自动润色（23 号计划 §3.1：显式可选）
  useEffect(() => {
    if (presetState?.rawText) {
      setCleanText(presetState.rawText)
      setScript(presetState.rawText)
      setScriptVersion('clean')
    }
  }, [])

  // 恢复草稿时若上次停留在改写版，回填改写文案（草稿只存最终 script 与 cleanText）
  useEffect(() => {
    if (!presetState?.rawText && draft.wizardCleanText && draft.wizardScript
      && draft.wizardScript !== draft.wizardCleanText) {
      setRewriteText(draft.wizardScript)
      setScriptVersion('rewrite')
    }
  }, [])

  const { tasks } = useGenerationTasks({ refetchInterval: false })
  const { ready: subjects } = useSubjectList({ refetchInterval: false })
  const myVoices = useMemo(() => {
    const ids = new Set<string>()
    for (const t of tasks) {
      if (t.sub_type !== 'voice_clone' || t.state !== 'success') continue
      const vid = parseGenerationTaskParams(t).voice_id
      if (typeof vid === 'string' && vid) ids.add(vid)
    }
    return Array.from(ids)
  }, [tasks])

  const selectedSubject = useMemo(
    () => subjects.find((s) => s.serverId === subjectServerId),
    [subjects, subjectServerId],
  )

  // 链接提取统一走异步轮询：提交 → 3s 间隔轮询直至 done/error（后端任务 TTL 30min）
  // 提取等待态阶段提示（§3.3）：按已耗时轮播 解析→下载→识别
  useEffect(() => {
    if (!extracting) { setExtractStage(''); return }
    const started = Date.now()
    const STAGES: Array<{ after: number; label: string }> = [
      { after: 0, label: '正在解析链接…' },
      { after: 6_000, label: '正在下载原视频…（时长取决于原视频大小）' },
      { after: 18_000, label: '正在 AI 语音识别，提取口播文案…' },
    ]
    const tick = () => {
      const elapsed = Date.now() - started
      const stage = [...STAGES].reverse().find((s) => elapsed >= s.after)
      setExtractStage(stage?.label || '')
    }
    tick()
    const timer = window.setInterval(tick, 1000)
    return () => window.clearInterval(timer)
  }, [extracting])

  const extractWithPolling = async (payload: { share_url?: string; video_url?: string; asset_url?: string }) => {
    const start = await businessApi.extractTranscriptAsync(payload)
    const deadline = Date.now() + 29 * 60 * 1000
    while (Date.now() < deadline) {
      await new Promise(r => setTimeout(r, 3000))
      const t = await businessApi.getTranscriptTask(start.task_id)
      if (t.raw_text !== undefined) return t as { raw_text: string; raw_text_lines?: string[]; title: string; method: string }
      // error 时后端返回 HTTP 400（axios 拦截器已 throw）——能走到这里只会是 pending
    }
    throw new Error('提取超时（超过 29 分钟），请尝试较短的视频或下载后上传')
  }

  const doExtract = async (payload: { share_url?: string; asset_url?: string }) => {
    setExtracting(true); setError('')
    try {
      // 链接与上传均走异步轮询（23 §3.2）——上传用 video_url=本站素材地址，避免同步 120s 超时
      const r = payload.asset_url
        ? await extractWithPolling({ video_url: payload.asset_url, asset_url: payload.asset_url })
        : await extractWithPolling(payload.share_url ? { share_url: payload.share_url } : {})
      const lines = transcriptLines(r.raw_text, r.raw_text_lines)
      // 原样逐句进编辑器（不自动润色——润色是显式可选项，23 号计划 §3.1）
      setCleanText(r.raw_text)
      setRewriteText('')
      setScript(r.raw_text)
      setScriptVersion('clean')
      message.success(`提取完成，共 ${lines.length} 句——请在下方逐句确认文案`)
    } catch (e: any) {
      const raw = e?.response?.data?.msg || e?.message || '提取失败'
      setError(friendlyGenerationError(raw))
      // 链接解析类失败：引导切到上传，避免用户反复试链接
      if (/详情解析|分享链|unexpected end of JSON|Cookie|风控/i.test(raw)) {
        message.info('链接提取暂不可用，可改用下方「上传视频」')
      }
    } finally { setExtracting(false) }
  }

  /** 显式 AI 润色（23 号计划 §3.1②：点按钮 → 输入一句话需求 → 双版本二选一） */
  const doRewrite = async (req: string) => {
    if (!script.trim()) { message.warning('请先输入文案'); return }
    setRewriting(true)
    setRewritePopOpen(false)
    try {
      // 服务端 rewrite 的 topic 是自由文本进 LLM 提示——需求拼入（待服务端加独立字段）
      const topicFull = [
        topic.trim() || '口播获客',
        req.trim() ? `润色要求：${req.trim()}` : '',
      ].filter(Boolean).join('；')
      const rw = await businessApi.rewriteScript({ raw_text: script, topic: topicFull })
      setCleanText(rw.clean)
      setRewriteText(rw.rewrite || rw.clean)
      setScript(rw.rewrite || rw.clean)
      setScriptVersion('rewrite')
      message.success('已润色——可在「原文 / AI 改写版」间切换应用')
    } catch { /* 拦截器已提示 */ } finally {
      setRewriting(false)
      setRewriteReq('')
    }
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
    if (presence === 'avatar' && !subjectServerId && !selectedSubject?.portraitUrl) {
      message.warning('请选择数字分身')
      return
    }
    if (presence === 'real' && !realVideoUrl) {
      message.warning('请上传出镜视频')
      return
    }
    if (audioSource === 'upload' && !uploadedAudioUrl) {
      message.warning('请先上传已录音频，或改选 TTS 配音')
      return
    }
    setProducing(true); setError(''); setFailedStage('')
    cancelRequested.current = false
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
        subjectServerId: subjectServerId || undefined,
        subjectName: selectedSubject?.name,
        intent,
        audioSource,
        uploadedAudioUrl: uploadedAudioUrl || undefined,
      }, {
        onStage: setPipelineStage,
        onTaskSubmit: (_stage, taskId) => setActiveTaskId(taskId),
        retryFrom,
        resume: retryFrom ? {
          ttsTaskId,
          refTaskId,
          lipsyncTaskId,
          audioUrl: audioSource === 'upload'
            ? uploadedAudioUrl
            : audioSource === 'direct' ? '' : resolveTaskUrl(ttsTaskId),
          videoUrl: presence === 'avatar' ? resolveTaskUrl(refTaskId) : realVideoUrl,
        } : undefined,
      })
      setTtsTaskId(result.ttsTaskId)
      setRefTaskId(result.refTaskId || '')
      setLipsyncTaskId(result.lipsyncTaskId)
      setResultUrl(result.resultUrl)
      message.success('成片完成')
      queryClient.invalidateQueries({ queryKey: GENERATION_TASKS_KEY })
      goStep(3)
    } catch (e: any) {
      if (cancelRequested.current) {
        // 用户主动取消：不算失败——已完成阶段的产物保留，可断点重试
        message.info('已取消生成——已完成阶段的产物已保留，可重新生成或从断点重试')
      } else {
        setError(friendlyGenerationError(e?.response?.data?.msg || e?.message || '成片失败'))
        setFailedStage(pipelineStage)
      }
    } finally {
      setProducing(false)
      setPipelineStage('')
      setCancelling(false)
      setActiveTaskId('')
    }
  }

  /** 取消当前生成（服务端真实取消：上游尽力取消 + 本地置 cancelled；23 号计划 §4.1"可取消"） */
  const cancelProduce = async () => {
    if (!activeTaskId) return
    cancelRequested.current = true
    setCancelling(true)
    try {
      await businessApi.cancelGenerationTask(activeTaskId)
    } catch { /* 轮询到 cancelled 终态后管线自行退出 */ }
  }

  const scriptSec = estSeconds(script)
  const scriptMin = scriptSec / 60
  // 逐句统计（非空行；口播"句"= 一行）
  const scriptLineCount = useMemo(
    () => script.split('\n').filter((l) => l.trim()).length,
    [script],
  )
  // 成片预计耗时（按音频路径与出镜形态；路径 B 单步最快）
  const produceEta = audioSource === 'direct'
    ? '预计 1-2 分钟（单步直出）'
    : audioSource === 'upload'
      ? (presence === 'avatar' ? '预计 2-4 分钟' : '预计 1-3 分钟')
      : (presence === 'avatar' ? '预计 3-5 分钟' : '预计 2-4 分钟')
  const durationMismatch = presence === 'real' && realVideoSec > 0 && scriptSec > 0
    && (Math.max(realVideoSec, scriptSec) / Math.min(realVideoSec, scriptSec) > 2)
  const longAvatarScript = presence === 'avatar' && script.length > 60
  const kuaishouHint = sourceMode === 'link' && isKuaishouUrl(shareUrl)
  const stepKey = WIZARD_STEPS[step]?.key || 'source'

  const pipelineStages = useMemo((): PipelineStage[] => {
    const stages: PipelineStage[] = []
    // 音频路径 A 才有语音合成阶段；B/C 单段直达（23 号计划 §4.2）
    if (audioSource === 'tts') {
      stages.push({ key: 'tts', label: '语音合成', status: 'pending' })
    }
    if (presence === 'avatar') {
      stages.push({
        key: 'ref',
        label: subjectServerId
          ? (audioSource === 'direct' ? '台词直生成片' : '主体一致性视频')
          : '口播画面生成',
        status: 'pending',
      })
    } else {
      stages.push({ key: 'lipsync', label: '对口型成片', status: 'pending' })
    }

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
  }, [presence, producing, pipelineStage, resultUrl, error, subjectServerId, audioSource])

  const canNext = (): boolean => {
    if (step === 0) return !!script.trim()
    if (step === 1) {
      if (!brandId) return false
      const presenceOk = presence === 'real' ? !!realVideoUrl : !!subjectServerId || !!selectedSubject?.portraitUrl
      const audioOk = audioSource !== 'upload' || !!uploadedAudioUrl // 路径 C 需先上传音频
      return presenceOk && audioOk
    }
    return false
  }

  const nextHint = (): string | undefined => {
    if (step === 0 && !script.trim()) return '请先填写口播文案'
    if (step === 1 && !brandId) return '请选择品牌 / 人设'
    if (step === 1 && presence === 'real' && !realVideoUrl) return '请上传出镜视频'
    if (step === 1 && presence === 'avatar' && !subjectServerId && !selectedSubject?.portraitUrl) {
      return '请选择数字分身'
    }
    if (step === 1 && audioSource === 'upload' && !uploadedAudioUrl) return '请先上传已录音频'
    return undefined
  }

  const goPublish = () => {
    const q = new URLSearchParams()
    if (brandId) q.set('brandId', brandId)
    if (resultUrl) q.set('mediaUrls', resultUrl)
    q.set('contentType', 'video')
    if (script.trim()) q.set('content', script.trim().slice(0, 8000))
    const pubTitle = (draft.selectedTitle || topic || '').trim()
    if (pubTitle) q.set('title', pubTitle)
    navigate(`/m/distribution?${q.toString()}`)
  }

  const handleNext = () => {
    if (step === 2) {
      if (resultUrl) goStep(3)
      else if (!producing) produce()
      return
    }
    if (step === 3) {
      if (resultUrl) goPublish()
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
      {presetFromVideo && step === 0 && (
        <Alert
          type="info" showIcon closable className="wz-draft-banner"
          message="文案已从发视频入口自动带入"
        />
      )}
      {error && step !== 2 && step !== 3 && (
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
      <CapabilityBanner required={['lip_sync', 'tts', 'reference2video']} />
    </>
  )

  const footerNextLabel = step === 2
    ? (producing ? '生成中…' : '一键成片')
    : undefined

  const hideFooterNext = !!resultUrl && (step === 2 || step === 3)

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
      nextDisabled={((step === 0 || step === 1) && !canNext()) || ((step === 2 || step === 3) && producing)}
      nextHint={nextHint()}
      nextLoading={extracting || rewriting || producing}
      nextLabel={footerNextLabel}
      hideNext={hideFooterNext}
      backLabel={step === 0 ? '返回工作台' : undefined}
      alerts={alerts}
    >
      {step === 0 && (
        <div className="wz-studio">
          {/* 左：来源与主题 */}
          <section className="wz-studio-col wz-studio-col--source">
            <div className="wz-studio-tabs" role="tablist" aria-label="文案来源">
              <button
                type="button"
                role="tab"
                aria-selected={sourceMode === 'link'}
                className={sourceMode === 'link' ? 'is-active' : undefined}
                onClick={() => setSourceMode('link')}
              >
                爆款链接
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={sourceMode === 'upload'}
                className={sourceMode === 'upload' ? 'is-active' : undefined}
                onClick={() => setSourceMode('upload')}
              >
                上传音视频
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={sourceMode === 'manual'}
                className={sourceMode === 'manual' ? 'is-active' : undefined}
                onClick={() => setSourceMode('manual')}
              >
                手写文案
              </button>
            </div>

            {sourceMode === 'link' && (
              <div className="wz-studio-block">
                <Input.Search
                  size="large"
                  enterButton="提取文案"
                  placeholder="粘贴抖音 / B站分享链接或口令"
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
                      message.warning('未识别到抖音/B站链接。请粘贴完整分享口令，或改用上传视频')
                      return
                    }
                    if (link !== shareUrl.trim()) setShareUrl(link)
                    doExtract({ share_url: link })
                  }}
                />
                <Text type="secondary" className="wz-studio-hint">
                  系统将提取口播原文，在中间栏按「一行一句」展示，可继续润色。
                </Text>
                {kuaishouHint && (
                  <Text type="secondary" className="wz-studio-hint">快手请下载后用上传方式</Text>
                )}
                {extracting && extractStage && (
                  <div className="wz-extract-stage"><span className="wz-extract-dot" />{extractStage}</div>
                )}
              </div>
            )}

            {sourceMode === 'upload' && (
              <div className="wz-studio-block">
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
                {extracting && extractStage && (
                  <div className="wz-extract-stage"><span className="wz-extract-dot" />{extractStage}</div>
                )}
              </div>
            )}

            {sourceMode === 'manual' && (
              <div className="wz-studio-block">
                <Text type="secondary" className="wz-studio-hint">
                  在中间栏直接编写口播稿，保持一行一句，便于后续插入画面对齐。
                </Text>
              </div>
            )}

            <div className="wz-studio-block">
              <label className="wz-studio-label">主题 / 系列</label>
              <Input
                placeholder="如：酸菜鱼餐馆新菜品推广"
                value={topic}
                onChange={e => setTopic(e.target.value)}
                maxLength={100}
              />
            </div>

            <div className="wz-studio-actions">
              <Popover
                open={rewritePopOpen}
                onOpenChange={(v) => { setRewritePopOpen(v); if (v) setRewriteReq('') }}
                trigger="click"
                placement="topLeft"
                content={
                  <Space direction="vertical" size={8} style={{ width: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      输入一句话润色需求（留空则按主题常规改写）
                    </Text>
                    <Input
                      placeholder="如：更口语化 / 突出限时优惠"
                      value={rewriteReq}
                      onChange={(e) => setRewriteReq(e.target.value)}
                      maxLength={60}
                      onPressEnter={() => doRewrite(rewriteReq)}
                    />
                    <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
                      <Button size="small" onClick={() => setRewritePopOpen(false)}>取消</Button>
                      <Button size="small" type="primary" loading={rewriting} onClick={() => doRewrite(rewriteReq)}>
                        开始润色
                      </Button>
                    </div>
                  </Space>
                }
              >
                <Button type="primary" block size="large" loading={rewriting} disabled={!script.trim()}>
                  AI 润色改写
                </Button>
              </Popover>
              <Button
                block
                size="large"
                icon={<EditOutlined />}
                onClick={() => setSourceMode('manual')}
              >
                自定义编写
              </Button>
            </div>
          </section>

          {/* 中：文案工作区（逐句） */}
          <section className="wz-studio-col wz-studio-col--work">
            <header className="wz-studio-col-head">
              <strong>口播文案</strong>
              <span>一行一句 · 插入画面按句对齐</span>
            </header>
            {(cleanText || rewriteText) && (
              <Segmented
                size="small"
                value={scriptVersion}
                onChange={v => switchScriptVersion(v as ScriptVersion)}
                options={[
                  { label: '原文', value: 'clean', disabled: !cleanText },
                  { label: 'AI 改写版', value: 'rewrite', disabled: !rewriteText },
                ]}
                style={{ marginBottom: 10 }}
              />
            )}
            <ScriptLinesEditor
              value={script}
              onChange={setScript}
              placeholder="输入或提取口播文案，一行一句…"
            />
            <div className="wz-studio-work-foot">
              <span>
                单集约 <strong>{scriptMin >= 1 ? `${scriptMin.toFixed(1)} 分钟` : `${scriptSec} 秒`}</strong>
                {' · '}{scriptLineCount} 句 · {script.length} 字
              </span>
              {scriptMin > 10 && <Text type="danger">文案过长，建议精简</Text>}
            </div>
          </section>

          {/* 右：结果预览 */}
          <section className="wz-studio-col wz-studio-col--result">
            <header className="wz-studio-col-head">
              <strong>文案预览</strong>
              <span>提取 / 润色结果</span>
            </header>
            {script.trim() ? (
              <ol className="wz-studio-preview-lines">
                {transcriptLines(script).map((line, i) => (
                  <li key={`${i}-${line.slice(0, 12)}`}>
                    <em>{i + 1}</em>
                    <span>{line}</span>
                  </li>
                ))}
              </ol>
            ) : (
              <div className="wz-studio-empty">
                <EditOutlined />
                <p>暂无文案内容</p>
                <span>从左侧提取爆款或手写后，将在此逐句预览</span>
              </div>
            )}
            <div className="wz-studio-phone-slot">
              <PhonePreview
                script={script}
                videoUrl={undefined}
                resultUrl={undefined}
                presence={presence}
                stepKey="script"
                estimatedSeconds={scriptSec}
                topic={topic}
              />
            </div>
          </section>
        </div>
      )}

      {step === 1 && (
        <div className="wz-studio wz-studio--later">
          <div className="wz-studio-later-main wz-config">
            <section className="wz-config-section">
              <div className="wz-config-row">
                <label className="wz-config-label">品牌</label>
                <Select
                  size="large"
                  placeholder={brands.length ? '选择品牌' : '暂无品牌'}
                  value={brandId || undefined}
                  onChange={(id) => setCurrentBrand(id)}
                  options={brands.map((b) => ({ value: b.id, label: b.name }))}
                  className="wz-config-select"
                  notFoundContent={<Link to="/m/brands">去创建品牌</Link>}
                />
              </div>
            </section>

            <section className="wz-config-section">
              <div className="wz-config-label">出镜形态</div>
              <div className="wz-presence-grid">
                <button
                  type="button"
                  className={`wz-presence-card${presence === 'real' ? ' is-active' : ''}`}
                  onClick={() => setPresence('real')}
                >
                  <span className="wz-presence-card-icon"><VideoCameraOutlined /></span>
                  <strong>真人出镜</strong>
                  <span>上传出镜视频对口型</span>
                </button>
                <button
                  type="button"
                  className={`wz-presence-card${presence === 'avatar' ? ' is-active' : ''}`}
                  onClick={() => setPresence('avatar')}
                >
                  <span className="wz-presence-card-icon"><UserOutlined /></span>
                  <strong>数字分身</strong>
                  <span>选分身一键成片</span>
                </button>
              </div>

              <div className="wz-presence-detail">
                {presence === 'real' ? (
                  <>
                    <MaterialDropzone
                      accept="video/mp4,video/quicktime,video/x-msvideo"
                      hint={`正脸、不说话 · 建议时长约 ${scriptSec} 秒`}
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
                        message.success(dur > 0 ? `已上传（${Math.round(dur)} 秒）` : '出镜视频已上传')
                      }}
                    />
                    {realVideoSec > 0 && (
                      <Text type="secondary" className="wz-config-meta">
                        视频 {Math.round(realVideoSec)}s · 文案约 {scriptSec}s
                        {durationMismatch ? ' · 时长差距较大，建议对齐' : ''}
                      </Text>
                    )}
                  </>
                ) : (
                  <>
                    <SubjectPicker
                      subjects={subjects}
                      value={subjectServerId}
                      onChange={setSubjectServerId}
                      highlightServerId={presetSubjectId}
                      createHref="/m/compose/avatar?create=1&from=wizard"
                    />
                    <Input
                      placeholder="场景意图（可选，如：厨房边做菜边讲解）"
                      value={intent}
                      onChange={e => setIntent(e.target.value)}
                      maxLength={200}
                    />
                    {longAvatarScript && (
                      <Text type="secondary" className="wz-config-meta">文案偏长，建议精简到约 60 字内</Text>
                    )}
                  </>
                )}
              </div>
            </section>

            <section className="wz-config-section">
              <div className="wz-config-label">音频来源</div>
              <div className="wz-source-grid wz-source-grid--audio">
                <button
                  type="button"
                  className={`wz-source-card${audioSource === 'tts' ? ' is-active' : ''}`}
                  onClick={() => setAudioSource('tts')}
                >
                  <span className="wz-source-card-icon"><SoundOutlined /></span>
                  <strong>文本 + 音色 <Tag color="green">推荐</Tag></strong>
                  <span>稳定可控</span>
                </button>
                <button
                  type="button"
                  className={`wz-source-card${audioSource === 'direct' ? ' is-active' : ''}${presence === 'real' ? ' is-disabled' : ''}`}
                  onClick={() => {
                    if (presence === 'real') {
                      message.info('文本直生仅数字分身可用')
                      return
                    }
                    setAudioSource('direct')
                  }}
                >
                  <span className="wz-source-card-icon"><RocketOutlined /></span>
                  <strong>文本直生 <Tag color="blue">最快</Tag></strong>
                  <span>{presence === 'real' ? '仅分身可用' : '单步出片'}</span>
                </button>
                <button
                  type="button"
                  className={`wz-source-card${audioSource === 'upload' ? ' is-active' : ''}`}
                  onClick={() => setAudioSource('upload')}
                >
                  <span className="wz-source-card-icon"><UploadOutlined /></span>
                  <strong>上传录音 <Tag color="gold">最真实</Tag></strong>
                  <span>真人声对口型</span>
                </button>
              </div>

              {audioSource === 'tts' && (
                <div className="wz-source-expand">
                  <VoicePicker value={voiceId} onChange={setVoiceId} myVoices={myVoices} style={{ maxWidth: 480 }} />
                </div>
              )}

              {audioSource === 'upload' && (
                <div className="wz-source-expand">
                  {uploadedAudioUrl ? (
                    <Space size={12} wrap>
                      <CheckCircleOutlined style={{ color: 'var(--wr-success)', fontSize: 18 }} />
                      <Text>音频已就绪</Text>
                      <audio controls src={uploadedAudioUrl} style={{ height: 36 }} />
                      <Button size="small" onClick={() => setUploadedAudioUrl('')}>重选</Button>
                    </Space>
                  ) : (
                    <MaterialDropzone
                      accept="audio/*"
                      hint="mp3 / wav / m4a"
                      loading={uploadingAudio}
                      onUpload={async (file) => {
                        const check = checkMaterialFileSize(file)
                        if (check?.error) { message.warning(check.error); return }
                        setUploadingAudio(true)
                        try {
                          const r = await businessApi.uploadAsset(file)
                          setUploadedAudioUrl(r.url)
                          message.success('音频已上传')
                        } catch (e: any) {
                          message.error(e?.response?.data?.msg || '上传失败，请重试')
                        } finally {
                          setUploadingAudio(false)
                        }
                      }}
                    />
                  )}
                </div>
              )}
            </section>
          </div>
          <aside className="wz-studio-later-preview" aria-label="成片预览">
            <PhonePreview
              script={script}
              videoUrl={realVideoUrl || undefined}
              resultUrl={resultUrl || undefined}
              presence={presence}
              stepKey={stepKey}
              estimatedSeconds={scriptSec}
              topic={topic}
            />
          </aside>
        </div>
      )}

      {step === 2 && (
        <div className="ip-stagger">
          <div className="wz-ready-tags">
            <Tag color="green">文案 {script.length} 字 · 约 {scriptSec} 秒</Tag>
            <Tag color="green">{presence === 'real' ? '真人出镜' : '数字分身'}</Tag>
            <Tag color={voiceId ? 'green' : 'default'}>
              {audioSource === 'direct' ? '文本直生（分身端内合成语音）'
                : audioSource === 'upload' ? '已录音频' : (voiceId ? '音色已选' : '默认音色')}
            </Tag>
            <Tag color="blue">
              {audioSource === 'direct'
                ? '链路：台词直生成片（单步）'
                : audioSource === 'upload'
                  ? '链路：上传音频 → 对口型（跳过配音）'
                  : presence === 'real'
                    ? '链路：配音 → 对口型（口型与音频对齐）'
                    : '链路：配音 → 主体一致性成片（画面由分身生成）'}
            </Tag>
            <Tag icon={<ClockCircleOutlined />}>{produceEta}</Tag>
          </div>

          <PipelineProgress
            stages={pipelineStages}
            errorMessage={error || undefined}
            onRetry={error && !producing ? () => { setError(''); setFailedStage(''); produce() } : undefined}
          />

          {!resultUrl && !producing && (
            <div className="wz-produce-actions">
              <Button type="primary" size="large" icon={<RocketOutlined />} onClick={() => produce()}>
                一键成片
              </Button>
            </div>
          )}

          {producing && (
            <Alert
              type="info" showIcon style={{ marginTop: 14 }}
              message={`生成中（${produceEta}），请勿关闭页面…`}
              action={
                <Popconfirm
                  title="取消当前生成？"
                  description="已完成阶段的产物会保留，之后可从断点重试"
                  okText="取消生成"
                  cancelText="继续生成"
                  okButtonProps={{ danger: true, loading: cancelling }}
                  onConfirm={cancelProduce}
                  disabled={!activeTaskId}
                >
                  <Button size="small" danger icon={<StopOutlined />} disabled={!activeTaskId}>
                    取消生成
                  </Button>
                </Popconfirm>
              }
            />
          )}

          {resultUrl && (
            <>
              <Alert
                type="success" showIcon icon={<CheckCircleOutlined />}
                message="成片已完成，可在完成页继续操作"
                style={{ marginTop: 14 }}
                action={<Button size="small" type="primary" onClick={() => goStep(3)}>进入完成页</Button>}
              />
              <video
                src={resultUrl}
                controls
                style={{ width: '100%', maxWidth: 420, borderRadius: 12, marginTop: 12 }}
              />
            </>
          )}
        </div>
      )}

      {step === 3 && (
        <div className="ip-stagger">
          {!resultUrl ? (
            <Alert
              type="info" showIcon
              message="尚未生成成片"
              description={<Button size="small" onClick={() => goStep(2)}>返回上一步生成</Button>}
            />
          ) : (
            <>
              <video
                src={resultUrl}
                controls
                style={{ width: '100%', maxWidth: 480, borderRadius: 14, background: '#000' }}
              />
              <div className="wz-produce-actions wz-produce-actions--done">
                <Button
                  type="primary"
                  size="large"
                  icon={<VideoCameraAddOutlined />}
                  onClick={() => {
                    if (!lipsyncTaskId) {
                      message.warning('成片任务尚未就绪')
                      return
                    }
                    navigate(`/m/works/${encodeURIComponent(`g-${lipsyncTaskId}`)}`)
                  }}
                >
                  插入画面
                </Button>
                <Button type="primary" size="large" ghost icon={<ExportOutlined />} onClick={goPublish}>
                  去发布
                </Button>
                <Button
                  size="large"
                  icon={<RocketOutlined />}
                  disabled={producing}
                  onClick={() => { setResultUrl(''); goStep(2); produce() }}
                >
                  重新生成
                </Button>
                <Button href={resultUrl} target="_blank" download>下载成片</Button>
              </div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                建议先按台词插入产品/场景画面再发布；跳过也可直接去发布。合成后新成片入作品库，源片保留。
              </Text>
            </>
          )}
        </div>
      )}
    </WizardShell>
  )
}
