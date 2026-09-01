import { useEffect, useState } from 'react'
import { Modal, Space, Spin, Button, Typography, Alert } from 'antd'
import { CheckCircleOutlined, ReloadOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../../api/business'
import { toast } from '../../../utils/feedback'

const { Text, Paragraph } = Typography

export const QR_PLATFORMS = [
  { key: 'douyin', name: '抖音' },
  { key: 'kuaishou', name: '快手' },
  { key: 'zhihu', name: '知乎' },
  { key: 'xiaohongshu', name: '小红书' },
  { key: 'bilibili', name: 'B站' },
  { key: 'weixin', name: '视频号' },
]

export default function QRLoginModal({
  open,
  platform,
  onClose,
}: {
  open: boolean
  platform: string
  onClose: () => void
}) {
  const queryClient = useQueryClient()
  const [sessionId, setSessionId] = useState('')
  const [loginMethod, setLoginMethod] = useState('')
  const [startFailed, setStartFailed] = useState(false)

  useEffect(() => {
    if (!open) return
    setSessionId('')
    setLoginMethod('')
    setStartFailed(false)
    if (platform !== 'zhihu') void startQR()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, platform])

  const startQR = async (method?: string) => {
    setLoginMethod(method || '')
    setSessionId('')
    setStartFailed(false)
    try {
      const res = await businessApi.startQRLogin(platform, method)
      setSessionId(res.session_id)
    } catch {
      setStartFailed(true)
    }
  }

  const { data: pollData } = useQuery({
    queryKey: ['qr-status', sessionId, platform],
    queryFn: () => businessApi.pollQRLogin(sessionId, platform, loginMethod),
    enabled: open && !!sessionId,
    refetchInterval: (query) => {
      const s = query.state.data?.status
      return s && (s === 'preparing' || s === 'waiting' || s === 'scanned') ? 2000 : false
    },
  })

  useEffect(() => {
    if (pollData?.status === 'success' && open) {
      const pfName = QR_PLATFORMS.find((p) => p.key === platform)?.name || '账号'
      const name = (pollData.account_name || '').trim()
      toast.ok(name ? `${pfName}「${name}」已绑定` : `${pfName}账号已绑定`, 'qr-bind')
      if (sessionId) { try { void businessApi.cancelQRLogin(sessionId) } catch { /* */ } }
      queryClient.invalidateQueries({ queryKey: ['geo-accounts'] })
      onClose()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pollData?.status])

  const handleClose = async () => {
    if (sessionId) { try { await businessApi.cancelQRLogin(sessionId) } catch { /* */ } }
    onClose()
  }

  const pfName = QR_PLATFORMS.find((p) => p.key === platform)?.name || ''
  const qrImage = pollData?.qr_image
  const status = pollData?.status
  const needRefresh = status === 'expired' || status === 'cancelled' || startFailed

  return (
    <Modal
      title={`绑定 ${pfName} 账号`}
      open={open}
      onCancel={handleClose}
      footer={
        needRefresh ? (
          <Button type="primary" icon={<ReloadOutlined />} onClick={() => startQR(loginMethod || undefined)}>
            重新获取二维码
          </Button>
        ) : null
      }
      width={400}
      centered
      destroyOnClose
    >
      <div style={{ textAlign: 'center', padding: '12px 0' }}>
        {platform === 'zhihu' && !sessionId && !startFailed && (
          <div style={{ marginBottom: 16 }}>
            <Text type="secondary" style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>选择登录方式</Text>
            <Space wrap>
              <Button size="small" onClick={() => startQR('zhihu')}>知乎 App</Button>
              <Button size="small" onClick={() => startQR('wechat')}>微信</Button>
              <Button size="small" onClick={() => startQR('qq')}>QQ</Button>
              <Button size="small" onClick={() => startQR('weibo')}>微博</Button>
            </Space>
          </div>
        )}
        {startFailed && (
          <Alert
            type="warning"
            showIcon
            style={{ textAlign: 'left', marginBottom: 12 }}
            message="暂时无法打开扫码"
            description="请确认本机浏览器自动化可用后重试，或改用官方授权（抖音）。"
          />
        )}
        {qrImage ? (
          <>
            <div style={{ display: 'inline-block', padding: 16, background: '#fff', borderRadius: 12, marginBottom: 16 }}>
              <img
                src={qrImage.startsWith('http') ? qrImage : `data:image/png;base64,${qrImage}`}
                alt="登录二维码"
                style={{ width: 240, height: 'auto', maxHeight: 320, display: 'block' }}
              />
            </div>
            <QRStatusIndicator status={status} platform={platform} />
          </>
        ) : sessionId ? (
          <div style={{ padding: 48 }}>
            <Spin size="large" />
            <Paragraph type="secondary" style={{ marginTop: 16 }}>正在获取二维码，请稍候…</Paragraph>
          </div>
        ) : null}
      </div>
    </Modal>
  )
}

function QRStatusIndicator({ status, platform }: { status?: string; platform: string }) {
  const pfName = QR_PLATFORMS.find((p) => p.key === platform)?.name || ''
  if (!status || status === 'preparing') {
    return <Space><Spin size="small" /><Text type="secondary">正在获取二维码…</Text></Space>
  }
  if (status === 'waiting') {
    return (
      <Space>
        <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--wr-primary)', display: 'inline-block', animation: 'wr-pulse 1.5s infinite' }} />
        <Text type="secondary">请用{pfName} App 扫码</Text>
      </Space>
    )
  }
  if (status === 'scanned') {
    return <Space><CheckCircleOutlined style={{ color: 'var(--wr-accent)' }} /><Text style={{ color: 'var(--wr-accent)' }}>已扫码，请在手机上确认</Text></Space>
  }
  if (status === 'expired') return <Text type="warning">二维码已过期，请点击下方重新获取</Text>
  if (status === 'cancelled') return <Text type="secondary">扫码已取消</Text>
  if (status === 'success') {
    return <Space><CheckCircleOutlined style={{ color: 'var(--wr-success)' }} /><Text style={{ color: 'var(--wr-success)' }}>登录成功，正在绑定…</Text></Space>
  }
  return <Text type="danger">扫码异常，请重新获取</Text>
}
