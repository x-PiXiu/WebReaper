import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Tabs, Button } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../../../api/business'
import { useBrands } from '../../../hooks/useBrands'
import type { EngineOption } from '../../../types/api'
import OverviewTab from './OverviewTab'
import MatrixTab from './MatrixTab'
import CitationsTab from './CitationsTab'
import QuickCheckTab from './QuickCheckTab'

/**
 * AI 可见度（合并原「可见度报表」+「AI 提及监测」）。
 *
 * 信息架构：效果证明集中化——总览（驾驶舱/竞品对标）→ 矩阵（监测执行）
 * → 引用与信源（归因证据）→ 速查（随手验证）。数据在本容器统一加载，
 * 各 Tab 通过 props 共享，避免重复查询（前端"用例层"的数据契约收敛）。
 */
export default function VisibilityHub() {
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState('overview')

  const { data: brands = [] } = useBrands()
  const { data: keywords = [], isLoading: kwLoading } = useQuery({
    queryKey: ['geo-all-keywords'],
    queryFn: () => businessApi.listAllKeywords(),
  })
  const { data: monitorResults = [], isLoading: monLoading } = useQuery({
    queryKey: ['geo-monitor-results'],
    queryFn: () => businessApi.getAllMonitorResults(),
  })
  // 引擎名单（仅 name/provider/model，不含厂商密钥——速查/矩阵执行选择用）
  const { data: engines = [] } = useQuery({
    queryKey: ['geo-engines'],
    queryFn: () => businessApi.listEngines().catch(() => [] as EngineOption[]),
  })

  const brandMap = new Map(brands.map((b) => [b.id, b.name]))

  return (
    <div className="wr-page-content wr-index-report" style={{ paddingTop: 4 }}>
      <div className="wr-page-header" style={{ marginBottom: 8, display: 'flex', justifyContent: 'space-between', gap: 16, flexWrap: 'wrap' }}>
        <div>
          <h1>AI 可见度</h1>
          <p>品牌在 AI 回答中的提及 · 情感 · 位次 · 引用归因 · 竞品对标</p>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/m/keywords')}>
          去添加关键词
        </Button>
      </div>

      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          { key: 'overview', label: '总览' },
          { key: 'matrix', label: `监测矩阵` },
          { key: 'citations', label: '引用与信源' },
          { key: 'quickcheck', label: '速查' },
        ]}
        style={{ marginBottom: 12 }}
      />

      {activeTab === 'overview' && (
        <OverviewTab brands={brands} monitorResults={monitorResults} navigate={navigate} />
      )}
      {activeTab === 'matrix' && (
        <MatrixTab
          keywords={keywords}
          monitorResults={monitorResults}
          engines={engines}
          brandMap={brandMap}
          loading={kwLoading || monLoading}
        />
      )}
      {activeTab === 'citations' && (
        <CitationsTab monitorResults={monitorResults} brands={brands} navigate={navigate} />
      )}
      {activeTab === 'quickcheck' && (
        <QuickCheckTab
          brands={brands}
          keywords={keywords}
          engines={engines}
          monitorResults={monitorResults}
        />
      )}
    </div>
  )
}
