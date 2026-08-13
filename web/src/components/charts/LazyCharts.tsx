import { lazy, Suspense, type ComponentProps } from 'react'
import { Skeleton } from 'antd'

// 图表按需加载：避免驾驶舱/可见度首屏同步拉入 @ant-design/charts（~1.4MB）
const LineChart = lazy(() =>
  import('@ant-design/charts').then((m) => ({ default: m.Line }))
)
const PieChart = lazy(() =>
  import('@ant-design/charts').then((m) => ({ default: m.Pie }))
)
const ColumnChart = lazy(() =>
  import('@ant-design/charts').then((m) => ({ default: m.Column }))
)

function ChartFallback({ height = 260 }: { height?: number }) {
  return (
    <Skeleton.Node active style={{ width: '100%', height, borderRadius: 12 }}>
      <span />
    </Skeleton.Node>
  )
}

type LineProps = ComponentProps<typeof LineChart>
type PieProps = ComponentProps<typeof PieChart>
type ColumnProps = ComponentProps<typeof ColumnChart>

export function LazyLine(props: LineProps) {
  const height = typeof props.height === 'number' ? props.height : 260
  return (
    <Suspense fallback={<ChartFallback height={height} />}>
      <LineChart {...props} />
    </Suspense>
  )
}

export function LazyPie(props: PieProps) {
  const height = typeof props.height === 'number' ? props.height : 260
  return (
    <Suspense fallback={<ChartFallback height={height} />}>
      <PieChart {...props} />
    </Suspense>
  )
}

export function LazyColumn(props: ColumnProps) {
  const height = typeof props.height === 'number' ? props.height : 260
  return (
    <Suspense fallback={<ChartFallback height={height} />}>
      <ColumnChart {...props} />
    </Suspense>
  )
}
