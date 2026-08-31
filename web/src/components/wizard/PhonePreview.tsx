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
  const lines = body ? body.split(/\n+/).map((l) => l.trim()).filter(Boolean) : []
  const cap =
    stepKey === 'source' || stepKey === 'script' ? (estimatedSeconds > 0 ? `约 ${estimatedSeconds} 秒` : '文案预览')
    : stepKey === 'config' || stepKey === 'presence' ? (presence === 'real' ? '真人出镜' : '数字分身')
    : stepKey === 'voice' ? '配音预览'
    : resultUrl ? '成片预览' : stepKey === 'produce' ? '生成中' : '预览'

  return (
    <div className="cf-phone wz-phone">
      <div className="cf-phone-notch" />
      <div className="cf-phone-stage">
        {resultUrl ? (
          <video className="cf-phone-video" src={resultUrl} controls playsInline />
        ) : videoUrl ? (
          <video className="cf-phone-video" src={videoUrl} muted playsInline />
        ) : (
          <div className={`cf-phone-cover${body ? '' : ' cf-phone-cover-empty'}`}>
            {topic ? <strong className="wz-phone-topic">{topic}</strong> : null}
            {lines.length > 0 ? (
              <ul className="wz-phone-lines">
                {lines.slice(0, 6).map((line, i) => (
                  <li key={`${i}-${line.slice(0, 8)}`}>{line}</li>
                ))}
                {lines.length > 6 && <li className="wz-phone-more">…还有 {lines.length - 6} 句</li>}
              </ul>
            ) : (
              <p className="wz-phone-placeholder">完成左侧操作后，这里同步预览口播效果</p>
            )}
          </div>
        )}
      </div>
      <div className="cf-phone-meta">
        <span className="cf-phone-cap">{cap}</span>
        {body && (
          <span className="cf-phone-cap">{lines.length || 0} 句 · {body.length} 字</span>
        )}
      </div>
    </div>
  )
}
