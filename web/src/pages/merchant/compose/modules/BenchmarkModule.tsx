import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Button, Input, Space, Typography, Upload, message, Alert, Select } from 'antd'
import { LinkOutlined, UploadOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'

const { Text, Paragraph } = Typography
const { TextArea } = Input

/** 爆款对标：链接 / 本地音视频 → 转写原文（ASR 未接时支持粘贴与上传） */
export default function BenchmarkModule() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const { brandId, setCurrentBrand, brands } = useBrandContext()
  const draft = useComposeDraft()
  const [url, setUrl] = useState(draft.sourceUrl || params.get('sourceUrl') || '')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    const topic = params.get('topic')
    const refTitle = params.get('refTitle')
    const hotPoint = params.get('hotPoint')
    const sourceUrl = params.get('sourceUrl')
    if (!refTitle && !topic && !hotPoint && !sourceUrl) return
    draft.patch({
      refTitle: refTitle || undefined,
      hotPoint: hotPoint || undefined,
      sourceUrl: sourceUrl || undefined,
      script: topic || undefined,
      transcript: hotPoint ? `【对标要点】${hotPoint}` : undefined,
    })
    if (sourceUrl) setUrl(sourceUrl)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const applyTranscript = (text: string) => {
    draft.patch({
      brandId: brandId || draft.brandId,
      sourceUrl: url || draft.sourceUrl,
      transcript: text,
      script: draft.script || text,
    })
    message.success('已写入共享草稿，可去「文案工作室」继续')
  }

  const mockTranscribeFromLink = () => {
    if (!url.trim()) {
      message.warning('请先粘贴爆款链接')
      return
    }
    setBusy(true)
    try {
      const bits = [
        draft.refTitle ? `参考标题：${draft.refTitle}` : '',
        draft.hotPoint ? `爆款要点：${draft.hotPoint}` : '',
        `来源链接：${url.trim()}`,
        '',
        '（自动转写尚未接入——请粘贴口播原文，或根据要点自行补全文案）',
      ].filter(Boolean)
      draft.patch({ sourceUrl: url.trim() })
      applyTranscript(bits.join('\n'))
    } finally {
      setBusy(false)
    }
  }

  const onUpload = async (file: File) => {
    setBusy(true)
    try {
      const { asset } = await businessApi.uploadAsset(file)
      draft.patch({ sourceUrl: asset.url, brandId: brandId || draft.brandId })
      applyTranscript(
        `【本地文件】${file.name}\n素材地址：${asset.url}\n\n（语音转写即将接入——请粘贴转写文案，或去「文案工作室」撰写）`,
      )
      message.success('素材已上传')
    } catch {
      /* 拦截器 */
    } finally {
      setBusy(false)
    }
    return false
  }

  return (
    <div className="wr-page-content ip-page">
      <ComposeModuleHeader
        title="爆款对标"
        lead="粘贴平台链接或上传本地音视频，沉淀对标原文到共享草稿"
        badge="部分可用"
      />
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="语音/视频自动转写（ASR）尚未接入服务端"
        description="当前可上传素材、保存链接并手动粘贴口播原文；也可从人设档案「热门同款」带选题跳入。"
      />
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <Space direction="vertical" style={{ width: '100%' }} size={14}>
          <div>
            <Text strong>当前人设</Text>
            <Select
              style={{ display: 'block', marginTop: 8, maxWidth: 360 }}
              placeholder="选择人设档案"
              value={brandId}
              onChange={(v) => setCurrentBrand(v)}
              options={brands.map((b) => ({ value: b.id, label: b.name }))}
              allowClear
            />
          </div>
          <div>
            <Text strong><LinkOutlined /> 爆款链接</Text>
            <Input
              style={{ marginTop: 8 }}
              placeholder="粘贴抖音 / 小红书等视频链接"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              allowClear
            />
          </div>
          <Space wrap>
            <Button type="primary" className="ip-btn-primary" loading={busy} onClick={mockTranscribeFromLink}>
              解析链接并写入草稿
            </Button>
            <Upload accept="audio/*,video/*" showUploadList={false} beforeUpload={onUpload}>
              <Button icon={<UploadOutlined />} loading={busy}>上传本地音视频</Button>
            </Upload>
            <Button type="link" onClick={() => navigate('/m/brands')}>去热门同款发现 →</Button>
          </Space>
          <div>
            <Text strong>转写 / 对标原文</Text>
            <TextArea
              style={{ marginTop: 8 }}
              rows={10}
              placeholder="粘贴口播原文"
              value={draft.transcript || ''}
              onChange={(e) => draft.patch({ transcript: e.target.value, script: e.target.value })}
            />
          </div>
          <Space wrap>
            <Button
              type="primary"
              className="ip-btn-primary"
              icon={<ThunderboltOutlined />}
              disabled={!(draft.transcript || '').trim()}
              onClick={() => navigate('/m/compose/copy')}
            >
              去文案工作室
            </Button>
            <Button type="link" onClick={() => navigate(draft.track === 'graphic' ? '/m/compose/graphic' : '/m/compose/video')}>
              返回{draft.track === 'graphic' ? '发图文' : '发视频'}
            </Button>
          </Space>
          {(draft.refTitle || draft.hotPoint) && (
            <Paragraph type="secondary" style={{ margin: 0, fontSize: 12 }}>
              参考：{draft.refTitle || '—'}
              {draft.hotPoint ? ` · ${draft.hotPoint}` : ''}
            </Paragraph>
          )}
        </Space>
      </div>
    </div>
  )
}
