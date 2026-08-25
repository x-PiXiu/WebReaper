import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Alert, Button, Input, Select, Space, Typography, message } from 'antd'
import { RobotOutlined } from '@ant-design/icons'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { ensureMaterialId, submitUnified } from '../../../../api/generationSubmit'
import { useMediaAssets } from '../../../../hooks/useMediaAssets'

const { Text } = Typography
const { TextArea } = Input

/** 口播数字人：人像图 + 配音 → digital_human；仅人像+文案 → img2video */
export default function AvatarModule() {
  const navigate = useNavigate()
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)
  const [imageId, setImageId] = useState('')
  const text = draft.rewritten || draft.script || ''

  useEffect(() => {
    draft.setTrack('video')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const { data: assets = [] } = useMediaAssets()
  const imageAssets = useMemo(
    () => assets.filter((a) => (a.mime || '').startsWith('image/')),
    [assets],
  )

  const submit = async () => {
    if (!text.trim()) {
      message.warning('请先准备口播文案')
      return
    }
    const bid = brandId || draft.brandId
    if (!bid) {
      message.warning('请先选择人设/品牌')
      return
    }
    if (!imageId) {
      message.warning('请选择人像参考图')
      return
    }
    setBusy(true)
    try {
      const materials = [await ensureMaterialId(imageId)]
      if (draft.voiceUrl) {
        materials.push(await ensureMaterialId(draft.voiceUrl))
        const task = await submitUnified({
          brand_id: bid,
          materials,
          text: text.slice(0, 2000),
        })
        draft.patch({ avatarTaskId: task.id, brandId: bid })
        message.success('数字人口播任务已提交（图+音频）')
      } else {
        const task = await submitUnified({
          brand_id: bid,
          type: 'video',
          materials,
          text: text.slice(0, 2000),
        })
        draft.patch({ avatarTaskId: task.id, brandId: bid })
        message.success('已提交图生视频（建议先去「爆款配音」再生成口播）')
      }
      navigate('/m/compose/tools?tab=media')
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : ''
      if (msg) message.error(msg)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="wr-page-content ip-page">
      <ComposeModuleHeader title="口播数字人" lead="人像 + 配音 → 口播成片（发视频专属）" badge="发视频" />
      <Alert
        style={{ marginBottom: 16 }}
        type="info"
        showIcon
        message={draft.voiceUrl
          ? '将按「1 图 + 1 音频」提交，系统自动走 digital_human'
          : '当前无配音：将按图生视频提交。完整口播请先生成配音，或使用「拍口播」向导'}
      />
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <Text strong>口播文案</Text>
        <TextArea rows={8} style={{ marginTop: 8, marginBottom: 12 }} value={text} onChange={(e) => draft.patch({ script: e.target.value })} />
        <Text strong>人像参考图（素材库 ID）</Text>
        <Select
          style={{ display: 'block', marginTop: 8, marginBottom: 12, maxWidth: 480 }}
          placeholder={imageAssets.length ? '从素材库选择' : '请先在多媒体工作台上传人像'}
          value={imageId || undefined}
          onChange={setImageId}
          options={imageAssets.map((a) => ({
            value: a.id,
            label: (a.url || a.id).split('/').pop() || a.id,
          }))}
        />
        <Space style={{ marginTop: 16 }} wrap>
          <Button type="primary" className="ip-btn-primary" icon={<RobotOutlined />} loading={busy} onClick={submit}>
            提交数字人任务
          </Button>
          <Button onClick={() => navigate('/m/compose/voice')}>先去配音</Button>
          <Button onClick={() => navigate('/m/compose/edit')}>去成片确认</Button>
          <Button type="link" onClick={() => navigate('/m/assets')}>资产库</Button>
        </Space>
      </div>
    </div>
  )
}
