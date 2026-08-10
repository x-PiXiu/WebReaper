import { useState } from 'react'
import { Table, Tag, Typography, Button, Modal, Form, Input, Space, message, Popconfirm, Row, Col, Empty } from 'antd'
import { PlusOutlined, TeamOutlined, CrownOutlined, ShopOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { UserView } from '../../types/api'

const { Text } = Typography

// 商户管理（管理后台）：平台商户一览 + 创建 + 删除。
export default function AdminUsers() {
  const queryClient = useQueryClient()
  const [modalOpen, setModalOpen] = useState(false)
  const [form] = Form.useForm()

  const { data: users = [] } = useQuery({
    queryKey: ['admin-users'],
    queryFn: () => businessApi.listUsers(),
  })

  const merchants = users.filter((u) => u.role !== 'admin')
  const admins = users.filter((u) => u.role === 'admin')

  const handleCreate = async (values: { username: string; password: string; tenant_id?: string }) => {
    try {
      await businessApi.createMerchant(values)
      message.success(`商户「${values.username}」创建成功`)
      setModalOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['admin-users'] })
    } catch {}
  }

  const handleDelete = async (id: string) => {
    try {
      await businessApi.deleteUser(id)
      message.success('已删除')
      queryClient.invalidateQueries({ queryKey: ['admin-users'] })
    } catch {}
  }

  const columns = [
    {
      title: '用户名', dataIndex: 'username', key: 'username',
      render: (u: string, r: UserView) => (
        <Space direction="vertical" size={0}>
          <Text strong style={{ fontFamily: 'monospace', fontSize: 13.5 }}>{u}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>{r.id}</Text>
        </Space>
      ),
    },
    {
      title: '角色', dataIndex: 'role', key: 'role', width: 110,
      render: (r: string) => <Tag color={r === 'admin' ? 'red' : 'blue'}>{r === 'admin' ? '管理员' : '商户'}</Tag>,
    },
    {
      title: '租户 ID', dataIndex: 'tenant_id', key: 'tenant_id',
      render: (t: string) => <Text type="secondary" style={{ fontFamily: 'monospace', fontSize: 12 }}>{t || '-'}</Text>,
    },
    {
      title: '操作', key: 'action', width: 90,
      render: (_: unknown, record: UserView) => {
        if (record.role === 'admin') return <Text type="secondary" style={{ fontSize: 12 }}>系统账号</Text>
        return (
          <Popconfirm title={`删除商户「${record.username}」？其品牌/关键词/内容将一并清理`} onConfirm={() => handleDelete(record.id)}>
            <Button size="small" type="text" danger>删除</Button>
          </Popconfirm>
        )
      },
    },
  ]

  return (
    <div className="wr-page-content">
      <div className="wr-page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h1>商户管理</h1>
          <p>管理平台用户与商户账号——商户的数据与资产全链路租户隔离</p>
        </div>
        <Button type="primary" size="large" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
          创建商户
        </Button>
      </div>

      {/* 统计卡 */}
      <Row gutter={[16, 16]} className="wr-stagger" style={{ marginBottom: 16 }}>
        <Col xs={12} md={6}>
          <div className="wr-metric-card">
            <div className="wr-metric-value wr-gradient-text">{users.length}</div>
            <div className="wr-metric-label"><TeamOutlined style={{ marginRight: 4 }} />全部用户</div>
          </div>
        </Col>
        <Col xs={12} md={6}>
          <div className="wr-metric-card">
            <div className="wr-metric-value">{merchants.length}</div>
            <div className="wr-metric-label"><ShopOutlined style={{ marginRight: 4 }} />商户数</div>
          </div>
        </Col>
        <Col xs={12} md={6}>
          <div className="wr-metric-card">
            <div className="wr-metric-value">{admins.length}</div>
            <div className="wr-metric-label"><CrownOutlined style={{ marginRight: 4 }} />管理员</div>
          </div>
        </Col>
        <Col xs={12} md={6}>
          <div className="wr-metric-card">
            <div className="wr-metric-value">{merchants.length * 1}</div>
            <div className="wr-metric-label">独立租户空间</div>
          </div>
        </Col>
      </Row>

      {/* 用户表格 */}
      <div className="wr-glass-card" style={{ padding: 8 }}>
        <Table
          dataSource={users}
          columns={columns}
          rowKey="id"
          pagination={false}
          size="small"
          locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无用户" /> }}
        />
      </div>

      <Modal title="创建商户账号" open={modalOpen} onCancel={() => setModalOpen(false)} footer={null} width={480}>
        <Form form={form} layout="vertical" onFinish={handleCreate} requiredMark={false}>
          <Form.Item label="用户名" name="username" rules={[{ required: true, min: 3, message: '至少 3 个字符' }]}>
            <Input placeholder="如 company-a" />
          </Form.Item>
          <Form.Item label="密码" name="password" rules={[{ required: true, min: 6, message: '至少 6 个字符' }]}>
            <Input.Password placeholder="至少 6 位" />
          </Form.Item>
          <Form.Item label="租户 ID（可选）" name="tenant_id" tooltip="留空则自动生成独立租户">
            <Input placeholder="留空自动生成" />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">创建</Button>
              <Button onClick={() => setModalOpen(false)}>取消</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
