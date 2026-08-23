import { businessApi } from '../api/business'
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
  script: string
  voiceId: string
  presence: 'real' | 'avatar'
  realVideoUrl?: string
  subjectServerId?: string
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

/** 口播成片链路：TTS →（分身：参考生）→ 对口型 */
export async function runLipSyncPipeline(
  input: LipSyncPipelineInput,
  opts: {
    onStage?: (stage: LipSyncPipelineStage) => void
    resume?: LipSyncPipelineResume
    retryFrom?: 'tts' | 'ref' | 'lipsync'
  } = {},
): Promise<LipSyncPipelineResult> {
  const { onStage, resume, retryFrom } = opts

  let audioUrl = resume?.audioUrl || ''
  let videoUrl = input.presence === 'real'
    ? (input.realVideoUrl || resume?.videoUrl || '')
    : (resume?.videoUrl || '')
  let ttsTaskId = resume?.ttsTaskId || ''
  let refTaskId = resume?.refTaskId || ''
  let lipsyncTaskId = resume?.lipsyncTaskId || ''

  const runTts = !retryFrom || retryFrom === 'tts' || !audioUrl
  const runRef = input.presence === 'avatar' && (!retryFrom || retryFrom === 'ref' || retryFrom === 'tts' || !videoUrl)
  const runLipsync = !retryFrom || retryFrom === 'lipsync' || retryFrom === 'ref' || retryFrom === 'tts'

  if (retryFrom === 'lipsync') {
    if (!audioUrl) throw new Error('语音产物缺失，请从语音合成重试')
    if (!videoUrl) throw new Error('画面产物缺失，请从画面生成重试')
    onStage?.('lipsync')
    const lipsync = await businessApi.submitGenerationTask({
      sub_type: 'lip_sync', model: '',
      params: { video_url: videoUrl, audio_url: audioUrl },
    })
    lipsyncTaskId = lipsync.id
    const done = await waitGenerationTask(lipsync.id)
    const resultUrl = creationUrl(done)
    if (!resultUrl) throw new Error('对口型产物缺失（可重试）')
    return { ttsTaskId, refTaskId, lipsyncTaskId, resultUrl, audioUrl, videoUrl }
  }

  if (runTts) {
    onStage?.('tts')
    const tts = await businessApi.submitGenerationTask({
      sub_type: 'tts', model: '',
      params: { text: input.script, voice_setting_voice_id: input.voiceId },
    })
    ttsTaskId = tts.id
    const ttsDone = await waitGenerationTask(tts.id)
    audioUrl = creationUrl(ttsDone)
    if (!audioUrl) throw new Error('语音产物缺失（可重试）')
  }

  if (input.presence === 'avatar' && runRef) {
    if (!input.subjectServerId) throw new Error('请选择数字分身')
    onStage?.('ref')
    const prompt = (input.intent || '').trim() || '人物面对镜头自然口播'
    const ref = await businessApi.submitGenerationTask({
      sub_type: 'reference2video', model: '',
      params: { subjects: [{ name: '主角', server_id: input.subjectServerId }], prompt },
    })
    refTaskId = ref.id
    const refDone = await waitGenerationTask(ref.id)
    videoUrl = creationUrl(refDone)
    if (!videoUrl) throw new Error('分身画面产物缺失（可重试）')
  } else if (input.presence === 'real') {
    videoUrl = input.realVideoUrl || videoUrl
    if (!videoUrl) throw new Error('请上传出镜视频')
  }

  if (runLipsync) {
    onStage?.('lipsync')
    const lipsync = await businessApi.submitGenerationTask({
      sub_type: 'lip_sync', model: '',
      params: { video_url: videoUrl, audio_url: audioUrl },
    })
    lipsyncTaskId = lipsync.id
    const done = await waitGenerationTask(lipsync.id)
    const resultUrl = creationUrl(done)
    if (!resultUrl) throw new Error('对口型产物缺失（可重试）')
    return { ttsTaskId, refTaskId, lipsyncTaskId, resultUrl, audioUrl, videoUrl }
  }

  throw new Error('链路状态异常，请重试')
}
