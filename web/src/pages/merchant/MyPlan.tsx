import { Typography, Card, Row, Col, Tag, Button, Progress, Space, Table, message, Modal, Empty, Statistic } from 'antd'
import { CrownOutlined, CheckCircleOutlined, ThunderboltOutlined } from '@ant-design/icons'
import PageLoading from '../../components/PageLoading'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { Plan } from '../../types/api'

const { Text, Title } = Typography

const yuan = (cents: number) => `¥${(cents / 100).toFixed(0)}`
const sceneLabels: Record<string, string> = {
  monitor: '效果监测',
  'content-gen': '内容合成',
  'content-opt': '内容优化',
  chat: 'AI 对话',
  'keyword-distill': '选题蒸馏',
  video: '视频生成',
}

// 我的套餐（商户端）：当前套餐用量 + 升级购买 + 订单记录。
export default function MyPlan() {
  const queryClient = useQueryClient()
  const [buyModal, setBuyModal] = useState<Plan | null>(null)

  const { data: usage, isLoading: usageLoading } = useQuery({ queryKey: ['my-usage'], queryFn: () => businessApi.getMyUsage() })
  const { data: plansRes, isLoading: plansLoading } = useQuery({ queryKey: ['active-plans'], queryFn: () => businessApi.listActivePlans() })
  const { data: ordersRes } = useQuery({ queryKey: ['my-orders'], queryFn: () => businessApi.listMyOrders() })

  const plans = plansRes?.plans || []
  const orders = ordersRes?.orders || []
  const currentPlan = usage?.plan
  const subscription = usage?.subscription

  const createOrderMut = useMutation({
    mutationFn: (planId: string) => businessApi.createOrder(planId),
    onSuccess: (res) => {
      if (res.payment_url) {
        message.loading({ content: '正在处理支付…', key: 'pay', duration: 1 })
        setTimeout(() => {
          businessApi.confirmOrder(res.order.id).then(() => {
            message.success({ content: '支付成功，套餐已开通', key: 'pay' })
            queryClient.invalidateQueries({ queryKey: ['my-usage'] })
            queryClient.invalidateQueries({ queryKey: ['my-orders'] })
          }).catch(() => message.error({ content: '支付确认未完成，请稍后在订单列表重试', key: 'pay', duration: 4 }))
        }, 1200)
      } else {
        message.success('订单已创建（线下模式，等待管理员开通）')
        queryClient.invalidateQueries({ queryKey: ['my-orders'] })
      }
      setBuyModal(null)
    },
    onError: () => { /* 拦截器已提示失败原因 */ },
  })

  if (usageLoading || plansLoading) {
    return (
      <div className="wr-page-content" style={{ paddingTop: 8 }}>
        <PageLoading tip="套餐信息加载中..." />
      </div>
    )
  }

  return (
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">Billing</p>
          <h1>我的套餐</h1>
          <p className="ip-lead">当前套餐用量 · 升级续费 · 订单记录</p>
        </div>
      </div>

      {/* 当前套餐卡片 */}
      <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
        <Row gutter={24} align="middle">
          <Col>
            <Space direction="vertical" size={0}>
              <Space>
                <CrownOutlined style={{ fontSize: 24, color: currentPlan?.level === 'team' ? '#faad14' : currentPlan?.level === 'pro' ? 'var(--wr-primary)' : '#8c8c8c' }} />
                <Title level={4} style={{ margin: 0 }}>{currentPlan?.name || '免费版'}</Title>
                <Tag color={currentPlan?.level === 'team' ? 'gold' : currentPlan?.level === 'pro' ? 'cyan' : 'default'}>{currentPlan?.level || 'free'}</Tag>
              </Space>
              {subscription ? (
                <Text type="secondary">有效期至 {subscription.period_end?.slice(0, 10)}</Text>
              ) : (
                <Text type="secondary">未开通付费套餐，使用免费额度</Text>
              )}
            </Space>
          </Col>
          <Col flex="auto" />
          <Col>
            <Button type="primary" size="large" className="ip-btn-primary" icon={<ThunderboltOutlined />} onClick={() => setBuyModal(plans.find(p => p.level === 'pro') || plans[0])}>
              {subscription ? '续费 / 升级' : '立即开通'}
            </Button>
          </Col>
        </Row>
      </Card>

      {/* 用量进度 */}
      <Card className="wr-glass-card" title="本月用量" style={{ marginBottom: 16 }}>
        {Object.keys(usage?.usages || {}).length === 0 ? (
          <Empty description="当前套餐无配额限制" />
        ) : (
          <Row gutter={[16, 16]}>
            {Object.entries(usage?.usages || {}).map(([scene, u]) => {
              const unlimited = u.limit === -1
              const pct = unlimited ? 0 : u.limit > 0 ? Math.min(100, (u.used / u.limit) * 100) : 0
              return (
                <Col span={6} key={scene}>
                  <Card size="small">
                    <Statistic
                      title={sceneLabels[scene] || scene}
                      value={unlimited ? '∞' : u.used}
                      suffix={unlimited ? '' : `/ ${u.limit}`}
                    />
                    {!unlimited && (
                      <Progress
                        percent={pct}
                        size="small"
                        status={pct >= 100 ? 'exception' : pct >= 80 ? 'active' : 'normal'}
                        style={{ marginTop: 8 }}
                      />
                    )}
                  </Card>
                </Col>
              )
            })}
          </Row>
        )}
      </Card>

      {/* 可选套餐 */}
      <Card className="wr-glass-card" title="可选套餐" style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          {plans.map(p => (
            <Col span={8} key={p.id}>
              <Card
                size="small"
                hoverable
                actions={[<Button type={p.id === currentPlan?.id ? 'default' : 'primary'} size="small" disabled={p.id === currentPlan?.id} onClick={() => setBuyModal(p)}>{p.id === currentPlan?.id ? '当前套餐' : yuan(p.price_cents) === '¥0' ? '使用' : '购买'}</Button>]}
              >
                <Card.Meta
                  title={<Space><Text strong>{p.name}</Text><Tag color={p.level === 'team' ? 'gold' : p.level === 'pro' ? 'purple' : 'default'}>{p.level}</Tag></Space>}
                  description={
                    <div>
                      <Title level={3} style={{ margin: '8px 0' }}>{yuan(p.price_cents)}<Text type="secondary" style={{ fontSize: 14 }}>/月</Text></Title>
                      <Space direction="vertical" size={2}>
                        {Object.entries(p.quotas || {}).map(([k, v]) => (
                          <Text key={k} type="secondary" style={{ fontSize: 12 }}>{sceneLabels[k] || k}：{v === -1 ? '无限' : v} 次/月</Text>
                        ))}
                      </Space>
                      <Text type="secondary" style={{ fontSize: 11 }}>
                        配额按调用次数计：合成、发布与对话等均计入对应额度
                      </Text>
                    </div>
                  }
                />
              </Card>
            </Col>
          ))}
        </Row>
      </Card>

      {/* 订单记录 */}
      <Card className="wr-glass-card" title="订单记录">
        <Table dataSource={orders} rowKey="id" size="small" pagination={{ pageSize: 10 }}>
          <Table.Column title="订单号" dataIndex="id" key="id" render={(id) => <Text copyable style={{ fontSize: 12 }}>{id}</Text>} />
          <Table.Column title="套餐" dataIndex="plan_id" key="plan" render={(p) => <Tag>{plans.find(x => x.id === p)?.name || p}</Tag>} />
          <Table.Column title="金额" dataIndex="amount_cents" key="amount" width={100} render={(c) => <Text strong>{yuan(c)}</Text>} />
          <Table.Column title="状态" dataIndex="status" key="status" width={90} render={(s) => <Tag color={s === 'paid' ? 'success' : s === 'pending' ? 'processing' : 'default'}>{s}</Tag>} />
          <Table.Column title="时间" dataIndex="created_at" key="time" width={140} render={(t) => <Text type="secondary" style={{ fontSize: 12 }}>{t?.slice(0, 16).replace('T', ' ')}</Text>} />
        </Table>
      </Card>

      {/* 购买确认 Modal */}
      <Modal
        title="确认购买"
        open={!!buyModal}
        onOk={() => buyModal && createOrderMut.mutate(buyModal.id)}
        onCancel={() => setBuyModal(null)}
        okText="确认支付"
        confirmLoading={createOrderMut.isPending}
      >
        {buyModal && (
          <div style={{ textAlign: 'center', padding: '16px 0' }}>
            <CrownOutlined style={{ fontSize: 40, color: '#722ed1' }} />
            <Title level={4} style={{ marginTop: 12 }}>{buyModal.name}</Title>
            <Title level={2} style={{ margin: '8px 0' }}>{yuan(buyModal.price_cents)}<Text type="secondary" style={{ fontSize: 14 }}>/月</Text></Title>
            <Space direction="vertical" size={4}>
              {Object.entries(buyModal.quotas || {}).map(([k, v]) => (
                <Text key={k} type="secondary"><CheckCircleOutlined style={{ marginRight: 6, color: '#52c41a' }} />{sceneLabels[k] || k}：{v === -1 ? '无限' : v} 次/月</Text>
              ))}
            </Space>
          </div>
        )}
      </Modal>
    </div>
  )
}
