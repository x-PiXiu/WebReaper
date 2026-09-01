import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Button, Descriptions, Drawer, Popconfirm, Segmented, Space, Table, Tag, Typography } from 'antd'
import { businessApi } from '../../api/business'
import { message } from '../../utils/antdApp'

const { Text, Paragraph } = Typography

const STATE_META: Record<string, { label: string; color: string }> = {
  created: { label: '排队中', color: 'default' },
  queueing: { label: '排队中', color: 'default' },
  processing: { label: '处理中', color: 'processing' },
  success: { label: '成功', color: 'green' },
  failed: { label: '失败', color: 'red' },
  cancelled: { label: '已取消', color: 'default' },
}

/** 任务行类型（admin/tasks 返回字段） */
type AdminTaskRow = {
  id: string
  tenant_id: string
  brand_id?: string
  sub_type: string
  type?: string
  model?: string
  provider?: string
  state: string
  err_code?: string
  err_msg?: string
  params_json?: string
  creations_json?: string
  credits?: number
  created_at?: string
  finished_at?: string
}

/** 从 params/creations 提取全部 http(s) 媒体 URL */
function extractURLs(...raws: (string | undefined)[]) {
  const set = new Set<string>()
  for (const raw of raws) {
    if (!raw) continue
    for (const m of raw.matchAll(/https?:\/\/[^\s"\\,}\]]+/g)) {
      set.add(m[0].replace(/[.,]+$/, ''))
    }
  }
  return [...set]
}

/** 格式化 JSON（宽松——非 JSON 原样返回） */
function pretty(raw?: string) {
  if (!raw) return ''
  try { return JSON.stringify(JSON.parse(raw), null, 2) } catch { return raw }
}

/** 生成任务监控（管理后台——跨租户任务列表/详情/取消）。
 *  详情抽屉（2026-09-02）：任务全字段 + 提交参数/产物 JSON 展开 + 媒体产物预览链接 + 错误全文。 */
function TaskMonitor({ embedded = false }: { embedded?: boolean }) {
  void embedded
  const queryClient = useQueryClient()
  const [state, setState] = useState('active')
  const [detail, setDetail] = useState<AdminTaskRow | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['admin-tasks', state],
    queryFn: () => businessApi.adminListAllTasks({ state }).then((r) => r.tasks),
    refetchInterval: 10_000,
  })

  const doCancel = async (id: string) => {
    try {
      await businessApi.adminCancelTask(id)
      message.success('已取消')
      queryClient.invalidateQueries({ queryKey: ['admin-tasks'] })
    } catch { /* 拦截器已提示 */ }
  }

  const mediaURLs = detail ? extractURLs(detail.params_json, detail.creations_json) : []

  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Segmented
        value={state}
        onChange={(v) => setState(v as string)}
        options={[
          { value: 'active', label: '活跃任务' },
          { value: 'failed', label: '失败任务' },
        ]}
      />
      <Table
        rowKey="id"
        size="small"
        loading={isLoading}
        dataSource={data ?? []}
        pagination={{ pageSize: 20, showTotal: (t) => `共 ${t} 条` }}
        onRow={(r) => ({ onClick: () => setDetail(r), style: { cursor: 'pointer' } })}
        columns={[
          { title: '任务 ID', dataIndex: 'id', width: 200, render: (id: string) => <Text copyable style={{ fontSize: 12 }} onClick={(e) => e.stopPropagation()}>{id.slice(0, 24)}</Text> },
          { title: '租户', dataIndex: 'tenant_id', width: 140 },
          { title: '类型', dataIndex: 'sub_type', width: 100 },
          {
            title: '状态', dataIndex: 'state', width: 90, render: (st: string) => {
              const meta = STATE_META[st] ?? { label: st, color: 'default' }
              return <Tag color={meta.color}>{meta.label}</Tag>
            },
          },
          { title: '模型', dataIndex: 'model', width: 100 },
          { title: '错误', dataIndex: 'err_msg', ellipsis: true, render: (e: string) => e ? <Text type="danger" style={{ fontSize: 12 }}>{e}</Text> : '-' },
          { title: '创建时间', dataIndex: 'created_at', width: 140 },
          {
            title: '操作', width: 110,
            render: (_, r: AdminTaskRow) => (
              <Space onClick={(e) => e.stopPropagation()}>
                <Button size="small" onClick={() => setDetail(r)}>详情</Button>
                {!['success', 'failed', 'cancelled'].includes(r.state) ? (
                  <Popconfirm title="确定取消？" okText="取消任务" okButtonProps={{ danger: true }} cancelText="返回" onConfirm={() => doCancel(r.id)}>
                    <Button size="small" danger>取消</Button>
                  </Popconfirm>
                ) : null}
              </Space>
            ),
          },
        ]}
      />

      <Drawer
        title={`任务详情 · ${detail?.sub_type || ''}`}
        open={!!detail}
        onClose={() => setDetail(null)}
        width={620}
        destroyOnClose
      >
        {detail && (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <Descriptions column={2} size="small" bordered>
              <Descriptions.Item label="任务 ID" span={2}><Text copyable style={{ fontSize: 12 }}>{detail.id}</Text></Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={STATE_META[detail.state]?.color || 'default'}>{STATE_META[detail.state]?.label || detail.state}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="类型">{detail.sub_type}</Descriptions.Item>
              <Descriptions.Item label="租户" span={2}>{detail.tenant_id}</Descriptions.Item>
              <Descriptions.Item label="模型" span={1}>{detail.model || '-'}</Descriptions.Item>
              <Descriptions.Item label="厂商" span={1}>{detail.provider || '-'}</Descriptions.Item>
              {detail.brand_id && <Descriptions.Item label="品牌" span={2}>{detail.brand_id}</Descriptions.Item>}
              {detail.credits ? <Descriptions.Item label="积分" span={1}>{detail.credits}</Descriptions.Item> : null}
              {detail.created_at && <Descriptions.Item label="创建时间" span={1}>{new Date(detail.created_at).toLocaleString('zh-CN', { hour12: false })}</Descriptions.Item>}
              {detail.finished_at && <Descriptions.Item label="完成时间" span={2}>{new Date(detail.finished_at).toLocaleString('zh-CN', { hour12: false })}</Descriptions.Item>}
            </Descriptions>

            {detail.err_msg && (
              <div>
                <Text type="danger">失败原因（全文）</Text>
                <Paragraph style={{ background: '#fff2f0', padding: 12, borderRadius: 6, whiteSpace: 'pre-wrap', marginBottom: 0 }}>
                  {detail.err_msg}
                  {detail.err_code ? ` [${detail.err_code}]` : ''}
                </Paragraph>
              </div>
            )}

            {mediaURLs.length > 0 && (
              <div>
                <Text type="secondary">媒体产物 / 引用素材（{mediaURLs.length}）</Text>
                <Space direction="vertical" size={4} style={{ width: '100%' }}>
                  {mediaURLs.map((u) => (
                    <a key={u} href={u} target="_blank" rel="noreferrer" style={{ fontSize: 12, wordBreak: 'break-all' }}>{u}</a>
                  ))}
                </Space>
              </div>
            )}

            {detail.params_json && (
              <div>
                <Text type="secondary">提交参数</Text>
                <Paragraph style={{ background: '#fafafa', padding: 12, borderRadius: 6, maxHeight: 240, overflow: 'auto', fontSize: 12, marginBottom: 0 }}>
                  <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{pretty(detail.params_json)}</pre>
                </Paragraph>
              </div>
            )}
            {detail.creations_json && (
              <div>
                <Text type="secondary">产物信息</Text>
                <Paragraph style={{ background: '#fafafa', padding: 12, borderRadius: 6, maxHeight: 200, overflow: 'auto', fontSize: 12, marginBottom: 0 }}>
                  <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{pretty(detail.creations_json)}</pre>
                </Paragraph>
              </div>
            )}

            {!['success', 'failed', 'cancelled'].includes(detail.state) && (
              <Popconfirm title="确定取消该任务？" okText="取消任务" okButtonProps={{ danger: true }} cancelText="返回" onConfirm={() => { doCancel(detail.id); setDetail(null) }}>
                <Button danger>取消此任务</Button>
              </Popconfirm>
            )}
          </Space>
        )}
      </Drawer>
    </Space>
  )
}

export default TaskMonitor
