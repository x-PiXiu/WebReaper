import { Button, Empty, Result } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import type { ReactNode } from 'react'
import PageLoading from './PageLoading'

// QueryBoundary：react-query 页面数据三态统一容器（加载 / 出错 / 空数据 / 正常）。
//
// 背景：此前 33 个页面只处理 isLoading、不处理 isError——接口失败时页面永远停在
// 加载态，用户只能刷新。本组件把三态收敛为一处：出错给出原因 + 重试按钮，
// 空数据给统一空态，页面只写正常渲染分支。
//
// 用法（主查询接住 isError 与 refetch）：
//   const { data, isLoading, isError, refetch } = useXxx()
//   <QueryBoundary loading={isLoading} error={isError} onRetry={() => refetch()} empty={!data}>
//     <正常渲染 />
//   </QueryBoundary>
export default function QueryBoundary({
  loading,
  error,
  empty,
  onRetry,
  loadingTip,
  emptyText = '暂无数据',
  emptyExtra,
  children,
}: {
  loading: boolean
  error: boolean | unknown
  /** 无数据（error 优先级高于 empty） */
  empty?: boolean
  /** 出错时的重试回调（通常是 refetch） */
  onRetry?: () => void
  loadingTip?: string
  emptyText?: string
  /** 空态底部的自定义动作（如"去创建"引导） */
  emptyExtra?: ReactNode
  children: ReactNode
}) {
  if (loading) return <PageLoading tip={loadingTip} />
  if (error) {
    return (
      <Result
        status="warning"
        title="数据加载失败"
        subTitle="网络波动或服务暂不可用，请稍后重试"
        extra={
          onRetry && (
            <Button type="primary" icon={<ReloadOutlined />} onClick={onRetry}>
              重试
            </Button>
          )
        }
        style={{ padding: '64px 0' }}
      />
    )
  }
  if (empty) {
    return (
      <div className="wr-empty-hero">
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyText} />
        {emptyExtra}
      </div>
    )
  }
  return <>{children}</>
}
