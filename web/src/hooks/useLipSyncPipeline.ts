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

/** 解析任务 params 里的 source_task_id（compose 子任务血缘） */
function composeSourceId(t: GenerationTask): string {
  const p = t.params && typeof t.params === 'object' ? t.params as Record<string, unknown> : {}
  const v = p.source_task_id
  return typeof v === 'string' ? v : ''
}

/**
 * 等待源视频的 compose 子任务终态（29 号单阶段：服务端 chainBrollAfterGeneration
 * 在视频成功后自动定位时间轴并提交 compose——前端按 params.source_task_id 找到它等完）。
 */
export async function waitComposeChild(
  videoTaskId: string,
  onFound?: (taskId: string) => void,
  timeoutMs = 15 * 60 * 1000,
): Promise<GenerationTask> {
  const start = Date.now()
  let reported = false
  for (;;) {
    const tasks = await fetchGenerationTasks()
    const child = tasks.find(t => t.sub_type === 'compose' && composeSourceId(t) === videoTaskId)
    if (child) {
      if (!reported) { reported = true; onFound?.(child.id) }
      if (child.state === 'success' || child.state === 'failed' || child.state === 'cancelled') {
        if (child.state !== 'success') {
          throw new Error(child.err_msg || 'B-Roll 合成失败——可在完成页用「插入画面」手动重试')
        }
        return child
      }
    } else if (Date.now() - start > 90_000) {
      // 视频已成功但 90s 内未见 compose 子任务——链式钩子可能失败（定位/提交异常）
      throw new Error('未检测到 B-Roll 合成任务（链式可能失败）——请在完成页用「插入画面」手动合成')
    }
    if (Date.now() - start > timeoutMs) throw new Error('B-Roll 合成等待超时')
    await new Promise(r => setTimeout(r, 5000))
  }
}

export type LipSyncPipelineStage = 'video' | 'broll' | ''

/**
 * 音频来源（01 号业务线两模式）：text=文本驱动（默认——Vidu 端内合成语音，
 * 真人走 lip_sync(text+voice_id)、分身走 reference2video(text)，均单步无独立 TTS）；
 * upload=上传已录音频（audio_url 直传对口型）。
 */
export type LipSyncAudioSource = 'text' | 'upload'

export type LipSyncBrollSegment = { sentence_index: number; media_url: string }

export type LipSyncPipelineInput = {
  brandId: string
  script: string
  voiceId?: string
  presence: 'real' | 'avatar'
  realVideoUrl?: string
  portraitMaterial?: string
  subjectServerId?: string
  subjectName?: string
  intent?: string
  /** 音频路径（默认 text）；upload 时需 uploadedAudioUrl */
  audioSource?: LipSyncAudioSource
  /** audioSource=upload 时必填：已录音频 URL（先经 uploadAsset 入库） */
  uploadedAudioUrl?: string
  /** 组合出镜（25 号 §6.5）：环境主体作为第二参考主体——仅上传音频路径注入
   *  （该路径 text 为场景意图）；文本直驱路径 text 即台词不注入 */
  envSubject?: { serverId: string; name?: string }
  /** 29 号单阶段：生成前配置的 B-Roll 片段（服务端视频完成后自动链式合成） */
  brollSegments?: LipSyncBrollSegment[]
}

export type LipSyncPipelineResult = {
  videoTaskId: string
  resultUrl: string
  videoUrl: string
}

/**
 * 口播成片链路（01 号业务线 §二 阶段3 + 29 号单阶段）：
 * - 视频生成单步：真人 lip_sync（文本驱动/音频驱动）；分身 reference2video（主体一致性）
 * - 携带 brollSegments 时：视频成功 → 服务端已自动提交 compose → 前端等 compose 完成，
 *   最终 resultUrl = 合成片（一次等待拿到最终视频）
 */
export async function runLipSyncPipeline(
  input: LipSyncPipelineInput,
  opts: {
    onStage?: (stage: LipSyncPipelineStage) => void
    /** 每确认一个任务即回调（供调用方跟踪活动任务、实现"生成中可取消"） */
    onTaskSubmit?: (stage: LipSyncPipelineStage, taskId: string) => void
  } = {},
): Promise<LipSyncPipelineResult> {
  const { onStage, onTaskSubmit } = opts
  if (!input.brandId) throw new Error('请先选择人设/品牌')

  const audioSource = input.audioSource || 'text'
  if (audioSource === 'upload' && !input.uploadedAudioUrl) {
    throw new Error('请先上传已录音频')
  }

  onStage?.('video')
  let videoTask: GenerationTask

  if (input.presence === 'real') {
    // 真人：对口型——上传音频=音频驱动；文本=Vidu 文本驱动（端内合成语音，无独立 TTS）
    const videoId = await ensureMaterialId(input.realVideoUrl || '')
    const materials = audioSource === 'upload'
      ? [videoId, await ensureMaterialId(input.uploadedAudioUrl!)]
      : [videoId]
    const task = await submitUnified({
      brand_id: input.brandId,
      text: audioSource === 'upload' ? undefined : input.script,
      materials,
      params: {
        ...deliverableWorkParams(),
        ...(audioSource === 'text' && input.voiceId ? { voice_id: input.voiceId } : {}),
      },
      broll_segments: input.brollSegments,
    })
    onTaskSubmit?.('video', task.id)
    videoTask = await waitGenerationTask(task.id)
  } else {
    // 分身：reference2video 主体一致性
    if (!input.subjectServerId) {
      throw new Error(
        '请先在数字资产页创建数字分身（上传形象照即可），再选择该分身生成分身口播视频。' +
        '数字分身能保证跨视频的人物一致性，且不依赖已废弃的数字人接口。',
      )
    }
    const env = audioSource === 'upload' ? input.envSubject : undefined
    const sceneIntent = input.intent?.trim() || input.script.slice(0, 2000)
    const ref = await submitUnified({
      ...buildSubjectReferencePayload({
        brand_id: input.brandId,
        server_id: input.subjectServerId,
        name: input.subjectName,
        // 文本驱动：text=台词（音色可选，分身绑定优先）；上传音频：text=场景意图
        text: audioSource === 'upload' ? (env ? `${sceneIntent}（在「${env.name || '环境'}」中）` : sceneIntent) : input.script,
        audioMaterialId: audioSource === 'upload' ? await ensureMaterialId(input.uploadedAudioUrl!) : undefined,
        envSubject: env ? { serverId: env.serverId, name: env.name } : undefined,
        voiceId: audioSource === 'text' ? input.voiceId : undefined,
      }),
      broll_segments: input.brollSegments,
    })
    onTaskSubmit?.('video', ref.id)
    videoTask = await waitGenerationTask(ref.id)
  }

  const videoUrl = creationUrl(videoTask)
  if (!videoUrl) throw new Error('成片产物缺失（可重试）')

  // 29 号单阶段：等 B-Roll 链式合成完成——最终结果即合成片
  if (input.brollSegments?.length) {
    onStage?.('broll')
    const child = await waitComposeChild(videoTask.id, (id) => onTaskSubmit?.('broll', id))
    const composed = creationUrl(child)
    if (!composed) throw new Error('合成片产物缺失——可在完成页用「插入画面」查看')
    return { videoTaskId: videoTask.id, resultUrl: composed, videoUrl }
  }

  return { videoTaskId: videoTask.id, resultUrl: videoUrl, videoUrl }
}

/** @deprecated 保留导出以免旧引用断裂；新代码请用 submitUnified */
export const submitGenerationTask = businessApi.submitGenerationTask
