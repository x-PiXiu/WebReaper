import { Popover, Progress, Space, Tag, Tooltip, Typography } from 'antd'
import { healthLevel } from '../utils/geoHealth'

const { Text } = Typography

// GEO 健康总分 + 五指数（驾驶舱核心组件，工作台与 AI 可见度页共用）。
// 设计：一个总分回答"好不好"，五指数回答"差在哪"，每项可点击跳转对应页。
export default function HealthScorePanel({
  total,
  indicators,
  deltaText,
  competitorGap,
  onNavigate,
}: {
  total: number
  indicators: { label: string; key: string; value: number; hint: string; path?: string }[]
  deltaText?: string        // 较上期变化（如 "+3.2"）
  competitorGap?: string    // 竞品差距（如 "领先 12%" / "落后 8%"）
  onNavigate: (path: string) => void
}) {
  const lv = healthLevel(total)
  return (
    <div className="wr-glass-card" style={{ padding: '20px 24px', marginBottom: 16, borderColor: 'rgba(124,108,255,0.25)' }}>
      <div style={{ display: 'flex', gap: 28, flexWrap: 'wrap', alignItems: 'center' }}>
        {/* 总分环 */}
        <div style={{ flexShrink: 0, textAlign: 'center' }}>
          <Progress
            type="dashboard"
            percent={total}
            size={120}
            strokeColor={{ '0%': '#7c6cff', '100%': '#22d3ee' }}
            format={(p) => (
              <div>
                <div style={{ fontSize: 30, fontWeight: 800, lineHeight: 1.1, color: 'var(--wr-text-primary)', letterSpacing: '-0.03em' }}>{p}</div>
                <div style={{ fontSize: 11, color: 'var(--wr-text-muted)' }}>GEO 健康分</div>
              </div>
            )}
          />
          <Tag color={lv.color} style={{ marginTop: 4 }}>{lv.label}</Tag>
        </div>

        {/* 五指数 */}
        <div style={{ flex: 1, minWidth: 260 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 10, flexWrap: 'wrap' }}>
            <Text strong style={{ fontSize: 14 }}>五维指数</Text>
            {/* F2-4 数据口径说明：一次性回答"数字怎么来的、从什么时候开始算" */}
            <Popover
              title="数据口径说明"
              content={(
                <div style={{ maxWidth: 300, fontSize: 12, lineHeight: 1.8 }}>
                  <div><b>提及覆盖</b>：全部关键词最近一次监测的平均提及率（无数据品牌按 0 计入）。</div>
                  <div><b>情感指数</b>：AI 提到你时的态度（正面+1/负面-1 映射 0-100）。</div>
                  <div><b>首选提及</b>：AI 第一个推荐你的比例——需 ≥3 次采样才展示，不足显示"—"积累中。</div>
                  <div><b>内容资产</b>：已发布内容 ×15 + 草稿 ×5（封顶 100）。</div>
                  <div><b>信源完整</b>：AI 回答引用你公开站的比例——2026-08-15 后的新监测才记录引用来源，历史结果不计入。</div>
                  <div style={{ marginTop: 4, color: 'var(--wr-text-muted)' }}>环比（较上期）取 7 天前窗口的监测重算。</div>
                </div>
              )}
            >
              <a style={{ fontSize: 11, fontWeight: 400 }}>数据口径?</a>
            </Popover>
            {deltaText && (
              <Tag style={{ margin: 0, fontSize: 11, color: deltaText.startsWith('+') ? 'var(--wr-success)' : deltaText.startsWith('-') ? 'var(--wr-danger)' : undefined }}>
                较上期 {deltaText}
              </Tag>
            )}
            {competitorGap && (
              <Tooltip title="与配置竞品的平均提及率对比（最近一次监测）">
                <Tag color={competitorGap.startsWith('领先') ? 'success' : competitorGap.startsWith('落后') ? 'error' : 'default'} style={{ margin: 0, fontSize: 11 }}>
                  {competitorGap}
                </Tag>
              </Tooltip>
            )}
          </div>
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            {indicators.map((ind) => {
              // F1-2 积累中态：value<0 = 采样不足（如首选率需 ≥3 次采样）——显示灰态而非误导性数值
              const accumulating = ind.value < 0
              return (
              <div
                key={ind.key}
                onClick={() => ind.path && onNavigate(ind.path)}
                style={{ display: 'flex', alignItems: 'center', gap: 10, cursor: ind.path ? 'pointer' : 'default' }}
                role={ind.path ? 'button' : undefined}
              >
                <Text style={{ width: 72, flexShrink: 0, fontSize: 12, color: 'var(--wr-text-secondary)' }}>{ind.label}</Text>
                <div style={{ flex: 1 }}>
                  <Tooltip title={ind.hint}>
                    <Progress percent={accumulating ? 0 : ind.value} size="small" showInfo={false}
                      strokeColor={accumulating ? 'var(--wr-text-muted)' : ind.value >= 60 ? 'var(--wr-success)' : ind.value >= 30 ? 'var(--wr-warning)' : 'var(--wr-danger)'} />
                  </Tooltip>
                </div>
                <Text style={{ width: 36, textAlign: 'right', fontSize: 12, fontWeight: 600, color: accumulating ? 'var(--wr-text-muted)' : 'var(--wr-text-primary)' }}>
                  {accumulating ? '—' : ind.value}
                </Text>
              </div>
              )
            })}
          </Space>
        </div>
      </div>
    </div>
  )
}
