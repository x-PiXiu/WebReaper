import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Alert, Button, Input, Select, Space, Typography, message } from 'antd'
import { RobotOutlined } from '@ant-design/icons'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'
import { useMediaAssets } from '../../../../hooks/useMediaAssets'

const { Text } = Typography
const { TextArea } = Input

/** 口播数字人：文案/音色 → digital_human 任务（发视频专属） */
export default function AvatarModule() {
  const navigate = useNavigate()
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)
  const [model, setModel] = useState<string>()
  const [imageUrl, setImageUrl] = useState('')
  const text = draft.rewritten || draft.script || ''

  useEffect(() => {
    draft.setTrack('video')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const { data: types = [] } = useQuery({
    queryKey: ['generation-types'],
    queryFn: () => businessApi.listGenerationTypes().then((r) => r.types),
  })
  const { data: assets = [] } = useMediaAssets()

  const dhModels = useMemo(() => {
    const t = types.find((x) => x.sub_type === 'digital_human')
    return (t?.models || []).map((m) => m.model)
  }, [types])

  const imageAssets = assets.filter((a) => (a.mime || '').startsWith('image/'))

  const submit = async () => {
    if (!text.trim()) {
      message.warning('请先准备口播文案')
      return
    }
    const m = model || dhModels[0]
    if (!m) {
      message.warning('暂无数字人模型——请配置生成规格或打开多媒体工作台')
      return
    }
    if (!imageUrl) {
      message.warning('请选择人像参考图')
      return
    }
    setBusy(true)
    try {
      const task = await businessApi.submitGenerationTask({
        brand_id: brandId || draft.brandId,
        sub_type: 'digital_human',
        model: m,
        params: { prompt: text.slice(0, 2000) },
        refs: [
          { id: 'portrait', name: 'portrait', url: imageUrl, kind: 'image' },
          ...(draft.voiceUrl
            ? [{ id: 'audio', name: 'audio', url: draft.voiceUrl, kind: 'audio' as const }]
            : []),
        ],
      })
      draft.patch({ avatarTaskId: task.id, brandId: brandId || draft.brandId })
      message.success('数字人任务已提交')
      navigate('/m/compose/tools?tab=media')
    } catch {
      /* 拦截器 */
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="wr-page-content ip-page">
      <ComposeModuleHeader title="口播数字人" lead="人像 + 文案生成口播视频（发视频专属）" badge="发视频" />
      <Alert style={{ marginBottom: 16 }} type="info" showIcon message="调用 digital_human 生成端点；复杂参数可在多媒体工作台细调" />
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <Text strong>口播文案</Text>
        <TextArea rows={8} style={{ marginTop: 8, marginBottom: 12 }} value={text} onChange={(e) => draft.patch({ script: e.target.value })} />
        <Text strong>人像参考图</Text>
        <Select
          style={{ display: 'block', marginTop: 8, marginBottom: 12, maxWidth: 480 }}
          placeholder={imageAssets.length ? '从素材库选择' : '请先在多媒体工作台上传人像'}
          value={imageUrl || undefined}
          onChange={setImageUrl}
          options={imageAssets.map((a) => ({ value: a.url, label: a.url.split('/').pop() || a.id }))}
        />
        <Text strong>模型</Text>
        <Select
          style={{ display: 'block', marginTop: 8, maxWidth: 360 }}
          value={model || dhModels[0]}
          onChange={setModel}
          options={dhModels.map((m) => ({ value: m, label: m }))}
          placeholder="选择模型"
        />
        <Space style={{ marginTop: 16 }} wrap>
          <Button type="primary" className="ip-btn-primary" icon={<RobotOutlined />} loading={busy} onClick={submit}>
            提交数字人任务
          </Button>
          <Button onClick={() => navigate('/m/compose/edit')}>去智能剪辑</Button>
          <Button type="link" onClick={() => navigate('/m/assets')}>资产库</Button>
        </Space>
      </div>
    </div>
  )
}
