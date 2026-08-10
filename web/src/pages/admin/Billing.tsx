import { Typography, Table, Tag, Space, Button, message, Popconfirm, Card, Row, Col, Statistic, Modal, Input, InputNumber, Form, Select, Divider, Tabs } from 'antd'
import { DollarOutlined, CrownOutlined, TeamOutlined, RiseOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { Plan, Subscription } from '../../types/api'

const { Text } = Typography

const yuan = (cents: number) => `¥${(cents / 100).toFixed(2)}`
const statusColor: Record<string, string> = { active: 'success', expired: 'default', cancelled: 'error', pending: 'processing', paid: 'success', failed: 'error', refunded: 'warning' }

// 计费管理（管理后台）：收入概览 + 套餐管理 + 订阅/订单全局视图。
export default function AdminBilling() {
  const queryClient = useQueryClient()
  const [tab, setTab] = useState('overview')
  const [planModal, setPlanModal] = useState<Plan | null>(null)
  const [assignModal, setAssignModal] = useState<{ open: boolean; tenant: string; planId: string }>({ open: false, tenant: '', planId: '' })
  const [form] = Form.useForm()

  const { data: revenue } = useQuery({ queryKey: ['billing-revenue'], queryFn: () => businessApi.adminRevenueReport() })
  const { data: plansRes } = useQuery({ queryKey: ['admin-plans'], queryFn: () => businessApi.adminListPlans() })
  const { data: subsRes } = useQuery({ queryKey: ['admin-subs'], queryFn: () => businessApi.adminListSubscriptions() })
  const { data: ordersRes } = useQuery({ queryKey: ['admin-orders'], queryFn: () => businessApi.adminListOrders() })

  const plans = plansRes?.plans || []
  const subs = subsRes?.subscriptions || []
  const orders = ordersRes?.orders || []

  const savePlanMut = useMutation({
    mutationFn: (p: Plan) => businessApi.adminSavePlan(p),
    onSuccess: () => { message.success('套餐已保存'); setPlanModal(null); queryClient.invalidateQueries({ queryKey: ['admin-plans'] }) },
    onError: () => message.error('保存失败'),
  })
  const deletePlanMut = useMutation({
    mutationFn: (id: string) => businessApi.adminDeletePlan(id),
    onSuccess: () => { message.success('套餐已删除'); queryClient.invalidateQueries({ queryKey: ['admin-plans'] }) },
  })
  const assignMut = useMutation({
    mutationFn: ({ tenant, planId }: { tenant: string; planId: string }) => businessApi.adminAssignPlan(tenant, planId),
    onSuccess: () => { message.success('套餐已开通'); setAssignModal({ open: false, tenant: '', planId: '' }); queryClient.invalidateQueries({ queryKey: ['admin-subs'] }) },
    onError: () => message.error('开通失败'),
  })

  const openNewPlan = () => {
    form.resetFields()
    setPlanModal({ id: '', name: '', level: 'free', price_cents: 0, quotas: {}, features: [], status: 'active', sort_order: 0, created_at: '', updated_at: '' } as Plan)
  }

  const openEditPlan = (p: Plan) => {
    form.setFieldsValue({
      ...p,
      features_text: p.features?.join('\n') || '',
      quotas_text: Object.entries(p.quotas || {}).map(([k, v]) => `${k}:${v}`).join('\n'),
    })
    setPlanModal(p)
  }

  const handleSavePlan = async () => {
    const v = await form.validateFields()
    const quotas: Record<string, number> = {}
    ;(v.quotas_text || '').split('\n').filter(Boolean).forEach((line: string) => {
      const [k, val] = line.split(':').map(s => s.trim())
      if (k) quotas[k] = parseInt(val || '0', 10)
    })
    const features = (v.features_text || '').split('\n').map((s: string) => s.trim()).filter(Boolean)
    savePlanMut.mutate({
      id: planModal?.id || `plan-${v.level}-${Date.now()}`,
      name: v.name, level: v.level, price_cents: v.price_cents || 0,
      quotas, features, status: v.status || 'active', sort_order: v.sort_order || 0,
      created_at: planModal?.created_at || '', updated_at: '',
    })
  }

  const planName = (id: string) => plans.find(p => p.id === id)?.name || id

  return (
    <div className="wr-page-content">
      <div className="wr-page-header">
        <h1>计费管理</h1>
        <p>收入概览 · 套餐管理 · 订阅与订单</p>
      </div>

      <Tabs
        activeKey={tab}
        onChange={setTab}
        items={[
          {
            key: 'overview',
            label: '收入概览',
            children: (
              <div>
                <Row gutter={16}>
                  <Col span={6}><Card><Statistic title="累计收入" value={yuan(revenue?.total_revenue_cents || 0)} prefix={<DollarOutlined />} /></Card></Col>
                  <Col span={6}><Card><Statistic title="当月收入" value={yuan(revenue?.month_revenue_cents || 0)} prefix={<RiseOutlined />} /></Card></Col>
                  <Col span={6}><Card><Statistic title="已支付订单" value={revenue?.paid_orders || 0} prefix={<CrownOutlined />} /></Card></Col>
                  <Col span={6}><Card><Statistic title="有效订阅" value={revenue?.active_subscriptions || 0} prefix={<TeamOutlined />} /></Card></Col>
                </Row>
                <Card title="套餐分布" style={{ marginTop: 16 }}>
                  {Object.entries(revenue?.plan_distribution || {}).length === 0 ? (
                    <Text type="secondary">暂无有效订阅</Text>
                  ) : (
                    <Space wrap>
                      {Object.entries(revenue?.plan_distribution || {}).map(([pid, n]) => (
                        <Tag key={pid} color="blue">{planName(pid)}: {n} 个订阅</Tag>
                      ))}
                    </Space>
                  )}
                </Card>
              </div>
            ),
          },
          {
            key: 'plans',
            label: '套餐管理',
            children: (
              <div>
                <div style={{ marginBottom: 16 }}><Button type="primary" onClick={openNewPlan}>新建套餐</Button></div>
                <Table dataSource={plans} rowKey="id" size="small" pagination={false}>
                  <Table.Column title="套餐" dataIndex="name" key="name" render={(n, r: Plan) => <Space direction="vertical" size={0}><Text strong>{n}</Text><Text type="secondary">{r.id}</Text></Space>} />
                  <Table.Column title="层级" dataIndex="level" key="level" width={80} render={(l) => <Tag color={l === 'team' ? 'gold' : l === 'pro' ? 'purple' : 'default'}>{l}</Tag>} />
                  <Table.Column title="月费" dataIndex="price_cents" key="price" width={100} render={(c) => <Text strong>{yuan(c)}</Text>} />
                  <Table.Column title="配额" dataIndex="quotas" key="quotas" render={(q: Record<string, number>) => (
                    <Space wrap size={4}>
                      {Object.entries(q || {}).map(([k, v]) => <Tag key={k} style={{ fontSize: 11 }}>{k}: {v === -1 ? '∞' : v}</Tag>)}
                    </Space>
                  )} />
                  <Table.Column title="状态" dataIndex="status" key="status" width={80} render={(s) => <Tag color={statusColor[s] || 'default'}>{s}</Tag>} />
                  <Table.Column title="操作" key="action" width={140} render={(_: unknown, r: Plan) => (
                    <Space size={4}>
                      <Button size="small" type="link" onClick={() => openEditPlan(r)}>编辑</Button>
                      <Popconfirm title="删除该套餐？" onConfirm={() => deletePlanMut.mutate(r.id)}>
                        <Button size="small" type="link" danger>删除</Button>
                      </Popconfirm>
                    </Space>
                  )} />
                </Table>
              </div>
            ),
          },
          {
            key: 'subs',
            label: '订阅列表',
            children: (
              <div>
                <div style={{ marginBottom: 16 }}>
                  <Button type="primary" onClick={() => setAssignModal({ open: true, tenant: '', planId: '' })}>手动开通套餐</Button>
                </div>
                <Table dataSource={subs} rowKey="id" size="small" pagination={{ pageSize: 20 }}>
                  <Table.Column title="租户" dataIndex="tenant_id" key="tenant" render={(t) => <Text copyable style={{ fontSize: 12 }}>{t}</Text>} />
                  <Table.Column title="套餐" dataIndex="plan_id" key="plan" render={(p) => <Tag color="blue">{planName(p)}</Tag>} />
                  <Table.Column title="状态" dataIndex="status" key="status" width={100} render={(s) => <Tag color={statusColor[s]}>{s}</Tag>} />
                  <Table.Column title="计费周期" key="period" width={200} render={(_: unknown, r: Subscription) => (
                    <Text type="secondary" style={{ fontSize: 12 }}>{r.period_start?.slice(0, 10)} ~ {r.period_end?.slice(0, 10)}</Text>
                  )} />
                  <Table.Column title="操作" key="action" width={100} render={(_: unknown, r: Subscription) => (
                    <Button size="small" type="link" onClick={() => setAssignModal({ open: true, tenant: r.tenant_id, planId: r.plan_id })}>变更</Button>
                  )} />
                </Table>
              </div>
            ),
          },
          {
            key: 'orders',
            label: '订单流水',
            children: (
              <Table dataSource={orders} rowKey="id" size="small" pagination={{ pageSize: 20 }}>
                <Table.Column title="订单号" dataIndex="id" key="id" render={(id) => <Text copyable style={{ fontSize: 12 }}>{id}</Text>} />
                <Table.Column title="租户" dataIndex="tenant_id" key="tenant" render={(t) => <Text style={{ fontSize: 12 }}>{t?.slice(0, 16)}…</Text>} />
                <Table.Column title="套餐" dataIndex="plan_id" key="plan" render={(p) => <Tag>{planName(p)}</Tag>} />
                <Table.Column title="金额" dataIndex="amount_cents" key="amount" width={100} render={(c) => <Text strong>{yuan(c)}</Text>} />
                <Table.Column title="状态" dataIndex="status" key="status" width={90} render={(s) => <Tag color={statusColor[s]}>{s}</Tag>} />
                <Table.Column title="支付方式" dataIndex="payment_gateway" key="gw" width={90} />
                <Table.Column title="创建时间" dataIndex="created_at" key="created" width={130} render={(t) => <Text type="secondary" style={{ fontSize: 12 }}>{t?.slice(0, 16).replace('T', ' ')}</Text>} />
              </Table>
            ),
          },
        ]}
      />

      {/* 套餐编辑 Modal */}
      <Modal title={planModal?.id ? '编辑套餐' : '新建套餐'} open={!!planModal} onOk={handleSavePlan} onCancel={() => setPlanModal(null)} width={560} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="套餐名称" rules={[{ required: true }]}><Input placeholder="专业版" /></Form.Item>
          <Form.Item name="level" label="层级" rules={[{ required: true }]}>
            <Select options={[{ value: 'free' }, { value: 'pro' }, { value: 'team' }]} />
          </Form.Item>
          <Form.Item name="price_cents" label="月费（分）"><InputNumber min={0} style={{ width: '100%' }} placeholder="29900 = ¥299" /></Form.Item>
          <Form.Item name="sort_order" label="排序"><InputNumber min={0} style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="quotas_text" label="配额（每行 格式：场景:数量，-1=无限）" tooltip="如 monitor:500&#10;content-gen:50">
            <Input.TextArea rows={4} placeholder={'monitor:500\ncontent-gen:50\nchat:-1'} />
          </Form.Item>
          <Form.Item name="features_text" label="功能白名单（每行一个）"><Input.TextArea rows={3} placeholder={'auto-monitor\nvideo\nmulti-account'} /></Form.Item>
          <Form.Item name="status" label="状态">
            <Select options={[{ value: 'active' }, { value: 'archived' }]} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 手动开通 Modal */}
      <Modal title="手动开通套餐" open={assignModal.open} onOk={() => assignMut.mutate({ tenant: assignModal.tenant, planId: assignModal.planId })} onCancel={() => setAssignModal({ open: false, tenant: '', planId: '' })}>
        <Divider />
        <Form layout="vertical">
          <Form.Item label="租户 ID" required>
            <Input value={assignModal.tenant} onChange={(e) => setAssignModal({ ...assignModal, tenant: e.target.value })} placeholder="tenant-user-xxx" />
          </Form.Item>
          <Form.Item label="套餐" required>
            <Select value={assignModal.planId || undefined} onChange={(v) => setAssignModal({ ...assignModal, planId: v })} options={plans.map(p => ({ value: p.id, label: `${p.name} (${yuan(p.price_cents)})` }))} placeholder="选择套餐" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
