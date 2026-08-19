import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Card, Empty, Space, Tag, Typography } from 'antd'
import { HistoryOutlined, ReloadOutlined, DownOutlined, UpOutlined } from '@ant-design/icons'
import { timeAgo } from '../../../utils/geoTerms'
import { EngineRow } from './AskTab'
import type { Keyword, MonitoringResult } from '../../../types/api'

const { Text } = Typography

const BATCH_GAP_MS = 30 * 60 * 1000 // 两次监测间隔超过 30 分钟视为不同一次体检

interface AskHistoryRow {
  keywordId: string
  question: string
  brandName: string
  /** 最近一次体检：每引擎最新一条 */
  latest: { at: string; results: MonitoringResult[] }
  /** 上一次体检的提及数（有则展示变化） */
  prevMentioned: number | null
  prevTotal: number
}

/**
 * 问答历史（体检记录主体）：把服务端监测留痕按"问题"分组，还原成人话——
 * 「问过什么、几个 AI 提到了你、比上次多了还是少了、AI 原话是什么」。
 * 取代原"监测矩阵"（关键词×引擎控制台）——同一份数据的商户视角。
 */
export default function AskHistory({
  keywords,
  monitorResults,
  brandMap,
}: {
  keywords: Keyword[]
  monitorResults: MonitoringResult[]
  brandMap: Map<string, string>
}) {
  const navigate = useNavigate()
  const [expanded, setExpanded] = useState<string | null>(null)

  const rows = useMemo<AskHistoryRow[]>(() => {
    const kwTerm = new Map(keywords.map((k) => [k.id, k.term]))
    // 按关键词分组，组内按时间升序切分为"次"（间隔 > 30 分钟）
    const byKw = new Map<string, MonitoringResult[]>()
    for (const r of monitorResults) {
      const arr = byKw.get(r.keyword_id) || []
      arr.push(r)
      byKw.set(r.keyword_id, arr)
    }
    const out: AskHistoryRow[] = []
    for (const [keywordId, list] of byKw) {
      const question = kwTerm.get(keywordId)
      if (!question) continue // 问题已被删除——历史不展示（数据仍在服务端）
      const sorted = [...list].sort((a, b) => new Date(a.probed_at).getTime() - new Date(b.probed_at).getTime())
      // 切分为多次体检
      const runs: MonitoringResult[][] = []
      let cur: MonitoringResult[] = []
      let lastT = 0
      for (const r of sorted) {
        const t = new Date(r.probed_at).getTime()
        if (cur.length > 0 && t - lastT > BATCH_GAP_MS) {
          runs.push(cur)
          cur = []
        }
        cur.push(r)
        lastT = t
      }
      if (cur.length > 0) runs.push(cur)
      if (runs.length === 0) continue
      // 最近一次：每引擎取最新一条
      const latestRun = runs[runs.length - 1]
      const byEngine = new Map<string, MonitoringResult>()
      for (const r of latestRun) {
        const k = r.engine_name || 'default'
        const existed = byEngine.get(k)
        if (!existed || new Date(r.probed_at) > new Date(existed.probed_at)) byEngine.set(k, r)
      }
      const results = Array.from(byEngine.values())
      const prevRun = runs.length > 1 ? runs[runs.length - 2] : null
      const prevMentioned = prevRun ? new Set(prevRun.filter((r) => (r.mention_rate || 0) > 0).map((r) => r.engine_name || 'default')).size : null
      const latestAt = results.reduce((m, r) => (r.probed_at > m ? r.probed_at : m), results[0]?.probed_at || '')
      out.push({
        keywordId,
        question,
        brandName: brandMap.get(results[0]?.brand_id || '') || '',
        latest: { at: latestAt, results },
        prevMentioned,
        prevTotal: prevRun ? new Set(prevRun.map((r) => r.engine_name || 'default')).size : 0,
      })
    }
    return out.sort((a, b) => (a.latest.at < b.latest.at ? 1 : -1)).slice(0, 50)
  }, [keywords, monitorResults, brandMap])

  const retest = (question: string) => {
    navigate(`/m/checkup?tab=ask&q=${encodeURIComponent(question)}`)
  }

  if (rows.length === 0) {
    return (
      <Card className="wr-glass-card">
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有问答记录" style={{ padding: 32 }}>
          <div style={{ textAlign: 'center', maxWidth: 380, margin: '0 auto' }}>
            <Text type="secondary" style={{ fontSize: 13, display: 'block', marginBottom: 10 }}>
              每次在「问问 AI」测过的问题都会留痕在这里——隔周复测，就能看到 AI 对你的态度变化
            </Text>
            <Button type="primary" onClick={() => navigate('/m/checkup?tab=ask')}>去问第一个问题</Button>
          </div>
        </Empty>
      </Card>
    )
  }

  return (
    <div>
      <Card className="wr-glass-card" styles={{ body: { padding: '10px 16px' } }} style={{ marginBottom: 12 }}>
        <Text type="secondary" style={{ fontSize: 12 }}>
          每次体检的问题与结论都留痕在这里——点开看各 AI 的原话，随时「再测一次」对比变化。
        </Text>
      </Card>
      {rows.map((row) => {
        const mentioned = row.latest.results.filter((r) => (r.mention_rate || 0) > 0).length
        const isOpen = expanded === row.keywordId
        const delta = row.prevMentioned !== null ? mentioned - row.prevMentioned : null
        return (
          <Card key={row.keywordId} className="wr-glass-card" style={{ marginBottom: 12 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
              <Space size={10} wrap>
                <HistoryOutlined style={{ color: 'var(--wr-text-muted)' }} />
                <Text strong style={{ fontSize: 14.5 }}>“{row.question}”</Text>
                <Tag color={mentioned > 0 ? 'success' : 'warning'} style={{ margin: 0 }}>
                  {row.latest.results.length} 个 AI 问了，{mentioned} 个提到了你
                </Tag>
                {delta !== null && row.prevTotal > 0 && (
                  <Tag color={delta > 0 ? 'success' : delta < 0 ? 'error' : 'default'} style={{ margin: 0, fontSize: 11 }}>
                    比上次 {delta > 0 ? `+${delta}` : delta}
                  </Tag>
                )}
              </Space>
              <Space size={8}>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {timeAgo(row.latest.at)}{row.brandName ? ` · ${row.brandName}` : ''}
                </Text>
                <Button size="small" type="text" icon={isOpen ? <UpOutlined /> : <DownOutlined />} onClick={() => setExpanded(isOpen ? null : row.keywordId)}>
                  {isOpen ? '收起' : '展开明细'}
                </Button>
                <Button size="small" icon={<ReloadOutlined />} onClick={() => retest(row.question)}>再测一次</Button>
              </Space>
            </div>
            {isOpen && (
              <div style={{ marginTop: 12 }}>
                {row.latest.results.map((r, i) => <EngineRow key={r.id || i} r={r} />)}
              </div>
            )}
          </Card>
        )
      })}
    </div>
  )
}
