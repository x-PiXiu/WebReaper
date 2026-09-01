import { useState, useEffect, useMemo, useRef } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Card, Typography, Button, Row, Col, Tag, Space, Alert, Popconfirm, Tabs } from 'antd'
import { CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined, LinkOutlined, ExportOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { useBrandContext } from '../../hooks/useBrands'
import { PlatformBadge } from '../../components/PlatformBadge'
import QRLoginModal from './distribution/QRLoginModal'
import PublishJobTable from './distribution/PublishJobTable'
import PublishWizard from './distribution/PublishWizard'
import { PublishResultPanel } from './distribution/PublishResultPanel'
import type { WizardDraft } from './distribution/wizardModel'
import { emptyDraft, hasPrefilledMedia, resolveEntryStep } from './distribution/wizardModel'
import type { PublishJob } from '../../types/api'
import { toast, notifyResult } from '../../utils/feedback'

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
  const [semiPublishJobs, setSemiPublishJobs] = useState<PublishJob[]>([])
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
    const partial: Partial<WizardDraft> = {
      contentId: qContentId,
      brandId: qBrandId,
      title: qTitle,
      content: qContent,
      coverURL: qCoverUrl,
      mediaURLs: qMediaUrls ? qMediaUrls.split(',').filter(Boolean) : [],
      contentType: contentType as WizardDraft['contentType'] | undefined,
    }
    return { ...partial, step: resolveEntryStep(partial) }
  }, [searchParams])

  useEffect(() => {
    const qBrandId = searchParams.get('brandId')
    if (qBrandId) setCurrentBrand(qBrandId)
  }, [searchParams, setCurrentBrand])

  // 抖音 OAuth 回调与授权入口（已删除 2026-09-01：发布全走 RPA cookie 通道——
  // 抖音绑定统一走扫码弹窗 openBindModal('douyin')）

  const healthyAccounts = accounts.filter((a) => a.health === 'active')
  const expiredAccounts = accounts.filter((a) => a.health === 'expired')
  const prefilledFromWorks = wizardInitial && hasPrefilledMedia(emptyDraft(wizardInitial))
  const tabTouched = useRef(false)
  useEffect(() => {
    if (wizardInitial) {
      setActiveTab('publish')
      return
    }
    if (!accountsLoading && !tabTouched.current) {
      setActiveTab(healthyAccounts.length > 0 ? 'publish' : 'accounts')
    }
  }, [accountsLoading, healthyAccounts.length, wizardInitial])

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

  // 全部完成：面板已展示明细；仅失败时用通知补一句，避免与结果面板重复刷 toast
  const notifiedRef = useRef(false)
  useEffect(() => {
    if (autoStatus && autoStatus.every((j) => j.status === 'published' || j.status === 'failed')) {
      if (!notifiedRef.current) {
        notifiedRef.current = true
        const successCount = autoStatus.filter((j) => j.status === 'published').length
        const failCount = autoStatus.filter((j) => j.status === 'failed').length
        if (failCount > 0) {
          notifyResult({
            type: 'warning',
            title: `${successCount} 成功 · ${failCount} 失败`,
            desc: '可在上方结果面板查看详情，或稍后重试失败项。',
            key: 'auto-publish-done',
          })
        }
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
      toast.ok('账号已解绑', 'unbind-acc')
      queryClient.invalidateQueries({ queryKey: ['geo-accounts'] })
    } catch { /* */ }
  }

  const handleMarkPublished = async (jobId: string) => {
    try {
      await businessApi.markPublished(jobId)
      toast.ok('已标记为发布完成', 'mark-pub')
      setSemiPublishJobs((prev) => prev.map((j) => (j.id === jobId ? { ...j, status: 'published' } : j)))
      queryClient.invalidateQueries({ queryKey: ['geo-publish-jobs'] })
    } catch { /* */ }
  }

  return (
    <div className="wr-page-content ip-page dist-page" style={{ paddingTop: 4, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        <header className="dist-head">
          <div className="dist-head-copy">
            <h1>账号发布</h1>
            <p>
              {healthyAccounts.length > 0
                ? `${healthyAccounts.length} 个健康账号可用`
                : '尚未绑定账号，先在账号池扫码'}
              {expiredAccounts.length > 0 && ` · ${expiredAccounts.length} 个已过期需重绑`}
            </p>
          </div>
          <div className="dist-head-cta">
            <Button onClick={() => navigate('/m/works')}>我的作品</Button>
            <Button
              type="primary"
              className="ip-btn-primary"
              icon={<ExportOutlined />}
              onClick={() => { tabTouched.current = true; setActiveTab('publish') }}
            >
              去发布
            </Button>
          </div>
        </header>

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
                  {!accountsLoading && healthyAccounts.length === 0 && (
                    <Alert
                      type="warning"
                      showIcon
                      style={{ marginBottom: 16 }}
                      message="尚未绑定可用账号"
                      description="请先在下方扫码绑定至少一个平台账号，再前往「发布」Tab。"
                      action={
                        <Button size="small" type="primary" onClick={() => { tabTouched.current = true; setActiveTab('publish') }}>
                          仍去发布
                        </Button>
                      }
                    />
                  )}
                  {!accountsLoading && expiredAccounts.length > 0 && (
                    <Alert
                      className="dist-expired-alert"
                      type="error"
                      showIcon
                      style={{ marginBottom: 16 }}
                      message={`${expiredAccounts.length} 个账号登录已过期`}
                      description="过期账号无法发布，请在对应平台卡片内点「重新绑定」或「重新授权」。"
                    />
                  )}

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
                                      <div className="dist-account-row-main">
                                        <span style={{ color: cfg.color }}>{cfg.icon}</span>
                                        <div>
                                          <Text strong style={{ color: isExpired ? 'var(--wr-danger)' : 'inherit', display: 'block' }}>
                                            {a.display_name || '未命名账号'}
                                          </Text>
                                          <Text type="secondary" style={{ fontSize: 11 }}>
                                            {cfg.label}
                                            {isOAuth ? ' · 官方通道' : ''}
                                            {a.expires_at ? ` · ${new Date(a.expires_at).toLocaleDateString()}` : ''}
                                          </Text>
                                        </div>
                                      </div>
                                      <Space size={8} wrap>
                                        {isExpired && (
                                          <Button
                                            size="small"
                                            type="primary"
                                            onClick={() => openBindModal(pf.key)}
                                          >
                                            重新绑定
                                          </Button>
                                        )}
                                        <Popconfirm
                                          title="解绑此账号？"
                                          description="解绑后需重新扫码才能发布到该账号。"
                                          okText="解绑"
                                          cancelText="取消"
                                          okButtonProps={{ danger: true }}
                                          onConfirm={() => handleDeleteAccount(a.id)}
                                        >
                                          <Button size="small" type="text" danger>解绑</Button>
                                        </Popconfirm>
                                      </Space>
                                    </div>
                                  )
                                })}
                                {pf.key === 'douyin' ? (
                                  <Button type="dashed" block icon={<LinkOutlined />} onClick={() => openBindModal('douyin')}>
                                    绑定更多抖音账号
                                  </Button>
                                ) : (
                                  <Button type="dashed" block icon={<LinkOutlined />} onClick={() => openBindModal(pf.key)}>
                                    绑定更多 {pf.name} 账号
                                  </Button>
                                )}
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
                </>
              ),
            },
            {
              key: 'publish',
              label: '发布',
              children: (
                <>
                  {prefilledFromWorks && (
                    <Alert
                      className="dist-prefill-banner"
                      type="success"
                      showIcon
                      style={{ marginBottom: 16 }}
                      message="已带入成片"
                      description={
                        wizardInitial?.title
                          ? `「${wizardInitial.title}」— 选账号与平台参数即可发布`
                          : '视频与标题已预填，向导将自动跳过素材步骤'
                      }
                    />
                  )}
                  {!accountsLoading && healthyAccounts.length === 0 && (
                    <Alert
                      type="warning"
                      showIcon
                      style={{ marginBottom: 16 }}
                      message="暂无健康账号"
                      description="发布前建议先在「账号池」绑定账号；也可继续填写向导，稍后再绑。"
                      action={
                        <Button size="small" onClick={() => { tabTouched.current = true; setActiveTab('accounts') }}>
                          去绑定
                        </Button>
                      }
                    />
                  )}
                  {autoJobIds.length > 0 && autoStatus && (
                    <PublishResultPanel
                      mode="auto"
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
                  {semiPublishJobs.length > 0 && (
                    <PublishResultPanel
                      mode="semi"
                      jobs={semiPublishJobs.map((j) => ({
                        id: j.id,
                        platform: j.platform,
                        status: j.status,
                        external_url: j.external_url,
                      }))}
                      onMarkPublished={handleMarkPublished}
                      onDismiss={() => setSemiPublishJobs([])}
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
                  onBind={(pf) => openBindModal(pf)}
                  onPublished={(results, mode) => {
                    queryClient.invalidateQueries({ queryKey: ['geo-publish-jobs'] })
                    if (mode === 'auto') {
                      setAutoJobIds(results.map((j) => j.id))
                    } else {
                      setSemiPublishJobs(results)
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
                  <Card
                    className="wr-glass-card"
                    title="发布记录"
                    extra={
                      <Button type="link" size="small" onClick={() => navigate('/m/works')}>
                        回作品库
                      </Button>
                    }
                  >
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
    </div>
  )
}
