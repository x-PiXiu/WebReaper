import { Modal, Tag, Space, Typography } from 'antd'
import { scoreColor, scoreLevel } from '../utils/geo'
import type { OptimizedContent } from '../types/api'
import { MODAL_W, modalBodyScroll } from '../ui/modalFit'

const { Text } = Typography

/** 内容详情统一预览弹窗（商户端历史卡片 / 管理端内容列表共用）。 */
export default function ContentPreviewDrawer({
  content,
  onClose,
}: {
  content: OptimizedContent | null
  onClose: () => void
}) {
  const score = content?.score?.total ?? 0
  return (
    <Modal
      title={content?.title || '内容详情'}
      width={MODAL_W.xxl}
      open={!!content}
      onCancel={onClose}
      destroyOnHidden
      footer={null}
      className="wr-modal-preview"
      styles={{ body: modalBodyScroll.body }}
    >
      {content && (
        <>
          <Space size={8} style={{ marginBottom: 16 }} wrap>
            <Tag color={scoreColor(score)} style={{ fontWeight: 600 }}>AI 推荐度 {score.toFixed(0)}</Tag>
            <Text type="secondary" style={{ fontSize: 12 }}>{scoreLevel(score)}</Text>
            {content.status === 'published' ? (
              <Tag color="success">已发布</Tag>
            ) : (
              <Tag>草稿</Tag>
            )}
            {content.status === 'published' && content.index_status === 'indexed' && <Tag color="green">已收录</Tag>}
            {content.status === 'published' && content.index_status === 'pending' && <Tag color="warning">待收录</Tag>}
            <Text type="secondary" style={{ fontSize: 12 }}>v{content.version}</Text>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {content.created_at ? new Date(content.created_at).toLocaleString() : ''}
            </Text>
          </Space>

          {content.status === 'published' ? (
            <iframe
              src={`/public/articles/${content.id}`}
              title="公开页预览"
              style={{ width: '100%', height: 'min(62vh, 560px)', minHeight: 360, border: '1px solid var(--wr-border)', borderRadius: 10, background: '#fff' }}
            />
          ) : (
            <div
              style={{
                whiteSpace: 'pre-wrap', lineHeight: 1.9, fontSize: 14,
                color: 'var(--wr-text-secondary)', padding: '4px 8px',
                maxHeight: 'min(58vh, 520px)', overflowY: 'auto',
              }}
            >
              {content.optimized_text}
            </div>
          )}
        </>
      )}
    </Modal>
  )
}
