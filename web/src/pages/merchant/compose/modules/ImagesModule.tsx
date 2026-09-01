import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Alert, Button, Input, Space, Typography, Upload } from 'antd'
import { PictureOutlined, PlusOutlined } from '@ant-design/icons'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'
import { CapabilityBanner } from '../../../../components/wizard/CapabilityBanner'
import { catchGenerationError } from '../../../../utils/generationErrors'
import { useComposeTaskPoll } from '../../../../hooks/useComposeTaskPoll'
import { GenerationTaskStatusBar } from '../../../../components/compose/GenerationTaskStatusBar'
import { TaskStatusBar } from '../../../../components/compose/TaskStatusBar'
import { toast } from '../../../../utils/feedback'

const { Text } = Typography

/** 图文配图（发图文专属）：上传 / 文生图，与视频封面区分开 */
export default function ImagesModule() {
  const navigate = useNavigate()
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)
  const [prompt, setPrompt] = useState(draft.selectedTitle || draft.refTitle || '')

  useComposeTaskPoll()

  useEffect(() => {
    draft.setTrack('graphic')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const urls = draft.imageUrls || []
  const list = useMemo(() => urls.filter(Boolean), [urls])
  const pendingImages = (draft.imageTaskIds || []).length
  const activeImageTaskId = pendingImages > 0 ? draft.imageTaskIds?.[0] : undefined

  const gen = async () => {
    const bid = brandId || draft.brandId
    if (!bid) {
      toast.warn('请先选择人设', 'images-brand')
      return
    }
    setBusy(true)
    try {
      const res = await businessApi.submitGeneration({
        brand_id: bid,
        type: 'image',
        text: `小红书种草配图，竖版清爽，主题「${prompt || '产品种草'}」，真实场景感，不要水印`,
        aspect_ratio: '9:16',
        params: undefined,
      })
      draft.patch({
        imageTaskIds: [...(draft.imageTaskIds || []), res.id],
        track: 'graphic',
        lastUpdatedAt: new Date().toISOString(),
      })
      toast.ok('配图任务已提交，完成后自动加入', 'images-gen')
    } catch (e) {
      catchGenerationError(e)
    } finally {
      setBusy(false)
    }
  }

  const onUpload = async (file: File) => {
    setBusy(true)
    try {
      const asset = await businessApi.uploadAsset(file)
      draft.patch({ imageUrls: [...list, asset.url], track: 'graphic' })
      toast.ok('已加入配图', 'images-upload')
    } catch (e) {
      catchGenerationError(e)
    } finally {
      setBusy(false)
    }
    return false
  }

  const removeImage = (index: number) => {
    draft.patch({ imageUrls: list.filter((_, j) => j !== index) })
  }

  return (
    <div className="wr-page-content ip-page">
      <ComposeModuleHeader
        title="图文配图"
        lead="为种草笔记 / 图文帖准备配图——与「发视频」的成片、封面分开"
        badge="发图文"
      />
      <Alert
        style={{ marginBottom: 16 }}
        type="info"
        showIcon
        message="本模块仅服务发图文轨道；视频成片请走「发视频」里的数字人 / 剪辑"
      />
      <CapabilityBanner required={['text2image']} />
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <GenerationTaskStatusBar
          taskId={activeImageTaskId}
          resultReady={pendingImages === 0 && list.length > 0}
          doneLabel={`已选 ${list.length} 张配图`}
          fallbackPending="配图"
          onClearFailed={() => {
            const ids = draft.imageTaskIds || []
            if (ids.length) draft.patch({ imageTaskIds: ids.slice(1) })
          }}
          onRetry={gen}
        />
        {(pendingImages > 1 || (pendingImages > 0 && list.length === 0)) && (
          <TaskStatusBar
            pending
            pendingLabel={`${pendingImages} 张配图生成中…`}
          />
        )}

        <Text strong>文生图提示词</Text>
        <Input
          style={{ marginTop: 8, marginBottom: 12 }}
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          placeholder="例如：门店午市套餐种草图"
        />
        <Space wrap style={{ marginBottom: 16 }}>
          <Button type="primary" className="ip-btn-primary" icon={<PictureOutlined />} loading={busy} onClick={gen}>
            AI 生成配图
          </Button>
          <Upload accept="image/*" showUploadList={false} beforeUpload={onUpload}>
            <Button icon={<PlusOutlined />} loading={busy}>上传配图</Button>
          </Upload>
        </Space>

        {list.length > 0 ? (
          <div className="cf-image-grid">
            {list.map((u, i) => (
              <div key={u + i} className="cf-image-grid-item">
                <div className="cf-media-thumb" style={{ backgroundImage: `url(${u})` }} />
                <Button type="text" size="small" danger onClick={() => removeImage(i)}>移除</Button>
              </div>
            ))}
          </div>
        ) : (
          <Text type="secondary">上传或 AI 生成配图后在此预览</Text>
        )}

        <Space style={{ marginTop: 16 }} wrap>
          <Button
            type="primary"
            className="ip-btn-primary"
            onClick={() => navigate('/m/compose/cover?track=graphic')}
          >
            去图文封面
          </Button>
          <Button
            onClick={() => {
              const q = new URLSearchParams({ contentType: 'image' })
              if (list.length) q.set('mediaUrls', list.join(','))
              if (draft.selectedTitle) q.set('title', draft.selectedTitle)
              const body = (draft.rewritten || draft.script || '').trim()
              if (body) q.set('content', body.slice(0, 8000))
              if (brandId || draft.brandId) q.set('brandId', brandId || draft.brandId!)
              navigate(`/m/distribution?${q.toString()}`)
            }}
          >
            去发布图文
          </Button>
        </Space>
      </div>
    </div>
  )
}
