import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Button, Input, Space, Typography, Upload, Alert, Select } from 'antd'
import { LinkOutlined, UploadOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'
import { extractShareUrl, isKuaishouUrl } from '../../../../utils/shareUrl'
import { message } from '../../../../utils/antdApp'

const { Text, Paragraph } = Typography
const { TextArea } = Input

/** 爆款对标：链接 / 本地音视频 → 服务端转写原文 → 共享草稿 */
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

  const applyTranscript = (text: string, source?: string) => {
    draft.patch({
      brandId: brandId || draft.brandId,
      sourceUrl: source || url || draft.sourceUrl,
      transcript: text,
      script: text,
    })
    message.success('已写入共享草稿，可去「文案工作室」继续')
  }

  const extractFromLink = async () => {
    if (!url.trim()) {
      message.warning('请先粘贴爆款链接')
      return
    }
    if (isKuaishouUrl(url)) {
      message.info('快手暂不支持链接提取，请下载视频后用上传方式')
      return
    }
    const link = extractShareUrl(url)
    if (!link) {
      message.warning('未识别到抖音/B站链接。请粘贴完整分享口令（需含 https://v.douyin.com/…）')
      return
    }
    if (link !== url.trim()) setUrl(link)
    setBusy(true)
    try {
      const r = await businessApi.extractTranscript({
        share_url: link,
        title: draft.refTitle || undefined,
      })
      draft.patch({ sourceUrl: link })
      applyTranscript(r.raw_text || '', link)
    } catch {
      /* 拦截器 */
    } finally {
      setBusy(false)
    }
  }

  const onUpload = async (file: File) => {
    setBusy(true)
    try {
      const asset = await businessApi.uploadAsset(file)
      draft.patch({ sourceUrl: asset.url, brandId: brandId || draft.brandId })
      const r = await businessApi.extractTranscript({
        asset_url: asset.url,
        title: file.name,
      })
      applyTranscript(r.raw_text || '', asset.url)
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
        lead="粘贴平台链接或上传本地音视频，自动转写对标原文到共享草稿"
        badge="可用"
      />
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="已接入服务端文案提取（与口播向导相同）"
        description="支持分享链接与本站上传素材；也可手动改写转写结果。可从人设档案「热门同款」带选题跳入。"
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
            <Button type="primary" className="ip-btn-primary" loading={busy} onClick={extractFromLink}>
              解析链接并转写
            </Button>
            <Upload accept="audio/*,video/*" showUploadList={false} beforeUpload={onUpload}>
              <Button icon={<UploadOutlined />} loading={busy}>上传并转写</Button>
            </Upload>
            <Button type="link" onClick={() => navigate('/m/brands')}>去热门同款发现 →</Button>
          </Space>
          <div>
            <Text strong>转写 / 对标原文</Text>
            <TextArea
              style={{ marginTop: 8 }}
              rows={10}
              placeholder="转写结果会出现在这里，也可手动粘贴口播原文"
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
            <Button type="link" onClick={() => navigate(draft.track === 'graphic' ? '/m/compose/graphic' : '/m/compose/lipsync')}>
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
