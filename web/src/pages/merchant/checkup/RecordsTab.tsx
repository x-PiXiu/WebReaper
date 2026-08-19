import { useMemo } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Tabs, Typography } from 'antd'
import Keywords from '../Keywords'
import AskHistory from './AskHistory'
import CitationsTab from '../visibility/CitationsTab'
import Nearby from '../Nearby'
import type { Brand, Keyword, MonitoringResult } from '../../../types/api'

const { Text } = Typography

const SUB_KEYS = ['history', 'questions', 'citations', 'nearby'] as const

/**
 * 体检记录 Tab（子层）：
 * 问答历史（主体——服务端留痕的每次体检，按问题分组还原成人话）
 * + 问题库（去 Tab 化的一张表）+ 引用归因 + 附近对比（本地品牌）。
 * 子层用 searchParams 持久化（?tab=records&sub=nearby 深链可用）。
 * 原"监测矩阵"控制台已删除——同一份数据的人话版本就是问答历史。
 */
export default function RecordsTab({
  brands,
  keywords,
  monitorResults,
}: {
  brands: Brand[]
  keywords: Keyword[]
  monitorResults: MonitoringResult[]
}) {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const hasLocalBrand = brands.some((b) => b.biz_type !== 'online')
  const rawSub = searchParams.get('sub') || 'history'
  const sub = (SUB_KEYS as readonly string[]).includes(rawSub) && (rawSub !== 'nearby' || hasLocalBrand)
    ? rawSub
    : 'history'
  const setSub = (k: string) => setSearchParams({ tab: 'records', sub: k }, { replace: true })

  const brandMap = useMemo(() => new Map(brands.map((b) => [b.id, b.name])), [brands])

  return (
    <div>
      <Tabs
        activeKey={sub}
        onChange={setSub}
        items={[
          { key: 'history', label: '问答历史' },
          { key: 'questions', label: '问题库' },
          { key: 'citations', label: '引用归因' },
          ...(hasLocalBrand ? [{ key: 'nearby', label: '附近对比' }] : []),
        ]}
        style={{ marginBottom: 12 }}
      />

      {sub === 'history' && (
        <AskHistory keywords={keywords} monitorResults={monitorResults} brandMap={brandMap} />
      )}
      {sub === 'questions' && (
        <>
          <Text type="secondary" style={{ display: 'block', fontSize: 12, marginBottom: 12 }}>
            这里是你问过的所有问题（含 AI 出题入库的）——问题会用于体检报告汇总，也是内容生成的选题来源。
          </Text>
          <Keywords embedded />
        </>
      )}
      {sub === 'citations' && (
        <CitationsTab monitorResults={monitorResults} brands={brands} navigate={navigate} />
      )}
      {sub === 'nearby' && <Nearby embedded />}
    </div>
  )
}
