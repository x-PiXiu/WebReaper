import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Button, Input, Space, Typography, Alert } from 'antd'
import { PictureOutlined } from '@ant-design/icons'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'
import { COVER_STYLES } from '../../../../data/coverStyles'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'
import { CapabilityBanner } from '../../../../components/wizard/CapabilityBanner'
import { catchGenerationError } from '../../../../utils/generationErrors'
import { useComposeTaskPoll } from '../../../../hooks/useComposeTaskPoll'
import { GenerationTaskStatusBar } from '../../../../components/compose/GenerationTaskStatusBar'
import { MediaResultCard } from '../../../../components/compose/MediaResultCard'
import { ManualUrlField } from '../../../../components/compose/ManualUrlField'
import { toast } from '../../../../utils/feedback'

const { Text } = Typography

/** 封面：发视频 = 视频封面；发图文 = 图文封面（文案与跳转目标不同） */
export default function CoverModule() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)

  useComposeTaskPoll()

  useEffect(() => {
    if (params.get('track') === 'graphic') draft.setTrack('graphic')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const isGraphic = draft.track === 'graphic'
  const title = draft.selectedTitle || draft.refTitle || (isGraphic ? '图文封面标题' : '视频封面标题')

  const genImage = async () => {
    const bid = brandId || draft.brandId
    if (!bid) {
      toast.warn('请先选择人设', 'cover-brand')
      return
    }
    setBusy(true)
    try {
      const res = await businessApi.submitGeneration({
        brand_id: bid,
        type: 'image',
        text: isGraphic
          ? `小红书封面图，竖屏，大标题「${title}」，清爽种草风`
          : `短视频封面，竖屏 9:16，大标题「${title}」，简洁醒目，适合抖音`,
        aspect_ratio: '9:16',
        params: undefined,
      })
      draft.patch({
        coverTaskId: res.id,
        track: isGraphic ? 'graphic' : 'video',
        lastUpdatedAt: new Date().toISOString(),
      })
      toast.ok('封面任务已提交，完成后自动填入', 'cover-gen')
    } catch (e) {
      catchGenerationError(e)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="wr-page-content ip-page">
      <ComposeModuleHeader
        title={isGraphic ? '图文封面' : '视频封面'}
        lead={isGraphic ? '为种草笔记 / 图文帖准备封面' : '为短视频成片准备封面标题卡'}
        badge={isGraphic ? '发图文' : '发视频'}
      />
      <Alert
        style={{ marginBottom: 16 }}
        type="info"
        showIcon
        message={isGraphic
          ? '当前在发图文轨道——配图请用「图文配图」模块，这里只定封面'
          : '当前在发视频轨道——成片请用数字人 / 剪辑，这里只定视频封面'}
      />
      <CapabilityBanner required={['text2image']} />
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <Text strong>封面标题</Text>
        <Input style={{ marginTop: 8, marginBottom: 16 }} value={title} onChange={(e) => draft.patch({ selectedTitle: e.target.value })} />

        <GenerationTaskStatusBar
          taskId={draft.coverTaskId}
          resultReady={!!draft.coverUrl}
          doneLabel="封面已就绪"
          fallbackPending="封面"
          onClearFailed={() => draft.patch({ coverTaskId: undefined })}
          onRetry={genImage}
        />

        {draft.coverUrl ? (
          <MediaResultCard
            kind="image"
            url={draft.coverUrl}
            label={isGraphic ? '图文封面' : '视频封面'}
            onClear={() => draft.patch({ coverUrl: undefined, coverTaskId: undefined })}
          />
        ) : (
          <>
            <Text strong>模板</Text>
            <div className="ip-pick-grid" style={{ marginTop: 8 }}>
              {COVER_STYLES.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  className={`ip-pick-card${draft.coverAccent === c.accent ? ' is-active' : ''}`}
                  onClick={() => draft.patch({ coverAccent: c.accent, coverUrl: draft.coverUrl })}
                >
                  <span className="ip-pick-swatch" style={{ background: `linear-gradient(160deg,#111,${c.accent})` }} />
                  <strong>{c.name}</strong>
                  <span>{c.mood}</span>
                </button>
              ))}
            </div>
            <Button
              icon={<PictureOutlined />}
              loading={busy}
              onClick={genImage}
              style={{ marginTop: 16 }}
            >
              文生图生成封面
            </Button>
          </>
        )}

        <ManualUrlField
          value={draft.coverUrl || ''}
          placeholder="或手动粘贴封面图 URL"
          onChange={(v) => draft.patch({ coverUrl: v || undefined })}
        />

        <Space style={{ marginTop: 16 }} wrap>
          <Button
            type="primary"
            className="ip-btn-primary"
            onClick={() => {
              const q = new URLSearchParams()
              q.set('contentType', isGraphic ? 'image' : 'video')
              if (isGraphic) {
                const imgs = [...(draft.imageUrls || []), draft.coverUrl].filter(Boolean) as string[]
                if (imgs.length) q.set('mediaUrls', imgs.join(','))
              } else {
                const media = draft.editedVideoUrl || draft.avatarVideoUrl
                if (media) q.set('mediaUrls', media)
                if (draft.coverUrl) q.set('coverUrl', draft.coverUrl)
              }
              if (brandId || draft.brandId) q.set('brandId', brandId || draft.brandId!)
              if (draft.selectedTitle) q.set('title', draft.selectedTitle)
              const body = (draft.rewritten || draft.script || '').trim()
              if (body) q.set('content', body.slice(0, 8000))
              navigate(`/m/distribution?${q.toString()}`)
            }}
          >
            {isGraphic ? '去发布图文' : '去发布视频'}
          </Button>
        </Space>
      </div>
    </div>
  )
}
