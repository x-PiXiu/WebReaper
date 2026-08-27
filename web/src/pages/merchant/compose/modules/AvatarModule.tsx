import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Alert, Button, Collapse, Input, Select, Space, Typography, Upload } from 'antd'
import { RobotOutlined } from '@ant-design/icons'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import {
  buildSubjectReferencePayload,
  deliverableWorkParams,
  ensureMaterialId,
  submitUnified,
} from '../../../../api/generationSubmit'
import { useMediaAssets } from '../../../../hooks/useMediaAssets'
import { useSubjectList } from '../../../../hooks/useSubjectList'
import { useGenerationTypes } from '../../../../hooks/useGenerationTypes'
import { SubjectPicker } from '../../../../components/compose/SubjectPicker'
import { AssetPicker } from '../../../../components/compose/AssetPicker'
import { businessApi } from '../../../../api/business'
import { catchGenerationError } from '../../../../utils/generationErrors'
import { message } from '../../../../utils/antdApp'

const { Text } = Typography
const { TextArea } = Input

/** 口播数字人：主体一致性 reference2video（与 VideoAssetsStep / 拍口播向导同源） */
export default function AvatarModule() {
  const navigate = useNavigate()
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)
  const [subjectServerId, setSubjectServerId] = useState('')
  const [intent, setIntent] = useState('')
  const [legacyImageUrl, setLegacyImageUrl] = useState('')
  const [pickerOpen, setPickerOpen] = useState(false)
  const text = draft.rewritten || draft.script || ''

  const { isEnabled } = useGenerationTypes()
  const { ready: subjects } = useSubjectList()

  useEffect(() => {
    draft.setTrack('video')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const { data: assets = [] } = useMediaAssets()
  const imageAssets = useMemo(
    () => assets.filter((a) => (a.mime || '').startsWith('image/')),
    [assets],
  )

  const selectedSubject = useMemo(
    () => subjects.find((s) => s.serverId === subjectServerId),
    [subjects, subjectServerId],
  )

  const submitSubject = async () => {
    if (!text.trim()) {
      message.warning('请先准备口播文案')
      return
    }
    const bid = brandId || draft.brandId
    if (!bid) {
      message.warning('请先选择人设/品牌')
      return
    }
    if (!subjectServerId) {
      message.warning('请选择数字分身')
      return
    }
    if (!isEnabled('reference2video')) {
      message.warning('参考生视频未在后台启用')
      return
    }
    setBusy(true)
    try {
      let audioMaterialId: string | undefined
      if (draft.voiceUrl) {
        audioMaterialId = await ensureMaterialId(draft.voiceUrl)
      }
      const scriptText = [intent.trim(), text.trim()].filter(Boolean).join('，')
      const task = await submitUnified(buildSubjectReferencePayload({
        brand_id: bid,
        server_id: subjectServerId,
        name: selectedSubject?.name,
        text: scriptText,
        audioMaterialId,
      }))
      draft.patch({ avatarTaskId: task.id, brandId: bid })
      message.success('主体一致性口播已提交')
      navigate('/m/compose/tools?tab=media')
    } catch (e) {
      catchGenerationError(e)
    } finally {
      setBusy(false)
    }
  }

  const submitLegacy = async () => {
    if (!text.trim()) {
      message.warning('请先准备口播文案')
      return
    }
    const bid = brandId || draft.brandId
    if (!bid) {
      message.warning('请先选择人设/品牌')
      return
    }
    const imageRef = legacyImageUrl || imageAssets[0]?.url
    if (!imageRef) {
      message.warning('请选择人像参考图')
      return
    }
    if (!draft.voiceUrl) {
      message.warning('降级路径需要配音——请先在「爆款配音」生成')
      return
    }
    setBusy(true)
    try {
      const materials = [await ensureMaterialId(imageRef), await ensureMaterialId(draft.voiceUrl)]
      const task = await submitUnified({
        brand_id: bid,
        materials,
        text: text.slice(0, 2000),
        params: deliverableWorkParams(),
      })
      draft.patch({ avatarTaskId: task.id, brandId: bid })
      message.success('口播任务已提交（图+音频，降级路径）')
      navigate('/m/compose/tools?tab=media')
    } catch (e) {
      catchGenerationError(e)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="wr-page-content ip-page">
      <ComposeModuleHeader
        title="口播数字人"
        lead="数字分身 + 文案 → 主体一致性口播成片"
        badge="发视频"
      />
      <Alert
        style={{ marginBottom: 16 }}
        type="info"
        showIcon
        message="完整五步流程（真人出镜 / 音色 / 分段）请使用「拍口播」向导"
        action={
          <Button size="small" type="primary" onClick={() => navigate('/m/compose/lipsync')}>
            打开拍口播
          </Button>
        }
      />
      {!draft.voiceUrl && (
        <Alert
          style={{ marginBottom: 16 }}
          type="warning"
          showIcon
          message="可选：先在「爆款配音」生成配音，成片将附带音频轨道"
        />
      )}
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <Text strong>口播文案</Text>
        <TextArea
          rows={6}
          style={{ marginTop: 8, marginBottom: 16 }}
          value={text}
          onChange={(e) => draft.patch({ script: e.target.value })}
        />
        <Text strong style={{ display: 'block', marginBottom: 8 }}>选择数字分身</Text>
        <SubjectPicker
          subjects={subjects}
          value={subjectServerId}
          onChange={setSubjectServerId}
          className="wz-subject-picks--block"
        />
        <Input
          placeholder="场景意图（可选，如：在厨房边做菜边对镜头讲解）"
          value={intent}
          onChange={(e) => setIntent(e.target.value)}
          maxLength={200}
          style={{ marginTop: 12, marginBottom: 16 }}
        />
        <Button
          type="primary"
          loading={busy}
          icon={<RobotOutlined />}
          disabled={!isEnabled('reference2video') || !subjectServerId}
          onClick={submitSubject}
        >
          提交主体一致性成片
        </Button>
        <Collapse
          ghost
          style={{ marginTop: 20 }}
          items={[{
            key: 'legacy',
            label: '降级：人像图 + 配音（无分身时）',
            children: (
              <>
                <Text strong>人像参考图（素材库 ID）</Text>
                <Select
                  style={{ display: 'block', marginTop: 8, marginBottom: 12, maxWidth: 480 }}
                  placeholder={imageAssets.length ? '从素材库选择' : '请先在多媒体工作台上传人像'}
                  value={legacyImageUrl || undefined}
                  onChange={setLegacyImageUrl}
                  options={imageAssets.map((a) => ({
                    value: a.url,
                    label: `${a.id.slice(0, 8)}… · ${a.mime || 'image'}`,
                  }))}
                />
                <Space wrap>
                  <Upload
                    accept="image/*"
                    showUploadList={false}
                    beforeUpload={async (file) => {
                      try {
                        const asset = await businessApi.uploadAsset(file)
                        setLegacyImageUrl(asset.url)
                        message.success('形象已上传')
                      } catch (e) {
                        catchGenerationError(e)
                      }
                      return false
                    }}
                  >
                    <Button size="small">上传形象</Button>
                  </Upload>
                  <Button size="small" onClick={() => setPickerOpen(true)}>素材库</Button>
                </Space>
                <Button loading={busy} style={{ marginTop: 12 }} onClick={submitLegacy}>
                  提交降级口播
                </Button>
              </>
            ),
          }]}
        />
      </div>
      <AssetPicker
        open={pickerOpen}
        onClose={() => setPickerOpen(false)}
        kind="image"
        title="选择数字人形象图"
        onPick={(url) => setLegacyImageUrl(url)}
      />
    </div>
  )
}
