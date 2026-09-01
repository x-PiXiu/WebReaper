import { Typography, Table, Tag, Space, Button, Popconfirm, Modal, Input, Radio, Image, Tooltip } from 'antd'
import { StopOutlined, UndoOutlined, VideoCameraOutlined, EyeOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { AdminWorkItem, AdminWorkFlagged } from '../../types/api'
import { toast } from '../../utils/feedback'
import AdminWorkDetailDrawer from '../../components/AdminWorkDetailDrawer'

const { Text } = Typography

const kindMeta: Record<string, { color: string; label: string }> = {
  video: { color: 'processing', label: '视频' },
  image: { color: 'cyan', label: '图片' },
  audio: { color: 'purple', label: '音频' },
}

const actionMeta: Record<string, { color: string; label: string }> = {
  hidden: { color: 'warning', label: '已下架' },
  deleted: { color: 'error', label: '已删除' },
  flagged: { color: 'orange', label: '待复核' },
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
  // 32号 F2：详情抽屉（看内容——媒体/文案/处置记录）
  const [detailKey, setDetailKey] = useState<string | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['admin-works'],
    queryFn: () => businessApi.adminListWorks(100),
  })
  const items = data?.items ?? []

  // 32号 P2：机审待复核队列（flagged——机器标记，处置权在管理员）
  const { data: flaggedData } = useQuery({
    queryKey: ['admin-works-flagged'],
    queryFn: () => businessApi.adminListFlaggedWorks(200),
  })
  const flagged = flaggedData?.items ?? []

  // 32号 P2 终批：用户申诉待复核队列（采纳=恢复 / 维持=终审驳回）
  const { data: appealsData } = useQuery({
    queryKey: ['admin-works-appeals'],
    queryFn: () => businessApi.adminListAppeals(200),
  })
  const appeals = appealsData?.items ?? []

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ['admin-works'] })
    queryClient.invalidateQueries({ queryKey: ['admin-works-flagged'] })
    queryClient.invalidateQueries({ queryKey: ['admin-works-appeals'] })
  }

  const rejectAppeal = async (workKey: string) => {
    try {
      await businessApi.adminRejectAppeal(workKey)
      toast.ok('已维持处置（终审；用户 24 小时后可再申诉）')
      refresh()
    } catch { /* 拦截器已提示 */ }
  }

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

  const disposeFlagged = (f: AdminWorkFlagged) => {
    setHideTarget({
      id: f.work_key, kind: f.work_kind, title: `(机审标记) ${f.reason}`, tenant_id: f.tenant_id,
    } as AdminWorkItem)
    setAction('hidden')
    setReason(f.reason)
  }

  const releaseFlagged = async (f: AdminWorkFlagged) => {
    try {
      await businessApi.adminRestoreWork(f.work_key)
      toast.ok('已放行（清除标记，复审不再提示）')
      refresh()
    } catch { /* 拦截器已提示 */ }
  }

  return (
    <div style={embedded ? undefined : { padding: 24 }}>
      {appeals.length > 0 && (
        <div style={{ marginBottom: 16 }}>
          <Space style={{ marginBottom: 8 }}>
            <Tag color="red">用户申诉</Tag>
            <Text type="secondary">{appeals.length} 条待复核——采纳=恢复作品；维持=终审驳回（用户 24 小时后可再申诉）</Text>
          </Space>
          <Table
            rowKey="work_key"
            dataSource={appeals}
            size="small"
            pagination={false}
            columns={[
              {
                title: '作品键', dataIndex: 'work_key', width: 180, ellipsis: true,
                render: (k: string) => <a onClick={(e) => { e.preventDefault(); setDetailKey(k) }}>{k}</a>,
              },
              { title: '处置', dataIndex: 'action', width: 80, render: (a: string) => <Tag color={actionMeta[a]?.color}>{actionMeta[a]?.label || a}</Tag> },
              { title: '处置原因', dataIndex: 'reason', width: 160, ellipsis: true },
              { title: '申诉理由', dataIndex: 'appeal_text', ellipsis: true },
              { title: '租户', dataIndex: 'tenant_id', width: 110, ellipsis: true },
              {
                title: '操作', key: 'ops', width: 220,
                render: (_, a: AdminWorkFlagged) => (
                  <Space>
                    <Button size="small" icon={<EyeOutlined />} onClick={() => setDetailKey(a.work_key)}>查看</Button>
                    <Popconfirm title="采纳申诉并恢复该作品？（建议先点查看确认内容）" onConfirm={() => releaseFlagged(a)}>
                      <Button size="small" type="primary">采纳</Button>
                    </Popconfirm>
                    <Popconfirm title="维持处置？（终审驳回，24 小时内不可再申诉）" onConfirm={() => rejectAppeal(a.work_key)}>
                      <Button size="small" danger>维持</Button>
                    </Popconfirm>
                  </Space>
                ),
              },
            ]}
          />
        </div>
      )}
      {flagged.length > 0 && (
        <div style={{ marginBottom: 16 }}>
          <Space style={{ marginBottom: 8 }}>
            <Tag color="orange">机审待复核</Tag>
            <Text type="secondary">
              {flagged.length} 条被机器标记（不拦截、用户无感）——请人工判定：处置或放行（放行后复审不再提示）
            </Text>
          </Space>
          <Table
            rowKey="work_key"
            dataSource={flagged}
            size="small"
            pagination={false}
            columns={[
              {
                title: '作品键', dataIndex: 'work_key', width: 200, ellipsis: true,
                render: (k: string) => <a onClick={(e) => { e.preventDefault(); setDetailKey(k) }}>{k}</a>,
              },
              { title: '类型', dataIndex: 'work_kind', width: 70, render: (k: string) => <Tag>{kindMeta[k]?.label || k}</Tag> },
              { title: '租户', dataIndex: 'tenant_id', width: 110, ellipsis: true },
              { title: '机审理由', dataIndex: 'reason', ellipsis: true },
              { title: '标记时间', dataIndex: 'updated_at', width: 140, render: (t: string) => new Date(t).toLocaleString('zh-CN', { hour12: false }) },
              {
                title: '操作', key: 'ops', width: 200,
                render: (_, f: AdminWorkFlagged) => (
                  <Space>
                    <Button size="small" icon={<EyeOutlined />} onClick={() => setDetailKey(f.work_key)}>查看</Button>
                    <Button size="small" danger onClick={() => disposeFlagged(f)}>处置</Button>
                    <Popconfirm title="确认放行？（清除标记，复审不再提示）" onConfirm={() => releaseFlagged(f)}>
                      <Button size="small" type="primary" ghost>放行</Button>
                    </Popconfirm>
                  </Space>
                ),
              },
            ]}
          />
        </div>
      )}
      <Space style={{ marginBottom: 12 }}>
        <Text type="secondary">
          最新成片巡查流（{items.length} 条）——下架后用户端作品库即刻不可见且无法发布；处置记录含操作者与原因（审计）
        </Text>
      </Space>
      <Table
        rowKey="id"
        loading={isLoading}
        dataSource={[...items].sort((a, b) => (a.moderation_action === 'flagged' ? -1 : 0) - (b.moderation_action === 'flagged' ? -1 : 0))}
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
                  ? <Image src={w.cover_url || w.media_urls?.[0]} width={64} height={40} style={{ objectFit: 'cover', borderRadius: 4, cursor: 'pointer' }} preview={false} fallback="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg'/%3E" onClick={() => setDetailKey(w.id)} />
                  : <div style={{ width: 64, height: 40, borderRadius: 4, background: '#f5f5f5', display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer' }} onClick={() => setDetailKey(w.id)}><VideoCameraOutlined style={{ color: '#bbb' }} /></div>}
                <div style={{ maxWidth: 220 }}>
                  <a onClick={(e) => { e.preventDefault(); setDetailKey(w.id) }} style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', display: 'block' }}>{w.title || '(未命名)'}</a>
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
            title: '操作', key: 'ops', width: 200,
            render: (_, w: AdminWorkItem) => (
              <Space>
                <Button size="small" icon={<EyeOutlined />} onClick={() => setDetailKey(w.id)}>详情</Button>
                {!w.moderation_action && (
                  <Button size="small" danger icon={<StopOutlined />}
                    onClick={() => { setHideTarget(w); setAction('hidden'); setReason('') }}>
                    处置
                  </Button>
                )}
                {w.moderation_action && (
                  <Popconfirm title={w.moderation_action === 'flagged' ? '确认放行？（清除机审标记）' : '恢复该作品的用户端可见性与发布权限？'} onConfirm={() => restore(w)}>
                    <Button size="small" icon={<UndoOutlined />}>{w.moderation_action === 'flagged' ? '放行' : '恢复'}</Button>
                  </Popconfirm>
                )}
                {w.moderation_action === 'flagged' && (
                  <Button size="small" danger icon={<StopOutlined />} onClick={() => { setHideTarget(w); setAction('hidden'); setReason(w.moderation_reason || '') }}>
                    处置
                  </Button>
                )}
              </Space>
            ),
          },
        ]}
      />

      <AdminWorkDetailDrawer workKey={detailKey} onClose={() => setDetailKey(null)} />

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
