type Props = {
  script?: string
  videoUrl?: string
  resultUrl?: string
  presence?: 'real' | 'avatar'
  stepKey: string
  estimatedSeconds?: number
  topic?: string
}

/** 口播向导右侧手机预览（复用 cf-phone 样式） */
export function PhonePreview({
  script = '',
  videoUrl,
  resultUrl,
  presence = 'real',
  stepKey,
  estimatedSeconds = 0,
  topic,
}: Props) {
  const body = script.trim()
  const cap =
    stepKey === 'source' ? '文案来源'
    : stepKey === 'script' ? `约 ${estimatedSeconds} 秒口播`
    : stepKey === 'presence' ? (presence === 'real' ? '真人出镜' : '数字分身')
    : stepKey === 'voice' ? '配音预览'
    : resultUrl ? '成片预览' : '生成中'

  return (
    <div className="cf-phone">
      <div className="cf-phone-notch" />
      <div className="cf-phone-stage">
        {resultUrl ? (
          <video className="cf-phone-video" src={resultUrl} controls playsInline />
        ) : videoUrl ? (
          <video className="cf-phone-video" src={videoUrl} muted playsInline />
        ) : (
          <div className="cf-phone-cover cf-phone-cover-empty">
            {topic && <strong>{topic}</strong>}
            <p>
              {body
                ? body.slice(0, 160) + (body.length > 160 ? '…' : '')
                : '左侧完成每一步后，这里会同步预览成片效果'}
            </p>
          </div>
        )}
      </div>
      <div className="cf-phone-meta">
        <span className="cf-phone-cap">{cap}</span>
        {body && stepKey !== 'produce' && (
          <span className="cf-phone-cap">{body.length} 字</span>
        )}
      </div>
    </div>
  )
}
