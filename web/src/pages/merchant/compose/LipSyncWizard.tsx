import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearchParams, useLocation, Link } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { Alert, Button, Input, Popconfirm, Popover, Segmented, Select, Space, Tag, Typography } from 'antd'
import { message } from '../../../utils/antdApp'
import {
  LinkOutlined, EditOutlined, UploadOutlined, VideoCameraOutlined, UserOutlined,
  RocketOutlined, CheckCircleOutlined, SoundOutlined, ExportOutlined, ClockCircleOutlined,
  RightOutlined, VideoCameraAddOutlined, StopOutlined, EnvironmentOutlined,
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
import { parseGenerationTaskParams, listSceneSubjects } from '../../../utils/subjectTask'
import { ScriptLinesEditor } from '../../../components/compose/ScriptLinesEditor'
import { SubjectPicker } from '../../../components/compose/SubjectPicker'
import BrollDrawer from '../../../components/compose/BrollDrawer'
import {
  WizardShell, PhonePreview, PipelineProgress, MaterialDropzone, CapabilityBanner,
  type WizardStepDef, type PipelineStage,
} from '../../../components/wizard'

const { Text } = Typography

const WIZARD_STEPS: WizardStepDef[] = [
  { key: 'script', label: '确定文案', title: '第一步：确定文案', tip: '粘贴链接提取、上传音视频提取，或直接手写口播稿', nextLabel: '下一步：出镜与配音' },
  { key: 'config', label: '出镜与配音', title: '第二步：出镜与配音', tip: '选谁出镜、声音从哪来（配音/文本直生/上传录音三选一）', nextLabel: '下一步：生成成片' },
  { key: 'produce', label: '生成成片', title: '第三步：生成成片', tip: '按所选路径自动完成配音与对口型各阶段', nextLabel: '下一步：发布' },
  { key: 'publish', label: '发布', title: '第四步：发布', tip: '成片完成，可直接发布，或先插入画面再发布', nextLabel: '去发布' },
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
  const { brandId } = useBrandContext()
  const [searchParams] = useSearchParams()
  const location = useLocation()
  const presetSubjectId = searchParams.get('subject') || location.state?.subjectId || ''
  const presetState = location.state as { rawText?: string; title?: string; method?: string } | null

  const hasDraft = (draft.wizardStep ?? 0) > 0
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
  const [sourceMode, setSourceMode] = useState<SourceMode>(null)
  const [shareUrl, setShareUrl] = useState('')
  const [extractLineCount, setExtractLineCount] = useState(0)
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
  // 出镜环境（25 号 §6.5 组合出镜）：环境主体 server_id；空=默认棚拍
  const [envSubjectId, setEnvSubjectId] = useState(draft.wizardEnvSubjectId || '')
  // ④ 音色
  const [voiceId, setVoiceId] = useState(draft.wizardVoiceId || '')
  // ④ 音频来源（23 号计划三选一）：A 文本+音色（默认）/ B 文本直生 / C 上传已录音频
  const [audioSource, setAudioSource] = useState<LipSyncAudioSource>(draft.wizardAudioSource || 'tts')
  const [uploadedAudioUrl, setUploadedAudioUrl] = useState(draft.wizardUploadedAudioUrl || '')
  const [uploadingAudio, setUploadingAudio] = useState(false)
  const [brollOpen, setBrollOpen] = useState(false)
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
      wizardEnvSubjectId: envSubjectId,
      wizardSchema: 4,
    })
  }, [step, presence, topic, script, cleanText, voiceId, realVideoUrl, subjectServerId, intent, ttsTaskId, refTaskId, lipsyncTaskId, resultUrl, audioSource, uploadedAudioUrl, envSubjectId])

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
  const { subjects: allSubjects, ready: subjects } = useSubjectList({ refetchInterval: false })
  // 组合出镜（25 号 §6.5）：已注册环境主体（我的店面/后厨/产品展台）
  const sceneSubjects = useMemo(() => listSceneSubjects(allSubjects), [allSubjects])
  const selectedEnv = useMemo(
    () => sceneSubjects.find((e) => e.serverId === envSubjectId && e.state === 'success'),
    [sceneSubjects, envSubjectId],
  )
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

  const extractWithPolling = async (payload: { share_url?: string; video_url?: string }) => {
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
      // asset_url（本站素材）走同步接口；链接走异步轮询——长视频不再受 120s 连接超时约束
      const r = payload.asset_url
        ? await businessApi.extractTranscript(payload)
        : await extractWithPolling(payload.share_url ? { share_url: payload.share_url } : {})
      const lines = transcriptLines(r.raw_text, r.raw_text_lines)
      setExtractLineCount(lines.length)
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
      const rw = await businessApi.rewriteScript({
        raw_text: script,
        topic: topic.trim() || '口播获客',
        requirement: req.trim() || undefined,
      })
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
        envSubject: selectedEnv ? { serverId: selectedEnv.serverId, name: selectedEnv.name } : undefined,
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
      const presenceOk = presence === 'real' ? !!realVideoUrl : !!subjectServerId || !!selectedSubject?.portraitUrl
      const audioOk = audioSource !== 'upload' || !!uploadedAudioUrl // 路径 C 需先上传音频
      return presenceOk && audioOk
    }
    return false
  }

  const nextHint = (): string | undefined => {
    if (step === 0 && !script.trim()) return '请先填写口播文案'
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
      {presetFromVideo && (
        <Alert
          type="info" showIcon closable className="wz-draft-banner"
          message="发视频已升级为口播向导——你的文案已自动带入"
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

  const footerNextLabel = step === 2
    ? (resultUrl ? '下一步：发布' : producing ? '生成中…' : '一键成片')
    : step === 3
      ? '去发布'
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
      nextDisabled={((step === 0 || step === 1) && !canNext()) || ((step === 2 || step === 3) && producing)}
      nextHint={nextHint()}
      nextLoading={extracting || rewriting || producing}
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
              <span>从抖音 / B站等平台爆款提取说话内容</span>
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
              onClick={() => setSourceMode('manual')}
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
                placeholder="粘贴抖音 / B站分享链接（也支持 YouTube/微博/西瓜等平台）"
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
              {extracting && extractStage && (
                <div className="wz-extract-stage"><span className="wz-extract-dot" />{extractStage}</div>
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
              {extracting && extractStage && (
                <div className="wz-extract-stage"><span className="wz-extract-dot" />{extractStage}</div>
              )}
            </div>
          )}

          <div className="ip-form-stack ip-stagger" style={{ marginTop: 18 }}>
          <label>品牌/主题（润色与改写围绕它展开）</label>
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
                { label: '原文', value: 'clean', disabled: !cleanText },
                { label: 'AI 改写版', value: 'rewrite', disabled: !rewriteText },
              ]}
            />
          )}
          <label>口播文案 · 一行一句（后续插入画面按句对齐，逐句编辑效果最佳）</label>
          <ScriptLinesEditor
            value={script}
            onChange={setScript}
            placeholder="输入或提取口播文案，一行一句…"
          />
          <div className="wz-script-toolbar">
            <span className="wz-duration-ring">
              预计口播约 <strong>{scriptMin >= 1 ? `${scriptMin.toFixed(1)} 分钟` : `${scriptSec} 秒`}</strong>
              <Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>
                共 {scriptLineCount} 句 · {script.length} 字
              </Text>
              {extractLineCount > 0 && scriptLineCount !== extractLineCount && (
                <Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>（提取 {extractLineCount} 句）</Text>
              )}
              {scriptMin > 10 && (
                <Text type="danger" style={{ marginLeft: 8, fontSize: 12 }}>文案过长，建议精简</Text>
              )}
            </span>
            <Popover
              open={rewritePopOpen}
              onOpenChange={(v) => { setRewritePopOpen(v); if (v) setRewriteReq('') }}
              trigger="click"
              placement="topRight"
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
              <Button loading={rewriting} disabled={!script.trim()}>
                AI 润色/改写
              </Button>
            </Popover>
          </div>
          </div>
        </div>
      )}

      {step === 1 && (
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
              <span>选择已注册分身，按主体一致性生成口播视频</span>
            </button>
          </div>

          <div className="wz-presence-compare" role="note">
            <div>
              <strong>真人出镜</strong>
              <span>上传不说话视频 + 配音，系统对口型（口型与音频严格对齐）</span>
            </div>
            <div>
              <strong>数字分身</strong>
              <span>选已注册分身 + 文案，系统生成一致性口播（无需真人视频）</span>
            </div>
          </div>

          <div className="wz-presence-detail">
            {presence === 'real' ? (
              <>
                <Alert
                  type="info"
                  showIcon
                  message="正脸、光线稳定、不说话的视频效果最好"
                  description="对口型需服务端能访问视频 URL。本地大文件若生成失败，请压缩后重试或部署公网/OSS 素材地址。"
                />
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
                <Alert
                  type="info"
                  showIcon
                  message="选择资产库里的数字分身；已选分身将走 reference2video 主体一致性"
                />
                {longAvatarScript && (
                  <Alert
                    type="warning"
                    showIcon
                    style={{ marginBottom: 8 }}
                    message="文案较长，将生成长视频（等待分段功能上线前建议压缩到 60 字内获得最佳效果）"
                  />
                )}
                <SubjectPicker
                  subjects={subjects}
                  value={subjectServerId}
                  onChange={setSubjectServerId}
                  highlightServerId={presetSubjectId}
                  createHref="/m/compose/avatar?create=1&from=wizard"
                />
                <Input
                  placeholder="场景意图（如：在厨房边做菜边对镜头讲解）"
                  value={intent}
                  onChange={e => setIntent(e.target.value)}
                  maxLength={200}
                />
                <div>
                  <label style={{ display: 'block', marginBottom: 4 }}>
                    <EnvironmentOutlined /> 出镜环境（可选——分身在你的店里口播）
                  </label>
                  <Select
                    allowClear
                    placeholder="默认纯色棚拍；选择已注册的环境（店内大堂/后厨/门头…）"
                    value={envSubjectId || undefined}
                    onChange={(v) => setEnvSubjectId(v || '')}
                    style={{ width: '100%', maxWidth: 480 }}
                    notFoundContent={
                      <span style={{ fontSize: 12 }}>
                        还没有环境——去<a href="/m/compose/avatar" target="_blank" rel="noreferrer">数字资产页</a>拍 2-3 张店内照片注册
                      </span>
                    }
                    options={sceneSubjects
                      .filter((e) => e.state === 'success' && e.serverId)
                      .map((e) => ({ value: e.serverId, label: e.name }))}
                  />
                  {envSubjectId && audioSource === 'direct' && (
                    <Text type="secondary" style={{ display: 'block', marginTop: 4, fontSize: 12 }}>
                      「文本直生」路径台词即提示词，环境暂不注入画面——建议改用「文本 + 音色配音」获得组合出镜效果
                    </Text>
                  )}
                </div>
              </>
            )}
          </div>

          <div className="ip-form-stack ip-stagger" style={{ marginTop: 18 }}>
          <label><SoundOutlined /> 音频来源（三选一）</label>
          <div className="wz-source-grid">
            {/* 路径 A：文本 + 音色 → TTS（默认推荐） */}
            <button
              type="button"
              className={`wz-source-card${audioSource === 'tts' ? ' is-active' : ''}`}
              onClick={() => setAudioSource('tts')}
            >
              <span className="wz-source-card-icon"><SoundOutlined /></span>
              <strong>文本 + 音色配音 <Tag color="green" style={{ marginInlineEnd: 0 }}>推荐</Tag></strong>
              <span>选音色合成语音，声音稳定可控</span>
            </button>
            {/* 路径 B：文本直生（仅数字分身） */}
            <button
              type="button"
              className={`wz-source-card${audioSource === 'direct' ? ' is-active' : ''}`}
              onClick={() => {
                if (presence === 'real') {
                  message.info('「文本直生」由数字分身端内合成语音——真人出镜请配音色或上传录音')
                  return
                }
                setAudioSource('direct')
              }}
            >
              <span className="wz-source-card-icon"><RocketOutlined /></span>
              <strong>文本直生 <Tag color="blue" style={{ marginInlineEnd: 0 }}>最快</Tag></strong>
              <span>{presence === 'real' ? '仅数字分身可用（真人需真实音源）' : '台词直接驱动分身开口，单步出片'}</span>
            </button>
            {/* 路径 C：上传已录音频 */}
            <button
              type="button"
              className={`wz-source-card${audioSource === 'upload' ? ' is-active' : ''}`}
              onClick={() => setAudioSource('upload')}
            >
              <span className="wz-source-card-icon"><UploadOutlined /></span>
              <strong>上传已录音频 <Tag color="gold" style={{ marginInlineEnd: 0 }}>最真实</Tag></strong>
              <span>自己录的声音 / 专业录音，成片对口型</span>
            </button>
          </div>

          {audioSource === 'tts' && (
            <div className="wz-source-expand">
              <label style={{ marginTop: 0 }}><SoundOutlined /> 选择口播音色（可试听，可跳过用默认）</label>
              <VoicePicker value={voiceId} onChange={setVoiceId} myVoices={myVoices} style={{ maxWidth: 480 }} />
              <Text type="secondary" style={{ fontSize: 12 }}>
                想用自己的声音？<a href="/m/compose/tools?tab=media" target="_blank" rel="noreferrer">去声音克隆</a>
              </Text>
            </div>
          )}

          {audioSource === 'direct' && (
            <div className="wz-source-expand">
              <Text type="secondary" style={{ fontSize: 12.5, lineHeight: 1.7 }}>
                台词文本将直接驱动数字分身开口（语音由模型端内合成，无需配音）。
                成片最快，但音色由分身决定——需要指定音色请回到「文本 + 音色」。
              </Text>
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
                  hint="支持 mp3 / wav / m4a，上传后自动入库并用于对口型"
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
              <Text type="secondary" style={{ display: 'block', marginTop: 8, fontSize: 12 }}>
                用自己录的声音最有真实感；台词无需手改——后续插入画面时系统按语音自动分行定位。
              </Text>
            </div>
          )}
          </div>
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
            {selectedEnv && (
              <Tag color="cyan" icon={<EnvironmentOutlined />}>出镜环境：{selectedEnv.name}</Tag>
            )}
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
                message="成片完成" style={{ marginTop: 14 }}
              />
              <video
                src={resultUrl}
                controls
                style={{ width: '100%', maxWidth: 420, borderRadius: 12, marginTop: 12 }}
              />
              <div className="wz-produce-actions">
                <Button href={resultUrl} target="_blank" download>下载成片</Button>
                <Button type="primary" icon={<RightOutlined />} onClick={() => goStep(3)}>
                  下一步：发布
                </Button>
              </div>
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
              <div className="wz-produce-actions">
                <Button href={resultUrl} target="_blank" download>下载成片</Button>
                <Button
                  icon={<VideoCameraAddOutlined />}
                  onClick={() => setBrollOpen(true)}
                >
                  插入画面（可选）
                </Button>
                <Button
                  icon={<RocketOutlined />}
                  disabled={producing}
                  onClick={() => { setResultUrl(''); produce() }}
                >
                  重新生成
                </Button>
                <Button type="primary" size="large" icon={<ExportOutlined />} onClick={goPublish}>
                  去发布
                </Button>
              </div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                「插入画面」为可选后处理：按台词逐句挂素材，合成后新成片自动入库（源片保留）
              </Text>
            </>
          )}
        </div>
      )}
      <BrollDrawer
        open={brollOpen}
        onClose={() => setBrollOpen(false)}
        source={lipsyncTaskId ? { taskId: lipsyncTaskId, title: topic || '口播成片', videoUrl: resultUrl || undefined } : null}
      />
    </WizardShell>
  )
}
