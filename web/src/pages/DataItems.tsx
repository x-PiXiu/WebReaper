import { useState, Fragment, type Key } from 'react'
import { Card, Table, Tag, Typography, Button, Space, message, Modal, Select } from 'antd'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import type { DataItem, ExternalSystem } from '../types/api'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeHighlight from 'rehype-highlight'

const { Title, Text } = Typography

const statusColor: Record<string, string> = {
  pending_review: 'orange', approved: 'green', rejected: 'red',
}

export default function DataItems() {
  const queryClient = useQueryClient()
  const [publishModal, setPublishModal] = useState<{ open: boolean; itemId?: string; systemName?: string; loading?: boolean }>({ open: false })
  const [expandedKeys, setExpandedKeys] = useState<string[]>([])
  const { data: items = [], refetch } = useQuery({
    queryKey: ['data-items'],
    queryFn: () => businessApi.listDataItems(),
  })
  // 外部系统列表（推送时选择目标）
  const { data: externalSystems = [] } = useQuery({
    queryKey: ['external-systems'],
    queryFn: () => businessApi.listExternalSystems(),
  })

  const handleApprove = async (id: string) => {
    try { await businessApi.approveItem(id); message.success('已通过'); queryClient.invalidateQueries({ queryKey: ['data-items'] }) } catch {}
  }
  const handleReject = async (id: string) => {
    try { await businessApi.rejectItem(id); message.success('已拒绝'); queryClient.invalidateQueries({ queryKey: ['data-items'] }) } catch {}
  }

  // 推送到外部系统
  const handlePublish = async () => {
    if (!publishModal.itemId || !publishModal.systemName) return
    setPublishModal(p => ({ ...p, loading: true }))
    try {
      const res = await businessApi.publishToExternal(publishModal.itemId, publishModal.systemName)
      if (res.success) {
        message.success(`推送成功${res.external_id ? '（外部ID: ' + res.external_id.slice(0, 16) + '）' : ''}`)
      } else {
        message.error(`推送失败：${res.error || '未知错误'}`)
      }
      setPublishModal({ open: false })
    } catch {
      // axios 拦截器已提示
    } finally {
      setPublishModal(p => ({ ...p, loading: false }))
    }
  }

  const columns = [
    { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
    { title: '摘要', dataIndex: 'summary', key: 'summary', ellipsis: true },
    {
      title: '标签', dataIndex: 'tags', key: 'tags', width: 200,
      render: (tags: string[]) => tags?.slice(0, 4).map(t => <Tag key={t}>{t}</Tag>),
    },
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 110,
      render: (s: string) => <Tag color={statusColor[s]}>{s}</Tag>,
    },
    {
      title: '操作', key: 'action', width: 180,
      render: (_: unknown, record: DataItem) => (
        <Space>
          {record.status === 'pending_review' && (
            <>
              <Button size="small" type="primary" onClick={() => handleApprove(record.id)}>通过</Button>
              <Button size="small" danger onClick={() => handleReject(record.id)}>拒绝</Button>
            </>
          )}
          {record.status === 'approved' && (
            <Button size="small" onClick={() => setPublishModal({ open: true, itemId: record.id })}>推送</Button>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Title level={4}>数据管理</Title>
      <Text type="secondary">Agent 采集的所有数据项。点击行展开查看详情。待审核项需人工确认。</Text>
      <Card style={{ marginTop: 16 }}>
        <Button style={{ marginBottom: 16 }} onClick={() => refetch()}>刷新</Button>
        <Table
          dataSource={items}
          columns={columns}
          rowKey="id"
          pagination={{ pageSize: 20 }}
          size="middle"
          expandable={{
            expandedRowRender: (record: DataItem) => (
              <div style={{ padding: '16px 24px', maxWidth: 1000 }}>
                {/* 字段逐一排列，每个字段独立一行/块，清晰分隔 */}
                <div style={{ display: 'grid', gridTemplateColumns: '90px 1fr', rowGap: 12, columnGap: 16, alignItems: 'start' }}>
                  <Text strong>标题</Text>
                  <Text>{record.title || '（无）'}</Text>

                  <Text strong>来源</Text>
                  {record.source_url
                    ? <a href={record.source_url} target="_blank" rel="noreferrer" style={{ wordBreak: 'break-all' }}>{record.source_url}</a>
                    : <Text type="secondary">（无）</Text>}

                  <Text strong>摘要</Text>
                  <Text>{record.summary || <Text type="secondary">（无）</Text>}</Text>

                  <Text strong>标签</Text>
                  <div>{record.tags && record.tags.length > 0
                    ? record.tags.map(t => <Tag key={t} style={{ marginBottom: 4 }}>{t}</Tag>)
                    : <Text type="secondary">（无）</Text>}</div>

                  <Text strong>状态</Text>
                  <Tag color={statusColor[record.status]}>{record.status}</Tag>
                </div>

                {/* 正文内容：独立区块，Markdown 渲染，限高滚动 */}
                <div style={{ marginTop: 16 }}>
                  <Text strong>正文内容</Text>
                  <div style={{
                    marginTop: 8, padding: 16,
                    background: 'rgba(255,255,255,0.03)', borderRadius: 8,
                    maxHeight: 500, overflowY: 'auto',
                    whiteSpace: 'pre-wrap', // 保留换行，避免长文本挤一行
                    wordBreak: 'break-word',
                    lineHeight: 1.7,
                  }}>
                    <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeHighlight]}>
                      {record.content || '（无内容）'}
                    </ReactMarkdown>
                  </div>
                </div>

                {/* 元数据：键值表格式，而非一行 JSON */}
                {record.metadata && Object.keys(record.metadata).length > 0 && (
                  <div style={{ marginTop: 16 }}>
                    <Text strong>元数据</Text>
                    <div style={{
                      marginTop: 8, padding: 12,
                      background: 'rgba(255,255,255,0.02)', borderRadius: 8,
                      display: 'grid', gridTemplateColumns: 'auto 1fr', rowGap: 6, columnGap: 16,
                      fontSize: 13,
                    }}>
                      {Object.entries(record.metadata).map(([k, v]) => (
                        <Fragment key={k}>
                          <Text type="secondary" style={{ fontWeight: 500 }}>{k}</Text>
                          <Text style={{ wordBreak: 'break-all' }}>{String(v)}</Text>
                        </Fragment>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            ),
            // 受控展开：点击整行即展开/收起
            expandedRowKeys: expandedKeys,
            onExpandedRowsChange: (keys: Key[]) => setExpandedKeys(keys as string[]),
            onRow: (record: DataItem) => ({
              onClick: () => {
                setExpandedKeys(prev =>
                  prev.includes(record.id) ? prev.filter(k => k !== record.id) : [...prev, record.id],
                )
              },
              style: { cursor: 'pointer' },
            }),
          }}
        />
      </Card>

      {/* 推送 Modal */}
      <Modal
        title="推送到外部系统"
        open={publishModal.open}
        onCancel={() => setPublishModal({ open: false })}
        onOk={handlePublish}
        confirmLoading={publishModal.loading}
        okText="推送"
        cancelText="取消"
        okButtonProps={{ disabled: !publishModal.systemName }}
      >
        {externalSystems.length === 0 ? (
          <Text type="secondary">
            暂无可用外部系统。请先到「外部系统」页面配置目标系统的 API 地址和字段映射。
          </Text>
        ) : (
          <>
            <div style={{ marginBottom: 8 }}>
              <Text type="secondary" style={{ fontSize: 13 }}>
                选择目标系统。系统会按该系统配置的字段映射，把数据项转换后推送。
              </Text>
            </div>
            <Select
              style={{ width: '100%' }}
              placeholder="选择目标系统"
              value={publishModal.systemName}
              onChange={(v) => setPublishModal(p => ({ ...p, systemName: v }))}
              options={externalSystems.filter((s: ExternalSystem) => s.enabled !== false).map((s: ExternalSystem) => ({
                value: s.name,
                label: `${s.name}${s.description ? '（' + s.description + '）' : ''}`,
              }))}
            />
          </>
        )}
      </Modal>
    </div>
  )
}
