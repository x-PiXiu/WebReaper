import { Collapse, Typography } from 'antd'
import { LazyLine as Line } from '../../../components/charts/LazyCharts'
import LazyMonitorMarkdown from '../../../components/markdown/LazyMonitorMarkdown'
import { markLastPoint, rateColor } from '../../../utils/geo'
import type { MonitoringResult } from '../../../types/api'
import { CompareBar, StatBlock, sentimentMeta, splitSamples } from './MonitorWidgets'

const { Text } = Typography

/** 单关键词监测明细：趋势、引擎对比、采样原文。 */
export default function MonitorDetailPanel({
  results,
  brandName,
}: {
  results: MonitoringResult[]
  brandName: string
}) {
  if (results.length === 0) {
    return (
      <Text type="secondary" style={{ fontSize: 13, padding: '8px 0', display: 'block' }}>
        暂无监测数据，点击「监测」更新收录状态
      </Text>
    )
  }

  return (
    <div style={{ padding: '4px 0' }}>
      {results.length > 1 && (() => {
        const trendData = markLastPoint(results.map((r) => ({
          date: new Date(r.probed_at).toLocaleDateString(),
          rate: Math.round((r.mention_rate || 0) * 1000) / 10,
          engine: r.engine_name || 'default',
        })).sort((a, b) => a.date.localeCompare(b.date)))
        return (
          <div style={{ marginBottom: 16, padding: 12, background: 'var(--wr-bg-elevated)', borderRadius: 10 }}>
            <Text strong style={{ fontSize: 13, marginBottom: 8, display: 'block' }}>提及率趋势</Text>
            <Line
              data={trendData}
              xField="date"
              yField="rate"
              seriesField="engine"
              smooth
              height={180}
              color={['#6366f1', '#0891b2', '#10b981', '#f59e0b']}
              yAxis={{ label: { formatter: (v: string) => `${v}%` } }}
            />
          </div>
        )
      })()}

      {results.length > 1 && (() => {
        const engines = [...new Set(results.map((r) => r.engine_name || 'default'))]
        return (
          <div style={{ marginBottom: 12, padding: 12, background: 'var(--wr-bg-elevated)', borderRadius: 10 }}>
            <Text strong style={{ fontSize: 13, marginBottom: 8, display: 'block' }}>AI 引擎提及率对比</Text>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {engines.map((eng) => {
                const r = results.filter((x) => (x.engine_name || 'default') === eng)
                  .sort((a, b) => new Date(b.probed_at).getTime() - new Date(a.probed_at).getTime())[0]
                const rate = r?.mention_rate || 0
                return (
                  <div key={eng} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    <Text style={{ fontSize: 12, minWidth: 90, fontWeight: 600 }}>{eng}</Text>
                    <div style={{ flex: 1, height: 10, borderRadius: 5, background: 'var(--wr-bg-hover)', overflow: 'hidden' }}>
                      <div style={{
                        width: `${Math.min(100, rate * 100)}%`,
                        height: '100%',
                        background: rate >= 0.5
                          ? 'var(--wr-gradient)'
                          : rate >= 0.2
                            ? 'linear-gradient(90deg,#f59e0b,#fbbf24)'
                            : 'linear-gradient(90deg,#fb7185,#f87171)',
                        borderRadius: 5,
                      }}
                      />
                    </div>
                    <Text style={{ fontSize: 12, minWidth: 56, textAlign: 'right', fontWeight: 700, color: rateColor(rate) }}>
                      {(rate * 100).toFixed(0)}%
                    </Text>
                  </div>
                )
              })}
            </div>
          </div>
        )
      })()}

      <Text strong style={{ fontSize: 13, marginBottom: 10, display: 'block' }}>各 AI 引擎检测详情</Text>
      {(() => {
        const byEng = new Map<string, MonitoringResult[]>()
        results.forEach((rr) => {
          const eng = rr.engine_name || 'default'
          const arr = byEng.get(eng)
          if (arr) arr.push(rr)
          else byEng.set(eng, [rr])
        })
        const engineGroups: { eng: string; latest: MonitoringResult; count: number }[] = []
        byEng.forEach((arr, eng) => {
          arr.sort((a, b) => new Date(b.probed_at).getTime() - new Date(a.probed_at).getTime())
          engineGroups.push({ eng, latest: arr[0], count: arr.length })
        })
        return (
          <Collapse
            className="wr-engine-collapse"
            defaultActiveKey={engineGroups.map((g) => g.eng)}
            items={engineGroups.map((g) => {
              const r = g.latest
              const compRates = Object.entries(r.competitor_rates || {}).sort((a, b) => b[1] - a[1]).slice(0, 4)
              const sm = sentimentMeta(r.sentiment || '')
              const ratePct = Math.round(r.mention_rate * 100)
              const position = r.avg_position > 0 ? `#${r.avg_position}` : '未被提及'
              const samples = splitSamples(r.raw_sample || '')
              return {
                key: g.eng,
                label: (
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, width: '100%', paddingRight: 16 }}>
                    <span style={{ width: 28, height: 28, borderRadius: 8, background: 'var(--wr-gradient)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: 12, fontWeight: 700, flexShrink: 0 }}>AI</span>
                    <Text strong style={{ fontSize: 13 }}>{r.engine_name || 'default'}</Text>
                    <span style={{ fontSize: 11, color: 'var(--wr-text-muted)', background: 'var(--wr-bg-hover)', padding: '2px 9px', borderRadius: 10 }}>{r.sample_count} 次采样</span>
                    {g.count > 1 && (
                      <span style={{ fontSize: 11, color: 'var(--wr-text-muted)', background: 'rgba(124,108,255,0.12)', padding: '2px 9px', borderRadius: 10 }}>共监测 {g.count} 次</span>
                    )}
                    <span style={{ fontSize: 12, fontWeight: 600, color: sm.color }}>{sm.emoji} {sm.label}</span>
                    <span style={{ marginLeft: 'auto', fontSize: 18, fontWeight: 800, color: rateColor(r.mention_rate) }}>{ratePct}%</span>
                  </div>
                ),
                children: (
                  <div>
                    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(110px, 1fr))', gap: 8, marginBottom: 12 }}>
                      <StatBlock label="提及率" value={`${ratePct}%`} color={rateColor(r.mention_rate)} sub={`${r.mention_count}/${r.sample_count} 次提到`} />
                      <StatBlock label="AI 排名" value={position} />
                      <StatBlock label="置信度" value={`${Math.round((r.confidence || 0) * 100)}%`} />
                      <StatBlock label="竞品提及" value={`${compRates.length} 家`} sub={compRates[0] ? `最多：${compRates[0][0]}` : '暂无'} />
                    </div>
                    {compRates.length > 0 && (
                      <div style={{ marginBottom: 12, padding: '10px 12px', borderRadius: 10, background: 'var(--wr-bg-elevated)' }}>
                        <Text type="secondary" style={{ fontSize: 10, display: 'block', marginBottom: 8 }}>提及率对比</Text>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                          <CompareBar name={brandName || '我'} rate={r.mention_rate || 0} isMine barColor="var(--wr-gradient)" textColor={rateColor(r.mention_rate)} nameColor="var(--wr-primary)" />
                          {compRates.map(([name, rate]) => (
                            <CompareBar key={name} name={name} rate={rate} barColor="var(--wr-warning)" textColor="var(--wr-text-muted)" />
                          ))}
                        </div>
                      </div>
                    )}
                    {samples.length > 0 && (
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                        <Text type="secondary" style={{ fontSize: 10 }}>AI 回答内容（品牌绿 / 竞品橙高亮）</Text>
                        {samples.map((s, i) => (
                          <div key={i} style={{ borderRadius: 10, background: 'rgba(0,0,0,0.14)', overflow: 'hidden' }}>
                            {s.header && (
                              <div style={{ padding: '6px 12px', background: 'rgba(124,108,255,0.10)', borderBottom: '1px solid rgba(124,108,255,0.10)' }}>
                                <Text strong style={{ fontSize: 11, color: 'var(--wr-primary)' }}>{s.header}</Text>
                              </div>
                            )}
                            <div style={{ padding: '10px 14px' }}>
                              <LazyMonitorMarkdown text={s.body} brand={[brandName]} competitors={r.competitors || []} />
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                ),
              }
            })}
          />
        )
      })()}
    </div>
  )
}
