import { useState } from 'react'
import { Typography, Tag, Empty, Button, Space, List } from 'antd'
import { CheckCircleOutlined, ArrowRightOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useNotificationList, useMarkNotificationRead } from '../../hooks/useNotifications'

const { Text } = Typography

const TYPE_META: Record<string, { label: string; color: string }> = {
  mention_drop: { label: '表现下降', color: 'error' },
  competitor_overtake: { label: '竞品领先', color: 'warning' },
  recheck_done: { label: '复测完成', color: 'blue' },
  scheduled_publish: { label: '排期发布', color: 'processing' },
  content_indexed: { label: '内容已收录', color: 'success' },
  system: { label: '系统', color: 'default' },
}

export default function Notifications() {
  const navigate = useNavigate()
  const [filter, setFilter] = useState<string>('')

  const { data: notifications = [] } = useNotificationList()
  const markRead = useMarkNotificationRead()

  const markAllRead = () => markRead.mutate(undefined)

  const filtered = filter === 'unread' ? notifications.filter((n: { read?: boolean }) => !n.read) : notifications
  const unreadCount = notifications.filter((n: { read?: boolean }) => !n.read).length

  return (
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero" style={{ alignItems: 'flex-end' }}>
        <div>
          <p className="ip-kicker">Inbox</p>
          <h1>通知中心</h1>
          <p className="ip-lead">
            发布结果 · 表现变化 · 系统提醒
            {unreadCount > 0 ? ` · ${unreadCount} 条未读` : ''}
          </p>
        </div>
        <Space>
          <Button
            size="small"
            type={filter === 'unread' ? 'primary' : 'default'}
            className={filter === 'unread' ? 'ip-btn-primary' : undefined}
            onClick={() => setFilter(filter === 'unread' ? '' : 'unread')}
          >
            {filter === 'unread' ? '显示全部' : `仅未读 (${unreadCount})`}
          </Button>
          {unreadCount > 0 && (
            <Button size="small" icon={<CheckCircleOutlined />} onClick={markAllRead}>全部已读</Button>
          )}
        </Space>
      </div>

      <div className="ip-panel" style={{ padding: 0, overflow: 'hidden' }}>
        {filtered.length === 0 ? (
          <Empty
            style={{ padding: 60 }}
            description={filter === 'unread' ? '没有未读通知' : '暂无通知——发布、复测与系统消息会出现在这里'}
          />
        ) : (
          <List
            dataSource={filtered}
            renderItem={(n: {
              id: string
              type?: string
              title?: string
              body?: string
              read?: boolean
              link?: string
              created_at?: string
            }) => {
              const meta = TYPE_META[n.type || ''] || { label: n.type || '通知', color: 'default' }
              return (
                <List.Item
                  style={{
                    padding: '14px 20px',
                    cursor: n.link ? 'pointer' : 'default',
                    background: n.read ? 'transparent' : 'var(--wr-primary-bg)',
                    borderBottom: '1px solid var(--wr-border)',
                    transition: 'background 180ms ease',
                  }}
                  onClick={() => {
                    if (!n.read) markRead.mutate(n.id)
                    if (n.link) {
                      // 旧链接兼容到新 IA
                      const link = n.link
                        .replace('/m/checkup', '/m/analytics')
                        .replace('/m/studio', '/m/compose')
                        .replace('/m/visibility', '/m/analytics')
                      navigate(link)
                    }
                  }}
                >
                  <Space align="start" style={{ width: '100%' }}>
                    <div style={{
                      flexShrink: 0, width: 8, height: 8, borderRadius: '50%', marginTop: 6,
                      background: n.read ? 'transparent' : 'var(--wr-primary)',
                    }}
                    />
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <Space size={8} style={{ marginBottom: 4 }}>
                        <Tag color={meta.color} style={{ fontSize: 10, margin: 0 }}>{meta.label}</Tag>
                        <Text strong style={{ fontSize: 14, color: n.read ? 'var(--wr-text-secondary)' : 'var(--wr-text-primary)' }}>
                          {n.title}
                        </Text>
                      </Space>
                      {n.body && (
                        <Text type="secondary" style={{ fontSize: 13, display: 'block' }}>{n.body}</Text>
                      )}
                      <Text type="secondary" style={{ fontSize: 11 }}>
                        {n.created_at ? new Date(n.created_at).toLocaleString('zh-CN', { hour12: false }) : ''}
                      </Text>
                    </div>
                    {n.link && <ArrowRightOutlined style={{ color: 'var(--wr-text-muted)', fontSize: 12 }} />}
                  </Space>
                </List.Item>
              )
            }}
          />
        )}
      </div>
    </div>
  )
}
