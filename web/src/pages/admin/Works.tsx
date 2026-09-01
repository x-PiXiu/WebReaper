import { Typography, Table, Tag, Space, Button, Popconfirm, Modal, Input, Radio, Image, Tooltip } from 'antd'
import { StopOutlined, UndoOutlined, VideoCameraOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { AdminWorkItem } from '../../types/api'
import { toast } from '../../utils/feedback'

const { Text } = Typography

const kindMeta: Record<string, { color: string; label: string }> = {
  video: { color: 'processing', label: '视频' },
  image: { color: 'cyan', label: '图片' },
  audio: { color: 'purple', label: '音频' },
}

const actionMeta: Record<string, { color: string; label: string }> = {
  hidden: { color: 'warning', label: '已下架' },
  deleted: { color: 'error', label: '已删除' },
}

/**
 * 作品管理 Tab（32号：内容安全合规）——跨租户作品巡查流。
 * 最新成片倒序 + 处置状态；下架（原因必填，审计）/恢复。
 * 处置后：用户端作品库即刻不可见 + 发布拦截（防扩散，服务端双端点保障）。
 */
export default function AdminWorks({ embedded = false }: { embedded?: boolean }) {
  const queryClient = useQueryClient()
  const [hideTarget, setHideTarget] = useState<AdminWorkItem | null>(null)
  const [reason, setReason] = useState('')
  const [action, setAction] = useState<'hidden' | 'deleted'>('hidden')
  const [acting, setActing] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['admin-works'],
    queryFn: () => businessApi.adminListWorks(100),
  })
  const items = data?.items ?? []

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin-works'] })

  const submitHide = async () => {
    if (!hideTarget) return
    if (!reason.trim()) {
      toast.warn('请填写处置原因（审计必填）')
      return
    }
    setActing(true)
    try {
      await businessApi.adminHideWork(hideTarget.id, {
        kind: hideTarget.kind,
        tenant_id: hideTarget.tenant_id,
        action,
        reason: reason.trim(),
      })
      toast.ok(action === 'deleted' ? '已删除（用户不可见且不可再发布）' : '已下架（用户端即刻不可见）')
      setHideTarget(null)
      setReason('')
      refresh()
    } catch { /* 拦截器已提示 */ } finally {
      setActing(false)
    }
  }

  const restore = async (w: AdminWorkItem) => {
    try {
      await businessApi.adminRestoreWork(w.id)
      toast.ok('已恢复')
      refresh()
    } catch { /* 拦截器已提示 */ }
  }

  return (
    <div style={embedded ? undefined : { padding: 24 }}>
      <Space style={{ marginBottom: 12 }}>
        <Text type="secondary">
          最新成片巡查流（{items.length} 条）——下架后用户端作品库即刻不可见且无法发布；处置记录含操作者与原因（审计）
        </Text>
      </Space>
      <Table
        rowKey="id"
        loading={isLoading}
        dataSource={items}
        size="middle"
        pagination={{ pageSize: 10, showSizeChanger: false }}
        locale={{ emptyText: '暂无成片作品' }}
        columns={[
          {
            title: '作品',
            key: 'work',
            width: 320,
            render: (_, w: AdminWorkItem) => (
              <Space>
                {w.cover_url || w.media_urls?.[0]
                  ? <Image src={w.cover_url || w.media_urls?.[0]} width={64} height={40} style={{ objectFit: 'cover', borderRadius: 4 }} preview={false} fallback="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg'/%3E" />
                  : <div style={{ width: 64, height: 40, borderRadius: 4, background: '#f5f5f5', display: 'flex', alignItems: 'center', justifyContent: 'center' }}><VideoCameraOutlined style={{ color: '#bbb' }} /></div>}
                <div style={{ maxWidth: 220 }}>
                  <div style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{w.title || '(未命名)'}</div>
                  <Text type="secondary" style={{ fontSize: 12 }}>{w.id}</Text>
                </div>
              </Space>
            ),
          },
          {
            title: '类型', dataIndex: 'kind', width: 70,
            render: (k: string) => <Tag color={kindMeta[k]?.color || 'default'}>{kindMeta[k]?.label || k}</Tag>,
          },
          { title: '归属租户', dataIndex: 'tenant_id', width: 130, ellipsis: true },
          { title: '创建时间', dataIndex: 'created_at', width: 150, render: (t: string) => new Date(t).toLocaleString('zh-CN', { hour12: false }) },
          {
            title: '处置状态', key: 'moderation', width: 150,
            render: (_, w: AdminWorkItem) => w.moderation_action
              ? <Tooltip title={w.moderation_reason}><Tag color={actionMeta[w.moderation_action]?.color}>{actionMeta[w.moderation_action]?.label || w.moderation_action}</Tag></Tooltip>
              : <Tag>正常</Tag>,
          },
          {
            title: '操作', key: 'ops', width: 150,
            render: (_, w: AdminWorkItem) => (
              <Space>
                {!w.moderation_action && (
                  <Button size="small" danger icon={<StopOutlined />}
                    onClick={() => { setHideTarget(w); setAction('hidden'); setReason('') }}>
                    处置
                  </Button>
                )}
                {w.moderation_action && (
                  <Popconfirm title="恢复该作品的用户端可见性与发布权限？" onConfirm={() => restore(w)}>
                    <Button size="small" icon={<UndoOutlined />}>恢复</Button>
                  </Popconfirm>
                )}
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title={`处置作品：${hideTarget?.title || hideTarget?.id || ''}`}
        open={!!hideTarget}
        onOk={submitHide}
        onCancel={() => setHideTarget(null)}
        confirmLoading={acting}
        okText="确认处置"
        okButtonProps={{ danger: true }}
      >
        <Radio.Group value={action} onChange={(e) => setAction(e.target.value)} style={{ marginBottom: 12 }}>
          <Radio value="hidden">下架（用户端不可见，可恢复）</Radio>
          <Radio value="deleted">删除（不可见且不可再发布）</Radio>
        </Radio.Group>
        <Input.TextArea
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          placeholder="处置原因（必填，审计记录）"
          rows={3}
          maxLength={500}
          showCount
        />
      </Modal>
    </div>
  )
}
