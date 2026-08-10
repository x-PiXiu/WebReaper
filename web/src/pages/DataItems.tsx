import { useState, Fragment, type Key } from 'react'
import { Card, Table, Tag, Typography, Button, Space, message, Modal } from 'antd'
import { DeleteOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import type { DataItem } from '../types/api'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeHighlight from 'rehype-highlight'

const { Title, Text } = Typography

const statusColor: Record<string, string> = {
  pending_review: 'orange', approved: 'green', rejected: 'red',
}

// tryParseJSON 尝试把字符串解析为 JSON 对象。失败返回 null。
// 用于智能判断 content 是结构化 JSON 还是纯文本/Markdown。
function tryParseJSON(s: string): Record<string, unknown> | null {
  const trimmed = s.trim()
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return null
  try {
    const obj = JSON.parse(trimmed)
    // 只认对象类型（数组不算字段表）
    if (typeof obj === 'object' && obj !== null && !Array.isArray(obj)) {
      return obj as Record<string, unknown>
    }
    return null
  } catch {
    return null
  }
}

// renderValue 渲染字段值（按类型分派）。
function renderValue(v: unknown): React.ReactNode {
  if (v === null || v === undefined) return <Text type="secondary">null</Text>
  if (typeof v === 'string') return <span style={{ wordBreak: 'break-all' }}>{v}</span>
  if (typeof v === 'number' || typeof v === 'boolean') return <span>{String(v)}</span>
  if (Array.isArray(v)) {
    return <span>{v.map((x, i) => <Tag key={i} style={{ marginBottom: 2 }}>{String(x)}</Tag>)}</span>
  }
  if (typeof v === 'object') {
    return <code style={{ fontSize: 12, wordBreak: 'break-all' }}>{JSON.stringify(v)}</code>
  }
  return <span>{String(v)}</span>
}

// ContentRenderer 正文智能渲染：JSON → 字段表；否则 → Markdown。
function ContentRenderer({ content }: { content: string }) {
  if (!content) return <Text type="secondary">（无内容）</Text>

  const jsonObj = tryParseJSON(content)
  if (jsonObj) {
    // 结构化 JSON：渲染成字段表（键值对），比一堆 JSON 字符串易读
    const entries = Object.entries(jsonObj)
    return (
      <div style={{
        padding: 16, background: 'rgba(255,255,255,0.03)', borderRadius: 8,
        display: 'grid', gridTemplateColumns: 'minmax(100px, auto) 1fr',
        rowGap: 10, columnGap: 16, alignItems: 'start', fontSize: 13,
      }}>
        {entries.map(([k, v]) => (
          <Fragment key={k}>
            <Text strong style={{ color: 'var(--wr-text-muted)' }}>{k}</Text>
            <div>{renderValue(v)}</div>
          </Fragment>
        ))}
      </div>
    )
  }

  // 纯文本/Markdown：渲染 Markdown，保留换行
  return (
    <div style={{
      padding: 16, background: 'rgba(255,255,255,0.03)', borderRadius: 8,
      maxHeight: 500, overflowY: 'auto',
      whiteSpace: 'pre-wrap', wordBreak: 'break-word', lineHeight: 1.7,
    }}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeHighlight]}>
        {content}
      </ReactMarkdown>
    </div>
  )
}

export default function DataItems() {
  const queryClient = useQueryClient()
  const [expandedKeys, setExpandedKeys] = useState<string[]>([])
  const { data: items = [], refetch } = useQuery({
    queryKey: ['data-items'],
    queryFn: () => businessApi.listDataItems(),
  })

  const handleApprove = async (id: string) => {
    try { await businessApi.approveItem(id); message.success('已通过'); queryClient.invalidateQueries({ queryKey: ['data-items'] }) } catch {}
  }
  const handleReject = async (id: string) => {
    try { await businessApi.rejectItem(id); message.success('已拒绝'); queryClient.invalidateQueries({ queryKey: ['data-items'] }) } catch {}
  }

  // 删除（带确认弹框）
  const handleDelete = (record: DataItem) => {
    Modal.confirm({
      title: '确认删除',
      content: `确定删除「${record.title || record.id}」吗？此操作不可恢复。`,
      okText: '删除', okType: 'danger', cancelText: '取消',
      onOk: async () => {
        try {
          await businessApi.deleteDataItem(record.id)
          message.success('已删除')
          queryClient.invalidateQueries({ queryKey: ['data-items'] })
        } catch {}
      },
    })
  }

  const columns = [
    { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
    {
      title: '摘要', dataIndex: 'summary', key: 'summary', ellipsis: true,
      render: (s: string) => s ? <Text type="secondary" ellipsis={{ tooltip: s }}>{s}</Text> : <Text type="secondary">—</Text>,
    },
    {
      title: '标签', dataIndex: 'tags', key: 'tags', width: 200,
      render: (tags: string[]) => tags?.slice(0, 4).map(t => <Tag key={t}>{t}</Tag>),
    },
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 110,
      render: (s: string) => <Tag color={statusColor[s]}>{s}</Tag>,
    },
    {
      title: '操作', key: 'action', width: 200,
      render: (_: unknown, record: DataItem) => (
        <Space>
          {record.status === 'pending_review' && (
            <>
              <Button size="small" type="primary" onClick={(e) => { e.stopPropagation(); handleApprove(record.id) }}>通过</Button>
              <Button size="small" danger onClick={(e) => { e.stopPropagation(); handleReject(record.id) }}>拒绝</Button>
            </>
          )}
          <Button size="small" type="text" danger icon={<DeleteOutlined />}
            onClick={(e) => { e.stopPropagation(); handleDelete(record) }}
            title="删除" />
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Title level={4}>数据管理</Title>
      <Text type="secondary">点击任意一行展开查看详情。支持删除、审核、推送。</Text>
      <Card style={{ marginTop: 16 }}>
        <Button style={{ marginBottom: 16 }} onClick={() => refetch()}>刷新</Button>
        <Table
          dataSource={items}
          columns={columns}
          rowKey="id"
          pagination={{ pageSize: 20 }}
          size="middle"
          onRow={(record: DataItem) => ({
            onClick: () => {
              setExpandedKeys(prev =>
                prev.includes(record.id) ? prev.filter(k => k !== record.id) : [...prev, record.id],
              )
            },
            style: { cursor: 'pointer' },
          })}
          expandable={{
            expandedRowRender: (record: DataItem) => (
              <div style={{ padding: '16px 24px', maxWidth: 1000 }}>
                {/* 字段逐一排列 */}
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

                {/* 正文内容：智能渲染（JSON 字段表 / Markdown） */}
                <div style={{ marginTop: 16 }}>
                  <Text strong>正文内容</Text>
                  <div style={{ marginTop: 8 }}>
                    <ContentRenderer content={record.content} />
                  </div>
                </div>

                {/* 元数据：键值表 */}
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
            expandedRowKeys: expandedKeys,
            onExpandedRowsChange: (keys: readonly Key[]) => setExpandedKeys(keys as string[]),
          }}
        />
      </Card>

      {/* 推送 Modal */}
    </div>
  )
}
