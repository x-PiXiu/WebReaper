import { Button, Modal, Space, Tag, Typography } from 'antd'
import { PlayCircleOutlined, RocketOutlined, UserOutlined } from '@ant-design/icons'
import type { ViduSubject } from '../../utils/subjectTask'

const { Text, Title } = Typography

type Props = {
  open: boolean
  subject: ViduSubject | null
  onClose: () => void
  /** 跳转向导并预选该分身 */
  onUse: (serverId: string) => void
}

/**
 * 分身预览弹窗（23 §2.3）：形象视频（有则循环）/ 形象照 +「用此分身去创作」。
 * 链式 10s 形象视频上线前，无 videoUrl 时降级为形象照预览。
 */
export function SubjectPreviewModal({ open, subject, onClose, onUse }: Props) {
  if (!subject) return null
  const ready = subject.state === 'success' && !!subject.serverId
  const videoUrl = subject.videoUrl

  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={null}
      width={440}
      destroyOnHidden
      className="dh-preview-modal"
      title={null}
    >
      <div className="dh-preview">
        <div className="dh-preview-media">
          {videoUrl ? (
            <video src={videoUrl} autoPlay loop muted playsInline controls />
          ) : subject.portraitUrl ? (
            <img src={subject.portraitUrl} alt="" />
          ) : (
            <span className="dh-preview-placeholder"><UserOutlined /></span>
          )}
          {!videoUrl && (
            <span className="dh-preview-hint">
              <PlayCircleOutlined /> 形象视频生成接入后可在此预览 10s 循环
            </span>
          )}
        </div>
        <div className="dh-preview-body">
          <Space size={8} wrap>
            <Title level={4} style={{ margin: 0 }}>{subject.name}</Title>
            {subject.state === 'success' ? (
              <Tag color="success">{subject.hasVideo ? '可用 · 已有视频' : '可用'}</Tag>
            ) : subject.state === 'failed' ? (
              <Tag color="error">创建失败</Tag>
            ) : (
              <Tag color="processing">创建中</Tag>
            )}
          </Space>
          <Text type="secondary" style={{ fontSize: 13, display: 'block', marginTop: 6 }}>
            同一分身跨视频人物一致；用此分身去口播向导将自动预选
          </Text>
          <div className="dh-preview-actions">
            <Button onClick={onClose}>关闭</Button>
            <Button
              type="primary"
              icon={<RocketOutlined />}
              disabled={!ready}
              onClick={() => {
                if (!subject.serverId) return
                onUse(subject.serverId)
              }}
            >
              用此分身去创作
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  )
}
