import { Alert } from 'antd'
import { useGenerationTypes } from '../../hooks/useGenerationTypes'

const CAP_LABELS: Record<string, string> = {
  lip_sync: '对口型',
  tts: '语音合成',
  reference2video: '参考生视频',
  voice_clone: '声音克隆',
  subject: '主体创建',
}

type Props = {
  /** 本流程依赖的 sub_type 列表 */
  required?: string[]
  className?: string
}

/**
 * 商户端能力可用性提示（08 D7——ASR/视频生成等未配置时在入口拦截）。
 * 通过 listGenerationTypes 判断端点是否在服务端启用。
 */
export function CapabilityBanner({ required = ['lip_sync', 'tts'], className }: Props) {
  const { types, isError } = useGenerationTypes()
  const enabled = new Set(types.map(t => t.sub_type))
  const missing = required.filter(s => !enabled.has(s))

  if (!isError && missing.length === 0) return null

  const label = missing.map(m => CAP_LABELS[m] || m).join('、')

  return (
    <Alert
      type="warning"
      showIcon
      className={className || 'wz-draft-banner'}
      message={isError ? '生成服务暂不可用' : `需要先开通「${label}」`}
      description={
        isError
          ? '请稍后重试，或联系管理员检查生成服务配置。'
          : '请联系管理员在后台「第三方集成」中启用后，再继续口播创作。'
      }
    />
  )
}
