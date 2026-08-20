import { useNavigate, useSearchParams } from 'react-router-dom'
import { Tabs, Card, Typography, Space, Tag } from 'antd'
import { SearchOutlined, BarChartOutlined, FolderOpenOutlined, SettingOutlined, ClockCircleOutlined } from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../../../api/business'
import { useBrands } from '../../../hooks/useBrands'
import AutoMonitorControl from '../../../components/AutoMonitorControl'
import { timeAgo } from '../../../utils/geoTerms'
import AskTab from './AskTab'
import ReportTab from './ReportTab'
import RecordsTab from './RecordsTab'
import type { EngineOption, MonitoringResult } from '../../../types/api'

const { Text } = Typography

/**
 * AI 体检中心（四步主线第 2 步"做体检"）。
 *
 * 主交互是「问问 AI」：像顾客一样问一句 → 选引擎 → 一问多答看 AI 推不推荐你。
 * 体检报告（自动汇总）/ 体检记录（问题库·明细·归因）/ 自动体检（付费可选）
 * 全部是子层。Tab 用 searchParams 持久化——全站深链（?tab=records 等）可用。
 */
export default function Checkup() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = searchParams.get('tab') || 'ask'
  const setTab = (k: string) => setSearchParams({ tab: k }, { replace: true })

  const { data: brands = [] } = useBrands()
  const { data: keywords = [] } = useQuery({
    queryKey: ['geo-all-keywords'],
    queryFn: () => businessApi.listAllKeywords(),
  })
  const { data: monitorResults = [] } = useQuery({
    queryKey: ['geo-monitor-results'],
    queryFn: () => businessApi.getAllMonitorResults(),
  })
  const { data: engines = [] } = useQuery({
    queryKey: ['geo-engines'],
    queryFn: () => businessApi.listEngines().catch(() => [] as EngineOption[]),
  })

  return (
    <div className="wr-page-content" style={{ paddingTop: 4 }}>
      <div className="wr-page-header" style={{ marginBottom: 8 }}>
        <h1>AI 效果</h1>
        <p>测一测 AI 怎么推荐你——效果报告告诉你哪里做得好、哪里还能提升</p>
      </div>

      <Tabs
        activeKey={activeTab}
        onChange={setTab}
        items={[
          { key: 'ask', label: <span><SearchOutlined /> 测一测</span> },
          { key: 'report', label: <span><BarChartOutlined /> 效果报告</span> },
          { key: 'records', label: <span><FolderOpenOutlined /> 效果记录</span> },
          { key: 'auto', label: <span><SettingOutlined /> 自动追踪</span> },
        ]}
        style={{ marginBottom: 12 }}
      />

      {activeTab === 'ask' && <AskTab keywords={keywords} engines={engines} monitorResults={monitorResults} />}
      {activeTab === 'report' && <ReportTab brands={brands} navigate={navigate} goAsk={() => setTab('ask')} />}
      {activeTab === 'records' && (
        <RecordsTab
          brands={brands}
          keywords={keywords}
          monitorResults={monitorResults}
        />
      )}
      {activeTab === 'auto' && <AutoTab monitorResults={monitorResults} />}
    </div>
  )
}

/**
 * 自动体检 Tab（三段充实——不再是孤零零一个开关卡）：
 * ① 价值说明（为什么开）② 开关与配置 ③ 最近体检摘要 + 下次运行节奏。
 * 自动/手动的结果同源监测数据，摘要按全量最近一批推导（口径随文说明）。
 */
function AutoTab({ monitorResults }: { monitorResults: MonitoringResult[] }) {
  const { data: autoMon } = useQuery({
    queryKey: ['tenant-auto-monitor'],
    queryFn: () => businessApi.getTenantAutoMonitor().catch(() => null),
  })
  // 最近一批体检（30 分钟窗口视为一次，与问问 AI 回显同口径）
  const lastBatch = (() => {
    if (monitorResults.length === 0) return null
    let latestT = 0
    for (const r of monitorResults) {
      const ts = new Date(r.probed_at).getTime() || 0
      if (ts > latestT) latestT = ts
    }
    const batch = monitorResults.filter((r: MonitoringResult) => latestT - (new Date(r.probed_at).getTime() || 0) <= 30 * 60 * 1000)
    return {
      at: new Date(latestT),
      questions: new Set(batch.map((r: MonitoringResult) => r.keyword_id)).size,
      mentioned: batch.filter((r: MonitoringResult) => (r.mention_rate || 0) > 0).length,
      total: batch.length,
    }
  })()
  const freqLabel: Record<string, string> = { daily: '每天 1 次', half_day: '每 12 小时', weekly: '每周 1 次' }
  const cfg = autoMon?.config || { frequency: 'daily' }
  const enabled = autoMon?.tenant_enabled && autoMon?.platform_enabled

  return (
    <div>
      {/* ① 价值说明 */}
      <Card className="wr-glass-card" styles={{ body: { padding: 16 } }} style={{ marginBottom: 16 }}>
        <Space size={8} style={{ marginBottom: 6 }}>
          <ClockCircleOutlined style={{ color: 'var(--wr-primary)' }} />
          <Text strong style={{ fontSize: 14 }}>自动追踪是什么？</Text>
        </Space>
        <Text type="secondary" style={{ fontSize: 13, lineHeight: 1.8, display: 'block' }}>
          不用惦记复测——系统按你设置的节奏（{freqLabel[cfg.frequency] || '每天 1 次'}）自动把问题库里的问题问一遍 AI，
          提及率有明显下降或竞品反超时推送通知给你。适合"没空天天盯，但有变化必须知道"的老板。
          每次自动体检消耗体检额度（与手动共用）。
        </Text>
      </Card>

      {/* ② 开关与配置（免费版显示升级解锁） */}
      <AutoMonitorControl />

      {/* ③ 最近体检摘要 + 运行节奏 */}
      <Card className="wr-glass-card" styles={{ body: { padding: 16 } }} style={{ marginTop: 16 }}>
        <Space size={8} style={{ marginBottom: 6 }}>
          <BarChartOutlined style={{ color: 'var(--wr-accent)' }} />
          <Text strong style={{ fontSize: 14 }}>最近的体检</Text>
          {enabled && <Tag color="processing" style={{ fontSize: 11, margin: 0 }}>自动追踪运行中 · {freqLabel[cfg.frequency] || '每天'}</Tag>}
        </Space>
        {lastBatch ? (
          <Text type="secondary" style={{ fontSize: 13, display: 'block' }}>
            {timeAgo(lastBatch.at.toISOString())}（含手动和自动）：问了 <b>{lastBatch.questions}</b> 个问题共 <b>{lastBatch.total}</b> 次问答，其中 <b>{lastBatch.mentioned}</b> 次提到了你——完整明细在「体检记录 · 问答历史」
          </Text>
        ) : (
          <Text type="secondary" style={{ fontSize: 13 }}>还没有体检记录——去「问问 AI」测第一题，或开启上方自动体检</Text>
        )}
      </Card>
    </div>
  )
}
