import { Tag } from 'antd'
import { useComposeDraft, type ComposeTrack } from '../../../store/composeDraft'

function estimateSeconds(text: string) {
  const chars = text.replace(/\s/g, '').length
  // 口播约 4 字/秒
  return Math.max(0, Math.round(chars / 4))
}

/** 右侧预览：视频手机框 / 图文笔记卡 */
export function ComposePreview({ track, stepKey }: { track: ComposeTrack; stepKey: string }) {
  const draft = useComposeDraft()
  const body = draft.rewritten || draft.script || draft.transcript || ''
  const title = draft.selectedTitle || draft.refTitle || (track === 'video' ? '短视频标题' : '图文标题')
  const secs = estimateSeconds(body)
  const cover = draft.coverUrl
  const images = (draft.imageUrls || []).filter(Boolean)
  const videoUrl = draft.editedVideoUrl || draft.avatarVideoUrl

  if (track === 'graphic') {
    return (
      <div className="cf-phone cf-phone-note">
        <div className="cf-phone-notch" />
        <div className="cf-note-scroll">
          {(cover || images[0]) ? (
            <div
              className="cf-note-cover"
              style={{ backgroundImage: `url(${cover || images[0]})` }}
            />
          ) : (
            <div className="cf-note-cover cf-note-cover-empty">封面预览</div>
          )}
          <div className="cf-note-pad">
            <h3>{title}</h3>
            <p className="cf-note-body">
              {body.trim() || '在左侧写好种草文案后，这里会同步预览…'}
            </p>
            {images.length > 1 && (
              <div className="cf-note-thumbs">
                {images.slice(0, 6).map((u) => (
                  <div key={u} className="cf-note-thumb" style={{ backgroundImage: `url(${u})` }} />
                ))}
              </div>
            )}
            {(draft.topics || []).length > 0 && (
              <div className="cf-note-tags">
                {(draft.topics || []).slice(0, 5).map((t) => (
                  <Tag key={t} style={{ marginBottom: 4 }}>{t}</Tag>
                ))}
              </div>
            )}
          </div>
        </div>
        <div className="cf-phone-cap">
          {stepKey === 'script' && '文案预览'}
          {stepKey === 'assets' && `配图 ${images.length} 张`}
          {stepKey === 'produce' && '发布前核对'}
        </div>
      </div>
    )
  }

  return (
    <div className="cf-phone">
      <div className="cf-phone-notch" />
      <div className="cf-phone-stage">
        {videoUrl ? (
          <video className="cf-phone-video" src={videoUrl} controls playsInline />
        ) : cover ? (
          <div className="cf-phone-cover" style={{ backgroundImage: `url(${cover})` }}>
            <div className="cf-phone-overlay">
              <strong>{title}</strong>
            </div>
          </div>
        ) : (
          <div className="cf-phone-cover cf-phone-cover-empty">
            <strong>{title}</strong>
            <p>{body.trim() ? body.slice(0, 120) + (body.length > 120 ? '…' : '') : '左侧写口播稿后，这里预览成片效果'}</p>
          </div>
        )}
      </div>
      <div className="cf-phone-meta">
        <span>{body.replace(/\s/g, '').length} 字</span>
        <span>约 {secs}s</span>
        {draft.voiceTaskId && <span>配音已提交</span>}
        {draft.avatarTaskId && <span>数字人已提交</span>}
      </div>
      <div className="cf-phone-cap">
        {stepKey === 'script' && '口播预览'}
        {stepKey === 'assets' && '素材预览'}
        {stepKey === 'produce' && '成片预览'}
      </div>
    </div>
  )
}
