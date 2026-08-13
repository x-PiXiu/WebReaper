import { useState, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Card, Typography, Button, Row, Col, Select, Tag, Space, message, Empty, Table, Radio, Modal, Alert, Switch, Popconfirm, Spin, DatePicker, Input, Tooltip } from 'antd'
import { ExportOutlined, CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined, LoadingOutlined, LinkOutlined, PictureOutlined, DeleteOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { businessApi } from '../../api/business'
import type { Brand, OptimizedContent, Account, PublishJob } from '../../types/api'

const { Text, Paragraph } = Typography

// ---- 平台元信息 ----
const PLATFORMS = [
  { key: 'zhihu', name: '知乎', color: 'var(--wr-primary)', desc: '知识问答社区，长文 SEO 效果好' },
  { key: 'xiaohongshu', name: '小红书', color: 'var(--wr-danger)', desc: '种草社区，本地生活/装修类精准触达' },
]
const PLATFORM_NAMES: Record<string, string> = { zhihu: '知乎', xiaohongshu: '小红书' }

// 健康度 → 显示配置
function healthConfig(health: string) {
  switch (health) {
    case 'active': return { color: 'var(--wr-success)', label: '健康', icon: <CheckCircleOutlined /> }
    case 'expired': return { color: 'var(--wr-warning)', label: '已过期', icon: <ClockCircleOutlined /> }
    case 'banned': return { color: 'var(--wr-danger)', label: '已封禁', icon: <CloseCircleOutlined /> }
    default: return { color: 'var(--wr-text-muted)', label: '未知', icon: <ClockCircleOutlined /> }
  }
}

// 发布状态 → 显示
function statusConfig(status: string) {
  switch (status) {
    case 'published': return { color: 'var(--wr-success)', label: '已发布', icon: <CheckCircleOutlined /> }
    case 'running': return { color: 'var(--wr-primary)', label: '自动发布中', icon: <LoadingOutlined /> }
    case 'pending': return { color: 'var(--wr-warning)', label: '待确认', icon: <ClockCircleOutlined /> }
    case 'failed': return { color: 'var(--wr-danger)', label: '失败', icon: <CloseCircleOutlined /> }
    default: return { color: 'var(--wr-text-muted)', label: status, icon: <ClockCircleOutlined /> }
  }
}

function scoreColor(s: number): string {
  if (s >= 80) return 'var(--wr-success)'
  if (s >= 65) return 'var(--wr-accent)'
  if (s >= 50) return 'var(--wr-warning)'
  return 'var(--wr-danger)'
}

// 分发中心：账号池（绑定/维护）+ 内容发布（半自动/全自动）整合页。
// 发布强依赖账号——账号池是分发基础设施，合并后一个页面完成「选号 → 发布 → 复测」闭环。
export default function Distribution() {
  const queryClient = useQueryClient()

  // ---- 账号池状态 ----
  const [qrModalOpen, setQrModalOpen] = useState(false)
  const [activePlatform, setActivePlatform] = useState('')
  const [sessionId, setSessionId] = useState('')
  const [qrImage, setQrImage] = useState('')
  const [loginMethod, setLoginMethod] = useState('')

  // ---- 发布状态 ----
  const [selectedBrand, setSelectedBrand] = useState<string | undefined>()
  const [selectedContentId, setSelectedContentId] = useState<string>()
  const [selectedAccountIds, setSelectedAccountIds] = useState<string[]>([])
  const [publishing, setPublishing] = useState(false)
  const [publishMode, setPublishMode] = useState<'semi-auto' | 'auto'>('semi-auto')
  const [autoSelect, setAutoSelect] = useState(false)
  const [linkModalOpen, setLinkModalOpen] = useState(false)
  const [scheduleTime, setScheduleTime] = useState<Date | null>(null) // 定时发送（null=立即）
  const [publishLinks, setPublishLinks] = useState<PublishJob[]>([])
  const [autoJobIds, setAutoJobIds] = useState<string[]>([])
  // 小红书图文配图（MediaType：小红书 image 必填，URL 来自素材库）
  const [mediaUrls, setMediaUrls] = useState<string[]>([])
  const [showAssetPicker, setShowAssetPicker] = useState(false)
  // D: 可编辑发布标题/正文（发布前可调整，不再只读 selectedContent）
  const [publishTitle, setPublishTitle] = useState('')
  const [publishContentText, setPublishContentText] = useState('')

  // 选中内容变化时，初始化可编辑标题/正文
  useEffect(() => {
    if (selectedContent) {
      setPublishTitle(selectedContent.title || '')
      setPublishContentText(selectedContent.optimized_text || '')
    }
  }, [selectedContentId])

  // B+C: 接收跳转参数（Content/Creation 的「去发布」入口预选内容/预填配图）
  const [searchParams] = useSearchParams()
  useEffect(() => {
    const qContentId = searchParams.get('contentId')
    const qBrandId = searchParams.get('brandId')
    const qMediaUrls = searchParams.get('mediaUrls')
    if (qBrandId) setSelectedBrand(qBrandId)
    if (qContentId) setSelectedContentId(qContentId)
    if (qMediaUrls) setMediaUrls(qMediaUrls.split(',').filter(Boolean))
  }, [searchParams])

  const { data: pickerAssets = [] } = useQuery({
    queryKey: ['media-assets'],
    queryFn: () => businessApi.listAssets().then(r => r.assets),
    enabled: showAssetPicker,
  })
  const pickerImages = pickerAssets.filter(a => a.mime.startsWith('image'))

  // ---- 数据 ----
  const { data: accounts = [], isLoading: accountsLoading } = useQuery({
    queryKey: ['geo-accounts'],
    queryFn: () => businessApi.listAccounts(),
  })
  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
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

  // 扫码状态轮询（条件轮询：会话进行中每 2s）
  const { data: pollData } = useQuery({
    queryKey: ['qr-status', sessionId, activePlatform],
    queryFn: () => businessApi.pollQRLogin(sessionId, activePlatform, loginMethod),
    enabled: !!sessionId && qrModalOpen,
    refetchInterval: (query) => {
      const s = query.state.data?.status
      return s && (s === 'preparing' || s === 'waiting' || s === 'scanned') ? 2000 : false
    },
  })

  // 全自动发布状态轮询
  const { data: autoStatus } = useQuery({
    queryKey: ['auto-publish-status', autoJobIds],
    queryFn: async () => Promise.all(autoJobIds.map(id => businessApi.getPublishJobStatus(id))),
    enabled: autoJobIds.length > 0,
    refetchInterval: (query) => {
      const data = query.state.data
      const allDone = data?.every(j => j.status === 'published' || j.status === 'failed')
      return allDone ? false : 3000
    },
  })

  useEffect(() => {
    if (autoStatus && autoStatus.every(j => j.status === 'published' || j.status === 'failed')) {
      const successCount = autoStatus.filter(j => j.status === 'published').length
      const failCount = autoStatus.filter(j => j.status === 'failed').length
      if (successCount > 0) message.success(`${successCount} 篇内容自动发布成功`)
      if (failCount > 0) message.error(`${failCount} 篇发布失败`)
      setAutoJobIds([])
      setPublishing(false)
      queryClient.invalidateQueries({ queryKey: ['geo-publish-jobs'] })
    }
  }, [autoStatus, queryClient])

  // 轮询登录成功 → 刷新账号池
  useEffect(() => {
    if (pollData?.status === 'success' && qrModalOpen) {
      const pfName = PLATFORMS.find((p) => p.key === activePlatform)?.name || '账号'
      message.success(`${pfName}「${pollData.account_name || ''}」绑定成功`)
      setQrModalOpen(false)
      setSessionId('')
      setQrImage('')
      queryClient.invalidateQueries({ queryKey: ['geo-accounts'] })
    }
  }, [pollData, qrModalOpen, activePlatform, queryClient])

  const selectedContent = contents.find((c) => c.id === selectedContentId)
  const healthyAccounts = accounts.filter((a) => a.health === 'active')
  const currentQrImage = pollData?.qr_image || qrImage

  // ---- 账号操作 ----
  const openBindModal = (platform: string) => {
    setActivePlatform(platform)
    setLoginMethod('')
    setQrImage('')
    setSessionId('')
    setQrModalOpen(true)
    if (platform === 'xiaohongshu') handleStartQR(platform)
  }

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

  const handleCloseModal = async () => {
    if (sessionId) { try { await businessApi.cancelQRLogin(sessionId) } catch {} }
    setQrModalOpen(false)
    setSessionId('')
    setQrImage('')
  }

  const handleDeleteAccount = async (id: string) => {
    try {
      await businessApi.deleteAccount(id)
      message.success('已解绑')
      queryClient.invalidateQueries({ queryKey: ['geo-accounts'] })
    } catch {}
  }

  // ---- 发布操作 ----
  const handlePublish = async () => {
    if (!selectedContent) { message.warning('请选择内容'); return }
    if (!autoSelect && selectedAccountIds.length === 0) { message.warning('请选择至少一个目标账号，或开启自动选号'); return }
    setPublishing(true)
    setPublishLinks([])
    // 定时发送：选了排期时间则只落库排期任务（到期由调度器自动发布）
    const scheduledAt = scheduleTime ? scheduleTime.toISOString() : undefined
    try {
      const results: PublishJob[] = []
      if (autoSelect && publishMode === 'auto') {
        const platforms = [...new Set(healthyAccounts.map(a => a.platform))]
        for (const platform of platforms) {
          // D: 按平台截断标题（小红书≤20字），用可编辑的 publishTitle/publishContentText
          const titleForPlatform = platform === 'xiaohongshu'
            ? (publishTitle && [...publishTitle].slice(0, 20).join(''))
            : publishTitle
          results.push(await businessApi.publishContent({
            account_id: '', platform, content_id: selectedContent.id, brand_id: selectedBrand,
            title: titleForPlatform, content: publishContentText, mode: publishMode,
            scheduled_at: scheduledAt,
            content_type: platform === 'xiaohongshu' ? 'image' : 'article',
            media_urls: platform === 'xiaohongshu' ? mediaUrls : undefined,
          }))
        }
      } else {
        for (const accId of selectedAccountIds) {
          const acc = accounts.find((a) => a.id === accId)
          if (!acc) continue
          const titleForPlatform = acc.platform === 'xiaohongshu'
            ? (publishTitle && [...publishTitle].slice(0, 20).join(''))
            : publishTitle
          results.push(await businessApi.publishContent({
            account_id: accId, platform: acc.platform, content_id: selectedContent.id, brand_id: selectedBrand,
            title: titleForPlatform, content: publishContentText, mode: publishMode,
            scheduled_at: scheduledAt,
            content_type: acc.platform === 'xiaohongshu' ? 'image' : 'article',
            media_urls: acc.platform === 'xiaohongshu' ? mediaUrls : undefined,
          }))
        }
      }
      if (scheduledAt) {
        // 排期模式：任务已落库，到期自动发布
        message.success(`已排期 ${results.length} 个发布任务（${scheduleTime!.toLocaleString()} 自动执行）`)
        setPublishing(false)
      } else if (publishMode === 'auto') {
        setAutoJobIds(results.map(j => j.id))
        message.success(`已启动 ${results.length} 个自动发布任务`)
      } else {
        setPublishLinks(results)
        setLinkModalOpen(true)
        message.success(`已生成 ${results.length} 个发布链接`)
        setPublishing(false)
      }
      queryClient.invalidateQueries({ queryKey: ['geo-publish-jobs'] })
    } catch (e) {
      message.error('发布失败：' + ((e as Error)?.message || ''))
      setPublishing(false)
    }
  }

  const handleMarkPublished = async (jobId: string) => {
    try {
      await businessApi.markPublished(jobId)
      message.success('已标记为发布')
      queryClient.invalidateQueries({ queryKey: ['geo-publish-jobs'] })
    } catch {}
  }

  const handleReMonitor = async (jobId: string) => {
    try {
      const job = await businessApi.reMonitorJob(jobId)
      message.success(`复测完成：提及率 ${(job.pre_mention_rate * 100).toFixed(1)}% → ${(job.post_mention_rate * 100).toFixed(1)}%`)
      queryClient.invalidateQueries({ queryKey: ['geo-publish-jobs'] })
    } catch (e) {
      message.error('复测失败：' + ((e as Error)?.message || '监测可能未配置'))
    }
  }

  // ---- 表格列 ----
  const accountColumns = [
    {
      title: '平台', dataIndex: 'platform', key: 'platform', width: 110,
      render: (p: string) => <Tag color={PLATFORMS.find(x => x.key === p)?.color}>{PLATFORM_NAMES[p] || p}</Tag>,
    },
    { title: '账号', dataIndex: 'display_name', key: 'name', render: (n: string) => <Text strong>{n || '-'}</Text> },
    {
      title: '登录方式', dataIndex: 'login_method', key: 'method', width: 100,
      render: (m: string) => {
        const labels: Record<string, string> = { zhihu: '知乎App', wechat: '微信', qq: 'QQ', weibo: '微博', xiaohongshu: '小红书' }
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
      title: '最后使用', dataIndex: 'last_used_at', key: 'last_used', width: 170,
      render: (t: string) => <Text type="secondary" style={{ fontSize: 12 }}>{t ? new Date(t).toLocaleString() : '-'}</Text>,
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

  const jobColumns = [
    {
      title: '标题', dataIndex: 'title', key: 'title',
      render: (t: string) => <Text strong style={{ fontSize: 13 }}>{t || '-'}</Text>,
    },
    {
      title: '平台', dataIndex: 'platform', key: 'platform', width: 90,
      render: (p: string) => <Tag>{PLATFORM_NAMES[p] || p}</Tag>,
    },
    {
      title: '模式', dataIndex: 'mode', key: 'mode', width: 90,
      render: (m: string) => <Text type="secondary" style={{ fontSize: 12 }}>{m === 'semi-auto' ? '半自动' : m}</Text>,
    },
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 110,
      render: (s: string) => {
        const cfg = statusConfig(s)
        return <Space><span style={{ color: cfg.color }}>{cfg.icon}</span><Text style={{ color: cfg.color, fontSize: 12 }}>{cfg.label}</Text></Space>
      },
    },
    {
      title: '创建时间', dataIndex: 'created_at', key: 'time', width: 150,
      render: (t: string) => <Text type="secondary" style={{ fontSize: 12 }}>{t ? new Date(t).toLocaleString() : '-'}</Text>,
    },
    {
      title: '提及率变化', key: 'mention_rate', width: 140,
      render: (_: unknown, r: PublishJob) => {
        if (!r.post_mention_rate) return <Text type="secondary" style={{ fontSize: 12 }}>-</Text>
        const pre = (r.pre_mention_rate * 100).toFixed(1)
        const post = (r.post_mention_rate * 100).toFixed(1)
        const diff = r.post_mention_rate - r.pre_mention_rate
        const color = diff > 0 ? 'var(--wr-success)' : diff < 0 ? 'var(--wr-danger)' : 'var(--wr-text-muted)'
        return <Text style={{ fontSize: 12, color }}>{pre}% → {post}%{diff !== 0 && ` (${diff > 0 ? '+' : ''}${(diff * 100).toFixed(1)}%)`}</Text>
      },
    },
    {
      title: '操作', key: 'action', width: 140,
      render: (_: unknown, r: PublishJob) => (
        <Space>
          {r.external_url && (
            <Button size="small" type="link" icon={<ExportOutlined />} href={r.external_url} target="_blank">跳转</Button>
          )}
          {r.status === 'published' && (
            <Button size="small" type="link" onClick={() => handleReMonitor(r.id)}>复测提及率</Button>
          )}
          {r.status === 'pending' && (
            <Button size="small" type="link" onClick={() => handleMarkPublished(r.id)}>标记已发布</Button>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ paddingTop: 8, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        <div className="wr-page-header">
          <h1>自动发布</h1>
          <p>账号池维护 · 内容一键发布到各社媒平台 · 发布后提及率复测</p>
        </div>

        {/* P4-03 半自动定位指引：平台 POI 挂载（RPA 自动定位暂缓） */}
        <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
          <Space align="start" style={{ width: '100%' }}>
            <LinkOutlined style={{ color: 'var(--wr-warning)', marginTop: 3 }} />
            <div style={{ flex: 1 }}>
              <Text strong style={{ fontSize: 14 }}>📍 发布带定位 = 附近搜索曝光（半自动指引）</Text>
              <Paragraph type="secondary" style={{ fontSize: 12, margin: '4px 0 8px' }}>
                平台"添加定位"（POI 挂载）自动化为暂缓项（RPA 定位风控高、抖音官方通道需服务商资质）——先用最稳的半自动方式：
              </Paragraph>
              <Space direction="vertical" size={2} style={{ fontSize: 12, color: 'var(--wr-text-secondary)' }}>
                <div>① 门店档案维护好真实地址（品牌管理 → 附近同行 → 门店档案）</div>
                <div>② 发布内容已自动附带"📍 门店地址"行（发布时 store_address 非空）</div>
                <div>③ 平台发布页手动选择"添加定位 → 搜索门店地址 → 选中"后发布</div>
                <div>④ 带定位的内容更容易被附近的人搜到，也增强 AI 本地回答的引用概率</div>
              </Space>
            </div>
          </Space>
        </Card>

        {/* ===== ① 账号池 ===== */}
        <Row gutter={[16, 16]} className="wr-stagger" style={{ marginBottom: 16 }}>
          {PLATFORMS.map((pf) => {
            const accs = accounts.filter((a) => a.platform === pf.key)
            return (
              <Col xs={24} sm={12} key={pf.key}>
                <Card className="wr-glass-card" styles={{ body: { padding: 24 } }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
                    <div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                        <span style={{ width: 8, height: 8, borderRadius: '50%', background: pf.color, display: 'inline-block' }} />
                        <Text strong style={{ fontSize: 17 }}>{pf.name}</Text>
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
                              {isExpired && (
                                <Button size="small" type="primary" danger onClick={() => openBindModal(pf.key)}>重新绑定</Button>
                              )}
                              <Popconfirm title="确定解绑？" onConfirm={() => handleDeleteAccount(a.id)}>
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
                    <Button type="primary" block onClick={() => openBindModal(pf.key)} disabled={accountsLoading}>
                      点击绑定 {pf.name} 账号
                    </Button>
                  )}
                </Card>
              </Col>
            )
          })}
        </Row>

        {accounts.length > 0 && (
          <Card className="wr-glass-card" title="账号池状态" style={{ marginBottom: 16 }}>
            <Table dataSource={accounts} columns={accountColumns} rowKey="id" pagination={false} size="small" />
          </Card>
        )}

        {/* ===== ② 发布工作台 ===== */}
        <Card className="wr-glass-card" title="内容发布" styles={{ body: { padding: 16 } }} style={{ marginBottom: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16 }}>
            <Text strong style={{ whiteSpace: 'nowrap' }}>选择品牌</Text>
            <Select
              style={{ maxWidth: 320, minWidth: 200, flex: 1 }}
              placeholder="选择品牌查看其内容"
              value={selectedBrand}
              onChange={(v) => { setSelectedBrand(v); setSelectedContentId(undefined) }}
              options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
            />
          </div>

          <Row gutter={16}>
            <Col xs={24} lg={12}>
              <div style={{ minHeight: 260 }}>
                {!selectedBrand ? (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="请先选择品牌" style={{ padding: 40 }} />
                ) : contents.length === 0 ? (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="该品牌暂无内容，前往内容工作台生成" style={{ padding: 40 }} />
                ) : (
                  <Radio.Group value={selectedContentId} onChange={(e) => setSelectedContentId(e.target.value)} style={{ width: '100%' }}>
                    <Space direction="vertical" style={{ width: '100%' }}>
                      {contents.map((c: OptimizedContent) => {
                        const total = c.score?.total || 0
                        return (
                          <Radio key={c.id} value={c.id} style={{ width: '100%', alignItems: 'flex-start' }}>
                            <div style={{ marginLeft: 4 }}>
                              <Space size={8} style={{ marginBottom: 4 }}>
                                <Tag color={scoreColor(total)} style={{ fontWeight: 600 }}>GEO {total.toFixed(0)}</Tag>
                                <Text type="secondary" style={{ fontSize: 12 }}>v{c.version}</Text>
                              </Space>
                              <Paragraph ellipsis={{ rows: 2 }} style={{ margin: 0, color: 'var(--wr-text-secondary)', fontSize: 13, lineHeight: 1.6 }}>
                                {c.optimized_text}
                              </Paragraph>
                            </div>
                          </Radio>
                        )
                      })}
                    </Space>
                  </Radio.Group>
                )}
              </div>
            </Col>

            <Col xs={24} lg={12}>
              <div style={{ minHeight: 260 }}>
                {healthyAccounts.length === 0 ? (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无健康账号，请先在上方绑定" style={{ padding: 40 }} />
                ) : (
                  <>
                    <Radio.Group
                      value={publishMode}
                      onChange={(e) => setPublishMode(e.target.value)}
                      style={{ marginBottom: 16 }}
                      optionType="button" buttonStyle="solid"
                    >
                      <Radio.Button value="semi-auto">半自动（推荐）</Radio.Button>
                      <Radio.Button value="auto">全自动</Radio.Button>
                    </Radio.Group>

                    {publishMode === 'semi-auto' ? (
                      <Alert type="info" showIcon style={{ marginBottom: 16 }} message="半自动发布模式"
                        description="系统生成内容并预填发布链接，你点击跳转后在各平台确认发布。零封号风险。发布后约 1-2 周被 AI 引擎收录，届时可在发布记录点「复测提及率」验证效果。" />
                    ) : (
                      <>
                        <Alert type="warning" showIcon style={{ marginBottom: 12 }} message="全自动发布模式"
                          description="系统自动打开浏览器，注入登录态，自动填充标题正文并点击发布。请确保服务器已安装 Chrome。收录周期约 1-2 周，发布记录支持复测验证。" />
                        <div style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 8 }}>
                          <Switch checked={autoSelect} onChange={setAutoSelect} size="small" />
                          <Text style={{ fontSize: 13 }}>自动选号（系统自动选择最久未使用的健康账号，避免单号高频被封）</Text>
                        </div>
                      </>
                    )}

                    {/* D: 可编辑发布标题（带平台字数计数器——发布前可调整，解决截断问题） */}
                    {selectedContent && (
                      <div style={{ marginBottom: 16 }}>
                        {(() => {
                          // 检测选中平台：小红书标题≤20字，知乎≤100字，混合取最严(20)
                          const selectedAccs = autoSelect ? healthyAccounts : accounts.filter(a => selectedAccountIds.includes(a.id))
                          const hasXHS = selectedAccs.some(a => a.platform === 'xiaohongshu')
                          const titleLimit = hasXHS ? 20 : 100
                          const titleLen = [...publishTitle].length
                          const overLimit = titleLen > titleLimit
                          return (
                            <>
                              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 6 }}>
                                <Text strong style={{ fontSize: 13 }}>发布标题{hasXHS && <Tag color="red" style={{ marginLeft: 6, fontSize: 10 }}>小红书≤{titleLimit}字</Tag>}</Text>
                                <Tooltip title={overLimit ? `超出${titleLen - titleLimit}字，发布时自动截断` : '可在发布前调整标题'}>
                                  <Text style={{ fontSize: 12 }} type={overLimit ? 'danger' : 'secondary'}>{titleLen}/{titleLimit}</Text>
                                </Tooltip>
                              </div>
                              <Input.TextArea
                                rows={2}
                                value={publishTitle}
                                onChange={e => setPublishTitle(e.target.value)}
                                placeholder="发布标题（可在发布前调整）"
                                style={{ fontSize: 14, borderColor: overLimit ? 'var(--wr-danger)' : undefined }}
                                showCount={false}
                              />
                              {overLimit && (
                                <Text type="danger" style={{ fontSize: 11 }}>
                                  当前{titleLen}字超限，发{hasXHS ? '小红书' : '该平台'}将截断为："{[...publishTitle].slice(0, titleLimit).join('')}"
                                </Text>
                              )}
                            </>
                          )
                        })()}
                      </div>
                    )}

                    {/* D: 正文预览（折叠展开，支持编辑） */}
                    {selectedContent && publishContentText && (
                      <details style={{ marginBottom: 16 }}>
                        <summary style={{ cursor: 'pointer', fontSize: 13, color: 'var(--wr-text-secondary)' }}>
                          正文预览（{[...publishContentText].length} 字，点击展开编辑）
                        </summary>
                        <Input.TextArea
                          rows={6}
                          value={publishContentText}
                          onChange={e => setPublishContentText(e.target.value)}
                          style={{ marginTop: 8, fontSize: 13 }}
                        />
                      </details>
                    )}

                    {/* 小红书图文配图（小红书账号存在时显示；image 类型必填 ≥1 张） */}
                    {healthyAccounts.some(a => a.platform === 'xiaohongshu') && (
                      <div style={{ marginBottom: 16, padding: 12, background: 'var(--wr-bg-elevated)', borderRadius: 8 }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                          <Text strong style={{ fontSize: 13 }}>小红书配图（图文必填 ≥1 张）</Text>
                          <Button size="small" icon={<PictureOutlined />} onClick={() => setShowAssetPicker(true)}>
                            从素材库选图
                          </Button>
                        </div>
                        {mediaUrls.length === 0 ? (
                          <Text type="secondary" style={{ fontSize: 12 }}>未选图——发小红书会失败（图文笔记硬约束）</Text>
                        ) : (
                          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                            {mediaUrls.map((u, i) => (
                              <div key={i} style={{ position: 'relative' }}>
                                <img src={u} alt="" style={{ width: 52, height: 52, borderRadius: 6, objectFit: 'cover', border: '1px solid #eee' }} />
                                <Button size="small" type="text" danger icon={<DeleteOutlined />}
                                  onClick={() => setMediaUrls(mediaUrls.filter((_, j) => j !== i))}
                                  style={{ position: 'absolute', top: -8, right: -8, padding: 0, background: '#fff', borderRadius: '50%' }} />
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    )}

                    {publishing && publishMode === 'auto' && autoStatus && (
                      <div style={{ marginBottom: 16, padding: 12, background: 'var(--wr-bg-elevated)', borderRadius: 8 }}>
                        {autoStatus.map(s => (
                          <div key={s.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
                            <Space>
                              <Tag>{PLATFORM_NAMES[s.platform] || s.platform}</Tag>
                              <Text style={{ fontSize: 13, color: statusConfig(s.status).color }}>
                                {statusConfig(s.status).icon} {statusConfig(s.status).label}
                              </Text>
                            </Space>
                            {s.external_url && s.status === 'published' && (
                              <Button size="small" type="link" icon={<ExportOutlined />} href={s.external_url} target="_blank">查看文章</Button>
                            )}
                          </div>
                        ))}
                      </div>
                    )}

                    <Space direction="vertical" style={{ width: '100%' }}>
                      {healthyAccounts.map((a) => {
                        const selected = selectedAccountIds.includes(a.id)
                        const toggle = () => {
                          setSelectedAccountIds(prev => selected ? prev.filter(x => x !== a.id) : [...prev, a.id])
                        }
                        return (
                          <div key={a.id} onClick={toggle} style={{
                            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                            padding: '10px 14px', borderRadius: 8, cursor: 'pointer',
                            border: selected ? '1px solid var(--wr-primary)' : '1px solid var(--wr-border)',
                            background: selected ? 'var(--wr-primary-bg)' : 'transparent',
                            transition: 'all 200ms ease',
                          }}>
                            <Space>
                              <Tag>{PLATFORM_NAMES[a.platform] || a.platform}</Tag>
                              <Text>{a.display_name}</Text>
                            </Space>
                            {selected && <CheckCircleOutlined style={{ color: 'var(--wr-primary)' }} />}
                          </div>
                        )
                      })}
                    </Space>

                    {/* 定时发送（排期发布：到期自动执行）*/}
                    <div style={{ marginTop: 16, padding: 12, borderRadius: 10, border: '1px solid var(--wr-border)', background: 'var(--wr-bg-elevated)' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                        <Switch size="small" checked={!!scheduleTime} onChange={(v) => setScheduleTime(v ? dayjs().add(1, 'hour').toDate() : null)} />
                        <Text style={{ fontSize: 13 }}>定时发送（排期发布）</Text>
                      </div>
                      {scheduleTime && (
                        <DatePicker
                          showTime
                          value={dayjs(scheduleTime)}
                          onChange={(v) => setScheduleTime(v ? v.toDate() : null)}
                          style={{ width: '100%' }}
                          disabledDate={(d) => d && d.isBefore(dayjs().startOf('day'))}
                          placeholder="选择自动发布时间"
                        />
                      )}
                    </div>
                    <Button
                      type="primary" size="large" block style={{ marginTop: 16 }}
                      loading={publishing}
                      disabled={!selectedContent || (!autoSelect && selectedAccountIds.length === 0)}
                      onClick={handlePublish}
                    >
                      {publishing && publishMode === 'auto' ? '自动发布中...'
                        : publishing ? '生成发布链接中...'
                        : `发布到 ${selectedAccountIds.length} 个平台`}
                    </Button>
                  </>
                )}
              </div>
            </Col>
          </Row>
        </Card>

        {/* ===== ③ 发布记录 ===== */}
        <Card className="wr-glass-card" title="发布记录">
          {jobs.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无发布记录" style={{ padding: 40 }} />
          ) : (
            <Table dataSource={jobs} columns={jobColumns} rowKey="id" pagination={{ pageSize: 10 }} size="small" />
          )}
        </Card>
      </div>

      {/* 扫码登录弹窗 */}
      <Modal
        title={`绑定 ${PLATFORMS.find((p) => p.key === activePlatform)?.name || ''} 账号`}
        open={qrModalOpen} onCancel={handleCloseModal} footer={null} width={400} centered
      >
        <div style={{ textAlign: 'center', padding: '12px 0' }}>
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
              <div style={{ display: 'inline-block', padding: 16, background: '#fff', borderRadius: 12, marginBottom: 16 }}>
                <img
                  src={currentQrImage.startsWith('http') ? currentQrImage : `data:image/png;base64,${currentQrImage}`}
                  alt="登录二维码" style={{ width: 240, height: 'auto', maxHeight: 320, display: 'block' }}
                />
              </div>
              <QRStatusIndicator status={pollData?.status} platform={activePlatform} />
            </>
          ) : (
            <div style={{ padding: 60 }}>
              <Spin size="large" />
              <Paragraph type="secondary" style={{ marginTop: 16 }}>正在启动浏览器获取二维码...</Paragraph>
            </div>
          )}
        </div>
      </Modal>

      {/* 发布链接弹窗 */}
      <Modal
        title="发布链接已生成"
        open={linkModalOpen}
        onCancel={() => setLinkModalOpen(false)}
        footer={<Button type="primary" onClick={() => setLinkModalOpen(false)}>完成</Button>}
        width={520}
      >
        <Alert type="success" showIcon style={{ marginBottom: 16 }} message="内容已准备就绪"
          description="点击下方链接跳转到各平台发布页，粘贴内容并确认发布后，回到这里点击「标记已发布」。发布后约 1-2 周被 AI 引擎收录，可在发布记录点「复测提及率」验证效果。" />
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          {publishLinks.map((job) => (
            <div key={job.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: 12, background: 'var(--wr-bg-elevated)', borderRadius: 8 }}>
              <Space>
                <Tag>{PLATFORM_NAMES[job.platform] || job.platform}</Tag>
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

      {/* 素材库选图 Modal（小红书图文配图） */}
      <Modal
        title="从素材库选择图片"
        open={showAssetPicker}
        onCancel={() => setShowAssetPicker(false)}
        onOk={() => setShowAssetPicker(false)}
        okText="完成"
        width={640}
      >
        {pickerImages.length === 0 ? (
          <Empty description="暂无图片素材，请先在创作工作台上传" />
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 10, maxHeight: 380, overflow: 'auto' }}>
            {pickerImages.map(a => {
              const active = mediaUrls.includes(a.url)
              return (
                <div
                  key={a.id}
                  onClick={() => setMediaUrls(prev => active ? prev.filter(u => u !== a.url) : [...prev, a.url])}
                  style={{
                    border: active ? '2px solid var(--wr-primary)' : '1px solid #e5e7eb',
                    borderRadius: 8, overflow: 'hidden', cursor: 'pointer',
                  }}
                >
                  <img src={a.url} alt="" style={{ width: '100%', height: 88, objectFit: 'cover' }} />
                  <div style={{ padding: 4, fontSize: 11, color: '#999', textAlign: 'center' }}>
                    {Math.round(a.size_bytes / 1024)}KB
                  </div>
                </div>
              )
            })}
          </div>
        )}
        <div style={{ marginTop: 8, fontSize: 12, color: '#999' }}>已选 {mediaUrls.length} 张（点击图片切换选中）</div>
      </Modal>
    </div>
  )
}

// 扫码状态指示器
function QRStatusIndicator({ status, platform }: { status?: string; platform: string }) {
  const pfName = PLATFORMS.find((p) => p.key === platform)?.name || ''
  if (!status || status === 'preparing') {
    return <Space><Spin size="small" /><Text type="secondary">浏览器已打开，正在获取二维码...</Text></Space>
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
    return <Space><CheckCircleOutlined style={{ color: 'var(--wr-accent)' }} /><Text style={{ color: 'var(--wr-accent)' }}>已扫码，请在手机确认登录</Text></Space>
  }
  if (status === 'expired') return <Text type="warning">二维码已过期，请关闭后重新获取</Text>
  if (status === 'success') {
    return <Space><CheckCircleOutlined style={{ color: 'var(--wr-success)' }} /><Text style={{ color: 'var(--wr-success)' }}>登录成功，正在绑定...</Text></Space>
  }
  return <Text type="danger">扫码异常：{status}</Text>
}
