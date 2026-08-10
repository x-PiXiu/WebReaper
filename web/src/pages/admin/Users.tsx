import { useState } from 'react'
import { Card, Table, Tag, Typography, Button, Modal, Form, Input, Space, message, Popconfirm } from 'antd'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { UserView } from '../../types/api'

const { Title, Text } = Typography

export default function AdminUsers() {
  const queryClient = useQueryClient()
  const [modalOpen, setModalOpen] = useState(false)
  const [form] = Form.useForm()

  const { data: users = [] } = useQuery({
    queryKey: ['admin-users'],
    queryFn: () => businessApi.listUsers(),
  })

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
      render: (u: string) => <Text strong style={{ fontFamily: 'monospace' }}>{u}</Text>,
    },
    {
      title: '角色', dataIndex: 'role', key: 'role', width: 100,
      render: (r: string) => <Tag color={r === 'admin' ? 'red' : 'blue'}>{r === 'admin' ? '管理员' : '商户'}</Tag>,
    },
    {
      title: '租户 ID', dataIndex: 'tenant_id', key: 'tenant_id',
      render: (t: string) => <Text type="secondary" style={{ fontFamily: 'monospace', fontSize: 12 }}>{t || '-'}</Text>,
    },
    {
      title: '', key: 'action', width: 80,
      render: (_: unknown, record: UserView) => {
        if (record.role === 'admin') return null
        return (
          <Popconfirm title={`删除商户「${record.username}」？`} onConfirm={() => handleDelete(record.id)}>
            <Button size="small" type="text" danger>删除</Button>
          </Popconfirm>
        )
      },
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div>
          <Title level={4} style={{ margin: 0 }}>商户管理</Title>
          <Text type="secondary" style={{ fontSize: 13 }}>管理平台用户与商户账号</Text>
        </div>
        <Button type="primary" onClick={() => setModalOpen(true)}>+ 创建商户</Button>
      </div>

      <Card>
        <Table dataSource={users} columns={columns} rowKey="id" pagination={false} size="middle" />
      </Card>

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
