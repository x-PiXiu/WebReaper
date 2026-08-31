import { Typography, Table, Tag, Space, Button, Popconfirm, Empty, Row, Col, Tooltip } from 'antd'
import { DeleteOutlined, AppstoreOutlined, TagOutlined, EnvironmentOutlined, BulbOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { Brand } from '../../types/api'
import { message } from '../../utils/antdApp'

const { Text } = Typography

// 品牌统一管理（管理后台）：全平台品牌资产一览 + 行展开详情 + 删除。
// admin 租户为空 → geo 接口返回全局数据（绝对控制落点）。
export default function AdminBrands({ embedded = false }: { embedded?: boolean }) {
  const queryClient = useQueryClient()
  const { data: brands = [] } = useQuery({
    queryKey: ['admin-brands'],
    queryFn: () => businessApi.adminListBrands(), // admin 旁路端点（全局，不走商户租户上下文）
  })

  const tenants = new Set(brands.map((b: Brand) => b.tenant_id)).size
  const totalSelling = brands.reduce((s: number, b: Brand) => s + (b.core_selling?.length || 0), 0)
  const totalCompetitors = brands.reduce((s: number, b: Brand) => s + (b.competitors?.length || 0), 0)

  const handleDelete = async (b: Brand) => {
    try {
      await businessApi.adminDeleteBrand(b.id) // admin 旁路（全局）
      message.success(`品牌「${b.name}」已删除（含关键词/内容）`)
      queryClient.invalidateQueries({ queryKey: ['admin-brands'] })
    } catch { message.error('删除失败') }
  }

  const columns = [
    {
      title: '品牌名', dataIndex: 'name', key: 'name',
      render: (n: string, r: Brand) => (
        <Space direction="vertical" size={0}>
          <Text strong style={{ fontSize: 13.5 }}>{n}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>{r.id}</Text>
        </Space>
      ),
    },
    {
      title: '商户', dataIndex: 'tenant_id', key: 'tenant_id', width: 130, ellipsis: true,
      render: (t: string) => <Tooltip title={t}><Tag style={{ fontFamily: 'monospace', fontSize: 11 }}>{t ? t.slice(0, 14) + '…' : '全局'}</Tag></Tooltip>,
    },
    {
      title: '品牌定位', dataIndex: 'positioning', key: 'positioning', ellipsis: true,
      render: (p: string) => p || <Text type="secondary">—</Text>,
    },
    {
      title: '核心卖点', dataIndex: 'core_selling', key: 'core_selling', width: 220,
      render: (list: string[]) => <Space wrap size={4}>{(list || []).slice(0, 3).map((s, i) => <Tag key={i} style={{ fontSize: 11, margin: 0 }}>{s}</Tag>)}</Space>,
    },
    {
      title: '竞品', dataIndex: 'competitors', key: 'competitors', width: 180,
      render: (list: string[]) => <Space wrap size={4}>{(list || []).slice(0, 3).map((c, i) => <Tag key={i} style={{ fontSize: 11, margin: 0 }}>{c}</Tag>)}</Space>,
    },
    {
      title: '行业', dataIndex: 'industry', key: 'industry', width: 100,
      render: (i: string) => i ? <Tag color="geekblue" style={{ fontSize: 11, margin: 0 }}>{i}</Tag> : <Text type="secondary" style={{ fontSize: 12 }}>—</Text>,
    },
    {
      title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 130,
      render: (t: string) => <Text type="secondary" style={{ fontSize: 12 }}>{t?.slice(0, 10)}</Text>,
    },
    {
      title: '操作', key: 'action', width: 90,
      render: (_: unknown, r: Brand) => (
        <Popconfirm title={`删除品牌「${r.name}」？其关键词与优化内容将一并删除`} onConfirm={() => handleDelete(r)}>
          <Button size="small" type="text" danger icon={<DeleteOutlined />}>删除</Button>
        </Popconfirm>
      ),
    },
  ]

  return (
    <div className={embedded ? "" : "wr-page-content"}>
      <div className="wr-page-header">
        <h1>品牌管理</h1>
        <p>全平台品牌资产一览 · 绝对控制（删除将级联清理关键词与内容）</p>
      </div>

      {/* 统计卡 */}
      <Row gutter={[16, 16]} className="wr-stagger" style={{ marginBottom: 16 }}>
        <Col xs={12} md={6}>
          <div className="wr-metric-card">
            <div className="wr-metric-value">{brands.length}</div>
            <div className="wr-metric-label"><AppstoreOutlined style={{ marginRight: 4 }} />品牌总数</div>
          </div>
        </Col>
        <Col xs={12} md={6}>
          <div className="wr-metric-card">
            <div className="wr-metric-value">{tenants}</div>
            <div className="wr-metric-label">覆盖商户数</div>
          </div>
        </Col>
        <Col xs={12} md={6}>
          <div className="wr-metric-card">
            <div className="wr-metric-value">{totalSelling}</div>
            <div className="wr-metric-label"><TagOutlined style={{ marginRight: 4 }} />核心卖点</div>
          </div>
        </Col>
        <Col xs={12} md={6}>
          <div className="wr-metric-card">
            <div className="wr-metric-value">{totalCompetitors}</div>
            <div className="wr-metric-label"><EnvironmentOutlined style={{ marginRight: 4 }} />竞品追踪</div>
          </div>
        </Col>
      </Row>

      <div className="wr-glass-card" style={{ padding: 8 }}>
        <Table
          dataSource={brands}
          columns={columns}
          rowKey="id"
          size="small"
          scroll={{ x: 'max-content' }}
          pagination={{ pageSize: 12, size: 'small' }}
          locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无品牌" /> }}
          expandable={{
            expandedRowRender: (r: Brand) => (
              <div style={{ padding: '4px 24px', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, fontSize: 13 }}>
                <div>
                  <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>
                    <BulbOutlined /> 品牌定位
                  </Text>
                  <Text style={{ color: 'var(--wr-text-secondary)', lineHeight: 1.7 }}>{r.positioning || '（未填写）'}</Text>
                </div>
                <div>
                  <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>完整卖点 / 竞品</Text>
                  <Space wrap size={6}>
                    {(r.core_selling || []).map((s, i) => <Tag key={'s' + i} style={{ fontSize: 11 }}>{s}</Tag>)}
                    {(r.competitors || []).map((c, i) => <Tag key={'c' + i} color="orange" style={{ fontSize: 11 }}>{c}</Tag>)}
                  </Space>
                </div>
              </div>
            ),
          }}
        />
      </div>
    </div>
  )
}
