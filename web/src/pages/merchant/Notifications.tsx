import { useState } from 'react'
import { Typography, Card, Tag, Empty, Button, Space, List, message } from 'antd'
import { CheckCircleOutlined, ArrowRightOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { businessApi } from '../../api/business'

const { Text } = Typography

// 通知类型 → 标签 + 颜色
const TYPE_META: Record<string, { label: string; color: string }> = {
  mention_drop: { label: '提及率下降', color: 'error' },
  competitor_overtake: { label: '竞品反超', color: 'warning' },
  recheck_done: { label: '复测完成', color: 'blue' },
  scheduled_publish: { label: '排期发布', color: 'processing' },
  system: { label: '系统', color: 'default' },
}

export default function Notifications() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [filter, setFilter] = useState<string>('') // 空=全部 / unread=仅未读

  const { data: notifications = [] } = useQuery({
    queryKey: ['merchant-notifications-all'],
    queryFn: () => businessApi.listNotifications(),
  })

  const markReadMutation = useMutation({
    mutationFn: (id: string) => businessApi.markNotificationRead(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['merchant-notifications-all'] }),
  })

  const markAllRead = async () => {
    try {
      await businessApi.markNotificationRead('')
      message.success('已全部标记为已读')
      queryClient.invalidateQueries({ queryKey: ['merchant-notifications-all'] })
    } catch {}
  }

  const filtered = filter === 'unread' ? notifications.filter((n: any) => !n.read) : notifications
  const unreadCount = notifications.filter((n: any) => !n.read).length

  return (
    <div className="wr-page-content">
      <div className="wr-page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h1>通知中心</h1>
          <p>监测变化 · 复测结果 · 排期发布 · 系统通知{unreadCount > 0 && ` · ${unreadCount} 条未读`}</p>
        </div>
        <Space>
          <Button
            size="small"
            type={filter === 'unread' ? 'primary' : 'default'}
            onClick={() => setFilter(filter === 'unread' ? '' : 'unread')}
          >
            {filter === 'unread' ? '显示全部' : `仅未读 (${unreadCount})`}
          </Button>
          {unreadCount > 0 && (
            <Button size="small" icon={<CheckCircleOutlined />} onClick={markAllRead}>全部已读</Button>
          )}
        </Space>
      </div>

      <Card className="wr-glass-card" styles={{ body: { padding: 0 } }}>
        {filtered.length === 0 ? (
          <Empty style={{ padding: 60 }} description={filter === 'unread' ? '没有未读通知' : '暂无通知——监测/复测/排期发布的结果会出现在这里'} />
        ) : (
          <List
            dataSource={filtered}
            renderItem={(n: any) => {
              const meta = TYPE_META[n.type] || { label: n.type || '通知', color: 'default' }
              return (
                <List.Item
                  style={{
                    padding: '14px 20px',
                    cursor: n.link ? 'pointer' : 'default',
                    background: n.read ? 'transparent' : 'rgba(124,108,255,0.04)',
                    borderBottom: '1px solid var(--wr-border)',
                  }}
                  onClick={() => {
                    if (!n.read) markReadMutation.mutate(n.id)
                    if (n.link) navigate(n.link)
                  }}
                >
                  <Space align="start" style={{ width: '100%' }}>
                    <div style={{ flexShrink: 0, width: 8, height: 8, borderRadius: '50%', marginTop: 6,
                      background: n.read ? 'transparent' : 'var(--wr-primary)' }} />
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <Space size={8} style={{ marginBottom: 4 }}>
                        <Tag color={meta.color} style={{ fontSize: 10, margin: 0 }}>{meta.label}</Tag>
                        <Text strong style={{ fontSize: 14, color: n.read ? 'var(--wr-text-secondary)' : 'var(--wr-text-primary)' }}>
                          {n.title}
                        </Text>
                        {!n.read && <Tag color="processing" style={{ fontSize: 10, margin: 0 }}>未读</Tag>}
                      </Space>
                      <Text type="secondary" style={{ fontSize: 13, display: 'block', lineHeight: 1.6 }}>
                        {n.content}
                      </Text>
                      <Text type="secondary" style={{ fontSize: 11 }}>
                        {(n.created_at || '').slice(0, 16).replace('T', ' ')}
                      </Text>
                    </div>
                    {n.link && <ArrowRightOutlined style={{ color: 'var(--wr-text-muted)', fontSize: 12, marginTop: 6 }} />}
                  </Space>
                </List.Item>
              )
            }}
          />
        )}
      </Card>
    </div>
  )
}
