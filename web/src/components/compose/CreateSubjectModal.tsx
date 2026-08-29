import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Button, Input, Modal, Segmented, Space, Typography, Upload } from 'antd'
import { PlusOutlined, UserOutlined, VideoCameraOutlined } from '@ant-design/icons'
import { businessApi } from '../../api/business'
import { buildSubjectRegisterPayload } from '../../api/generationSubmit'
import { useBrandContext } from '../../hooks/useBrands'
import { GENERATION_TASKS_KEY } from '../../hooks/useGenerationTasks'
import { MODAL_W } from '../../ui/modalFit'
import { message } from '../../utils/antdApp'
import VoicePicker from '../VoicePicker'

const { Text } = Typography

/** 客户端预检主体视频时长（≤5s；元数据读不出的容器放行，由上游兜底校验） */
function checkVideoDuration(file: File): Promise<void> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file)
    const v = document.createElement('video')
    v.preload = 'metadata'
    v.onloadedmetadata = () => {
      URL.revokeObjectURL(url)
      if (v.duration > 5.5) {
        message.error('主体视频不能超过 5 秒')
        reject(new Error('视频超过 5 秒'))
      } else {
        resolve()
      }
    }
    v.onerror = () => { URL.revokeObjectURL(url); resolve() }
    v.src = url
  })
}

type Props = {
  open: boolean
  voices: string[]
  onClose: () => void
  onCreated: () => void
  title?: string
}

/** 创建 Vidu 主体（人物分身 / 场景） */
export function CreateSubjectModal({
  open,
  voices,
  onClose,
  onCreated,
  title = '定制数字人',
}: Props) {
  const [name, setName] = useState('')
  const [kind, setKind] = useState<'person' | 'scene'>('person')
  const [imageAssets, setImageAssets] = useState<Array<{ id: string; url: string }>>([])
  const [videoAsset, setVideoAsset] = useState<{ id: string; url: string } | null>(null)
  const [voiceId, setVoiceId] = useState('')
  const [creating, setCreating] = useState(false)
  const { brandId } = useBrandContext()
  const queryClient = useQueryClient()

  const resetForm = () => {
    setName('')
    setKind('person')
    setImageAssets([])
    setVideoAsset(null)
    setVoiceId('')
  }

  const handleCreate = async () => {
    if (!name.trim()) {
      message.warning(kind === 'scene' ? '请输入场景名称' : '请输入数字人名称')
      return
    }
    if (!brandId) {
      message.warning('请先在顶栏或人设页选择品牌/人设')
      return
    }
    if (imageAssets.length === 0 && !videoAsset) {
      message.warning(kind === 'scene' ? '请上传 1-3 张场景照片' : '请至少上传 1 张形象照或 1 个主体视频')
      return
    }
    setCreating(true)
    try {
      const task = await businessApi.submitGeneration(buildSubjectRegisterPayload({
        brand_id: brandId,
        name: name.trim(),
        imageMaterialIds: imageAssets.map((a) => a.id),
        imageUrls: imageAssets.map((a) => a.url),
        videoUrl: videoAsset?.url,
        voice_id: voiceId || undefined,
      }))
      message.success(`数字分身「${name.trim()}」已创建（任务 ${task.id}）——生成视频时可直接复用该形象`)
      queryClient.invalidateQueries({ queryKey: GENERATION_TASKS_KEY })
      resetForm()
      onCreated()
      onClose()
    } catch { /* 拦截器已提示 */ } finally {
      setCreating(false)
    }
  }

  return (
    <Modal
      open={open}
      title={title}
      okText="创建"
      cancelText="取消"
      onOk={handleCreate}
      onCancel={() => { resetForm(); onClose() }}
      confirmLoading={creating}
      width={MODAL_W.md}
    >
      <Space direction="vertical" style={{ width: '100%' }} size={12}>
        <Segmented
          value={kind}
          onChange={(v) => setKind(v as 'person' | 'scene')}
          options={[
            { value: 'person', label: '人物分身', icon: <UserOutlined /> },
            { value: 'scene', label: '场景主体', icon: <VideoCameraOutlined /> },
          ]}
        />
        <Text type="secondary" style={{ fontSize: 12 }}>
          {kind === 'person'
            ? '上传 1-3 张形象照或 1 个 5 秒内的主体视频——创建后可生成口播/参考生视频'
            : '上传 2-3 张场景照片（厨房/门店/工作室）——生成视频时场景可复用，画面一致'}
        </Text>
        <Input
          placeholder={kind === 'scene' ? '场景名称（如：主厨房、门店前台）' : '数字人名称（如：张师傅、李老板）'}
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={64}
        />
        <div>
          <Text strong style={{ fontSize: 13 }}>{kind === 'scene' ? '场景照片（1-3 张）' : '形象照（1-3 张）'}</Text>
          <Upload
            listType="picture-card"
            maxCount={3}
            accept="image/png,image/jpeg,image/jpg,image/webp"
            customRequest={async ({ file, onSuccess, onError }) => {
              try {
                const r = await businessApi.uploadAsset(file as File)
                setImageAssets((prev) => [...prev, { id: r.id, url: r.url }])
                onSuccess?.(r)
              } catch (e) { onError?.(e as Error) }
            }}
            onRemove={(file) => {
              const id = (file.response as { id?: string } | undefined)?.id
              const url = (file.response as { url?: string } | undefined)?.url
              if (id || url) {
                setImageAssets((prev) => prev.filter((a) => a.id !== id && a.url !== url))
              }
            }}
          >
            {imageAssets.length < 3 && (
              <div>
                <PlusOutlined />
                <div style={{ fontSize: 12, marginTop: 4 }}>上传形象照</div>
              </div>
            )}
          </Upload>
        </div>
        {kind === 'person' && (
          <div>
            <Text strong style={{ fontSize: 13 }}>主体视频（可选，1 个 ≤5 秒）</Text>
            <Upload
              maxCount={1}
              accept="video/mp4,video/x-msvideo,video/quicktime"
              beforeUpload={checkVideoDuration}
              customRequest={async ({ file, onSuccess, onError }) => {
                try {
                  const r = await businessApi.uploadAsset(file as File)
                  setVideoAsset({ id: r.id, url: r.url })
                  onSuccess?.(r)
                } catch (e) { onError?.(e as Error) }
              }}
              onRemove={() => setVideoAsset(null)}
            >
              <Button icon={<VideoCameraOutlined />}>{videoAsset ? '重新上传' : '上传视频（mp4/avi/mov）'}</Button>
            </Upload>
          </div>
        )}
        {kind === 'person' && (
          <div>
            <Text strong style={{ fontSize: 13 }}>绑定音色（可选）</Text>
            <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>
              音视频直出时使用；q2-pro 及 q3 系列模型不支持音色
            </Text>
            <div style={{ marginTop: 4 }}>
              <VoicePicker value={voiceId} onChange={setVoiceId} myVoices={voices} />
            </div>
            <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
              官方音色可试听后选择；想要自己的声音？
              <a href="/m/compose/tools?tab=media" target="_blank" rel="noreferrer">去声音克隆</a>
              （复刻音色 7 天内在语音合成中调用一次即永久保留）
            </Text>
          </div>
        )}
      </Space>
    </Modal>
  )
}
