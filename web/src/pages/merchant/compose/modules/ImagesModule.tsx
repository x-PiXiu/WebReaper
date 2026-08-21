import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Alert, Button, Input, Space, Typography, message, Upload } from 'antd'
import { PictureOutlined, PlusOutlined } from '@ant-design/icons'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'

const { Text } = Typography

/** 图文配图（发图文专属）：上传 / 文生图，与视频封面区分开 */
export default function ImagesModule() {
  const navigate = useNavigate()
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)
  const [prompt, setPrompt] = useState(draft.selectedTitle || draft.refTitle || '')

  useEffect(() => {
    draft.setTrack('graphic')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  const urls = draft.imageUrls || []

  const { data: types = [] } = useQuery({
    queryKey: ['generation-types'],
    queryFn: () => businessApi.listGenerationTypes().then((r) => r.types),
  })
  const imgModel = types.find((t) => t.sub_type === 'text2image')?.models?.[0]?.model

  const gen = async () => {
    if (!imgModel) {
      message.warning('暂无文生图模型')
      return
    }
    setBusy(true)
    try {
      await businessApi.submitGenerationTask({
        brand_id: brandId || draft.brandId,
        sub_type: 'text2image',
        model: imgModel,
        params: {
          prompt: `小红书种草配图，竖版清爽，主题「${prompt || '产品种草'}」，真实场景感，不要水印`,
        },
      })
      message.success('配图任务已提交，完成后可在多媒体台取回 URL 填入下方')
      navigate('/m/compose/tools?tab=media')
    } catch {
      /* 拦截器 */
    } finally {
      setBusy(false)
    }
  }

  const onUpload = async (file: File) => {
    setBusy(true)
    try {
      const { asset } = await businessApi.uploadAsset(file)
      draft.patch({ imageUrls: [...urls, asset.url], track: 'graphic' })
      message.success('已加入配图')
    } catch {
      /* */
    } finally {
      setBusy(false)
    }
    return false
  }

  const list = useMemo(() => urls, [urls])

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
      <div className="wr-glass-card" style={{ padding: 24 }}>
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

        <Text strong>已选配图 URL</Text>
        <Space direction="vertical" style={{ width: '100%', marginTop: 8 }} size={8}>
          {list.length === 0 && <Text type="secondary">暂无配图</Text>}
          {list.map((u, i) => (
            <Space key={u + i} style={{ width: '100%' }}>
              <Input
                value={u}
                onChange={(e) => {
                  const next = [...list]
                  next[i] = e.target.value
                  draft.patch({ imageUrls: next })
                }}
              />
              <Button
                danger
                type="text"
                onClick={() => draft.patch({ imageUrls: list.filter((_, j) => j !== i) })}
              >
                移除
              </Button>
            </Space>
          ))}
          <Button
            type="dashed"
            block
            onClick={() => draft.patch({ imageUrls: [...list, ''] })}
          >
            手动添加一张
          </Button>
        </Space>

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
              const q = new URLSearchParams({ contentType: 'article' })
              if (list.filter(Boolean).length) q.set('mediaUrls', list.filter(Boolean).join(','))
              if (draft.selectedTitle) q.set('title', draft.selectedTitle)
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
