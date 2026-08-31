import { Modal, Space } from 'antd'
import { VideoCameraAddOutlined } from '@ant-design/icons'
import { BrollPanel, type BrollSource } from './BrollPanel'

export type { BrollSource }

/**
 * B-Roll 抽屉（兼容向导内快速打开）；完整体验见作品详情页 `/m/works/:id`。
 */
export default function BrollDrawer({ open, onClose, source }: {
  open: boolean
  onClose: () => void
  source: BrollSource | null
}) {
  if (!source) return null

  return (
    <Modal
      open={open}
      onCancel={onClose}
      width="min(1100px, 96vw)"
      title={<Space><VideoCameraAddOutlined /> 作品详情 · 插入画面 · {source.title || '口播成片'}</Space>}
      footer={null}
      destroyOnHidden
      className="wr-broll-modal"
      styles={{ body: { padding: 0, maxHeight: 'min(82vh, calc(100vh - 120px))', overflow: 'hidden' } }}
    >
      <BrollPanel source={source} variant="embedded" onClose={onClose} />
    </Modal>
  )
}
