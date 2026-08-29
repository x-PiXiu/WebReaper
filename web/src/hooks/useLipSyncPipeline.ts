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

/** 音频来源（23 号计划 §4.2）：A 文本+音色→TTS；B 文本直生（Vidu 端内合成）；C 上传已录音频 */
export type LipSyncAudioSource = 'tts' | 'direct' | 'upload'

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
  /** 音频路径（默认 tts=路径 A）；direct 仅数字分身可用（真人无端内合成） */
  audioSource?: LipSyncAudioSource
  /** audioSource=upload 时必填：已录音频 URL（先经 uploadAsset 入库） */
  uploadedAudioUrl?: string
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
 * 口播成片链路（对齐统一 submit + 23 号计划音频三路径）：
 * - 音频路径 A（tts，默认）：TTS 产音频 → 成片
 * - 音频路径 B（direct）：跳过 TTS，台词文本直传（分身 reference2video 端内合成语音，单段）
 * - 音频路径 C（upload）：跳过 TTS，已录音频直传 → 成片
 * 出镜形态：
 * - 分身（有 server_id）：reference2video(subjects)
 * - 真人：lip_sync(video + audio)
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

  const audioSource = input.audioSource || 'tts'
  if (audioSource === 'direct' && input.presence !== 'avatar') {
    throw new Error('「文本直生」仅支持数字分身模式——真人出镜请选 TTS 配音或上传已录音频')
  }
  if (audioSource === 'upload' && !input.uploadedAudioUrl) {
    throw new Error('请先上传已录音频')
  }

  let audioUrl = resume?.audioUrl || ''
  let videoUrl = input.presence === 'real'
    ? (input.realVideoUrl || resume?.videoUrl || '')
    : (resume?.videoUrl || '')
  let ttsTaskId = resume?.ttsTaskId || ''
  let refTaskId = resume?.refTaskId || ''
  let lipsyncTaskId = resume?.lipsyncTaskId || ''

  // 路径 C：音频来自上传（无需任务）；路径 B：全程无音频任务
  if (audioSource === 'upload') {
    audioUrl = input.uploadedAudioUrl || ''
  }
  const runTts = audioSource === 'tts' && (!retryFrom || retryFrom === 'tts' || !audioUrl)
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
    // 路径 B（无音频）：text=台词本身，Vidu 端内合成语音；A/C（有音频）：text 为场景意图
    const prompt = audioUrl
      ? (input.intent || '').trim() || input.script.slice(0, 2000)
      : input.script
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
      throw new Error('请选择数字分身，或切换到「真人出镜」模式上传视频')
    }
    // Vidu digital_human 端点已废弃（2026-08-27 确认）：
    // 无 server_id 的图+音降级路径不再支持。引导用户先创建数字分身。
    if (!input.subjectServerId) {
      throw new Error(
        '请先在素材库创建数字分身（上传形象照即可），再选择该分身生成分身口播视频。' +
        '数字分身能保证跨视频的人物一致性，且不依赖已废弃的数字人接口。',
      )
    }
  }

  if (input.presence === 'real') {
    videoUrl = input.realVideoUrl || videoUrl
    if (!videoUrl) throw new Error('请上传出镜视频')
  }

  if (runLipsync) {
    if (!audioUrl) throw new Error('音频缺失——请选择 TTS 配音或上传已录音频')
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
