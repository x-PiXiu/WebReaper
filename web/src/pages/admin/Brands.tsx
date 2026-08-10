import { Typography, Table, Tag, Space, Button, message, Popconfirm, Empty } from 'antd'
import { DeleteOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { Brand } from '../../types/api'

const { Text } = Typography

// 品牌统一管理（管理后台）：全平台品牌资产一览 + 详情 + 删除。
// admin 租户为空 → geo 接口返回全局数据（绝对控制落点）。
export default function AdminBrands() {
  const queryClient = useQueryClient()
  const { data: brands = [] } = useQuery({
    queryKey: ['admin-brands'],
    queryFn: () => businessApi.listBrands(),
  })

  const handleDelete = async (b: Brand) => {
    try {
      await businessApi.deleteBrand(b.id)
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
      title: '商户', dataIndex: 'tenant_id', key: 'tenant_id', width: 160,
      render: (t: string) => <Tag style={{ fontFamily: 'monospace' }}>{t || '全局'}</Tag>,
    },
    {
      title: '品牌定位', dataIndex: 'positioning', key: 'positioning', ellipsis: true,
      render: (p: string) => p || <Text type="secondary">—</Text>,
    },
    {
      title: '核心卖点', dataIndex: 'core_selling', key: 'core_selling', width: 220,
      render: (list: string[]) => (list || []).slice(0, 3).map((s, i) => <Tag key={i} style={{ fontSize: 11 }}>{s}</Tag>),
    },
    {
      title: '竞品', dataIndex: 'competitors', key: 'competitors', width: 180,
      render: (list: string[]) => (list || []).slice(0, 3).map((c, i) => <Tag key={i} style={{ fontSize: 11 }}>{c}</Tag>),
    },
    {
      title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 150,
      render: (t: string) => <Text type="secondary" style={{ fontSize: 12 }}>{t?.slice(0, 10)}</Text>,
    },
    {
      title: '操作', key: 'action', width: 100,
      render: (_: unknown, r: Brand) => (
        <Popconfirm title={`删除品牌「${r.name}」？其关键词与优化内容将一并删除`} onConfirm={() => handleDelete(r)}>
          <Button size="small" type="text" danger icon={<DeleteOutlined />}>删除</Button>
        </Popconfirm>
      ),
    },
  ]

  return (
    <div className="wr-page-content">
      <div className="wr-page-header">
        <h1>品牌统一管理</h1>
        <p>全平台品牌资产一览 · 绝对控制（删除将级联清理关键词与内容）</p>
      </div>

      <div className="wr-glass-card" style={{ padding: 8 }}>
        <Table
          dataSource={brands}
          columns={columns}
          rowKey="id"
          size="small"
          pagination={{ pageSize: 12, size: 'small' }}
          locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无品牌" /> }}
        />
      </div>
    </div>
  )
}
