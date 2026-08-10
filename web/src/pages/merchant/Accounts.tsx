import { useState } from 'react'
import { Card, Typography, Button, Row, Col, Tag, Space, Popconfirm, Modal, Spin, message, Empty, Table } from 'antd'
const { Title, Text, Paragraph } = Typography
import { CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined, LinkOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { Account } from '../../types/api'

// 平台元信息（名称 + 主色用于卡片标识）
const PLATFORMS = [
  { key: 'zhihu', name: '知乎', color: 'var(--wr-primary)', desc: '知识问答社区，长文 SEO 效果好' },
  { key: 'xiaohongshu', name: '小红书', color: 'var(--wr-danger)', desc: '种草社区，本地生活/装修类精准触达' },
]

// 健康度 → 显示配置
function healthConfig(health: string) {
  switch (health) {
    case 'active':
      return { color: 'var(--wr-success)', label: '健康', icon: <CheckCircleOutlined /> }
    case 'expired':
      return { color: 'var(--wr-warning)', label: '已过期', icon: <ClockCircleOutlined /> }
    case 'banned':
      return { color: 'var(--wr-danger)', label: '已封禁', icon: <CloseCircleOutlined /> }
    default:
      return { color: 'var(--wr-text-muted)', label: '未知', icon: <ClockCircleOutlined /> }
  }
}

export default function Accounts() {
  const queryClient = useQueryClient()
  const [qrModalOpen, setQrModalOpen] = useState(false)
  const [activePlatform, setActivePlatform] = useState<string>('')
  const [sessionId, setSessionId] = useState<string>('')
  const [qrImage, setQrImage] = useState<string>('')
  const [loginMethod, setLoginMethod] = useState<string>('')

  const { data: accounts = [], isLoading } = useQuery({
    queryKey: ['geo-accounts'],
    queryFn: () => businessApi.listAccounts(),
  })

  // 扫码状态轮询（条件轮询：仅在会话进行中时每 2s 轮询）
  const { data: pollData } = useQuery({
    queryKey: ['qr-status', sessionId, activePlatform],
    queryFn: () => businessApi.pollQRLogin(sessionId, activePlatform, loginMethod),
    enabled: !!sessionId && qrModalOpen,
    refetchInterval: (query) => {
      const s = query.state.data?.status
      // preparing / waiting / scanned → 继续轮询；success / expired / error → 停止
      return s && (s === 'preparing' || s === 'waiting' || s === 'scanned') ? 2000 : false
    },
  })

  // 轮询拿到二维码图片时更新本地状态
  const currentQrImage = pollData?.qr_image || qrImage

  // 按平台分组账号
  const accountsByPlatform = (platform: string) => accounts.filter((a) => a.platform === platform)

  // 打开绑定弹窗：知乎有多种登录方式先选方式，小红书直接启动
  const openBindModal = (platform: string) => {
    setActivePlatform(platform)
    setLoginMethod('')
    setQrImage('')
    setSessionId('')
    setQrModalOpen(true)
    if (platform === 'xiaohongshu') {
      // 小红书只有一种方式，直接启动
      handleStartQR(platform)
    }
    // 知乎先显示方式选择按钮，用户点击后再启动
  }

  // 启动扫码登录（异步：只启动浏览器，二维码通过轮询获取）
  const handleStartQR = async (platform: string, method?: string) => {
    setActivePlatform(platform)
    setLoginMethod(method || '')
    setQrImage('')
    setSessionId('')
    setQrModalOpen(true)
    try {
      const res = await businessApi.startQRLogin(platform, method)
      setSessionId(res.session_id)
    } catch (e) {
      message.error('启动扫码失败：' + ((e as Error)?.message || '浏览器自动化可能未配置'))
      setQrModalOpen(false)
    }
  }

  // 关闭弹窗时取消会话
  const handleCloseModal = async () => {
    if (sessionId) {
      try { await businessApi.cancelQRLogin(sessionId) } catch {}
    }
    setQrModalOpen(false)
    setSessionId('')
    setQrImage('')
  }

  // 监听轮询结果——登录成功
  if (pollData?.status === 'success' && qrModalOpen) {
    const pfName = PLATFORMS.find((p) => p.key === activePlatform)?.name || '账号'
    const accName = pollData.account_name || pfName
    const expTime = pollData.expires_at ? new Date(pollData.expires_at).toLocaleString() : ''
    message.success(`${pfName}「${accName}」绑定成功${expTime ? `，有效期至 ${expTime}` : ''}`)
    setQrModalOpen(false)
    setSessionId('')
    setQrImage('')
    queryClient.invalidateQueries({ queryKey: ['geo-accounts'] })
  }

  // 解绑账号
  const handleDelete = async (id: string) => {
    try {
      await businessApi.deleteAccount(id)
      message.success('已解绑')
      queryClient.invalidateQueries({ queryKey: ['geo-accounts'] })
    } catch {}
  }

  const pollStatus = pollData?.status

  const accountColumns = [
    {
      title: '平台', dataIndex: 'platform', key: 'platform', width: 120,
      render: (p: string) => {
        const pf = PLATFORMS.find((x) => x.key === p)
        return <Tag color={pf?.color}>{pf?.name || p}</Tag>
      },
    },
    { title: '账号', dataIndex: 'display_name', key: 'name', render: (n: string) => <Text strong>{n || '-'}</Text> },
    {
      title: '登录方式', dataIndex: 'login_method', key: 'method', width: 100,
      render: (m: string) => {
        const labels: Record<string,string> = { zhihu: '知乎App', wechat: '微信', qq: 'QQ', weibo: '微博', xiaohongshu: '小红书' }
        return <Tag>{labels[m] || m || '-'}</Tag>
      },
    },
    {
      title: '状态', dataIndex: 'health', key: 'health', width: 120,
      render: (h: string) => {
        const cfg = healthConfig(h)
        return <Space><span style={{ color: cfg.color }}>{cfg.icon}</span><Text style={{ color: cfg.color }}>{cfg.label}</Text></Space>
      },
    },
    {
      title: '过期时间', dataIndex: 'expires_at', key: 'expires', width: 180,
      render: (t: string) => {
        if (!t) return <Text type="secondary">-</Text>
        const exp = new Date(t)
        const isExpired = exp < new Date()
        return <Text type={isExpired ? 'danger' : 'secondary'} style={{ fontSize: 12 }}>{exp.toLocaleString()}</Text>
      },
    },
    {
      title: '最后使用', dataIndex: 'last_used_at', key: 'last_used', width: 180,
      render: (t: string) => <Text type="secondary" style={{ fontSize: 12 }}>{t ? new Date(t).toLocaleString() : '-'}</Text>,
    },
    {
      title: '操作', key: 'action', width: 100,
      render: (_: unknown, r: Account) => (
        <Popconfirm title="确定解绑此账号？" onConfirm={() => handleDelete(r.id)}>
          <Button size="small" type="text" danger>解绑</Button>
        </Popconfirm>
      ),
    },
  ]

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ paddingTop: 8, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        {/* Hero 区 */}
        <div style={{ marginBottom: 28 }}>
          <Title level={3} style={{ margin: 0, fontSize: 28, letterSpacing: '-0.03em' }}>
            账号管理
          </Title>
          <Text type="secondary" style={{ fontSize: 14 }}>
            绑定社媒平台账号，支持扫码登录与 cookie 自动续期
          </Text>
        </div>

        {/* 平台账号卡片 */}
        <Row gutter={[16, 16]} className="wr-stagger" style={{ marginBottom: 16 }}>
          {PLATFORMS.map((pf) => {
            const accs = accountsByPlatform(pf.key)
            return (
              <Col xs={24} sm={12} key={pf.key}>
                <Card className="wr-glass-card" styles={{ body: { padding: 24 } }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
                    <div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                        <span style={{ width: 8, height: 8, borderRadius: '50%', background: pf.color, display: 'inline-block' }} />
                        <Text strong style={{ fontSize: 18 }}>{pf.name}</Text>
                      </div>
                      <Text type="secondary" style={{ fontSize: 12 }}>{pf.desc}</Text>
                    </div>
                    {accs.length > 0 && <Tag>{accs.length} 个账号</Tag>}
                  </div>

                  {accs.length > 0 ? (
                    <Space direction="vertical" size={8} style={{ width: '100%' }}>
                      {accs.map((a) => {
                        const cfg = healthConfig(a.health)
                        const isExpired = a.health === 'expired'
                        return (
                          <div key={a.id} style={{
                            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                            padding: '8px 12px', background: 'var(--wr-bg-elevated)', borderRadius: 8,
                            borderLeft: isExpired ? '3px solid var(--wr-danger)' : '3px solid transparent',
                          }}>
                            <Space>
                              <span style={{ color: cfg.color }}>{cfg.icon}</span>
                              <Text style={{ color: isExpired ? 'var(--wr-text-muted)' : 'inherit' }}>{a.display_name}</Text>
                            </Space>
                            <Space size={8}>
                              <Text style={{ fontSize: 12, color: cfg.color }}>{cfg.label}</Text>
                              {isExpired ? (
                                <Button size="small" type="primary" danger onClick={() => openBindModal(pf.key)}>
                                  重新绑定
                                </Button>
                              ) : null}
                              <Popconfirm title="确定解绑？" onConfirm={() => handleDelete(a.id)}>
                                <Button size="small" type="text" danger>解绑</Button>
                              </Popconfirm>
                            </Space>
                          </div>
                        )
                      })}
                      <Button type="dashed" block icon={<LinkOutlined />} onClick={() => openBindModal(pf.key)}>
                        绑定更多 {pf.name} 账号
                      </Button>
                    </Space>
                  ) : (
                    <Button type="primary" block onClick={() => openBindModal(pf.key)} disabled={isLoading}>
                      点击绑定 {pf.name} 账号
                    </Button>
                  )}
                </Card>
              </Col>
            )
          })}
        </Row>

        {/* 账号池表格 */}
        {accounts.length > 0 && (
          <Card title="账号池状态">
            <Table
              dataSource={accounts}
              columns={accountColumns}
              rowKey="id"
              pagination={false}
              size="small"
            />
          </Card>
        )}

        {accounts.length === 0 && !isLoading && (
          <Card>
            <Empty description="还没有绑定任何平台账号，点击上方卡片开始绑定" style={{ padding: 40 }} />
          </Card>
        )}
      </div>

      {/* 扫码登录弹窗 */}
      <Modal
        title={`绑定 ${PLATFORMS.find((p) => p.key === activePlatform)?.name || ''} 账号`}
        open={qrModalOpen}
        onCancel={handleCloseModal}
        footer={null}
        width={400}
        centered
      >
        <div style={{ textAlign: 'center', padding: '12px 0' }}>
          {/* 知乎平台：显示登录方式选择（仅在无 session 时显示）*/}
          {activePlatform === 'zhihu' && !sessionId && (
            <div style={{ marginBottom: 16 }}>
              <Text type="secondary" style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>选择登录方式</Text>
              <Space>
                <Button size="small" onClick={() => handleStartQR('zhihu', 'zhihu')}>知乎App</Button>
                <Button size="small" onClick={() => handleStartQR('zhihu', 'wechat')}>微信</Button>
                <Button size="small" onClick={() => handleStartQR('zhihu', 'qq')}>QQ</Button>
                <Button size="small" onClick={() => handleStartQR('zhihu', 'weibo')}>微博</Button>
              </Space>
            </div>
          )}
          {currentQrImage ? (
            <>
              {/* 二维码白底卡片（即使暗色模式也保持白底确保可扫）*/}
              <div style={{ display: 'inline-block', padding: 16, background: '#fff', borderRadius: 12, marginBottom: 16 }}>
                <img
                  src={currentQrImage.startsWith('http') ? currentQrImage : `data:image/png;base64,${currentQrImage}`}
                  alt="登录二维码"
                  style={{ width: 240, height: 'auto', maxHeight: 320, display: 'block' }}
                />
              </div>
              <div style={{ marginBottom: 12 }}>
                <QRStatusIndicator status={pollStatus} platform={activePlatform} />
              </div>
            </>
          ) : (
            <div style={{ padding: 60 }}>
              <Spin size="large" />
              <Paragraph type="secondary" style={{ marginTop: 16 }}>
                正在启动浏览器获取二维码...
              </Paragraph>
            </div>
          )}
        </div>
      </Modal>
    </div>
  )
}

// 扫码状态指示器
function QRStatusIndicator({ status, platform }: { status?: string; platform: string }) {
  const pfName = PLATFORMS.find((p) => p.key === platform)?.name || ''
  if (!status || status === 'preparing') {
    return (
      <Space>
        <Spin size="small" />
        <Text type="secondary">浏览器已打开，正在获取二维码...</Text>
      </Space>
    )
  }
  if (status === 'waiting') {
    return (
      <Space>
        <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--wr-primary)', display: 'inline-block', animation: 'wr-pulse 1.5s infinite' }} />
        <Text type="secondary">请用{pfName}App扫码登录</Text>
      </Space>
    )
  }
  if (status === 'scanned') {
    return (
      <Space>
        <CheckCircleOutlined style={{ color: 'var(--wr-accent)' }} />
        <Text style={{ color: 'var(--wr-accent)' }}>已扫码，请在手机确认登录</Text>
      </Space>
    )
  }
  if (status === 'expired') {
    return <Text type="warning">二维码已过期，请关闭后重新获取</Text>
  }
  if (status === 'success') {
    return (
      <Space>
        <CheckCircleOutlined style={{ color: 'var(--wr-success)' }} />
        <Text style={{ color: 'var(--wr-success)' }}>登录成功，正在绑定...</Text>
      </Space>
    )
  }
  return <Text type="danger">扫码异常：{status}</Text>
}
