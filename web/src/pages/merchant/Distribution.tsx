import { useState, useEffect, useMemo, useRef } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Card, Typography, Button, Row, Col, Select, Tag, Space, message, Empty, Table, Modal, Alert, Switch, Popconfirm, Input, Tabs, Segmented, Collapse } from 'antd'
import { ExportOutlined, CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined, LinkOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { scoreColor } from '../../utils/geo'
import { useBrandContext } from '../../hooks/useBrands'
import AssetPicker from '../../components/AssetPicker'
import QRLoginModal from './distribution/QRLoginModal'
import PublishJobTable from './distribution/PublishJobTable'
import type { Brand, OptimizedContent, Account, PublishJob } from '../../types/api'

const { Text, Paragraph } = Typography

// ---- 平台元信息 ----
const PLATFORMS = [
  { key: 'douyin', name: '抖音', color: 'var(--wr-text-primary)', desc: '短视频获客主战场，本地商户首选' },
  { key: 'kuaishou', name: '快手', color: 'var(--wr-warning)', desc: '短视频平台，下沉市场覆盖广' },
  { key: 'xiaohongshu', name: '小红书', color: 'var(--wr-danger)', desc: '种草社区，本地生活/装修类精准触达' },
  { key: 'zhihu', name: '知乎', color: 'var(--wr-primary)', desc: '知识问答社区，长文 SEO 效果好' },
]
const PLATFORM_NAMES: Record<string, string> = { douyin: '抖音', kuaishou: '快手', zhihu: '知乎', xiaohongshu: '小红书' }

// 健康度 → 显示配置（账号池表格用）
function healthConfig(health: string) {
  switch (health) {
    case 'active': return { color: 'var(--wr-success)', label: '健康', icon: <CheckCircleOutlined /> }
    case 'expired': return { color: 'var(--wr-warning)', label: '已过期', icon: <ClockCircleOutlined /> }
    case 'banned': return { color: 'var(--wr-danger)', label: '已封禁', icon: <CloseCircleOutlined /> }
    default: return { color: 'var(--wr-text-muted)', label: '未知', icon: <ClockCircleOutlined /> }
  }
}

// 分发中心：账号池（绑定/维护）+ 内容发布（半自动/全自动）整合页。
// 发布强依赖账号——账号池是分发基础设施，合并后一个页面完成「选号 → 发布 → 复测」闭环。
// 子组件：QRLoginModal（扫码绑定）/ PublishJobTable（发布记录+复测）/ AssetPicker（配图，与多媒体创作共用）。
export default function Distribution() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  // ---- 账号池状态 ----
  // 扫码会话细节（二维码/轮询/取消）封装在 QRLoginModal 内，页面只管开关与平台
  const [qrModalOpen, setQrModalOpen] = useState(false)
  const [activeTab, setActiveTab] = useState('accounts') // 页内 Tab（账号池/发布/发布记录）
  const [activePlatform, setActivePlatform] = useState('')

  // ---- 发布状态 ----
  // 品牌走全局上下文（与内容生成/监测页共享）；仅内容 ID 是本页局部状态
  const { brands, brandId: selectedBrand, setCurrentBrand } = useBrandContext()
  const [selectedContentId, setSelectedContentId] = useState<string>()
  const [selectedAccountIds, setSelectedAccountIds] = useState<string[]>([])
  const [publishing, setPublishing] = useState(false)
  const [publishMode, setPublishMode] = useState<'semi-auto' | 'auto'>('semi-auto')
  const [autoSelect, setAutoSelect] = useState(false)
  const [linkModalOpen, setLinkModalOpen] = useState(false)
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

  // B+C: 接收跳转参数（内容生成的「去社媒分发」入口预选内容；配图来自多媒体创作）
  const [searchParams] = useSearchParams()
  useEffect(() => {
    const qContentId = searchParams.get('contentId')
    const qBrandId = searchParams.get('brandId')
    const qMediaUrls = searchParams.get('mediaUrls')
    if (qBrandId) setCurrentBrand(qBrandId)
    if (qContentId) setSelectedContentId(qContentId)
    if (qMediaUrls) {
      setMediaUrls(qMediaUrls.split(',').filter(Boolean))
      setPublishForm('video') // 多媒体创作跳转的是视频产物
    }
  }, [searchParams]) // eslint-disable-line react-hooks/exhaustive-deps

  // 抖音官方 OAuth 授权回调结果（服务端完成绑定后 302 跳回本页并带结果参数）
  useEffect(() => {
    const oauthResult = searchParams.get('douyin_oauth')
    if (!oauthResult) return
    if (oauthResult === 'success') {
      const name = searchParams.get('name') || ''
      message.success(`抖音账号「${name}」官方授权绑定成功`)
    } else {
      message.error(`官方授权失败：${searchParams.get('reason') || '未知原因'}`)
    }
    queryClient.invalidateQueries({ queryKey: ['geo-accounts'] })
    // 清掉 URL 参数，避免刷新重复弹提示
    window.history.replaceState({}, '', window.location.pathname)
  }, [searchParams, queryClient])

  // 抖音绑定（统一入口，用户无感知双通道）：优先官方 OAuth 授权（API 通道），
  // 服务端未配置 DOUYIN_* 时自动降级浏览器扫码（RPA 通道）
  const bindDouyin = async () => {
    try {
      const { url } = await businessApi.getDouyinOAuthURL()
      window.open(url, '_blank')
    } catch {
      openBindModal('douyin')
    }
  }

  const { data: accounts = [], isLoading: accountsLoading } = useQuery({
    queryKey: ['geo-accounts'],
    queryFn: () => businessApi.listAccounts(),
  })
  // 平台能力清单（服务端 ChannelRegistry 导出——能力驱动：平台过滤/动态检查清单，
  // 新平台注册即自动出现，前端零硬编码）
  const { data: channels = [] } = useQuery({
    queryKey: ['geo-publish-channels'],
    queryFn: () => businessApi.listPublishChannels().then((r) => r.channels).catch(() => []),
    staleTime: 5 * 60_000,
  })
  const channelByPlatform = useMemo(() => new Map(channels.map((ch) => [ch.platform, ch])), [channels])
  const supportsForm = (platform: string, form: string) =>
    channelByPlatform.get(platform)?.content_types?.includes(form) ?? false
  // 发布形态（发什么）：文章 / 图文笔记 / 视频——跳转参数预判（视频产物→video）
  const [publishForm, setPublishForm] = useState<'article' | 'image' | 'video'>('article')
  const { data: contents = [] } = useQuery({
    queryKey: ['geo-contents', selectedBrand],
    queryFn: () => businessApi.listContents(selectedBrand!),
    enabled: !!selectedBrand,
  })
  const { data: jobs = [] } = useQuery({
    queryKey: ['geo-publish-jobs'],
    queryFn: () => businessApi.listPublishJobs(),
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

  const selectedContent = contents.find((c) => c.id === selectedContentId)
  // 账号按所选内容形态过滤（不支持该形态的平台账号不出现——能力驱动）
  const healthyAccounts = accounts.filter((a) => a.health === 'active' && supportsForm(a.platform, publishForm))

  // ---- 智能默认（傻瓜化 Q5）----
  // 默认 Tab 跟着状态走：有健康账号直接落"发布"（主操作），没有则留在账号池（先绑定）
  const tabTouched = useRef(false)
  useEffect(() => {
    if (!accountsLoading && !tabTouched.current && healthyAccounts.length > 0) {
      setActiveTab('publish')
    }
  }, [accountsLoading, healthyAccounts.length])  
  // 未带跳转参数时默认选中最新一篇内容（优先草稿），标题正文自动带出
  const hasQueryParam = !!searchParams.get('contentId')
  useEffect(() => {
    if (hasQueryParam || selectedContentId) return
    if (contents.length === 0) return
    const drafts = contents.filter((c) => c.status === 'draft')
    const pool = drafts.length > 0 ? drafts : contents
    const latest = [...pool].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())[0]
    if (latest) setSelectedContentId(latest.id)
  }, [contents, hasQueryParam, selectedContentId])  
  // 只有一个健康账号时自动勾选——没有选择就不该是选择；多账号不预选（发布是外发动作，让用户明确选）
  useEffect(() => {
    if (healthyAccounts.length === 1 && selectedAccountIds.length === 0) {
      setSelectedAccountIds([healthyAccounts[0].id])
    }
  }, [healthyAccounts]) // eslint-disable-line react-hooks/exhaustive-deps

  // ---- 账号操作 ----
  const openBindModal = (platform: string) => {
    setActivePlatform(platform)
    setQrModalOpen(true)
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
    try {
      const results: PublishJob[] = []
      // 标题截断/素材按平台能力约束处理（constraints 从服务端下发——前端零硬编码）
      const titleFor = (platform: string): string => {
        const max = channelByPlatform.get(platform)?.constraints?.[publishForm]?.title_max_runes || 0
        return max > 0 && publishTitle ? [...publishTitle].slice(0, max).join('') : publishTitle
      }
      const needsMedia = (platform: string): boolean => {
        const c = channelByPlatform.get(platform)?.constraints?.[publishForm]
        return !!c && ((c.min_images || 0) > 0 || (c.min_videos || 0) > 0)
      }
      if (autoSelect && publishMode === 'auto') {
        const platforms = [...new Set(healthyAccounts.map(a => a.platform))]
        for (const platform of platforms) {
          results.push(await businessApi.publishContent({
            account_id: '', platform, content_id: selectedContent.id, brand_id: selectedBrand,
            title: titleFor(platform), content: publishContentText, mode: publishMode,
            content_type: publishForm,
            media_urls: needsMedia(platform) ? mediaUrls : undefined,
          }))
        }
      } else {
        for (const accId of selectedAccountIds) {
          const acc = accounts.find((a) => a.id === accId)
          if (!acc) continue
          results.push(await businessApi.publishContent({
            account_id: accId, platform: acc.platform, content_id: selectedContent.id, brand_id: selectedBrand,
            title: titleFor(acc.platform), content: publishContentText, mode: publishMode,
            content_type: publishForm,
            media_urls: needsMedia(acc.platform) ? mediaUrls : undefined,
          }))
        }
      }
      if (publishMode === 'auto') {
        setAutoJobIds(results.map(j => j.id))
        message.success(`已启动 ${results.length} 个自动发布任务`)
      } else {
        setPublishLinks(results)
        setLinkModalOpen(true)
        message.success(`已生成 ${results.length} 个发布链接`)
        setPublishing(false)
      }
      queryClient.invalidateQueries({ queryKey: ['geo-publish-jobs'] })
    } catch { /* 拦截器已提示 */ } finally {
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

  return (
    <div className="wr-page-content ip-page" style={{ paddingTop: 4, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        <div className="ip-page-hero">
          <div>
            <p className="ip-kicker">Publish</p>
            <h1>发布中心</h1>
            <p className="ip-lead">绑定平台账号、发布作品——成功后可在「我的作品」与「作品数据」复盘</p>
          </div>
          <Button onClick={() => navigate('/m/works')}>查看我的作品</Button>
        </div>

        {/* P1-6-1：单页五任务 → 三 Tab（账号池 / 发布 / 发布记录）——重页 Tab 化 */}
        <Tabs
          activeKey={activeTab}
          onChange={(k) => { tabTouched.current = true; setActiveTab(k) }}
          style={{ marginBottom: 12 }}
          items={[
            { key: 'accounts', label: '账号池', children: (<>
        {/* P4-03 半自动定位指引：平台 POI 挂载（RPA 自动定位暂缓） */}
        <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
          <Space align="start" style={{ width: '100%' }}>
            <LinkOutlined style={{ color: 'var(--wr-warning)', marginTop: 3 }} />
            <div style={{ flex: 1 }}>
              <Text strong style={{ fontSize: 14 }}>发布带定位 = 本地曝光更好（半自动指引）</Text>
              <Paragraph type="secondary" style={{ fontSize: 12, margin: '4px 0 8px' }}>
                平台「添加定位」自动化暂缓——先用最稳的半自动方式：
              </Paragraph>
              <Space direction="vertical" size={2} style={{ fontSize: 12, color: 'var(--wr-text-secondary)' }}>
                <div>① 在「人设档案 · 门店档案」维护真实地址</div>
                <div>② 发布内容会自动附带门店地址信息（有地址时）</div>
                <div>③ 平台发布页手动「添加定位 → 搜索门店 → 选中」</div>
                <div>④ 带定位内容更容易被附近的人看到，也利于账号本地信任感</div>
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
                        const isOAuth = a.auth_type === 'oauth'
                        return (
                          <div key={a.id} style={{
                            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                            padding: '8px 12px', background: 'var(--wr-bg-elevated)', borderRadius: 8,
                            borderLeft: isExpired ? '3px solid var(--wr-danger)' : '3px solid transparent',
                          }}>
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
                        <Button type="dashed" block onClick={bindDouyin}>绑定更多抖音账号</Button>
                      ) : (
                        <Button type="dashed" block icon={<LinkOutlined />} onClick={() => openBindModal(pf.key)}>
                          绑定更多 {pf.name} 账号
                        </Button>
                      )}
                    </Space>
                  ) : pf.key === 'douyin' ? (
                    <Button type="primary" block onClick={bindDouyin} disabled={accountsLoading}>
                      绑定抖音账号
                    </Button>
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
        </>
      )},
            { key: 'publish', label: '发布', children: (<>
        <Card className="wr-glass-card" styles={{ body: { padding: 24 } }}>
          {/* 人设上下文 */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20, paddingBottom: 12, borderBottom: '1px solid var(--wr-border)' }}>
            <Text strong>人设</Text>
            <Select style={{ maxWidth: 320, minWidth: 200, flex: 1 }} placeholder="选择人设" value={selectedBrand}
              onChange={(v) => { setCurrentBrand(v); setSelectedContentId(undefined) }}
              options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))} />
          </div>
          {/* 第一步：发什么？ */}
          <div style={{ marginBottom: 24 }}>
            <Text strong style={{ fontSize: 16, display: 'block', marginBottom: 12 }}>你想发什么内容？</Text>
            <Segmented block value={publishForm}
              onChange={(v) => { setPublishForm(v as 'article' | 'image' | 'video'); setSelectedContentId(undefined); setSelectedAccountIds([]); setMediaUrls([]) }}
              options={[{ value: 'article', label: '发文章' }, { value: 'image', label: '发图文笔记' }, { value: 'video', label: '发视频' }]}
              style={{ marginBottom: 16 }} />
            {!selectedBrand ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="请先选择人设" style={{ padding: 40 }} />
            ) : contents.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="该人设暂无可发内容，可先去内容合成" style={{ padding: 40 }}>
                <Space>
                  <Button type="primary" className="ip-btn-primary" onClick={() => navigate('/m/compose')}>去内容合成</Button>
                  <Button onClick={() => navigate('/m/works')}>我的作品</Button>
                </Space>
              </Empty>
            ) : (
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 12, maxHeight: 320, overflowY: 'auto' }}>
                {contents
                  .sort((a, b) => {
                    if (a.status === 'draft' && b.status !== 'draft') return -1
                    if (a.status !== 'draft' && b.status === 'draft') return 1
                    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
                  })
                  .slice(0, 20)
                  .map((c: OptimizedContent) => {
                    const total = c.score?.total || 0
                    const isSelected = selectedContentId === c.id
                    return (
                      <div key={c.id} onClick={() => setSelectedContentId(c.id)} style={{
                        padding: '12px 14px', borderRadius: 10, cursor: 'pointer',
                        border: isSelected ? '2px solid var(--wr-primary)' : '1px solid var(--wr-border)',
                        background: isSelected ? 'var(--wr-primary-bg)' : 'var(--wr-bg-surface)',
                        transition: 'all 200ms ease', opacity: c.status === 'published' ? 0.7 : 1,
                      }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
                          <Tag color={scoreColor(total)} style={{ margin: 0, fontSize: 10 }}>{total.toFixed(0)}</Tag>
                          {c.status === 'published' ? <Tag color="success" style={{ margin: 0, fontSize: 10 }}>已发布</Tag> : <Tag style={{ margin: 0, fontSize: 10 }}>草稿</Tag>}
                        </div>
                        <Text strong ellipsis style={{ fontSize: 13, display: 'block' }}>{c.title || '(无标题)'}</Text>
                        <Text type="secondary" ellipsis style={{ fontSize: 11, display: 'block', marginTop: 4 }}>{c.optimized_text?.slice(0, 60)}...</Text>
                      </div>
                    )
                  })}
              </div>
            )}
          </div>
          {/* 第二步：发到哪里？ */}
          <div style={{ marginBottom: 24 }}>
            <Text strong style={{ fontSize: 16, display: 'block', marginBottom: 12 }}>发到哪里？</Text>
            {healthyAccounts.length === 0 ? (
              <div style={{ textAlign: 'center', padding: '24px 0' }}>
                <Text type="secondary" style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>还没有绑定账号——先绑定才能发布</Text>
                <Space>
                  {PLATFORMS.map((pf) => (
                    <Button key={pf.key} type="dashed" size="small" icon={<LinkOutlined />} onClick={() => openBindModal(pf.key)}>绑定 {pf.name}</Button>
                  ))}
                </Space>
              </div>
            ) : (
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                {healthyAccounts.map((a) => {
                  const selected = selectedAccountIds.includes(a.id)
                  const ch = channelByPlatform.get(a.platform)
                  return (
                    <div key={a.id} onClick={() => setSelectedAccountIds(prev => selected ? prev.filter(x => x !== a.id) : [...prev, a.id])}
                      style={{ display: 'inline-flex', alignItems: 'center', gap: 8, padding: '8px 14px', borderRadius: 20, cursor: 'pointer',
                        border: selected ? '2px solid var(--wr-primary)' : '1px solid var(--wr-border)',
                        background: selected ? 'var(--wr-primary-bg)' : 'var(--wr-bg-surface)', transition: 'all 200ms ease' }}>
                      <Text style={{ fontSize: 13 }}>{ch?.name || PLATFORM_NAMES[a.platform] || a.platform}</Text>
                      <Text type="secondary" style={{ fontSize: 11 }}>{a.display_name}</Text>
                      {selected && <CheckCircleOutlined style={{ color: 'var(--wr-primary)', fontSize: 14 }} />}
                    </div>
                  )
                })}
              </div>
            )}
          </div>
          {/* 第三步：确认 + 检查清单 + 发布 */}
          {selectedContent && selectedAccountIds.length > 0 && (
            <div style={{ marginBottom: 24 }}>
              <Text strong style={{ fontSize: 16, display: 'block', marginBottom: 12 }}>确认发布信息</Text>
              {/* 标题编辑 */}
              {(() => {
                const selectedAccs = accounts.filter(a => selectedAccountIds.includes(a.id))
                let titleLimit = 0
                for (const a of selectedAccs) {
                  const max = channelByPlatform.get(a.platform)?.constraints?.[publishForm]?.title_max_runes || 0
                  if (max > 0 && (titleLimit === 0 || max < titleLimit)) titleLimit = max
                }
                const titleLen = [...publishTitle].length
                const overLimit = titleLimit > 0 && titleLen > titleLimit
                return (
                  <div style={{ marginBottom: 12 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 6 }}>
                      <Text strong style={{ fontSize: 13 }}>发布标题</Text>
                      {titleLimit > 0 && <Text style={{ fontSize: 11 }} type={overLimit ? 'danger' : 'secondary'}>{titleLen}/{titleLimit} 字</Text>}
                    </div>
                    <Input.TextArea rows={2} value={publishTitle} onChange={e => setPublishTitle(e.target.value)} placeholder="发布标题" style={{ fontSize: 14 }} />
                  </div>
                )
              })()}
              {/* 正文预览 */}
              {publishContentText && (
                <details style={{ marginBottom: 12 }}>
                  <summary style={{ cursor: 'pointer', fontSize: 13, color: 'var(--wr-text-secondary)' }}>正文预览（{[...publishContentText].length} 字，点击展开编辑）</summary>
                  <Input.TextArea rows={6} value={publishContentText} onChange={e => setPublishContentText(e.target.value)} style={{ marginTop: 8, fontSize: 13 }} />
                </details>
              )}
              {/* 高级选项 */}
              <Collapse ghost size="small" style={{ marginBottom: 16 }} items={[{
                key: 'advanced', label: <span style={{ fontSize: 13 }}>高级选项</span>,
                children: (<>
                  <div style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 8 }}>
                    <Switch checked={publishMode === 'auto'} onChange={(v) => setPublishMode(v ? 'auto' : 'semi-auto')} size="small" />
                    <Text style={{ fontSize: 13 }}>全自动发布（系统自动操作浏览器，有封号风险）</Text>
                  </div>
                  {publishMode === 'auto' && (
                    <div style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 8 }}>
                      <Switch checked={autoSelect} onChange={setAutoSelect} size="small" />
                      <Text style={{ fontSize: 13 }}>自动选号（用最久没发过的账号，降低封号风险）</Text>
                    </div>
                  )}
                </>),
              }]} />
              {/* 检查清单 */}
              {(() => {
                const selectedAccs = autoSelect && publishMode === 'auto' ? healthyAccounts : accounts.filter(a => selectedAccountIds.includes(a.id))
                let titleLimit = 0, minImages = 0, minVideos = 0
                for (const a of selectedAccs) {
                  const c = channelByPlatform.get(a.platform)?.constraints?.[publishForm]
                  const max = c?.title_max_runes || 0
                  if (max > 0 && (titleLimit === 0 || max < titleLimit)) titleLimit = max
                  minImages = Math.max(minImages, c?.min_images || 0)
                  minVideos = Math.max(minVideos, c?.min_videos || 0)
                }
                const needMedia = publishForm !== 'article' || minImages > 0 || minVideos > 0
                const titleLen = [...publishTitle].length
                const items: { ok: boolean; text: string }[] = [
                  { ok: true, text: `已选内容：${selectedContent.title || '(无标题)'}` },
                  { ok: true, text: `将发布到 ${selectedAccs.length} 个账号` },
                  ...(titleLimit > 0 ? [titleLen > 0 && titleLen <= titleLimit ? { ok: true, text: '标题符合限制' } : titleLen > titleLimit ? { ok: false, text: '标题超限' } : { ok: false, text: '还没填标题' }] : []),
                  ...(needMedia ? [mediaUrls.length > 0 ? { ok: true, text: '已选素材' } : { ok: false, text: '请选素材' }] : []),
                ]
                const allOk = items.every(i => i.ok)
                return (
                  <div style={{ marginBottom: 16, padding: 14, borderRadius: 10, background: allOk ? 'var(--wr-bg-elevated)' : 'rgba(251,191,36,0.08)', border: allOk ? '1px solid var(--wr-border)' : '1px solid rgba(251,191,36,0.2)' }}>
                    <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>{allOk ? '准备就绪' : '发布前检查'}</Text>
                    {items.map((it, i) => (
                      <div key={i} style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 4 }}>
                        <span style={{ color: it.ok ? 'var(--wr-success)' : 'var(--wr-warning)', fontSize: 13 }}>{it.ok ? '✓' : '⋯'}</span>
                        <Text style={{ fontSize: 12.5, color: it.ok ? 'var(--wr-text-secondary)' : 'var(--wr-warning)' }}>{it.text}</Text>
                      </div>
                    ))}
                  </div>
                )
              })()}
              <Button type="primary" size="large" block loading={publishing} onClick={handlePublish}>
                {publishing ? '发布中...' : `发布到 ${selectedAccountIds.length} 个平台`}
              </Button>
            </div>
          )}
        </Card>
        </>
      )},
            { key: 'records', label: '发布记录', children: (<>
        {/* 效果聚合卡（P1-6-2）：平台×篇数×复测均值变化——分发效果一目了然 */}
        <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 12 }}>
            {PLATFORMS.map((pf) => {
              const pj = jobs.filter((j: PublishJob) => j.platform === pf.key)
              const monitored = pj.filter((j: PublishJob) => j.post_mention_rate != null)
              const avgDelta = monitored.length > 0
                ? monitored.reduce((s: number, j: PublishJob) => s + (j.post_mention_rate - (j.pre_mention_rate || 0)), 0) / monitored.length
                : null
              return (
                <div key={pf.key} style={{ padding: '12px 16px', background: 'var(--wr-bg-elevated)', borderRadius: 10 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6 }}>
                    <span style={{ width: 8, height: 8, borderRadius: '50%', background: pf.color, display: 'inline-block' }} />
                    <Text strong style={{ fontSize: 13 }}>{pf.name}</Text>
                    <Tag style={{ margin: 0, fontSize: 10 }}>{pj.length} 篇</Tag>
                  </div>
                  <Text style={{ fontSize: 12, color: 'var(--wr-text-muted)' }}>
                    {avgDelta === null
                      ? '暂无复测数据（发布一段时间后可复测表现）'
                      : <>表现变化均值 <b style={{ color: avgDelta >= 0 ? 'var(--wr-success)' : 'var(--wr-danger)' }}>
                          {avgDelta >= 0 ? '+' : ''}{(avgDelta * 100).toFixed(1)}%</b>
                          （{monitored.length} 篇已复测）</>}
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
            reMonitorPending={null}
          />
        </Card>
        </>)},
          ]}
        />
      </div>

      {/* 扫码登录弹窗（二维码/轮询/取消自包含在组件内） */}
      <QRLoginModal open={qrModalOpen} platform={activePlatform} onClose={() => setQrModalOpen(false)} />

      {/* 发布链接弹窗 */}
      <Modal
        title="发布链接已生成"
        open={linkModalOpen}
        onCancel={() => setLinkModalOpen(false)}
        footer={<Button type="primary" onClick={() => setLinkModalOpen(false)}>完成</Button>}
        width={520}
      >
        <Alert type="success" showIcon style={{ marginBottom: 16 }} message="内容已准备就绪"
          description="点击下方链接前往各平台发布页完成发布，然后回到这里点「标记已发布」。效果可在「作品数据」查看；也可在发布记录中复测表现。" />
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

      {/* 配图选择（与多媒体创作共用 AssetPicker——支持就地上传，无需先去创作页） */}
      <AssetPicker
        open={showAssetPicker}
        mode="multi"
        accept="image"
        title="选择配图（小红书图文）"
        onClose={() => setShowAssetPicker(false)}
        onSelect={(assets) => setMediaUrls(assets.map(a => a.url))}
      />
    </div>
  )
}
