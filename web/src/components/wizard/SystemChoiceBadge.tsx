import { ThunderboltOutlined } from '@ant-design/icons'

const SUBTYPE_LABELS: Record<string, string> = {
  text2video: '文生视频',
  img2video: '图生视频',
  start_end2video: '首尾帧视频',
  reference2video: '参考生视频',
  multiframe: '智能多帧',
  lip_sync: '对口型',
  digital_human: '数字人口播',
  tts: '语音合成',
  voice_clone: '声音克隆',
  text2image: '文生图',
  text2audio: '文生音频',
  subject: '主体创建',
}

type Props = {
  subType?: string
  model?: string
  label?: string
}

/** 展示系统自动选择的端点/模型（09 统一架构透明反馈） */
export function SystemChoiceBadge({ subType, model, label }: Props) {
  if (!subType && !model && !label) return null
  const modeLabel = subType ? (SUBTYPE_LABELS[subType] || subType) : ''
  const text = label || [modeLabel, model].filter(Boolean).join(' · ')
  return (
    <div className="wz-choice" role="status">
      <ThunderboltOutlined />
      <span>系统已选择：<strong>{text}</strong></span>
    </div>
  )
}
