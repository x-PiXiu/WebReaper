import { Typography, Table, Tag, Space, Button, Popconfirm, Select, Empty, Tabs, Row, Col } from 'antd'
import { DeleteOutlined, EyeOutlined, SwapOutlined, FileTextOutlined, GlobalOutlined, EditOutlined, InboxOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { scoreColor } from '../../utils/geo'
import ContentPreviewDrawer from '../../components/ContentPreviewDrawer'
import type { Brand, OptimizedContent } from '../../types/api'
import { message } from '../../utils/antdApp'

const { Text } = Typography

const statusMeta: Record<string, { color: string; label: string }> = {
  draft: { color: 'default', label: '草稿' },
  approved: { color: 'processing', label: '已审核' },
  published: { color: 'success', label: '已发布' },
}

// 收录状态（IndexNow 提交 ≠ 被收录；收录验证任务每日回写）
const indexMeta: Record<string, { color: string; label: string }> = {
  indexed: { color: 'success', label: '已收录' },
  pending: { color: 'warning', label: '待收录' },
  error: { color: 'error', label: '查询失败' },
}

// 内容统一管理（管理后台）：全平台优化内容一览。
// 「区分开来」：按状态分区（全部/已发布/草稿/其他），每区独立统计与操作，
// 已发布区展示公开页链接（公网资产），草稿区提示可发布。
export default function AdminContents() {
  const queryClient = useQueryClient()
  const [brandFilter, setBrandFilter] = useState<string>()
  const [activeTab, setActiveTab] = useState('all')
  const [preview, setPreview] = useState<OptimizedContent | null>(null)

  const { data: brands = [] } = useQuery({
    queryKey: ['admin-brands'],
    queryFn: () => businessApi.adminListBrands(), // admin 旁路端点（全局）
  })

  // 全平台内容（admin 旁路端点一次拿全，不再按品牌循环——避免 N 次请求）
  const { data: contentsByBrand = [] } = useQuery({
    queryKey: ['admin-contents'],
    queryFn: () => businessApi.adminListContents(), // admin 旁路端点（全局，无租户过滤）
  })

  const filtered = contentsByBrand.filter((c: OptimizedContent) => !brandFilter || c.brand_id === brandFilter)
  const published = filtered.filter((c: OptimizedContent) => c.status === 'published')
  const drafts = filtered.filter((c: OptimizedContent) => c.status === 'draft')
  const others = filtered.filter((c: OptimizedContent) => c.status !== 'published' && c.status !== 'draft')

  const contents = activeTab === 'all' ? filtered
    : activeTab === 'published' ? published
    : activeTab === 'draft' ? drafts : others

  const brandName = (id: string) => brands.find((b: Brand) => b.id === id)?.name || id.slice(0, 10)

  const handleToggleStatus = async (c: OptimizedContent) => {
    const next = c.status === 'published' ? 'draft' : 'published'
    try {
      await businessApi.adminSetContentStatus(c.id, next) // admin 旁路（全局上下架）
      message.success(`已${next === 'published' ? '发布到公开站（AI 引擎可爬取）' : '下架'}`)
      queryClient.invalidateQueries({ queryKey: ['admin-contents'] })
    } catch { message.error('状态流转失败') }
  }

  const handleDelete = async (c: OptimizedContent) => {
    try {
      await businessApi.adminDeleteContent(c.id) // admin 旁路（全局删除）
      message.success('内容已删除')
      queryClient.invalidateQueries({ queryKey: ['admin-contents'] })
    } catch { message.error('删除失败') }
  }

  const columns = [
    {
      title: '标题', dataIndex: 'title', key: 'title', ellipsis: true,
      render: (t: string, r: OptimizedContent) => (
        <Space direction="vertical" size={0}>
          <Text strong style={{ fontSize: 13.5 }}>{t || '(无标题)'}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>{r.id}</Text>
        </Space>
      ),
    },
    {
      title: '品牌', dataIndex: 'brand_id', key: 'brand_id', width: 120,
      render: (id: string) => <Tag>{brandName(id)}</Tag>,
    },
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 110,
      render: (s: string) => {
        const meta = statusMeta[s] || { color: 'default', label: s }
        return <Tag color={meta.color}>{meta.label}</Tag>
      },
    },
    {
      title: '收录', dataIndex: 'index_status', key: 'index_status', width: 100,
      render: (s: string, r: OptimizedContent) => {
        if (r.status !== 'published') return <Text type="secondary" style={{ fontSize: 12 }}>—</Text>
        if (!s) return <Tag color="default">未验证</Tag>
        const meta = indexMeta[s] || { color: 'default', label: s }
        return <Tag color={meta.color}>{meta.label}</Tag>
      },
    },
    {
      title: 'GEO 评分', dataIndex: ['score', 'total'], key: 'score', width: 100,
      render: (v: number, r: OptimizedContent) => (
        <Text strong style={{ color: scoreColor(v || r.score?.total || 0), fontSize: 14 }}>
          {(v || r.score?.total || 0).toFixed(0)}
        </Text>
      ),
    },
    {
      title: '版本', dataIndex: 'version', key: 'version', width: 70,
      render: (v: number) => <Text type="secondary">v{v}</Text>,
    },
    {
      title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 130,
      render: (t: string) => <Text type="secondary" style={{ fontSize: 12 }}>{t?.slice(0, 10)}</Text>,
    },
    {
      title: '操作', key: 'action', width: 220,
      render: (_: unknown, r: OptimizedContent) => (
        <Space size={4}>
          <Button size="small" type="text" icon={<EyeOutlined />} onClick={() => setPreview(r)}>预览</Button>
          <Button
            size="small" type="text" icon={<SwapOutlined />}
            onClick={() => handleToggleStatus(r)}
            style={{ color: r.status === 'published' ? 'var(--wr-warning)' : 'var(--wr-success)' }}
          >
            {r.status === 'published' ? '下架' : '发布'}
          </Button>
          {r.status === 'published' && (
            <Button size="small" type="link" icon={<GlobalOutlined />} href={`/public/articles/${r.id}`} target="_blank">
              公开页
            </Button>
          )}
          <Popconfirm title="删除该内容？" onConfirm={() => handleDelete(r)}>
            <Button size="small" type="text" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div className="wr-page-content">
      <div className="wr-page-header">
        <h1>内容管理</h1>
        <p>全平台优化内容 · 按状态分区管理 · 上下架控制 · 公开页预览</p>
      </div>

      {/* 统计卡（状态区分）*/}
      <Row gutter={[16, 16]} className="wr-stagger" style={{ marginBottom: 16 }}>
        <Col xs={12} md={6}>
          <div className="wr-metric-card">
            <div className="wr-metric-value">{filtered.length}</div>
            <div className="wr-metric-label"><FileTextOutlined style={{ marginRight: 4 }} />内容总数</div>
          </div>
        </Col>
        <Col xs={12} md={6}>
          <div className="wr-metric-card" onClick={() => { setActiveTab('published'); setBrandFilter(undefined) }} style={{ cursor: 'pointer' }}>
            <div className="wr-metric-value" style={{ color: 'var(--wr-success)' }}>{published.length}</div>
            <div className="wr-metric-label"><GlobalOutlined style={{ marginRight: 4 }} />已发布（公网可爬）</div>
          </div>
        </Col>
        <Col xs={12} md={6}>
          <div className="wr-metric-card" onClick={() => { setActiveTab('draft'); setBrandFilter(undefined) }} style={{ cursor: 'pointer' }}>
            <div className="wr-metric-value">{drafts.length}</div>
            <div className="wr-metric-label"><EditOutlined style={{ marginRight: 4 }} />草稿</div>
          </div>
        </Col>
        <Col xs={12} md={6}>
          <div className="wr-metric-card" onClick={() => { setActiveTab('other'); setBrandFilter(undefined) }} style={{ cursor: 'pointer' }}>
            <div className="wr-metric-value">{others.length}</div>
            <div className="wr-metric-label"><InboxOutlined style={{ marginRight: 4 }} />其他状态</div>
          </div>
        </Col>
      </Row>

      {/* 品牌筛选 */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 16 }}>
        <Select
          style={{ width: 200 }}
          placeholder="按品牌筛选"
          allowClear
          value={brandFilter}
          onChange={setBrandFilter}
          options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
        />
        <Text type="secondary" style={{ alignSelf: 'center', fontSize: 12 }}>
          {contents.length} 条内容
        </Text>
      </div>

      {/* 状态分区 Tabs（区分管理）*/}
      <div className="wr-glass-card" style={{ padding: 8 }}>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            { key: 'all', label: <Space>全部<Tag>{filtered.length}</Tag></Space> },
            { key: 'published', label: <Space>已发布<Tag color="success">{published.length}</Tag></Space> },
            { key: 'draft', label: <Space>草稿<Tag>{drafts.length}</Tag></Space> },
            { key: 'other', label: <Space>其他<Tag>{others.length}</Tag></Space> },
          ]}
          style={{ padding: '0 12px' }}
        />
        <Table
          dataSource={contents}
          columns={columns}
          rowKey="id"
          size="small"
          scroll={{ x: 'max-content' }}
          pagination={{ pageSize: 12, size: 'small' }}
          locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={activeTab === 'published' ? '暂无已发布内容——将内容「发布」后进入此区' : '暂无内容'} /> }}
        />
      </div>

      {/* 预览抽屉（与商户端共用组件——详情展示跨角色一致） */}
      <ContentPreviewDrawer content={preview} onClose={() => setPreview(null)} />
    </div>
  )
}
