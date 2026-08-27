import type { GenerationTask } from '../types/api'

const PENDING_LABELS: Record<string, string> = {
  tts: '配音生成中',
  reference2video: '主体一致性成片中',
  lip_sync: '对口型处理中',
  text2image: '图片生成中',
  img2video: '图生视频生成中',
  text2video: '视频生成中',
  subject: '分身注册中',
  voice_clone: '声音克隆中',
  digital_human: '数字人口播生成中',
}

/** 进行中任务的人性化状态文案 */
export function generationPendingLabel(task: GenerationTask | undefined, fallback = '生成中'): string {
  const base = (task && (PENDING_LABELS[task.sub_type] || PENDING_LABELS[task.type])) || fallback
  return `${base}，完成后自动填入`
}
