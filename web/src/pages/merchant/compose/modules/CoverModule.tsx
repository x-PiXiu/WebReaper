import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Button, Input, Space, Typography, Alert } from 'antd'
import { PictureOutlined } from '@ant-design/icons'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'
import { COVER_STYLES } from '../../../../data/coverStyles'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'
import { message } from '../../../../utils/antdApp'

const { Text } = Typography

/** 封面：发视频 = 视频封面；发图文 = 图文封面（文案与跳转目标不同） */
export default function CoverModule() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (params.get('track') === 'graphic') draft.setTrack('graphic')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const isGraphic = draft.track === 'graphic'
  const title = draft.selectedTitle || draft.refTitle || (isGraphic ? '图文封面标题' : '视频封面标题')

  const genImage = async () => {
    const bid = brandId || draft.brandId
    if (!bid) {
      message.warning('请先选择人设/品牌')
      return
    }
    setBusy(true)
    try {
      await businessApi.submitGeneration({
        brand_id: bid,
        type: 'image',
        text: isGraphic
          ? `小红书封面图，竖屏，大标题「${title}」，清爽种草风`
          : `短视频封面，竖屏 9:16，大标题「${title}」，简洁醒目，适合抖音`,
        aspect_ratio: '9:16',
        params: undefined, // BE-GEN-01/03 已修
      })
      message.success('封面图任务已提交，请到多媒体工作台取回 URL 填入下方')
      navigate('/m/compose/tools?tab=media')
    } catch {
      /* 拦截器 */
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
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <Text strong>封面标题</Text>
        <Input style={{ marginTop: 8, marginBottom: 16 }} value={title} onChange={(e) => draft.patch({ selectedTitle: e.target.value })} />
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
        <Text strong style={{ display: 'block', marginTop: 16 }}>封面图 URL</Text>
        <Input
          style={{ marginTop: 8 }}
          placeholder="生成后的图片地址"
          value={draft.coverUrl || ''}
          onChange={(e) => draft.patch({ coverUrl: e.target.value })}
        />
        <Space style={{ marginTop: 16 }} wrap>
          <Button icon={<PictureOutlined />} loading={busy} onClick={genImage}>文生图生成封面</Button>
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
