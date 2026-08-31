import { useState, useEffect, useMemo, useRef } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Card, Typography, Button, Row, Col, Tag, Space, Table, Modal, Alert, Popconfirm, Tabs } from 'antd'
import { CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined, LinkOutlined, ExportOutlined, RightOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { useBrandContext } from '../../hooks/useBrands'
import { PlatformBadge } from '../../components/PlatformBadge'
import QRLoginModal from './distribution/QRLoginModal'
import PublishJobTable from './distribution/PublishJobTable'
import PublishWizard from './distribution/PublishWizard'
import { PublishResultPanel } from './distribution/PublishResultPanel'
import type { WizardDraft } from './distribution/wizardModel'
import type { Account, PublishJob } from '../../types/api'
import { message } from '../../utils/antdApp'
import OralJourneyNav from '../../components/compose/OralJourneyNav'

const { Text, Paragraph } = Typography

const PLATFORMS = [
  { key: 'douyin', name: '抖音', color: 'var(--wr-text-primary)', desc: '短视频获客主战场，本地商户首选' },
  { key: 'kuaishou', name: '快手', color: 'var(--wr-warning)', desc: '短视频平台，下沉市场覆盖广' },
  { key: 'xiaohongshu', name: '小红书', color: 'var(--wr-danger)', desc: '种草社区，本地生活/装修类精准触达' },
  { key: 'zhihu', name: '知乎', color: 'var(--wr-primary)', desc: '知识问答社区，长文 SEO 效果好' },
  { key: 'bilibili', name: 'B站', color: 'var(--wr-primary)', desc: '视频社区，年轻用户聚集地' },
  { key: 'weixin', name: '视频号', color: 'var(--wr-success)', desc: '微信生态，私域流量入口' },
]

function healthConfig(health: string) {
  switch (health) {
    case 'active': return { color: 'var(--wr-success)', label: '健康', icon: <CheckCircleOutlined /> }
    case 'expired': return { color: 'var(--wr-warning)', label: '已过期', icon: <ClockCircleOutlined /> }
    case 'banned': return { color: 'var(--wr-danger)', label: '已封禁', icon: <CloseCircleOutlined /> }
    default: return { color: 'var(--wr-text-muted)', label: '未知', icon: <ClockCircleOutlined /> }
  }
}

/**
 * 分发中心：账号池 + 五步发布向导（Plan-14）+ 发布记录。
 * 发布 Tab 能力声明驱动：形态/字数/B站分区标签/完备性检查/本机草稿。
 */
export default function Distribution() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { brands, brandId: selectedBrand, setCurrentBrand } = useBrandContext()

  const [qrModalOpen, setQrModalOpen] = useState(false)
  const [activeTab, setActiveTab] = useState('accounts')
  const [activePlatform, setActivePlatform] = useState('')
  const [linkModalOpen, setLinkModalOpen] = useState(false)
  const [publishLinks, setPublishLinks] = useState<PublishJob[]>([])
  const [autoJobIds, setAutoJobIds] = useState<string[]>([])

  const { data: accounts = [], isLoading: accountsLoading } = useQuery({
    queryKey: ['geo-accounts'],
    queryFn: () => businessApi.listAccounts(),
  })
  const { data: channels = [] } = useQuery({
    queryKey: ['geo-publish-channels'],
    queryFn: () => businessApi.listPublishChannels().then((r) => r.channels).catch(() => []),
    staleTime: 5 * 60_000,
  })
  const { data: contents = [] } = useQuery({
    queryKey: ['geo-contents', selectedBrand],
    queryFn: () => businessApi.listContents(selectedBrand!),
    enabled: !!selectedBrand,
  })
  const { data: jobs = [] } = useQuery({
    queryKey: ['geo-publish-jobs'],
    queryFn: () => businessApi.listPublishJobs(),
  })

  // 入口预填（口播成片 / 图文配图 / 作品库 / GEO 文章）
  const wizardInitial = useMemo((): Partial<WizardDraft> | undefined => {
    const qContentId = searchParams.get('contentId') || undefined
    const qBrandId = searchParams.get('brandId') || undefined
    const qMediaUrls = searchParams.get('mediaUrls')
    const qCoverUrl = searchParams.get('coverUrl') || ''
    const qContentType = searchParams.get('contentType') || searchParams.get('publishForm')
    const qTitle = searchParams.get('title') || ''
    const qContent = searchParams.get('content') || ''
    if (!qContentId && !qMediaUrls && !qTitle && !qContentType && !qBrandId && !qContent) return undefined
    const contentType =
      qContentType === 'article' || qContentType === 'image' || qContentType === 'video'
        ? qContentType
        : qMediaUrls
          ? (qMediaUrls.match(/\.(jpg|jpeg|png|webp)(\?|$)/i) ? 'image' : 'video')
          : undefined
    return {
      contentId: qContentId,
      brandId: qBrandId,
      title: qTitle,
      content: qContent,
      coverURL: qCoverUrl,
      mediaURLs: qMediaUrls ? qMediaUrls.split(',').filter(Boolean) : [],
      contentType: contentType as WizardDraft['contentType'] | undefined,
      step: qMediaUrls ? 1 : qContentId || qContent ? 2 : 1,
    }
  }, [searchParams])

  useEffect(() => {
    const qBrandId = searchParams.get('brandId')
    if (qBrandId) setCurrentBrand(qBrandId)
  }, [searchParams, setCurrentBrand])

  // 抖音 OAuth 回调
  useEffect(() => {
    const oauthResult = searchParams.get('douyin_oauth')
    if (!oauthResult) return
    if (oauthResult === 'success') {
      message.success(`抖音账号「${searchParams.get('name') || ''}」官方授权绑定成功`)
    } else {
      message.error(`官方授权失败：${searchParams.get('reason') || '未知原因'}`)
    }
    queryClient.invalidateQueries({ queryKey: ['geo-accounts'] })
    window.history.replaceState({}, '', window.location.pathname)
  }, [searchParams, queryClient])

  const bindDouyin = async () => {
    try {
      const { url } = await businessApi.getDouyinOAuthURL()
      window.open(url, '_blank')
    } catch {
      openBindModal('douyin')
    }
  }

  const healthyAccounts = accounts.filter((a) => a.health === 'active')
  const tabTouched = useRef(false)
  useEffect(() => {
    if (!accountsLoading && !tabTouched.current && healthyAccounts.length > 0) {
      setActiveTab('publish')
    }
  }, [accountsLoading, healthyAccounts.length])

  // 带入口参数时直接进发布
  useEffect(() => {
    if (wizardInitial) setActiveTab('publish')
  }, [wizardInitial])

  const { data: autoStatus } = useQuery({
    queryKey: ['auto-publish-status', autoJobIds],
    queryFn: async () => Promise.all(autoJobIds.map((id) => businessApi.getPublishJobStatus(id))),
    enabled: autoJobIds.length > 0,
    refetchInterval: (query) => {
      const data = query.state.data
      const allDone = data?.every((j) => j.status === 'published' || j.status === 'failed')
      return allDone ? false : 3000
    },
  })

  // 全部完成时提示一次（不清 autoJobIds——保留在结果面板中展示）
  const notifiedRef = useRef(false)
  useEffect(() => {
    if (autoStatus && autoStatus.every((j) => j.status === 'published' || j.status === 'failed')) {
      if (!notifiedRef.current) {
        notifiedRef.current = true
        const successCount = autoStatus.filter((j) => j.status === 'published').length
        const failCount = autoStatus.filter((j) => j.status === 'failed').length
        if (successCount > 0) message.success(`${successCount} 篇内容自动发布成功`)
        if (failCount > 0) message.error(`${failCount} 篇发布失败`)
        queryClient.invalidateQueries({ queryKey: ['geo-publish-jobs'] })
      }
    } else {
      notifiedRef.current = false
    }
  }, [autoStatus, queryClient])

  const openBindModal = (platform: string) => {
    setActivePlatform(platform)
    setQrModalOpen(true)
  }

  const handleDeleteAccount = async (id: string) => {
    try {
      await businessApi.deleteAccount(id)
      message.success('已解绑')
      queryClient.invalidateQueries({ queryKey: ['geo-accounts'] })
    } catch { /* */ }
  }

  const handleMarkPublished = async (jobId: string) => {
    try {
      await businessApi.markPublished(jobId)
      message.success('已标记为发布')
      queryClient.invalidateQueries({ queryKey: ['geo-publish-jobs'] })
    } catch { /* */ }
  }

  const accountColumns = [
    {
      title: '平台', dataIndex: 'platform', key: 'platform', width: 110,
      render: (p: string) => <PlatformBadge platform={p} size={14} />,
    },
    { title: '账号', dataIndex: 'display_name', key: 'name', render: (n: string) => <Text strong>{n || '-'}</Text> },
    {
      title: '登录方式', dataIndex: 'login_method', key: 'method', width: 100,
      render: (m: string) => {
        const labels: Record<string, string> = {
          zhihu: '知乎App', wechat: '微信', qq: 'QQ', weibo: '微博',
          xiaohongshu: '小红书', douyin: '抖音App', kuaishou: '快手App', bilibili: 'B站App', weixin: '微信扫码',
        }
        return <Tag>{labels[m] || m || '-'}</Tag>
      },
    },
    {
      title: '状态', dataIndex: 'health', key: 'health', width: 110,
      render: (h: string) => {
        const cfg = healthConfig(h)
        return <Space><span style={{ color: cfg.color }}>{cfg.icon}</span><Text style={{ color: cfg.color }}>{cfg.label}</Text></Space>
      },
    },
    {
      title: '过期时间', dataIndex: 'expires_at', key: 'expires', width: 170,
      render: (t: string) => {
        if (!t) return <Text type="secondary">-</Text>
        return <Text type={new Date(t) < new Date() ? 'danger' : 'secondary'} style={{ fontSize: 12 }}>{new Date(t).toLocaleString()}</Text>
      },
    },
    {
      title: '操作', key: 'action', width: 80,
      render: (_: unknown, r: Account) => (
        <Popconfirm title="确定解绑此账号？" onConfirm={() => handleDeleteAccount(r.id)}>
          <Button size="small" type="text" danger>解绑</Button>
        </Popconfirm>
      ),
    },
  ]

  return (
    <div className="wr-page-content ip-page dist-page ch-creative" style={{ paddingTop: 4, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        <OralJourneyNav />
        <section className="ch-hero dist-hero">
          <div className="ch-hero-copy">
            <p className="ch-hero-kicker">账号发布</p>
            <h1 className="ch-hero-title">把内容发到多平台。</h1>
            <p className="ch-hero-lead">
              绑定账号后按向导填写内容，支持半自动或全自动发布；Cookie 失效时可降级为手动发布。
            </p>
          </div>
          <div className="ch-hero-cta">
            <Button
              type="primary"
              size="large"
              className="ch-hero-btn"
              icon={<ExportOutlined />}
              onClick={() => { tabTouched.current = true; setActiveTab('publish') }}
            >
              去发布
              <RightOutlined />
            </Button>
            <div className="ch-hero-tags">
              <button type="button" className="ch-hero-tag" onClick={() => { tabTouched.current = true; setActiveTab('accounts') }}>
                账号池
              </button>
              <button type="button" className="ch-hero-tag" onClick={() => navigate('/m/works')}>
                我的作品
              </button>
              <button type="button" className="ch-hero-tag" onClick={() => navigate('/m/compose')}>
                回首页
              </button>
            </div>
            <p className="ch-hero-proof">
              {healthyAccounts.length > 0
                ? `${healthyAccounts.length} 个健康账号可用`
                : '尚未绑定账号，先去账号池扫码'}
            </p>
          </div>
        </section>

        <Tabs
          className="dist-tabs"
          activeKey={activeTab}
          onChange={(k) => { tabTouched.current = true; setActiveTab(k) }}
          items={[
            {
              key: 'accounts',
              label: '账号池',
              children: (
                <>
                  <div className="dist-hint">
                    <LinkOutlined className="dist-hint-icon" />
                    <div>
                      <Text strong style={{ fontSize: 14 }}>发布时带定位，本地曝光更好</Text>
                      <Paragraph type="secondary" style={{ fontSize: 12, margin: '4px 0 0' }}>
                        定位的自动化暂未开放，推荐流程：人设档案维护地址 → 发布附带门店信息 → 平台页手动选定位。
                      </Paragraph>
                    </div>
                  </div>

                  <Row gutter={[16, 16]} className="wr-stagger dist-platform-grid" style={{ marginBottom: 16 }}>
                    {PLATFORMS.map((pf) => {
                      const accs = accounts.filter((a) => a.platform === pf.key)
                      return (
                        <Col xs={24} sm={12} key={pf.key}>
                          <div className="dist-platform-card">
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
                              <div>
                                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                                  <PlatformBadge platform={pf.key} size={18} />
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
                                  const isOAuth = a.auth_type === 'oauth'
                                  return (
                                    <div key={a.id} className={`dist-account-row${isExpired ? ' is-expired' : ''}`}>
                                      <Space>
                                        <span style={{ color: cfg.color }}>{cfg.icon}</span>
                                        <Text style={{ color: isExpired ? 'var(--wr-text-muted)' : 'inherit' }}>{a.display_name}</Text>
                                        {isOAuth && <Tag color="green" style={{ marginInlineEnd: 0 }}>官方通道</Tag>}
                                      </Space>
                                      <Space size={8}>
                                        <Text style={{ fontSize: 12, color: cfg.color }}>{cfg.label}</Text>
                                        {isExpired && !isOAuth && (
                                          <Button size="small" type="primary" danger onClick={() => openBindModal(pf.key)}>重新绑定</Button>
                                        )}
                                        {isExpired && isOAuth && pf.key === 'douyin' && (
                                          <Button size="small" type="primary" onClick={bindDouyin}>重新授权</Button>
                                        )}
                                        <Popconfirm title="确定解绑？" onConfirm={() => handleDeleteAccount(a.id)}>
                                          <Button size="small" type="text" danger>解绑</Button>
                                        </Popconfirm>
                                      </Space>
                                    </div>
                                  )
                                })}
                                {pf.key === 'douyin' ? (
                                  <Space direction="vertical" size={6} style={{ width: '100%' }}>
                                    <Button type="dashed" block onClick={bindDouyin}>官方授权绑定更多</Button>
                                    <Button type="text" block onClick={() => openBindModal('douyin')}>浏览器扫码绑定</Button>
                                  </Space>
                                ) : (
                                  <Button type="dashed" block icon={<LinkOutlined />} onClick={() => openBindModal(pf.key)}>
                                    绑定更多 {pf.name} 账号
                                  </Button>
                                )}
                              </Space>
                            ) : pf.key === 'douyin' ? (
                              <Space direction="vertical" size={6} style={{ width: '100%' }}>
                                <Button type="primary" block onClick={bindDouyin} disabled={accountsLoading}>官方授权绑定抖音</Button>
                                <Button type="text" block onClick={() => openBindModal('douyin')} disabled={accountsLoading}>浏览器扫码绑定</Button>
                              </Space>
                            ) : (
                              <Button type="primary" block onClick={() => openBindModal(pf.key)} disabled={accountsLoading}>
                                点击绑定 {pf.name} 账号
                              </Button>
                            )}
                          </div>
                        </Col>
                      )
                    })}
                  </Row>

                  {accounts.length > 0 && (
                    <Card className="wr-glass-card" title="账号池状态">
                      <Table dataSource={accounts} columns={accountColumns} rowKey="id" pagination={false} size="small" />
                    </Card>
                  )}
                </>
              ),
            },
            {
              key: 'publish',
              label: '发布',
              children: (
                <>
                  {autoJobIds.length > 0 && autoStatus && (
                    <PublishResultPanel
                      jobs={autoStatus.map((j) => ({
                        id: j.id,
                        platform: (j as Record<string, unknown>).platform as string | undefined,
                        status: j.status,
                        error_msg: (j as Record<string, unknown>).error_msg as string | undefined,
                        external_url: (j as Record<string, unknown>).external_url as string | undefined,
                      }))}
                      onDismiss={() => setAutoJobIds([])}
                    />
                  )}
                  <PublishWizard
                  brands={brands}
                  brandId={selectedBrand}
                  setBrandId={setCurrentBrand}
                  accounts={accounts}
                  channels={channels}
                  contents={contents}
                  initial={wizardInitial}
                  onBind={(pf) => {
                    if (pf === 'douyin') bindDouyin()
                    else openBindModal(pf)
                  }}
                  onPublished={(results, mode) => {
                    queryClient.invalidateQueries({ queryKey: ['geo-publish-jobs'] })
                    if (mode === 'auto') {
                      setAutoJobIds(results.map((j) => j.id))
                    } else {
                      setPublishLinks(results)
                      setLinkModalOpen(true)
                    }
                  }}
                />
                </>
              ),
            },
            {
              key: 'records',
              label: '发布记录',
              children: (
                <>
                  <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
                    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 12 }}>
                      {PLATFORMS.map((pf) => {
                        const pj = jobs.filter((j: PublishJob) => j.platform === pf.key)
                        const monitored = pj.filter((j: PublishJob) => j.post_mention_rate != null)
                        const avgDelta = monitored.length > 0
                          ? monitored.reduce((s: number, j: PublishJob) => s + ((j.post_mention_rate || 0) - (j.pre_mention_rate || 0)), 0) / monitored.length
                          : null
                        return (
                          <div key={pf.key} className="dist-stat-tile">
                            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6 }}>
                              <span style={{ width: 8, height: 8, borderRadius: '50%', background: pf.color, display: 'inline-block' }} />
                              <Text strong style={{ fontSize: 13 }}>{pf.name}</Text>
                              <Tag style={{ margin: 0, fontSize: 10 }}>{pj.length} 篇</Tag>
                            </div>
                            <Text style={{ fontSize: 12, color: 'var(--wr-text-muted)' }}>
                              {avgDelta === null
                                ? '暂无复测数据'
                                : <>表现变化均值 <b style={{ color: avgDelta >= 0 ? 'var(--wr-success)' : 'var(--wr-danger)' }}>
                                    {avgDelta >= 0 ? '+' : ''}{(avgDelta * 100).toFixed(1)}%</b></>}
                            </Text>
                          </div>
                        )
                      })}
                    </div>
                  </Card>
                  <Card className="wr-glass-card" title="发布记录">
                    <PublishJobTable
                      jobs={jobs}
                      onRefresh={() => queryClient.invalidateQueries({ queryKey: ['geo-publish-jobs'] })}
                    />
                  </Card>
                </>
              ),
            },
          ]}
        />
      </div>

      <QRLoginModal open={qrModalOpen} platform={activePlatform} onClose={() => setQrModalOpen(false)} />

      <Modal
        title="发布链接已生成"
        open={linkModalOpen}
        onCancel={() => setLinkModalOpen(false)}
        footer={<Button type="primary" onClick={() => setLinkModalOpen(false)}>完成</Button>}
        width={520}
      >
        <Alert
          type="success"
          showIcon
          style={{ marginBottom: 16 }}
          message="内容已准备就绪"
          description="点击下方链接前往各平台发布页完成发布，然后点「标记已发布」。"
        />
        {publishLinks.some((j) => j.platform === 'zhihu') && (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="知乎需手动粘贴正文（平台限制），点击「前往发布」后请 Ctrl+V"
          />
        )}
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          {publishLinks.map((job) => (
            <div key={job.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: 12, background: 'var(--wr-bg-elevated)', borderRadius: 8 }}>
              <Space>
                <PlatformBadge platform={job.platform} size={14} />
                <Text type="secondary" style={{ fontSize: 12 }}>待确认</Text>
              </Space>
              <Space>
                <Button size="small" type="primary" icon={<ExportOutlined />} href={job.external_url} target="_blank">前往发布</Button>
                <Button size="small" type="link" onClick={() => handleMarkPublished(job.id)}>已发布</Button>
              </Space>
            </div>
          ))}
        </Space>
      </Modal>
    </div>
  )
}
