import { Typography, Card, Row, Col, Tag, Button, Progress, Space, Table, Modal, Empty, Statistic } from 'antd'
import { CrownOutlined, CheckCircleOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { Plan } from '../../types/api'
import QueryBoundary from '../../components/QueryBoundary'
import { toast } from '../../utils/feedback'

const { Text, Title } = Typography

const PENDING_ORDER_KEY = 'wr_pending_order_id'

const yuan = (cents: number) => `¥${(cents / 100).toFixed(0)}`
const sceneLabels: Record<string, string> = {
  monitor: '效果监测',
  'content-gen': '内容合成',
  'content-opt': '内容优化',
  chat: 'AI 对话',
  'keyword-distill': '选题蒸馏',
  video: '视频生成',
  generation: '多媒体生成',
}

/** 支付回跳后确认开通（webhook 可能已先到，confirm 幂等） */
async function settlePaidOrder(orderId: string) {
  try {
    await businessApi.confirmOrder(orderId)
    return true
  } catch {
    // 网关未确认或 webhook 尚未完成时，订单可能仍 pending——交由轮询
    return false
  }
}

// 我的套餐（商户端）：当前套餐用量 + 升级购买 + 订单记录。
export default function MyPlan() {
  const queryClient = useQueryClient()
  const [buyModal, setBuyModal] = useState<Plan | null>(null)
  const [searchParams, setSearchParams] = useSearchParams()
  const settlingRef = useRef(false)

  const { data: usage, isLoading: usageLoading, isError: usageError, refetch: refetchUsage } = useQuery({ queryKey: ['my-usage'], queryFn: () => businessApi.getMyUsage() })
  const { data: plansRes, isLoading: plansLoading, isError: plansError, refetch: refetchPlans } = useQuery({ queryKey: ['active-plans'], queryFn: () => businessApi.listActivePlans() })
  const { data: ordersRes, refetch: refetchOrders } = useQuery({
    queryKey: ['my-orders'],
    queryFn: () => businessApi.listMyOrders(),
    // 支付回跳后短暂轮询，等待 webhook / confirm
    refetchInterval: (q) => {
      const list = q.state.data?.orders || []
      const pending = list.some((o) => o.status === 'pending')
      const watching = !!sessionStorage.getItem(PENDING_ORDER_KEY)
      return pending || watching ? 4000 : false
    },
  })

  const plans = plansRes?.plans || []
  const orders = ordersRes?.orders || []
  const currentPlan = usage?.plan
  const subscription = usage?.subscription

  const refreshBilling = () => {
    queryClient.invalidateQueries({ queryKey: ['my-usage'] })
    queryClient.invalidateQueries({ queryKey: ['my-orders'] })
  }

  // ZPay return_url / mock 回跳：带 out_trade_no 或 order；另用 session 记住待确认订单
  useEffect(() => {
    if (settlingRef.current) return
    const fromQuery = searchParams.get('out_trade_no') || searchParams.get('order') || ''
    const fromSession = sessionStorage.getItem(PENDING_ORDER_KEY) || ''
    const orderId = fromQuery || fromSession
    const tradeOK = searchParams.get('trade_status') === 'TRADE_SUCCESS' || searchParams.get('status') === '1'
    if (!orderId && !tradeOK) return

    settlingRef.current = true
    const run = async () => {
      toast.loading('正在确认支付结果…', 'pay-return')
      const id = orderId || fromSession
      let ok = false
      if (id) ok = await settlePaidOrder(id)
      // 无论 confirm 成败都刷新——异步 webhook 可能已开通
      await refetchOrders()
      refreshBilling()
      sessionStorage.removeItem(PENDING_ORDER_KEY)
      if (ok || tradeOK) {
        toast.ok('支付已确认，套餐额度已更新', 'pay-return')
      } else {
        toast.info('若已付款，额度将自动开通；也可在订单列表手动确认', 'pay-return', 5)
      }
      // 清掉回跳参数，避免刷新重复 confirm
      if (fromQuery || searchParams.has('trade_status')) {
        setSearchParams({}, { replace: true })
      }
      settlingRef.current = false
    }
    void run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const createOrderMut = useMutation({
    mutationFn: (planId: string) => businessApi.createOrder(planId),
    onSuccess: (res) => {
      const payURL = res.payment_url || ''
      if (!payURL) {
        toast.ok('订单已创建，等待管理员开通', 'pay-order')
        refreshBilling()
        setBuyModal(null)
        return
      }
      const isMockPay = payURL.includes('mock-pay') || /\/billing\/pay\?/.test(payURL)
      if (isMockPay) {
        toast.loading('演示支付处理中…', 'pay')
        setTimeout(() => {
          businessApi.confirmOrder(res.order.id).then(() => {
            toast.ok('支付成功，套餐已开通', 'pay')
            refreshBilling()
          }).catch(() => toast.fail('支付确认未完成，请稍后在订单列表重试', 'pay', 4))
        }, 800)
      } else {
        sessionStorage.setItem(PENDING_ORDER_KEY, res.order.id)
        window.open(payURL, '_blank', 'noopener,noreferrer')
        toast.info('已打开支付页，完成后回到本页即可确认', 'pay', 6)
        refreshBilling()
      }
      setBuyModal(null)
    },
    onError: () => { /* 拦截器已提示失败原因 */ },
  })

  const confirmMut = useMutation({
    mutationFn: (orderId: string) => businessApi.confirmOrder(orderId),
    onSuccess: () => {
      toast.ok('订单已确认开通', 'pay-confirm')
      sessionStorage.removeItem(PENDING_ORDER_KEY)
      refreshBilling()
    },
  })

  return (
    <QueryBoundary
      loading={usageLoading || plansLoading}
      error={usageError || plansError}
      onRetry={() => { refetchUsage(); refetchPlans() }}
    >
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">Billing</p>
          <h1>我的套餐</h1>
          <p className="ip-lead">当前套餐用量 · 升级续费 · 订单记录</p>
        </div>
      </div>

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

      <Card className="wr-glass-card" title="本月用量" style={{ marginBottom: 16 }}>
        {Object.keys(usage?.usages || {}).length === 0 ? (
          <Empty description="当前套餐无配额限制" />
        ) : (
          <Row gutter={[16, 16]}>
            {Object.entries(usage?.usages || {}).map(([scene, u]) => {
              const unlimited = u.limit === -1
              const pct = unlimited ? 0 : u.limit > 0 ? Math.min(100, (u.used / u.limit) * 100) : 0
              return (
                <Col xs={12} md={6} key={scene}>
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

      <Card className="wr-glass-card" title="可选套餐" style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          {plans.map(p => (
            <Col xs={24} md={8} key={p.id}>
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
                    </div>
                  }
                />
              </Card>
            </Col>
          ))}
        </Row>
      </Card>

      <Card className="wr-glass-card" title="订单记录">
        <Table dataSource={orders} rowKey="id" size="small" pagination={{ pageSize: 10 }}>
          <Table.Column title="订单号" dataIndex="id" key="id" render={(id) => <Text copyable style={{ fontSize: 12 }}>{id}</Text>} />
          <Table.Column title="套餐" dataIndex="plan_id" key="plan" render={(p) => <Tag>{plans.find(x => x.id === p)?.name || p}</Tag>} />
          <Table.Column title="金额" dataIndex="amount_cents" key="amount" width={100} render={(c) => <Text strong>{yuan(c)}</Text>} />
          <Table.Column title="状态" dataIndex="status" key="status" width={90} render={(s) => <Tag color={s === 'paid' ? 'success' : s === 'pending' ? 'processing' : 'default'}>{s}</Tag>} />
          <Table.Column title="时间" dataIndex="created_at" key="time" width={140} render={(t) => <Text type="secondary" style={{ fontSize: 12 }}>{t?.slice(0, 16).replace('T', ' ')}</Text>} />
          <Table.Column
            title="操作"
            key="action"
            width={100}
            render={(_: unknown, r: { id: string; status: string }) => (
              r.status === 'pending' ? (
                <Button type="link" size="small" loading={confirmMut.isPending} onClick={() => confirmMut.mutate(r.id)}>
                  确认开通
                </Button>
              ) : null
            )}
          />
        </Table>
      </Card>

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
            <CrownOutlined style={{ fontSize: 40, color: 'var(--wr-primary)' }} />
            <Title level={4} style={{ marginTop: 12 }}>{buyModal.name}</Title>
            <Title level={2} style={{ margin: '8px 0' }}>{yuan(buyModal.price_cents)}<Text type="secondary" style={{ fontSize: 14 }}>/月</Text></Title>
            <Space direction="vertical" size={4}>
              {Object.entries(buyModal.quotas || {}).map(([k, v]) => (
                <Text key={k} type="secondary"><CheckCircleOutlined style={{ marginRight: 6, color: 'var(--wr-success)' }} />{sceneLabels[k] || k}：{v === -1 ? '无限' : v} 次/月</Text>
              ))}
            </Space>
          </div>
        )}
      </Modal>
    </div>
    </QueryBoundary>
  )
}
