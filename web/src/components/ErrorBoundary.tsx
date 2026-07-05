import React from 'react'
import { Result, Button } from 'antd'

interface Props { children: React.ReactNode }
interface State { hasError: boolean; error?: Error }

// ErrorBoundary 全局错误边界，防止任意组件运行时异常导致白屏。
//
// 用法：在 main.tsx 包裹 <App/>。捕获到错误时显示友好降级 UI + "刷新"按钮。
// 注意：错误边界不捕获事件回调、setTimeout、异步错误——那些由 window.onerror 兜底。
export default class ErrorBoundary extends React.Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    // 生产环境可接入错误上报（Sentry 等）；这里仅控制台输出
    console.error('Unhandled UI error:', error, info.componentStack)
  }

  handleReload = () => {
    this.setState({ hasError: false, error: undefined })
    window.location.reload()
  }

  render() {
    if (this.state.hasError) {
      return (
        <Result
          status="error"
          title="页面出了点问题"
          subTitle={this.state.error?.message || '发生了未预期的错误'}
          extra={[
        <Button type="primary" key="reload" onClick={this.handleReload}>刷新页面</Button>,
          ]}
        />
      )
    }
    return this.props.children
  }
}
