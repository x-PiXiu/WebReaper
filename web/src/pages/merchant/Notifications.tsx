import { Button, List, Space, Tag, Typography } from 'antd'
import { BellOutlined, CheckOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useMarkNotificationRead, useNotificationList, useUnreadCount } from '../../hooks/useNotifications'
import QueryBoundary from '../../components/QueryBoundary'

const { Text } = Typography

const TYPE_LABEL: Record<string, string> = {
  mention_drop: '提及率下降',
  competitor_overtake: '竞品反超',
  recheck_done: '复测完成',
  scheduled_publish: '排期发布',
  system: '系统通知',
}

/** 通知中心：全量列表 + 全部已读（与顶栏铃铛共享缓存） */
export default function Notifications() {
  const navigate = useNavigate()
  const { data: items = [], isLoading, isError, refetch } = useNotificationList()
  const { data: unread } = useUnreadCount()
  const markRead = useMarkNotificationRead()

  return (
    <QueryBoundary
      loading={isLoading}
      error={isError}
      onRetry={() => refetch()}
      empty={items.length === 0}
      emptyText="暂无通知"
    >
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">Inbox</p>
          <h1 style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <BellOutlined /> 通知中心
            {(unread?.unread || 0) > 0 && <Tag color="cyan">{unread?.unread} 未读</Tag>}
          </h1>
          <p className="ip-lead">提及率、复测与排期发布等主动提醒</p>
        </div>
        <Space>
          {(unread?.unread || 0) > 0 && (
            <Button icon={<CheckOutlined />} loading={markRead.isPending} onClick={() => markRead.mutate(undefined)}>
              全部已读
            </Button>
          )}
          <Button onClick={() => navigate('/m/compose')}>回工作台</Button>
        </Space>
      </div>

      <div className="ip-panel">
        <List
          itemLayout="vertical"
          dataSource={items}
            renderItem={(n) => (
              <List.Item
                key={n.id}
                style={{
                  cursor: n.link ? 'pointer' : 'default',
                  background: n.read ? 'transparent' : 'var(--wr-primary-bg)',
                  borderRadius: 10,
                  padding: '12px 16px',
                  marginBottom: 8,
                }}
                onClick={() => {
                  if (!n.read) markRead.mutate(n.id)
                  if (n.link) navigate(n.link)
                }}
                actions={[
                  !n.read ? (
                    <Button key="read" type="link" size="small" onClick={(e) => { e.stopPropagation(); markRead.mutate(n.id) }}>
                      标为已读
                    </Button>
                  ) : (
                    <Text key="done" type="secondary" style={{ fontSize: 12 }}>已读</Text>
                  ),
                ]}
              >
                <List.Item.Meta
                  title={
                    <Space size={8}>
                      <Tag style={{ margin: 0 }}>{TYPE_LABEL[n.type] || n.type}</Tag>
                      <Text strong>{n.title}</Text>
                    </Space>
                  }
                  description={
                    <>
                      <div style={{ marginTop: 4 }}>{n.content}</div>
                      <Text type="secondary" style={{ fontSize: 12 }}>{new Date(n.created_at).toLocaleString()}</Text>
                    </>
                  }
                />
              </List.Item>
            )}
        />
      </div>
    </div>
    </QueryBoundary>
  )
}
