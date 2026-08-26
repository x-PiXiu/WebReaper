import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Alert, Button, Input, Select, Space, Typography } from 'antd'
import { SoundOutlined } from '@ant-design/icons'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'
import { message } from '../../../../utils/antdApp'

const { Text } = Typography
const { TextArea } = Input

/** 爆款配音：口播文案 → TTS 任务（发视频专属） */
export default function VoiceModule() {
  const navigate = useNavigate()
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)
  const [model, setModel] = useState<string>()
  const text = draft.rewritten || draft.script || ''

  useEffect(() => {
    draft.setTrack('video')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const { data: types = [] } = useQuery({
    queryKey: ['generation-types'],
    queryFn: () => businessApi.listGenerationTypes().then((r) => r.types),
  })

  const ttsModels = useMemo(() => {
    const t = types.find((x) => x.sub_type === 'tts')
    return (t?.models || []).map((m) => m.model)
  }, [types])

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
    setBusy(true)
    try {
      const task = await businessApi.submitGeneration({
        brand_id: bid,
        text: text.slice(0, 2000),
        type: 'audio',
        params: model ? { model } : undefined, // TTS 模型选择（此前下拉是死控件——所选模型从未提交）
      })
      draft.patch({ voiceTaskId: task.id, brandId: bid })
      message.success('配音任务已提交，可在生成任务列表查看产物')
      navigate('/m/compose/tools?tab=media')
    } catch {
      /* 拦截器 */
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="wr-page-content ip-page">
      <ComposeModuleHeader title="爆款配音" lead="把口播文案交给 TTS 生成配音（发视频专属）" badge="发视频" />
      <Alert
        style={{ marginBottom: 16 }}
        type="info"
        showIcon
        message="依赖生成服务中的 tts 端点；提交后可在多媒体工作台查看任务与音频产物"
      />
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <Text strong>配音文案</Text>
        <TextArea rows={10} style={{ marginTop: 8 }} value={text} onChange={(e) => draft.patch({ script: e.target.value })} />
        <div style={{ marginTop: 12 }}>
          <Text strong>TTS 模型</Text>
          <Select
            style={{ display: 'block', marginTop: 8, maxWidth: 360 }}
            placeholder={ttsModels.length ? '选择模型' : '无可用模型'}
            value={model || ttsModels[0]}
            onChange={setModel}
            options={ttsModels.map((m) => ({ value: m, label: m }))}
          />
        </div>
        <Space style={{ marginTop: 16 }} wrap>
          <Button type="primary" className="ip-btn-primary" icon={<SoundOutlined />} loading={busy} onClick={submit}>
            提交配音任务
          </Button>
          <Button onClick={() => navigate('/m/compose/avatar')}>去口播数字人</Button>
          <Button type="link" onClick={() => navigate('/m/compose/tools?tab=media')}>打开多媒体工作台</Button>
        </Space>
      </div>
    </div>
  )
}
