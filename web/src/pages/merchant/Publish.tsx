import { useState, useEffect } from 'react'
import { Card, Typography, Button, Row, Col, Select, Tag, Space, message, Empty, Table, Radio, Modal, Alert, Spin, Switch } from 'antd'
import { ExportOutlined, CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined, LoadingOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { Brand, OptimizedContent, Account, PublishJob } from '../../types/api'

const { Title, Text, Paragraph } = Typography

// 评分 → 颜色
function scoreColor(s: number): string {
  if (s >= 80) return 'var(--wr-success)'
  if (s >= 65) return 'var(--wr-accent)'
  if (s >= 50) return 'var(--wr-warning)'
  return 'var(--wr-danger)'
}

const PLATFORM_NAMES: Record<string, string> = {
  zhihu: '知乎',
  xiaohongshu: '小红书',
}

// 发布状态 → 显示
function statusConfig(status: string) {
  switch (status) {
    case 'published':
      return { color: 'var(--wr-success)', label: '已发布', icon: <CheckCircleOutlined /> }
    case 'running':
      return { color: 'var(--wr-primary)', label: '自动发布中', icon: <LoadingOutlined /> }
    case 'pending':
      return { color: 'var(--wr-warning)', label: '待确认', icon: <ClockCircleOutlined /> }
    case 'failed':
      return { color: 'var(--wr-danger)', label: '失败', icon: <CloseCircleOutlined /> }
    default:
      return { color: 'var(--wr-text-muted)', label: status, icon: <ClockCircleOutlined /> }
  }
}

export default function Publish() {
  const queryClient = useQueryClient()
  const [selectedBrand, setSelectedBrand] = useState<string | undefined>()
  const [selectedContentId, setSelectedContentId] = useState<string>()
  const [selectedAccountIds, setSelectedAccountIds] = useState<string[]>([])
  const [publishing, setPublishing] = useState(false)
  const [publishMode, setPublishMode] = useState<'semi-auto' | 'auto'>('semi-auto')
  const [autoSelect, setAutoSelect] = useState(false) // 全自动模式下是否自动选号
  const [linkModalOpen, setLinkModalOpen] = useState(false)
  const [publishLinks, setPublishLinks] = useState<PublishJob[]>([])
  const [autoJobIds, setAutoJobIds] = useState<string[]>([])

  // 全自动模式：轮询 running 状态的 job 直到完成
  const { data: autoStatus } = useQuery({
    queryKey: ['auto-publish-status', autoJobIds],
    queryFn: async () => {
      const results = await Promise.all(
        autoJobIds.map(id => businessApi.getPublishJobStatus(id))
      )
      return results
    },
    enabled: autoJobIds.length > 0,
    refetchInterval: (query) => {
      const data = query.state.data
      const allDone = data?.every(j => j.status === 'published' || j.status === 'failed')
      return allDone ? false : 3000
    },
  })

  // 全自动发布完成后刷新列表
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

  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })
  const { data: contents = [] } = useQuery({
    queryKey: ['geo-contents', selectedBrand],
    queryFn: () => businessApi.listContents(selectedBrand!),
    enabled: !!selectedBrand,
  })
  const { data: accounts = [] } = useQuery({
    queryKey: ['geo-accounts'],
    queryFn: () => businessApi.listAccounts(),
  })
  const { data: jobs = [] } = useQuery({
    queryKey: ['geo-publish-jobs'],
    queryFn: () => businessApi.listPublishJobs(),
  })

  const selectedContent = contents.find((c) => c.id === selectedContentId)

  // 健康账号按平台分组
  const healthyAccounts = accounts.filter((a) => a.health === 'active')

  // 执行发布：半自动生成预填链接，全自动后台自动发布
  const handlePublish = async () => {
    if (!selectedContent) {
      message.warning('请选择内容')
      return
    }
    // 非自动选号时必须选账号
    if (!autoSelect && selectedAccountIds.length === 0) {
      message.warning('请选择至少一个目标账号，或开启自动选号')
      return
    }
    setPublishing(true)
    setPublishLinks([])

    try {
      const results: PublishJob[] = []

      if (autoSelect && publishMode === 'auto') {
        // 自动选号：每个平台各发一次（后端自动选最优账号）
        const platforms = [...new Set(healthyAccounts.map(a => a.platform))]
        for (const platform of platforms) {
          const job = await businessApi.publishContent({
            account_id: '', // 空=后端自动选号
            platform,
            content_id: selectedContent.id,
            brand_id: selectedBrand,
            title: selectedContent.optimized_text.slice(0, 50),
            content: selectedContent.optimized_text,
            mode: publishMode,
          })
          results.push(job)
        }
      } else {
        // 手动选号
        for (const accId of selectedAccountIds) {
          const acc = accounts.find((a) => a.id === accId)
          if (!acc) continue
          const job = await businessApi.publishContent({
            account_id: accId,
            platform: acc.platform,
            content_id: selectedContent.id,
            brand_id: selectedBrand,
            title: selectedContent.optimized_text.slice(0, 50),
            content: selectedContent.optimized_text,
            mode: publishMode,
          })
          results.push(job)
        }
      }

      if (publishMode === 'auto') {
        // 全自动模式：记录 job ID，开始轮询状态
        setAutoJobIds(results.map(j => j.id))
        message.success(`已启动 ${results.length} 个自动发布任务`)
      } else {
        // 半自动模式：弹出链接弹窗
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

  // 标记为已发布
  const handleMarkPublished = async (jobId: string) => {
    try {
      await businessApi.markPublished(jobId)
      message.success('已标记为发布')
      queryClient.invalidateQueries({ queryKey: ['geo-publish-jobs'] })
    } catch {}
  }

  const jobColumns = [
    {
      title: '标题', dataIndex: 'title', key: 'title',
      render: (t: string) => <Text strong style={{ fontSize: 13 }}>{t || '-'}</Text>,
    },
    {
      title: '平台', dataIndex: 'platform', key: 'platform', width: 100,
      render: (p: string) => <Tag>{PLATFORM_NAMES[p] || p}</Tag>,
    },
    {
      title: '模式', dataIndex: 'mode', key: 'mode', width: 100,
      render: (m: string) => <Text type="secondary" style={{ fontSize: 12 }}>{m === 'semi-auto' ? '半自动' : m}</Text>,
    },
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 100,
      render: (s: string) => {
        const cfg = statusConfig(s)
        return <Space><span style={{ color: cfg.color }}>{cfg.icon}</span><Text style={{ color: cfg.color, fontSize: 12 }}>{cfg.label}</Text></Space>
      },
    },
    {
      title: '创建时间', dataIndex: 'created_at', key: 'time', width: 160,
      render: (t: string) => <Text type="secondary" style={{ fontSize: 12 }}>{t ? new Date(t).toLocaleString() : '-'}</Text>,
    },
    {
      title: '发布时间', dataIndex: 'published_at', key: 'published', width: 160,
      render: (t: string) => <Text type="secondary" style={{ fontSize: 12 }}>{t && t !== '0001-01-01T00:00:00Z' ? new Date(t).toLocaleString() : '-'}</Text>,
    },
    {
      title: '提及率变化', key: 'mention_rate', width: 140,
      render: (_: unknown, r: PublishJob) => {
        if (!r.post_mention_rate) return <Text type="secondary" style={{ fontSize: 12 }}>-</Text>
        const pre = (r.pre_mention_rate * 100).toFixed(1)
        const post = (r.post_mention_rate * 100).toFixed(1)
        const diff = r.post_mention_rate - r.pre_mention_rate
        const color = diff > 0 ? 'var(--wr-success)' : diff < 0 ? 'var(--wr-danger)' : 'var(--wr-text-muted)'
        return (
          <Text style={{ fontSize: 12, color }}>
            {pre}% → {post}%
            {diff !== 0 && ` (${diff > 0 ? '+' : ''}${(diff * 100).toFixed(1)}%)`}
          </Text>
        )
      },
    },
    {
      title: '操作', key: 'action', width: 140,
      render: (_: unknown, r: PublishJob) => (
        <Space>
          {r.external_url && (
            <Button size="small" type="link" icon={<ExportOutlined />} href={r.external_url} target="_blank">
              跳转
            </Button>
          )}
          {r.status === 'pending' && (
            <Button size="small" type="link" onClick={() => handleMarkPublished(r.id)}>
              标记已发布
            </Button>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ paddingTop: 8, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        {/* Hero 区 */}
        <div style={{ marginBottom: 28 }}>
          <Title level={3} style={{ margin: 0, fontSize: 28, letterSpacing: '-0.03em' }}>
            内容发布
          </Title>
          <Text type="secondary" style={{ fontSize: 14 }}>
            将内容工作台生成的文章一键发布到各社媒平台
          </Text>
        </div>

        {/* 品牌选择 */}
        <Card className="wr-glass-card" styles={{ body: { padding: '16px 20px' } }} style={{ marginBottom: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <Text strong style={{ whiteSpace: 'nowrap' }}>选择品牌</Text>
            <Select
              style={{ maxWidth: 320, minWidth: 200, flex: 1 }}
              placeholder="选择品牌查看其内容"
              value={selectedBrand}
              onChange={(v) => { setSelectedBrand(v); setSelectedContentId(undefined) }}
              options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
            />
          </div>
        </Card>

        <Row gutter={16}>
          {/* 左：待发布内容 */}
          <Col xs={24} lg={12}>
            <Card title="待发布内容" styles={{ body: { padding: 16 } }} style={{ minHeight: '100%' }}>
              {!selectedBrand ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="请先选择品牌" style={{ padding: 40 }} />
              ) : contents.length === 0 ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="该品牌暂无内容，前往内容工作台生成" style={{ padding: 40 }} />
              ) : (
                <Radio.Group
                  value={selectedContentId}
                  onChange={(e) => setSelectedContentId(e.target.value)}
                  style={{ width: '100%' }}
                >
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
                            <Paragraph
                              ellipsis={{ rows: 2 }}
                              style={{ margin: 0, color: 'var(--wr-text-secondary)', fontSize: 13, lineHeight: 1.6 }}
                            >
                              {c.optimized_text}
                            </Paragraph>
                          </div>
                        </Radio>
                      )
                    })}
                  </Space>
                </Radio.Group>
              )}
            </Card>
          </Col>

          {/* 右：发布目标 */}
          <Col xs={24} lg={12}>
            <Card title="发布目标" styles={{ body: { padding: 16 } }} style={{ minHeight: '100%' }}>
              {healthyAccounts.length === 0 ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无健康账号，请先在账号管理页绑定" style={{ padding: 40 }} />
              ) : (
                <>
                  {/* 发布模式选择 */}
                  <Radio.Group
                    value={publishMode}
                    onChange={(e) => setPublishMode(e.target.value)}
                    style={{ marginBottom: 16 }}
                    optionType="button"
                    buttonStyle="solid"
                  >
                    <Radio.Button value="semi-auto">半自动（推荐）</Radio.Button>
                    <Radio.Button value="auto">全自动</Radio.Button>
                  </Radio.Group>

                  {publishMode === 'semi-auto' ? (
                    <Alert
                      type="info"
                      showIcon
                      style={{ marginBottom: 16 }}
                      message="半自动发布模式"
                      description="系统生成内容并预填发布链接，你点击跳转后在各平台确认发布。零封号风险。"
                    />
                  ) : (
                    <>
                      <Alert
                        type="warning"
                        showIcon
                        style={{ marginBottom: 12 }}
                        message="全自动发布模式"
                        description="系统自动打开浏览器，注入登录态，自动填充标题正文并点击发布。请确保服务器已安装 Chrome。"
                      />
                      <div style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 8 }}>
                        <Switch checked={autoSelect} onChange={setAutoSelect} size="small" />
                        <Text style={{ fontSize: 13 }}>自动选号（系统自动选择最久未使用的健康账号，避免单号高频被封）</Text>
                      </div>
                    </>
                  )}

                  {/* 全自动发布进度 */}
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
                            <Button size="small" type="link" icon={<ExportOutlined />} href={s.external_url} target="_blank">
                              查看文章
                            </Button>
                          )}
                        </div>
                      ))}
                    </div>
                  )}

                  <CheckboxGroup
                    accounts={healthyAccounts}
                    selected={selectedAccountIds}
                    onChange={setSelectedAccountIds}
                  />
                  <Button
                    type="primary"
                    size="large"
                    block
                    style={{ marginTop: 16 }}
                    loading={publishing}
                    disabled={!selectedContent || (!autoSelect && selectedAccountIds.length === 0)}
                    onClick={handlePublish}
                  >
                    {publishing && publishMode === 'auto'
                      ? '自动发布中...'
                      : publishing
                      ? '生成发布链接中...'
                      : `发布到 ${selectedAccountIds.length} 个平台`}
                  </Button>
                </>
              )}
            </Card>
          </Col>
        </Row>

        {/* 发布记录 */}
        <Card title="发布记录" style={{ marginTop: 16 }}>
          {jobs.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无发布记录" style={{ padding: 40 }} />
          ) : (
            <Table dataSource={jobs} columns={jobColumns} rowKey="id" pagination={{ pageSize: 10 }} size="small" />
          )}
        </Card>
      </div>

      {/* 发布链接弹窗 */}
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
          description="点击下方链接跳转到各平台发布页，粘贴内容并确认发布后，回到这里点击「标记已发布」。"
        />
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          {publishLinks.map((job) => (
            <div key={job.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: 12, background: 'var(--wr-bg-elevated)', borderRadius: 8 }}>
              <Space>
                <Tag>{PLATFORM_NAMES[job.platform] || job.platform}</Tag>
                <Text type="secondary" style={{ fontSize: 12 }}>待确认</Text>
              </Space>
              <Space>
                <Button size="small" type="primary" icon={<ExportOutlined />} href={job.external_url} target="_blank">
                  前往发布
                </Button>
                <Button size="small" type="link" onClick={() => handleMarkPublished(job.id)}>
                  已发布
                </Button>
              </Space>
            </div>
          ))}
        </Space>
      </Modal>
    </div>
  )
}

// 账号多选 Checkbox 组
function CheckboxGroup({ accounts, selected, onChange }: {
  accounts: Account[]
  selected: string[]
  onChange: (ids: string[]) => void
}) {
  const toggle = (id: string) => {
    if (selected.includes(id)) {
      onChange(selected.filter((x) => x !== id))
    } else {
      onChange([...selected, id])
    }
  }
  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      {accounts.map((a) => (
        <div
          key={a.id}
          onClick={() => toggle(a.id)}
          style={{
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            padding: '10px 14px', borderRadius: 8, cursor: 'pointer',
            border: selected.includes(a.id) ? '1px solid var(--wr-primary)' : '1px solid var(--wr-border)',
            background: selected.includes(a.id) ? 'var(--wr-primary-bg)' : 'transparent',
            transition: 'all 200ms ease',
          }}
        >
          <Space>
            <Tag>{PLATFORM_NAMES[a.platform] || a.platform}</Tag>
            <Text>{a.display_name}</Text>
          </Space>
          {selected.includes(a.id) && <CheckCircleOutlined style={{ color: 'var(--wr-primary)' }} />}
        </div>
      ))}
    </Space>
  )
}
