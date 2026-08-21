import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Button, Input, Select, Space, Upload, message } from 'antd'
import { PictureOutlined, SoundOutlined, UserOutlined } from '@ant-design/icons'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'
import { MOCK_COVERS } from '../../../../mock/ipAssets'

/** Step 2 发视频：配音 / 形象 / 封面 */
export function VideoAssetsStep() {
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)
  const [ttsModel, setTtsModel] = useState<string>()
  const [dhModel, setDhModel] = useState<string>()
  const [avatarImage, setAvatarImage] = useState('')
  const text = draft.rewritten || draft.script || ''

  const { data: types = [] } = useQuery({
    queryKey: ['generation-types'],
    queryFn: () => businessApi.listGenerationTypes().then((r) => r.types),
  })

  const ttsModels = useMemo(() => {
    const t = types.find((x) => x.sub_type === 'tts')
    return (t?.models || []).map((m) => m.model)
  }, [types])

  const dhModels = useMemo(() => {
    const t = types.find((x) => x.sub_type === 'digital_human')
    return (t?.models || []).map((m) => m.model)
  }, [types])

  const imgModel = types.find((t) => t.sub_type === 'text2image')?.models?.[0]?.model

  const runTts = async () => {
    const model = ttsModel || ttsModels[0]
    if (!model || !text.trim()) {
      message.warning('需要口播文案与 TTS 模型')
      return
    }
    setBusy(true)
    try {
      const res = await businessApi.submitGenerationTask({
        brand_id: brandId || draft.brandId,
        sub_type: 'tts',
        model,
        params: { text },
      })
      draft.patch({ voiceTaskId: res.id, track: 'video' })
      message.success('配音任务已提交')
    } catch {
      /* */
    } finally {
      setBusy(false)
    }
  }

  const runAvatar = async () => {
    const model = dhModel || dhModels[0]
    if (!model || !text.trim()) {
      message.warning('需要口播文案与数字人模型')
      return
    }
    setBusy(true)
    try {
      const res = await businessApi.submitGenerationTask({
        brand_id: brandId || draft.brandId,
        sub_type: 'digital_human',
        model,
        params: {
          text,
          image_url: avatarImage || undefined,
          audio_url: draft.voiceUrl || undefined,
        },
      })
      draft.patch({ avatarTaskId: res.id, track: 'video' })
      message.success('数字人口播任务已提交')
    } catch {
      /* */
    } finally {
      setBusy(false)
    }
  }

  const genCover = async () => {
    if (!imgModel) {
      message.warning('暂无文生图模型')
      return
    }
    const title = draft.selectedTitle || '短视频封面'
    setBusy(true)
    try {
      await businessApi.submitGenerationTask({
        brand_id: brandId || draft.brandId,
        sub_type: 'text2image',
        model: imgModel,
        params: {
          prompt: `短视频封面，竖屏 9:16，大标题「${title}」，简洁醒目`,
        },
      })
      message.success('封面生成任务已提交，完成后把 URL 填到下方')
    } catch {
      /* */
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="cf-panel cf-assets">
      <section className="cf-asset-block">
        <div className="cf-asset-title"><SoundOutlined /> 爆款配音</div>
        <Space wrap style={{ width: '100%' }}>
          <Select
            style={{ minWidth: 200 }}
            placeholder="选择 TTS 模型"
            value={ttsModel || ttsModels[0]}
            onChange={setTtsModel}
            options={ttsModels.map((m) => ({ value: m, label: m }))}
          />
          <Button type="primary" className="ip-btn-primary" loading={busy} onClick={runTts}>
            生成配音
          </Button>
        </Space>
        <Input
          style={{ marginTop: 10 }}
          placeholder="配音结果 URL（可选回填）"
          value={draft.voiceUrl || ''}
          onChange={(e) => draft.patch({ voiceUrl: e.target.value })}
        />
        {draft.voiceTaskId && <p className="cf-muted">任务 ID：{draft.voiceTaskId}</p>}
      </section>

      <section className="cf-asset-block">
        <div className="cf-asset-title"><UserOutlined /> 数字人形象</div>
        <Space wrap style={{ width: '100%' }}>
          <Select
            style={{ minWidth: 200 }}
            placeholder="数字人模型"
            value={dhModel || dhModels[0]}
            onChange={setDhModel}
            options={dhModels.map((m) => ({ value: m, label: m }))}
          />
          <Button loading={busy} onClick={runAvatar}>提交口播成片</Button>
        </Space>
        <Input
          style={{ marginTop: 10 }}
          placeholder="形象图 URL"
          value={avatarImage}
          onChange={(e) => setAvatarImage(e.target.value)}
        />
        <Upload
          accept="image/*"
          showUploadList={false}
          beforeUpload={async (file) => {
            try {
              const { asset } = await businessApi.uploadAsset(file)
              setAvatarImage(asset.url)
              message.success('形象已上传')
            } catch {
              /* */
            }
            return false
          }}
        >
          <Button style={{ marginTop: 8 }} size="small">上传形象图</Button>
        </Upload>
        {draft.avatarTaskId && <p className="cf-muted">任务 ID：{draft.avatarTaskId}</p>}
      </section>

      <section className="cf-asset-block">
        <div className="cf-asset-title"><PictureOutlined /> 视频封面</div>
        <div className="ip-pick-grid" style={{ marginTop: 8 }}>
          {MOCK_COVERS.map((c) => (
            <button
              key={c.id}
              type="button"
              className={`ip-pick-card${draft.coverAccent === c.accent ? ' is-active' : ''}`}
              onClick={() => draft.patch({ coverAccent: c.accent })}
            >
              <span className="ip-pick-swatch" style={{ background: `linear-gradient(160deg,#111,${c.accent})` }} />
              <strong>{c.name}</strong>
            </button>
          ))}
        </div>
        <Space wrap style={{ marginTop: 10 }}>
          <Button loading={busy} onClick={genCover}>AI 生成封面</Button>
        </Space>
        <Input
          style={{ marginTop: 10 }}
          placeholder="封面图 URL"
          value={draft.coverUrl || ''}
          onChange={(e) => draft.patch({ coverUrl: e.target.value })}
        />
      </section>
    </div>
  )
}
