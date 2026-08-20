import { useMemo } from 'react'
import type { NavigateFunction } from 'react-router-dom'
import { Card, Row, Col, Table, Tag, Space, Empty, Typography, Progress, Button, Tooltip } from 'antd'
import { LinkOutlined, GlobalOutlined, FileTextOutlined, CheckCircleOutlined } from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../../../api/business'
import type { Brand, MonitoringResult, OptimizedContent } from '../../../types/api'

const { Text } = Typography

/**
 * 引用与信源 Tab：效果证明的"归因证据"。
 * 引用排行（自营站被引用次数）+ 信源分类（官网/垂类媒体/自媒体占比）。
 */
export default function CitationsTab({
  monitorResults,
  brands,
  navigate,
}: {
  monitorResults: MonitoringResult[]
  brands: Brand[]
  navigate: NavigateFunction
}) {
  // 按品牌聚合：被引用次数（self_source_count）与来源清单
  const brandCites = useMemo(() => {
    const map = new Map<string, { cite: number; sources: Set<string> }>()
    monitorResults.forEach((r) => {
      if ((r.self_source_count || 0) <= 0) return
      const cur = map.get(r.brand_id) || { cite: 0, sources: new Set<string>() }
      cur.cite += r.self_source_count || 0
      ;(r.sources || []).forEach((s) => cur.sources.add(s))
      map.set(r.brand_id, cur)
    })
    return Array.from(map.entries()).map(([brandId, v]) => ({
      brandId,
      cite: v.cite,
      sources: Array.from(v.sources).slice(0, 6),
    })).sort((a, b) => b.cite - a.cite)
  }, [monitorResults])

  // 信源分类（v3 P2 三分类+其他）：自营站（品牌官网域名命中）/ 自媒体（UGC 平台）/
  // 垂类媒体（门户/垂媒域名）/ 其他——分类为启发式域名判断，仅供参考。
  const selfDomains = useMemo(() => {
    const set = new Set<string>()
    brands.forEach((b) => {
      const m = (b.website_url || '').toLowerCase().match(/https?:\/\/([^/]+)/)
      if (m) set.add(m[1])
    })
    return set
  }, [brands])
  const sourceClass = useMemo(() => {
    const classify = (s: string): '自营站' | '垂类媒体' | '自媒体' | '其他' => {
      const lower = s.toLowerCase()
      for (const d of selfDomains) {
        if (lower.includes(d)) return '自营站'
      }
      if (/xiaohongshu|xhslink|douyin|bilibili|weibo\.com|mp\.weixin|baijiahao|toutiao|kuaishou/.test(lower)) return '自媒体'
      if (/zhihu|sohu|sina|163\.com|qq\.com|ifeng|36kr|csdn|juejin|thepaper|eastmoney|caixin/.test(lower)) return '垂类媒体'
      return '其他'
    }
    const set = new Set<string>()
    monitorResults.forEach((r) => (r.sources || []).forEach((s) => set.add(s)))
    const cls = { 自营站: 0, 垂类媒体: 0, 自媒体: 0, 其他: 0 }
    set.forEach((s) => { cls[classify(s)] += 1 })
    return { total: set.size, cls }
  }, [monitorResults, selfDomains])

  // 内容级数据（收录状态清单 + 内容引用排行——共享缓存）
  const { data: contentsByBrand = [] } = useQuery<OptimizedContent[]>({
    queryKey: ['geo-contents-all', brands.map((b) => b.id).join(',')],
    queryFn: async () => {
      const lists = await Promise.all(
        brands.map((b) => businessApi.listContents(b.id).catch(() => [] as OptimizedContent[])),
      )
      return lists.flat()
    },
    enabled: brands.length > 0,
    staleTime: 60_000,
  })
  const { data: citationsByBrand = {} } = useQuery<Record<string, number>>({
    queryKey: ['geo-citations-all', brands.map((b) => b.id).join(',')],
    queryFn: async () => {
      // 并发扇出（v3 P2：此前逐品牌串行 await，品牌多时明显慢）
      const maps = await Promise.all(
        brands.map((b) => businessApi.getContentCitations(b.id).catch(() => ({}) as Record<string, number>)),
      )
      const merged: Record<string, number> = {}
      maps.forEach((m) => Object.assign(merged, m))
      return merged
    },
    enabled: brands.length > 0,
    staleTime: 60_000,
  })

  // 内容引用排行：每篇被 AI 引用次数（归因细化到篇——规划 4.4 Tab3 内容级排行）
  const contentCites = useMemo(() => {
    return Object.entries(citationsByBrand)
      .map(([contentId, cite]) => {
        const c = contentsByBrand.find((x) => x.id === contentId)
        return { contentId, cite, title: c?.title || '(无标题)', brandName: c ? (brands.find((b) => b.id === c.brand_id)?.name || '') : '' }
      })
      .sort((a, b) => b.cite - a.cite)
      .slice(0, 10)
  }, [citationsByBrand, contentsByBrand, brands])

  // 收录状态清单：已发布内容的 index_status（收录验证任务每日回写）
  const publishedContents = useMemo(() => {
    return contentsByBrand
      .filter((c) => c.status === 'published')
      .map((c) => ({ ...c, brandName: brands.find((b) => b.id === c.brand_id)?.name || '' }))
      .sort((a, b) => (b.index_status === 'indexed' ? 1 : 0) - (a.index_status === 'indexed' ? 1 : 0))
  }, [contentsByBrand, brands])
  const indexedCount = publishedContents.filter((c) => c.index_status === 'indexed').length

  const citeTotal = brandCites.reduce((s, b) => s + b.cite, 0)

  return (
    <div>
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} lg={10}>
          <Card className="wr-glass-card" styles={{ body: { padding: 20 } }}>
            <Space style={{ marginBottom: 12 }}>
              <GlobalOutlined style={{ color: 'var(--wr-accent)' }} />
              <Text strong style={{ fontSize: 14 }}>信源分布</Text>
              <Tooltip title="分类为启发式判断（按域名特征识别自营站/垂类媒体/自媒体/其他），仅供参考——AI 回答中出现的实际来源以右侧列表为准">
                <span className="wr-help-tip">?</span>
              </Tooltip>
            </Space>
            {sourceClass.total === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={
                <Space direction="vertical" size={2}>
                  <Text type="secondary" style={{ fontSize: 12 }}>暂无来源数据</Text>
                  <Text type="secondary" style={{ fontSize: 11 }}>发起监测后，AI 回答里出现的链接会记录在这里</Text>
                  <Button size="small" type="link" style={{ padding: 0 }} onClick={() => navigate('/m/checkup?tab=report')}>去发起监测 →</Button>
                </Space>
              } />
            ) : (
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                {Object.entries(sourceClass.cls).map(([label, n]) => {
                  const pct = Math.round((n / sourceClass.total) * 100)
                  return (
                    <div key={label}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                        <Text style={{ fontSize: 13 }}>{label}</Text>
                        <Text strong style={{ fontSize: 13 }}>{n} · {pct}%</Text>
                      </div>
                      <Progress percent={pct} size="small" showInfo={false} strokeColor="var(--wr-accent)" />
                    </div>
                  )
                })}
              </Space>
            )}
          </Card>
        </Col>
        <Col xs={24} lg={14}>
          <Card className="wr-glass-card" styles={{ body: { padding: 0 } }}>
            <div style={{ padding: '16px 20px 0' }}>
              <Space>
                <LinkOutlined style={{ color: 'var(--wr-success)' }} />
                <Text strong style={{ fontSize: 14 }}>内容被引用排行（归因）</Text>
                <Tag color="success" style={{ margin: 0 }}>共 {citeTotal} 次</Tag>
              </Space>
              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
                AI 回答中实际引用了你公开站的内容——内容 GEO 的直接效果证据
              </Text>
            </div>
            <Table
              dataSource={brandCites}
              rowKey="brandId"
              size="small"
              pagination={false}
              style={{ marginTop: 8 }}
              locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有内容被 AI 引用——发布内容并等待收录（约 1-2 周）" /> }}
              columns={[
                {
                  title: '品牌', key: 'brand', width: 140,
                  render: (_: unknown, r: { brandId: string }) => {
                    const b = brands.find((x) => x.id === r.brandId)
                    return <Text strong>{b?.name || r.brandId}</Text>
                  },
                },
                {
                  title: '被引用', dataIndex: 'cite', key: 'cite', width: 90, align: 'center',
                  render: (n: number) => <Text strong style={{ color: 'var(--wr-success)', fontSize: 15 }}>{n} 次</Text>,
                },
                {
                  title: '引用来源', key: 'sources',
                  render: (_: unknown, r: any) => (
                    <Space wrap size={[4, 4]}>
                      {r.sources.map((s: string) => <Tag key={s} style={{ fontSize: 10 }}>{s}</Tag>)}
                    </Space>
                  ),
                },
                {
                  title: '操作', key: 'action', width: 90,
                  render: (_: unknown) => (
                    <a onClick={() => navigate('/m/compose')} style={{ fontSize: 12 }}>去生产内容 →</a>
                  ),
                },
              ]}
            />
          </Card>
        </Col>
      </Row>

      {/* 内容级归因：每篇被引用排行 + 收录状态清单 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} lg={12}>
          <Card className="wr-glass-card" styles={{ body: { padding: 0 } }}>
            <div style={{ padding: '16px 20px 0' }}>
              <Space>
                <LinkOutlined style={{ color: 'var(--wr-accent)' }} />
                <Text strong style={{ fontSize: 14 }}>内容被引用排行（按篇）</Text>
              </Space>
            </div>
            <Table
              dataSource={contentCites}
              rowKey="contentId"
              size="small"
              pagination={false}
              style={{ marginTop: 8 }}
              locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有内容被 AI 引用——发布后约 1-2 周生效" /> }}
              columns={[
                {
                  title: '标题', key: 'title',
                  render: (_: unknown, r: { title: string; brandName: string }) => (
                    <Space direction="vertical" size={0}>
                      <Text style={{ fontSize: 13 }} ellipsis>{r.title}</Text>
                      <Text type="secondary" style={{ fontSize: 11 }}>{r.brandName}</Text>
                    </Space>
                  ),
                },
                {
                  title: '被引用', dataIndex: 'cite', key: 'cite', width: 90, align: 'center',
                  render: (n: number) => <Text strong style={{ color: 'var(--wr-success)', fontSize: 15 }}>{n} 次</Text>,
                },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card className="wr-glass-card" styles={{ body: { padding: 0 } }}>
            <div style={{ padding: '16px 20px 0' }}>
              <Space>
                <CheckCircleOutlined style={{ color: 'var(--wr-success)' }} />
                <Text strong style={{ fontSize: 14 }}>收录状态清单</Text>
                <Tag color={indexedCount > 0 ? 'success' : 'default'} style={{ margin: 0 }}>
                  {indexedCount}/{publishedContents.length} 已收录
                </Tag>
              </Space>
            </div>
            <Table
              dataSource={publishedContents}
              rowKey="id"
              size="small"
              pagination={{ pageSize: 6, size: 'small' }}
              style={{ marginTop: 8 }}
              locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无已发布内容——内容生成页发布后进入此清单" /> }}
              columns={[
                {
                  title: '标题', key: 'title',
                  render: (_: unknown, r: OptimizedContent & { brandName?: string }) => (
                    <Text style={{ fontSize: 13 }} ellipsis>{r.title || '(无标题)'} <Text type="secondary" style={{ fontSize: 11 }}>{r.brandName}</Text></Text>
                  ),
                },
                {
                  title: '收录', dataIndex: 'index_status', key: 'index_status', width: 110, align: 'center',
                  render: (s: string) => {
                    if (s === 'indexed') return <Tag color="success" style={{ margin: 0 }}>已收录</Tag>
                    if (s === 'pending') return <Tag color="warning" style={{ margin: 0 }}>待收录</Tag>
                    return <Tag style={{ margin: 0 }}>未验证</Tag>
                  },
                },
                {
                  title: '操作', key: 'action', width: 90,
                  render: (_: unknown, r: OptimizedContent) => (
                    <Button size="small" type="link" href={`/public/articles/${r.id}`} target="_blank" style={{ fontSize: 12 }}>
                      公开页
                    </Button>
                  ),
                },
              ]}
            />
          </Card>
        </Col>
      </Row>

      <Card className="wr-glass-card">
        <Space>
          <FileTextOutlined style={{ color: 'var(--wr-primary)' }} />
          <Text strong style={{ fontSize: 14 }}>怎么提升被引用次数</Text>
        </Space>
        <div style={{ marginTop: 10, fontSize: 13, color: 'var(--wr-text-secondary)', lineHeight: 2 }}>
          <div>① 内容发布到公开站（AI 引擎可爬取）→ ② 提交收录（IndexNow）→ ③ 等待 1-2 周收录 → ④ 复测验证引用。</div>
          <div>引用友好内容结构：结论前置、观点独立成段、数据标注来源、段落有明确小标题。</div>
        </div>
      </Card>
    </div>
  )
}
