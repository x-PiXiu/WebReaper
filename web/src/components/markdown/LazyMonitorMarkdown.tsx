import { lazy, Suspense, type CSSProperties } from 'react'
import { Skeleton } from 'antd'

const MonitorMarkdown = lazy(() => import('./MonitorMarkdown'))

/** 监测详情 Markdown：首屏不拉 react-markdown 包。 */
export default function LazyMonitorMarkdown(props: {
  text: string
  brand: string[]
  competitors: string[]
  style?: CSSProperties
}) {
  return (
    <Suspense fallback={<Skeleton active paragraph={{ rows: 4 }} title={false} />}>
      <MonitorMarkdown {...props} />
    </Suspense>
  )
}
