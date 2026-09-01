import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Alert, Button, Input, Modal, Space, Typography, Upload } from 'antd'
import { PlusOutlined, VideoCameraOutlined } from '@ant-design/icons'
import { businessApi } from '../../api/business'
import { buildSubjectRegisterPayload } from '../../api/generationSubmit'
import { subjectServerId } from '../../utils/subjectTask'
import { useBrandContext } from '../../hooks/useBrands'
import { GENERATION_TASKS_KEY } from '../../hooks/useGenerationTasks'
import { MODAL_W } from '../../ui/modalFit'
import { toast } from '../../utils/feedback'
import VoicePicker from '../VoicePicker'

const { Text } = Typography
const { TextArea } = Input

/** 客户端预检主体视频时长（≤5s；元数据读不出的容器放行，由上游兜底校验） */
function checkVideoDuration(file: File): Promise<void> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file)
    const v = document.createElement('video')
    v.preload = 'metadata'
    v.onloadedmetadata = () => {
      URL.revokeObjectURL(url)
      if (v.duration > 5.5) {
        toast.fail('主体视频请控制在 5 秒以内', 'subject-dur')
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
  /** 创建成功回调；带新分身 server_id（可能为空——注册响应缺失时由列表刷新兜底） */
  onCreated: (serverId?: string) => void
  title?: string
}

/** 创建数字分身主体（23 号计划 §2.1：形象照 + 名称 + 可选场景图/场景描述 + 可选音色绑定） */
export function CreateSubjectModal({
  open,
  voices,
  onClose,
  onCreated,
  title = '定制数字人',
}: Props) {
  const [name, setName] = useState('')
  const [imageAssets, setImageAssets] = useState<Array<{ id: string; url: string }>>([])
  const [videoAsset, setVideoAsset] = useState<{ id: string; url: string } | null>(null)
  const [sceneImage, setSceneImage] = useState<{ id: string; url: string } | null>(null)
  const [sceneDesc, setSceneDesc] = useState('')
  const [voiceId, setVoiceId] = useState('')
  const [creating, setCreating] = useState(false)
  const { brandId } = useBrandContext()
  const queryClient = useQueryClient()

  const resetForm = () => {
    setName('')
    setImageAssets([])
    setVideoAsset(null)
    setSceneImage(null)
    setSceneDesc('')
    setVoiceId('')
  }

  const handleCreate = async () => {
    if (!name.trim()) {
      toast.warn('请输入数字人名称', 'subject-create')
      return
    }
    if (!brandId) {
      toast.warn('请先选择人设', 'subject-create')
      return
    }
    if (imageAssets.length === 0 && !videoAsset) {
      toast.warn('请至少上传 1 张形象照或 1 段主体视频', 'subject-create')
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
        sceneImageUrl: sceneImage?.url,
        sceneDescription: sceneDesc || undefined,
      }))
      toast.ok(`「${name.trim()}」已开始创建，完成后可直接用于口播`, 'subject-create')
      queryClient.invalidateQueries({ queryKey: GENERATION_TASKS_KEY })
      resetForm()
      onCreated(subjectServerId(task) || undefined)
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
        <Text type="secondary" style={{ fontSize: 12 }}>
          上传 1-3 张形象照或 1 个 5 秒内的主体视频——创建后即可用于口播成片，跨视频人物形象一致
        </Text>
        <Input
          placeholder="数字人名称（如：张师傅、李老板）"
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={64}
        />
        <div>
          <Text strong style={{ fontSize: 13 }}>形象照（1-3 张）</Text>
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
        <div>
          <Text strong style={{ fontSize: 13 }}>场景图（可选，1 张）</Text>
          <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>
            让分身出现在期望的环境中（如厨房切菜）；不上传则默认纯色棚拍
          </Text>
          {sceneImage ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginTop: 6 }}>
              <img
                src={sceneImage.url}
                alt="场景图"
                style={{ width: 64, height: 64, objectFit: 'cover', borderRadius: 8 }}
              />
              <Button size="small" onClick={() => setSceneImage(null)}>移除</Button>
            </div>
          ) : (
            <Upload
              maxCount={1}
              accept="image/png,image/jpeg,image/jpg,image/webp"
              showUploadList={false}
              customRequest={async ({ file, onSuccess, onError }) => {
                try {
                  const r = await businessApi.uploadAsset(file as File)
                  setSceneImage({ id: r.id, url: r.url })
                  onSuccess?.(r)
                } catch (e) { onError?.(e as Error) }
              }}
            >
              <Button icon={<PlusOutlined />}>上传场景图</Button>
            </Upload>
          )}
          <Alert
            type="warning"
            showIcon
            style={{ marginTop: 8 }}
            message="场景图必须露出主角脸部；不可有分镜/多画面内容"
          />
        </div>
        <div>
          <Text strong style={{ fontSize: 13 }}>场景描述（可选，一句话）</Text>
          <TextArea
            placeholder="主角在哪个场景做什么（如：厨师系着围裙在明亮厨房切菜）"
            value={sceneDesc}
            onChange={(e) => setSceneDesc(e.target.value)}
            maxLength={120}
            showCount
            autoSize={{ minRows: 2, maxRows: 3 }}
            style={{ marginTop: 4 }}
          />
        </div>
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
      </Space>
    </Modal>
  )
}
