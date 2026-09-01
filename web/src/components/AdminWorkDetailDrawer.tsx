import { useState } from 'react'
import { Typography, Space, Tag, Image, Descriptions, Button, Popconfirm, Drawer } from 'antd'
import { StopOutlined, UndoOutlined, RobotOutlined, UserOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import { toast } from '../utils/feedback'

const { Text, Paragraph } = Typography

/**
 * 管理端作品详情抽屉（32号 F2）：巡查流"看内容"能力——
 * 媒体本体预览（审核判定核心证据）+ 生成文案 + 处置/申诉完整记录 + 快速处置。
 */
export default function AdminWorkDetailDrawer({ workKey, onClose }: { workKey: string | null; onClose: () => void }) {
  const queryClient = useQueryClient()
  const [acting, setActing] = useState(false)

  const { data: d, isLoading } = useQuery({
    queryKey: ['admin-work-detail', workKey],
    queryFn: () => businessApi.adminWorkDetail(workKey!),
    enabled: !!workKey,
  })

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin-works'] })

  const hide = async (action: 'hidden' | 'deleted') => {
    if (!d) return
    setActing(true)
    try {
      await businessApi.adminHideWork(d.id, {
        kind: d.kind, tenant_id: d.tenant_id, action,
        reason: d.moderation_reason || (action === 'deleted' ? '详情页升级处置' : '巡查下架'),
      })
      toast.ok(action === 'deleted' ? '已删除（不可见且不可发布）' : '已下架')
      refresh()
      onClose()
    } catch { /* 拦截器已提示 */ } finally {
      setActing(false)
    }
  }

  const restore = async () => {
    if (!workKey) return
    setActing(true)
    try {
      await businessApi.adminRestoreWork(workKey)
      toast.ok('已恢复/放行')
      refresh()
      onClose()
    } catch { /* 拦截器已提示 */ } finally {
      setActing(false)
    }
  }

  const media = d?.media_urls?.[0] || ''

  return (
    <Drawer title={`作品详情 · ${workKey || ''}`} open={!!workKey} onClose={onClose} width={560} destroyOnClose>
      {isLoading || !d ? <Text type="secondary">加载中…</Text> : (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <div style={{ textAlign: 'center', background: '#000', borderRadius: 8, padding: 8 }}>
            {d.kind === 'video' && media && <video src={media} controls style={{ width: '100%', maxHeight: 320 }} />}
            {d.kind === 'image' && media && <Image src={media} style={{ maxHeight: 320 }} />}
            {d.kind === 'audio' && media && <audio src={media} controls style={{ width: '100%' }} />}
            {!media && <Text type="secondary">无媒体产物</Text>}
          </div>

          {d.text && (
            <div>
              <Text type="secondary">生成文案（机审/人工审核判定对象）</Text>
              <Paragraph style={{ background: '#fafafa', padding: 12, borderRadius: 6, maxHeight: 160, overflow: 'auto', whiteSpace: 'pre-wrap', marginBottom: 0 }}>
                {d.text}
              </Paragraph>
            </div>
          )}

          <Descriptions column={2} size="small" bordered>
            <Descriptions.Item label="类型" span={1}>{d.kind}</Descriptions.Item>
            <Descriptions.Item label="端点" span={1}>{d.sub_type}</Descriptions.Item>
            <Descriptions.Item label="归属租户" span={2}>{d.tenant_id}</Descriptions.Item>
            <Descriptions.Item label="模型" span={1}>{d.model || '-'}</Descriptions.Item>
            <Descriptions.Item label="厂商" span={1}>{d.provider || '-'}</Descriptions.Item>
            {d.voice_id && <Descriptions.Item label="音色" span={2}>{d.voice_id}</Descriptions.Item>}
            <Descriptions.Item label="创建时间" span={2}>{new Date(d.created_at).toLocaleString('zh-CN', { hour12: false })}</Descriptions.Item>
          </Descriptions>

          {(d.moderation_action || d.appeal_status) && (
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="当前状态">
                {d.moderation_action === 'flagged' ? <Tag color="orange">机审待复核</Tag>
                  : d.moderation_action === 'deleted' ? <Tag color="error">已删除</Tag>
                  : d.moderation_action === 'hidden' ? <Tag color="warning">已下架</Tag>
                  : <Tag>正常</Tag>}
              </Descriptions.Item>
              {d.moderation_reason && <Descriptions.Item label="处置原因">{d.moderation_reason}</Descriptions.Item>}
              {d.moderation_source && (
                <Descriptions.Item label="处置来源">
                  <Space>
                    {d.moderation_source === 'machine' ? <RobotOutlined /> : <UserOutlined />}
                    {d.moderation_source === 'machine' ? '机审自动' : `人工（${d.moderation_operator || 'admin'}）`}
                  </Space>
                </Descriptions.Item>
              )}
              {d.appeal_status && d.appeal_status !== 'none' && (
                <>
                  <Descriptions.Item label="申诉状态">
                    {d.appeal_status === 'pending' ? <Tag color="processing">申诉中</Tag>
                      : d.appeal_status === 'rejected' ? <Tag color="error">已维持</Tag>
                      : <Tag color="success">已采纳</Tag>}
                  </Descriptions.Item>
                  {d.appeal_text && <Descriptions.Item label="申诉理由">{d.appeal_text}</Descriptions.Item>}
                </>
              )}
            </Descriptions>
          )}

          <Space>
            {d.moderation_action === 'flagged' && (
              <Popconfirm title="确认放行？（清除机审标记，作品恢复正常）" onConfirm={restore}>
                <Button icon={<UndoOutlined />} type="primary" ghost>放行</Button>
              </Popconfirm>
            )}
            {d.moderation_action === 'hidden' && (
              <Popconfirm title="确认恢复该作品？" onConfirm={restore}>
                <Button icon={<UndoOutlined />} loading={acting}>恢复</Button>
              </Popconfirm>
            )}
            {(d.moderation_action === 'hidden' || d.moderation_action === 'flagged') && (
              <Popconfirm title={`确认升级为删除？（不可见且不可发布）`} onConfirm={() => hide('deleted')}>
                <Button danger icon={<StopOutlined />} loading={acting}>升级为删除</Button>
              </Popconfirm>
            )}
            {!d.moderation_action && (
              <Popconfirm title="确认下架该作品？（用户端即刻不可见）" onConfirm={() => hide('hidden')}>
                <Button danger icon={<StopOutlined />} loading={acting}>下架</Button>
              </Popconfirm>
            )}
          </Space>
        </Space>
      )}
    </Drawer>
  )
}
