import { Drawer, Tag, Space, Typography } from 'antd'
import { scoreColor, scoreLevel } from '../utils/geo'
import type { OptimizedContent } from '../types/api'

const { Text } = Typography

// 内容详情统一预览抽屉（商户端历史卡片 / 管理端内容列表共用）。
//
// 设计动机：此前商户端历史卡片只有 2 行摘要、结果面板挤在半宽列——
// 长文章根本没法读；管理端却有完整 Drawer。统一为一个组件：
//   - 已发布 → iframe 渲染真实公开页（所见即爬虫所得）
//   - 草稿 → 全文视图（可读排版）+ AI 推荐度摘要
export default function ContentPreviewDrawer({
  content,
  onClose,
}: {
  content: OptimizedContent | null
  onClose: () => void
}) {
  const score = content?.score?.total ?? 0
  return (
    <Drawer
      title={content?.title || '内容详情'}
      width={720}
      open={!!content}
      onClose={onClose}
      destroyOnClose
    >
      {content && (
        <>
          <Space size={8} style={{ marginBottom: 16 }}>
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
              style={{ width: '100%', height: 'calc(100vh - 190px)', minHeight: 480, border: '1px solid var(--wr-border)', borderRadius: 10, background: '#fff' }}
            />
          ) : (
            <div
              style={{
                whiteSpace: 'pre-wrap', lineHeight: 1.9, fontSize: 14,
                color: 'var(--wr-text-secondary)', padding: '4px 8px',
                maxHeight: 'calc(100vh - 220px)', overflowY: 'auto',
              }}
            >
              {content.optimized_text}
            </div>
          )}
        </>
      )}
    </Drawer>
  )
}
