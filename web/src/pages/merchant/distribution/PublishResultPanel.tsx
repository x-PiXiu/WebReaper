import { Alert, Button, Card, Progress, Space, Typography } from 'antd'
import { CheckCircleFilled, CloseCircleFilled, ExportOutlined, FundOutlined, LinkOutlined, LoadingOutlined, ReloadOutlined, RocketOutlined } from '@ant-design/icons'
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
  mode?: 'auto' | 'semi'
  onRetry?: () => void
  onDismiss: () => void
  onMarkPublished?: (jobId: string) => void
}

/** 发布结果面板：全自动进度追踪，或半自动「前往发布」引导 */
export function PublishResultPanel({ jobs, mode = 'auto', onRetry, onDismiss, onMarkPublished }: Props) {
  const navigate = useNavigate()
  if (jobs.length === 0) return null

  const isSemi = mode === 'semi'
  const total = jobs.length
  const done = isSemi
    ? jobs.filter((j) => j.status === 'published')
    : jobs.filter((j) => j.status === 'published' || j.status === 'failed')
  const success = jobs.filter((j) => j.status === 'published')
  const failed = jobs.filter((j) => j.status === 'failed')
  const inFlight = isSemi ? 0 : total - done.length
  const allDone = isSemi ? success.length === total : done.length === total
  const percent = isSemi ? (success.length / total) * 100 : Math.round((done.length / total) * 100)

  return (
    <Card
      size="small"
      style={{ marginBottom: 16 }}
      title={
        <Space>
          <RocketOutlined />
          <Title level={5} style={{ margin: 0 }}>
            {isSemi
              ? (allDone ? '半自动发布已完成' : '半自动发布 · 请前往各平台')
              : (allDone ? '发布完成' : '发布进行中')}
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
          {allDone && success.length > 0 && (
            <Button size="small" type="primary" ghost onClick={() => navigate('/m/works')}>
              回作品库
            </Button>
          )}
          {allDone && success.length > 0 && !isSemi && (
            <Button size="small" icon={<FundOutlined />} onClick={() => navigate('/m/analytics')}>
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
      {!allDone && !isSemi && (
        <div style={{ marginBottom: 12 }}>
          <Progress percent={percent} size="small" status="active" />
          <Text type="secondary" style={{ fontSize: 12 }}>
            {inFlight > 0 && `${inFlight} 个平台正在发布...`}
          </Text>
        </div>
      )}

      {isSemi && !allDone && (
        <Alert
          type="success"
          showIcon
          style={{ marginBottom: 12 }}
          message="内容已准备就绪"
          description="点击下方「前往发布」打开各平台发布页，完成后点「已发布」标记。"
        />
      )}

      {isSemi && jobs.some((j) => j.platform === 'zhihu') && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="知乎需手动粘贴正文（平台限制），点击「前往发布」后请 Ctrl+V"
        />
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
              {!isSemi && (job.status === 'pending' || job.status === 'processing') && (
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {job.status === 'pending' ? '排队中...' : 'RPA 自动发布中...'}
                </Text>
              )}
              {isSemi && job.status !== 'published' && job.status !== 'failed' && (
                <Space>
                  {job.external_url && (
                    <Button size="small" type="primary" icon={<ExportOutlined />} href={job.external_url} target="_blank">
                      前往发布
                    </Button>
                  )}
                  {onMarkPublished && (
                    <Button size="small" type="link" icon={<LinkOutlined />} onClick={() => onMarkPublished(job.id)}>
                      已发布
                    </Button>
                  )}
                </Space>
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
          description="可能是平台风控或页面结构变化。可重试失败项，或改用半自动（生成链接后手动发布）。"
        />
      )}
    </Card>
  )
}
