import { useMemo, useState } from 'react'
import { Button, Card, Col, Input, Popconfirm, Row, Select, Space, Table, Tag, Typography } from 'antd'
import { DeleteOutlined, PushpinOutlined, StarOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { InspirationVideo } from '../../types/api'
import QueryBoundary from '../../components/QueryBoundary'
import { toast } from '../../utils/feedback'

const { Text, Title } = Typography

/** Admin 灵感运营：置顶 / 推荐 / 备注 / 删除 */
export default function AdminInspirations() {
  const queryClient = useQueryClient()
  const [platform, setPlatform] = useState<string>()
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<string[]>([])

  const { data: stats } = useQuery({
    queryKey: ['admin-inspiration-stats'],
    queryFn: () => businessApi.adminInspirationStats(),
  })
  const { data: platformsRes } = useQuery({
    queryKey: ['inspiration-platforms'],
    queryFn: () => businessApi.listInspirationPlatforms(),
  })
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['admin-inspirations', platform, keyword, page],
    queryFn: () => businessApi.listInspirations({
      platform: platform || undefined,
      keyword: keyword || undefined,
      page,
      page_size: 20,
      sort_by: 'viral_score',
    }),
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['admin-inspirations'] })
    queryClient.invalidateQueries({ queryKey: ['admin-inspiration-stats'] })
  }

  const updateMut = useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: { is_pinned?: boolean; is_recommended?: boolean; admin_note?: string } }) =>
      businessApi.adminUpdateInspiration(id, patch),
    onSuccess: () => { toast.ok('已更新'); invalidate() },
  })
  const deleteMut = useMutation({
    mutationFn: (id: string) => businessApi.adminDeleteInspiration(id),
    onSuccess: () => { toast.ok('已删除'); setSelected([]); invalidate() },
  })
  const batchMut = useMutation({
    mutationFn: (action: 'delete') => businessApi.adminBatchInspirations({ ids: selected, action }),
    onSuccess: (r) => { toast.ok(`已处理 ${r.affected} 条`); setSelected([]); invalidate() },
  })

  const items = data?.items || []
  const platformOptions = useMemo(
    () => (platformsRes?.platforms || []).map((p) => ({ value: p, label: p })),
    [platformsRes],
  )

  return (
    <QueryBoundary loading={isLoading} error={isError} onRetry={() => refetch()}>
    <div className="wr-page-content">
      <div style={{ marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>灵感运营</Title>
        <Text type="secondary">置顶、推荐与清理爬虫入库的灵感视频</Text>
      </div>

      <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
        <Col xs={12} md={6}>
          <Card size="small"><Text type="secondary">灵感总量</Text><div style={{ fontSize: 22, fontWeight: 600 }}>{stats?.total_videos ?? '—'}</div></Card>
        </Col>
        <Col xs={12} md={6}>
          <Card size="small"><Text type="secondary">关联品牌</Text><div style={{ fontSize: 22, fontWeight: 600 }}>{stats?.total_brands ?? '—'}</div></Card>
        </Col>
        {(stats?.by_platform || []).slice(0, 2).map((p) => (
          <Col xs={12} md={6} key={p.platform}>
            <Card size="small"><Text type="secondary">{p.platform}</Text><div style={{ fontSize: 22, fontWeight: 600 }}>{p.count}</div></Card>
          </Col>
        ))}
      </Row>

      <Card size="small" style={{ marginBottom: 12 }}>
        <Space wrap>
          <Select
            allowClear
            placeholder="平台"
            style={{ width: 140 }}
            value={platform}
            onChange={(v) => { setPlatform(v); setPage(1) }}
            options={platformOptions}
          />
          <Input.Search
            allowClear
            placeholder="标题关键词"
            style={{ width: 220 }}
            onSearch={(v) => { setKeyword(v); setPage(1) }}
          />
          <Popconfirm
            title={`批量删除 ${selected.length} 条？`}
            disabled={selected.length === 0}
            onConfirm={() => batchMut.mutate('delete')}
          >
            <Button danger disabled={selected.length === 0} loading={batchMut.isPending} icon={<DeleteOutlined />}>
              批量删除
            </Button>
          </Popconfirm>
        </Space>
      </Card>

      <Table
        rowKey="id"
        size="middle"
        loading={isLoading}
        dataSource={items}
        rowSelection={{
          selectedRowKeys: selected,
          onChange: (keys) => setSelected(keys as string[]),
        }}
        pagination={{
          current: page,
          pageSize: 20,
          total: data?.total || 0,
          onChange: setPage,
          showTotal: (t) => `共 ${t} 条`,
        }}
        columns={[
          {
            title: '封面',
            dataIndex: 'cover_url',
            width: 72,
            render: (url: string) => url
              ? <div style={{ width: 48, height: 64, borderRadius: 6, background: `center/cover no-repeat url(${url})`, backgroundColor: '#eee' }} />
              : '—',
          },
          {
            title: '标题',
            dataIndex: 'title',
            render: (t: string, r: InspirationVideo) => (
              <Space direction="vertical" size={2}>
                <Text strong ellipsis style={{ maxWidth: 320, display: 'block' }}>{t || '(无标题)'}</Text>
                <Text type="secondary" style={{ fontSize: 12 }}>{r.author} · {r.platform}</Text>
              </Space>
            ),
          },
          {
            title: '数据',
            width: 140,
            render: (_: unknown, r: InspirationVideo) => (
              <Text type="secondary" style={{ fontSize: 12 }}>
                热度 {r.viral_score?.toFixed?.(0) ?? r.viral_score}<br />
                播 {r.play_count?.toLocaleString?.() ?? r.play_count}
              </Text>
            ),
          },
          {
            title: '标记',
            width: 120,
            render: (_: unknown, r: InspirationVideo) => (
              <Space size={4} wrap>
                {r.is_pinned && <Tag color="blue">置顶</Tag>}
                {r.is_recommended && <Tag color="gold">推荐</Tag>}
              </Space>
            ),
          },
          {
            title: '操作',
            width: 220,
            render: (_: unknown, r: InspirationVideo) => (
              <Space size={4} wrap>
                <Button
                  size="small"
                  icon={<PushpinOutlined />}
                  loading={updateMut.isPending}
                  onClick={() => updateMut.mutate({ id: r.id, patch: { is_pinned: !r.is_pinned } })}
                >
                  {r.is_pinned ? '取消置顶' : '置顶'}
                </Button>
                <Button
                  size="small"
                  icon={<StarOutlined />}
                  loading={updateMut.isPending}
                  onClick={() => updateMut.mutate({ id: r.id, patch: { is_recommended: !r.is_recommended } })}
                >
                  {r.is_recommended ? '取消推荐' : '推荐'}
                </Button>
                <Popconfirm title="删除该灵感？" onConfirm={() => deleteMut.mutate(r.id)}>
                  <Button size="small" danger type="text" icon={<DeleteOutlined />} />
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />
    </div>
    </QueryBoundary>
  )
}
