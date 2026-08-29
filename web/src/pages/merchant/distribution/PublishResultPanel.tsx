import { Alert, Button, Card, Progress, Space, Typography } from 'antd'
import { CheckCircleFilled, CloseCircleFilled, FundOutlined, LoadingOutlined, ReloadOutlined, RocketOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { PlatformBadge } from '../../../components/PlatformBadge'

const { Text, Title } = Typography

export type PublishJobStatus = {
  id: string
  platform: string | undefined
  status: string
  error_msg?: string
  external_url?: string
}

type Props = {
  jobs: PublishJobStatus[]
  onRetry?: () => void
  onDismiss: () => void
}

/** 发布结果面板：展示多平台并行发布的实时进度与最终结果 */
export function PublishResultPanel({ jobs, onRetry, onDismiss }: Props) {
  const navigate = useNavigate()
  if (jobs.length === 0) return null

  const total = jobs.length
  const done = jobs.filter((j) => j.status === 'published' || j.status === 'failed')
  const success = jobs.filter((j) => j.status === 'published')
  const failed = jobs.filter((j) => j.status === 'failed')
  const inFlight = total - done.length
  const allDone = done.length === total
  const percent = Math.round((done.length / total) * 100)

  return (
    <Card
      size="small"
      style={{ marginBottom: 16 }}
      title={
        <Space>
          <RocketOutlined />
          <Title level={5} style={{ margin: 0 }}>
            {allDone ? '发布完成' : '发布进行中'}
          </Title>
          {allDone && (
            <Text type="secondary">
              {success.length}/{total} 成功
              {failed.length > 0 && `，${failed.length} 失败`}
            </Text>
          )}
        </Space>
      }
      extra={
        <Space>
          {/* 闭环入口：发布完成 → 引导去看数据表现 */}
          {allDone && success.length > 0 && (
            <Button size="small" type="primary" ghost icon={<FundOutlined />} onClick={() => navigate('/m/analytics')}>
              查看数据
            </Button>
          )}
          {allDone && failed.length > 0 && onRetry && (
            <Button size="small" icon={<ReloadOutlined />} onClick={onRetry}>
              重试失败项
            </Button>
          )}
          {allDone && (
            <Button size="small" type="text" onClick={onDismiss}>
              关闭
            </Button>
          )}
        </Space>
      }
    >
      {!allDone && (
        <div style={{ marginBottom: 12 }}>
          <Progress percent={percent} size="small" status="active" />
          <Text type="secondary" style={{ fontSize: 12 }}>
            {inFlight > 0 && `${inFlight} 个平台正在发布...`}
          </Text>
        </div>
      )}

      <Space direction="vertical" style={{ width: '100%' }} size={8}>
        {jobs.map((job) => (
          <div
            key={job.id}
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '8px 12px',
              borderRadius: 8,
              background: 'var(--wr-bg-surface, #fafafa)',
              border: '1px solid var(--wr-border, #f0f0f0)',
            }}
          >
            <Space>
              {job.status === 'published' && <CheckCircleFilled style={{ color: '#52c41a' }} />}
              {job.status === 'failed' && <CloseCircleFilled style={{ color: '#ff4d4f' }} />}
              {(job.status === 'pending' || job.status === 'processing') && (
                <LoadingOutlined spin style={{ color: '#1677ff' }} />
              )}
              <PlatformBadge platform={job.platform || ''} size={16} />
            </Space>
            <div style={{ flex: 1, marginLeft: 12, textAlign: 'right' }}>
              {job.status === 'published' && (
                <Space>
                  <Text type="success" style={{ fontSize: 13 }}>✓ 已发布</Text>
                  {job.external_url && (
                    <Button type="link" size="small" href={job.external_url} target="_blank">
                      查看作品
                    </Button>
                  )}
                </Space>
              )}
              {job.status === 'failed' && (
                <Text type="danger" style={{ fontSize: 12 }}>
                  ✗ {job.error_msg || '发布失败'}
                </Text>
              )}
              {(job.status === 'pending' || job.status === 'processing') && (
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {job.status === 'pending' ? '排队中...' : 'RPA 自动发布中...'}
                </Text>
              )}
            </div>
          </div>
        ))}
      </Space>

      {allDone && failed.length > 0 && (
        <Alert
          type="warning"
          showIcon
          style={{ marginTop: 12 }}
          message={`${failed.length} 个平台发布失败`}
          description="可能是平台风控或页面结构变化。可点击「重试失败项」重新发布，或切换到半自动模式（生成链接手动发布）。"
        />
      )}
    </Card>
  )
}
