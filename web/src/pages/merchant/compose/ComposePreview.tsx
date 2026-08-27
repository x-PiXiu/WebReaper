import { useEffect, useRef, useState } from 'react'
import { Segmented, Tag } from 'antd'
import { SoundOutlined, VideoCameraOutlined, PictureOutlined, FileTextOutlined } from '@ant-design/icons'
import { useComposeDraft, type ComposeTrack } from '../../../store/composeDraft'
import { previewPauseMarkers } from '../../../utils/pauseMarkers'

function estimateSeconds(text: string) {
  const chars = text.replace(/\s/g, '').length
  return Math.max(0, Math.round(chars / 4))
}

type VideoPreviewTab = 'script' | 'voice' | 'video' | 'cover'

/** 右侧预览：视频手机框 / 图文笔记卡 */
export function ComposePreview({ track, stepKey }: { track: ComposeTrack; stepKey: string }) {
  const draft = useComposeDraft()
  const body = draft.rewritten || draft.script || draft.transcript || ''
  const title = draft.selectedTitle || draft.refTitle || (track === 'video' ? '短视频标题' : '图文标题')
  const secs = estimateSeconds(body)
  const cover = draft.coverUrl
  const images = (draft.imageUrls || []).filter(Boolean)
  const videoUrl = draft.editedVideoUrl || draft.avatarVideoUrl
  const scriptPreview = previewPauseMarkers(body)

  const [videoTab, setVideoTab] = useState<VideoPreviewTab>('script')
  const prevAssets = useRef({ voice: '', video: '', cover: '' })

  useEffect(() => {
    if (track !== 'video') return
    const voice = draft.voiceUrl || ''
    const video = videoUrl || ''
    const cov = draft.coverUrl || ''
    const prev = prevAssets.current

    if (video && !prev.video) setVideoTab('video')
    else if (voice && !prev.voice && stepKey === 'assets') setVideoTab('voice')
    else if (cov && !prev.cover && stepKey === 'assets') setVideoTab('cover')

    prevAssets.current = { voice, video, cover: cov }
  }, [track, draft.voiceUrl, draft.coverUrl, videoUrl, stepKey])

  useEffect(() => {
    if (track !== 'video') return
    if (stepKey === 'script') setVideoTab('script')
    if (stepKey === 'produce' && videoUrl) setVideoTab('video')
  }, [track, stepKey, videoUrl])

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

  const tabOptions = [
    { value: 'script' as const, label: '文案', icon: <FileTextOutlined /> },
    ...(draft.voiceUrl || draft.voiceTaskId
      ? [{ value: 'voice' as const, label: draft.voiceUrl ? '配音 ✓' : '配音…', icon: <SoundOutlined /> }]
      : []),
    ...(videoUrl || draft.avatarTaskId
      ? [{ value: 'video' as const, label: videoUrl ? '成片 ✓' : '成片…', icon: <VideoCameraOutlined /> }]
      : []),
    ...(cover || draft.coverTaskId
      ? [{ value: 'cover' as const, label: cover ? '封面 ✓' : '封面…', icon: <PictureOutlined /> }]
      : []),
  ]

  const showTabs = stepKey !== 'script' && tabOptions.length > 1

  return (
    <div className="cf-phone">
      {showTabs && (
        <div className="cf-preview-tabs">
          <Segmented
            size="small"
            block
            value={videoTab}
            onChange={(v) => setVideoTab(v as VideoPreviewTab)}
            options={tabOptions.map((o) => ({ value: o.value, label: o.label }))}
          />
        </div>
      )}
      <div className="cf-phone-notch" />
      <div className="cf-phone-stage">
        {videoTab === 'voice' && draft.voiceUrl ? (
          <div className="cf-phone-audio-stage">
            <p className="cf-phone-audio-label">配音预览</p>
            <audio className="cf-phone-audio" src={draft.voiceUrl} controls />
          </div>
        ) : videoTab === 'cover' && cover ? (
          <div className="cf-phone-cover" style={{ backgroundImage: `url(${cover})` }}>
            <div className="cf-phone-overlay">
              <strong>{title}</strong>
            </div>
          </div>
        ) : videoTab === 'video' && videoUrl ? (
          <video className="cf-phone-video" src={videoUrl} controls playsInline />
        ) : (
          <div className="cf-phone-cover cf-phone-cover-empty">
            <strong>{title}</strong>
            <p>
              {body.trim()
                ? scriptPreview.slice(0, 160) + (scriptPreview.length > 160 ? '…' : '')
                : '左侧写口播稿后，这里预览成片效果'}
            </p>
          </div>
        )}
      </div>
      <div className="cf-phone-meta">
        <span>{body.replace(/\s/g, '').length} 字</span>
        <span>约 {secs}s</span>
        {draft.voiceUrl && <span>配音 ✓</span>}
        {videoUrl && <span>成片 ✓</span>}
        {cover && <span>封面 ✓</span>}
      </div>
      <div className="cf-phone-cap">
        {stepKey === 'script' && '口播预览'}
        {stepKey === 'assets' && '素材预览'}
        {stepKey === 'produce' && '成片预览'}
      </div>
    </div>
  )
}
