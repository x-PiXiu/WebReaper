import React from 'react'
import { Result, Button, Space } from 'antd'

interface Props {
  children: React.ReactNode
  /** 路由变化时自动清空错误态，避免卡在错误页 */
  resetKey?: string
}
interface State { hasError: boolean; error?: Error }

// ErrorBoundary：可嵌套在路由级，单页崩溃不拖垮整站壳。
export default class ErrorBoundary extends React.Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error('Unhandled UI error:', error, info.componentStack)
  }

  componentDidUpdate(prevProps: Props) {
    if (prevProps.resetKey !== this.props.resetKey && this.state.hasError) {
      this.setState({ hasError: false, error: undefined })
    }
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: undefined })
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
          extra={
            <Space>
              <Button type="primary" onClick={this.handleRetry}>重试</Button>
              <Button onClick={this.handleReload}>刷新页面</Button>
            </Space>
          }
        />
      )
    }
    return this.props.children
  }
}
