import { businessApi } from '../api/business'
import { buildSubjectReferencePayload, deliverableWorkParams, ensureMaterialId, submitUnified } from '../api/generationSubmit'
import { fetchGenerationTasks } from './useGenerationTasks'
import type { GenerationTask } from '../types/api'

/** 轮询生成任务到终态（共享 React Query 缓存的 fetch） */
export async function waitGenerationTask(
  id: string,
  onTick?: () => void,
  timeoutMs = 10 * 60 * 1000,
): Promise<GenerationTask> {
  const start = Date.now()
  for (;;) {
    onTick?.()
    const tasks = await fetchGenerationTasks()
    const t = tasks.find(x => x.id === id)
    if (t && (t.state === 'success' || t.state === 'failed' || t.state === 'cancelled')) {
      if (t.state !== 'success') throw new Error(t.err_msg || '任务失败')
      return t
    }
    if (Date.now() - start > timeoutMs) throw new Error('任务超时（10 分钟）')
    await new Promise(r => setTimeout(r, 5000))
  }
}

function creationUrl(task: GenerationTask): string {
  const c = task.creations?.[0]
  return c?.stored_url || c?.url || ''
}

export type LipSyncPipelineStage = 'tts' | 'ref' | 'lipsync' | ''

export type LipSyncPipelineInput = {
  brandId: string
  script: string
  voiceId?: string
  presence: 'real' | 'avatar'
  realVideoUrl?: string
  /** 无 server_id 时降级：人像图 URL → 图+音路径 */
  portraitMaterial?: string
  subjectServerId?: string
  subjectName?: string
  intent?: string
}

export type LipSyncPipelineResume = {
  ttsTaskId?: string
  refTaskId?: string
  lipsyncTaskId?: string
  audioUrl?: string
  videoUrl?: string
}

export type LipSyncPipelineResult = {
  ttsTaskId: string
  refTaskId?: string
  lipsyncTaskId: string
  resultUrl: string
  audioUrl: string
  videoUrl: string
}

/**
 * 口播成片链路（对齐统一 submit）：
 * - 真人：TTS → video+audio → lip_sync
 * - 分身（有 server_id）：TTS → reference2video(subjects)
 * - 分身（无 server_id）：TTS → 图+音降级（digital_human，厂商端点可能已废弃）
 */
export async function runLipSyncPipeline(
  input: LipSyncPipelineInput,
  opts: {
    onStage?: (stage: LipSyncPipelineStage) => void
    resume?: LipSyncPipelineResume
    retryFrom?: 'tts' | 'ref' | 'lipsync'
  } = {},
): Promise<LipSyncPipelineResult> {
  const { onStage, resume, retryFrom } = opts
  if (!input.brandId) throw new Error('请先选择人设/品牌')

  let audioUrl = resume?.audioUrl || ''
  let videoUrl = input.presence === 'real'
    ? (input.realVideoUrl || resume?.videoUrl || '')
    : (resume?.videoUrl || '')
  let ttsTaskId = resume?.ttsTaskId || ''
  let refTaskId = resume?.refTaskId || ''
  let lipsyncTaskId = resume?.lipsyncTaskId || ''

  const runTts = !retryFrom || retryFrom === 'tts' || !audioUrl
  const runAvatar = input.presence === 'avatar' && (!retryFrom || retryFrom === 'ref' || retryFrom === 'tts' || !videoUrl)
  const runLipsync = input.presence === 'real' && (!retryFrom || retryFrom === 'lipsync' || retryFrom === 'ref' || retryFrom === 'tts')

  if (runTts) {
    onStage?.('tts')
    const tts = await submitUnified({
      brand_id: input.brandId,
      text: input.script,
      type: 'audio',
      params: input.voiceId ? { voice_setting_voice_id: input.voiceId } : undefined,
    })
    ttsTaskId = tts.id
    const ttsDone = await waitGenerationTask(tts.id)
    audioUrl = creationUrl(ttsDone)
    if (!audioUrl) throw new Error('语音产物缺失（可重试）')
  }

  if (input.presence === 'avatar' && runAvatar) {
    const prompt = (input.intent || '').trim() || input.script.slice(0, 2000)
    onStage?.('ref')

    if (input.subjectServerId) {
      const audioId = audioUrl ? await ensureMaterialId(audioUrl) : undefined
      const ref = await submitUnified(buildSubjectReferencePayload({
        brand_id: input.brandId,
        server_id: input.subjectServerId,
        name: input.subjectName,
        text: prompt,
        audioMaterialId: audioId,
      }))
      refTaskId = ref.id
      lipsyncTaskId = ref.id
      const done = await waitGenerationTask(ref.id)
      const resultUrl = creationUrl(done)
      if (!resultUrl) throw new Error('主体口播视频产物缺失（可重试）')
      videoUrl = resultUrl
      return { ttsTaskId, refTaskId, lipsyncTaskId, resultUrl, audioUrl, videoUrl }
    }

    const portrait = input.portraitMaterial
    if (!portrait) {
      throw new Error('请选择数字分身，或上传人像参考图')
    }
    // EXPERIMENTAL: Vidu digital_human 端点已废弃（2026-08-27 确认），
    // 无 server_id 时降级为图+音 → digital_human 可能失败。
    // 建议用户先创建数字分身获取 server_id。
    console.warn('[LipSyncPipeline] 无 subjectServerId，降级 digital_human（Vidu 端点已废弃，可能失败）')
    const audioId = await ensureMaterialId(audioUrl)
    const imageId = await ensureMaterialId(portrait)
    const dh = await submitUnified({
      brand_id: input.brandId,
      materials: [imageId, audioId],
      text: prompt.slice(0, 200),
      params: deliverableWorkParams(),
    })
    refTaskId = dh.id
    lipsyncTaskId = dh.id
    const done = await waitGenerationTask(dh.id)
    const resultUrl = creationUrl(done)
    if (!resultUrl) throw new Error('口播视频产物缺失（可重试）')
    videoUrl = resultUrl
    return { ttsTaskId, refTaskId, lipsyncTaskId, resultUrl, audioUrl, videoUrl }
  }

  if (input.presence === 'real') {
    videoUrl = input.realVideoUrl || videoUrl
    if (!videoUrl) throw new Error('请上传出镜视频')
  }

  if (runLipsync) {
    onStage?.('lipsync')
    const videoId = await ensureMaterialId(videoUrl)
    const audioId = await ensureMaterialId(audioUrl)
    const lipsync = await submitUnified({
      brand_id: input.brandId,
      materials: [videoId, audioId],
      params: deliverableWorkParams(),
    })
    lipsyncTaskId = lipsync.id
    const done = await waitGenerationTask(lipsync.id)
    const resultUrl = creationUrl(done)
    if (!resultUrl) throw new Error('对口型产物缺失（可重试）')
    return { ttsTaskId, refTaskId, lipsyncTaskId, resultUrl, audioUrl, videoUrl }
  }

  throw new Error('链路状态异常，请重试')
}

/** @deprecated 保留导出以免旧引用断裂；新代码请用 submitUnified */
export const submitGenerationTask = businessApi.submitGenerationTask
