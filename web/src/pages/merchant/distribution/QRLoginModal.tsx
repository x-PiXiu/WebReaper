import { useEffect, useState } from 'react'
import { Modal, Space, Spin, Button, Typography, message } from 'antd'
import { CheckCircleOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../../api/business'

const { Text, Paragraph } = Typography

// 平台元信息（扫码弹窗内自用；页面级平台卡片仍在 Distribution 页）
export const QR_PLATFORMS = [
  { key: 'douyin', name: '抖音' },
  { key: 'kuaishou', name: '快手' },
  { key: 'zhihu', name: '知乎' },
  { key: 'xiaohongshu', name: '小红书' },
]

// 扫码登录弹窗（账号绑定）：二维码获取/轮询/状态指示自包含于此。
// 父组件只管 open + platform + onClose——扫码会话状态不再泄漏到页面。
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

  // 打开即重置会话；知乎需先选登录方式，其余平台（小红书/抖音/快手）只有自身扫码，直接拉码
  useEffect(() => {
    if (!open) return
    setSessionId('')
    setLoginMethod('')
    if (platform !== 'zhihu') startQR()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, platform])

  const startQR = async (method?: string) => {
    setLoginMethod(method || '')
    setSessionId('')
    try {
      const res = await businessApi.startQRLogin(platform, method)
      setSessionId(res.session_id)
    } catch { /* 拦截器已提示（常见原因：浏览器自动化未配置） */ onClose() }
  }

  // 扫码状态轮询（会话进行中每 2s）
  const { data: pollData } = useQuery({
    queryKey: ['qr-status', sessionId, platform],
    queryFn: () => businessApi.pollQRLogin(sessionId, platform, loginMethod),
    enabled: open && !!sessionId,
    refetchInterval: (query) => {
      const s = query.state.data?.status
      return s && (s === 'preparing' || s === 'waiting' || s === 'scanned') ? 2000 : false
    },
  })

  // 登录成功 → 刷新账号池并关闭
  useEffect(() => {
    if (pollData?.status === 'success' && open) {
      const pfName = QR_PLATFORMS.find((p) => p.key === platform)?.name || '账号'
      message.success(`${pfName}「${pollData.account_name || ''}」绑定成功`)
      if (sessionId) { try { void businessApi.cancelQRLogin(sessionId) } catch { /* 会话已结束 */ } }
      queryClient.invalidateQueries({ queryKey: ['geo-accounts'] })
      onClose()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pollData?.status])

  const handleClose = async () => {
    if (sessionId) { try { await businessApi.cancelQRLogin(sessionId) } catch { /* 忽略取消失败 */ } }
    onClose()
  }

  const pfName = QR_PLATFORMS.find((p) => p.key === platform)?.name || ''
  const qrImage = pollData?.qr_image

  return (
    <Modal
      title={`绑定 ${pfName} 账号`}
      open={open} onCancel={handleClose} footer={null} width={400} centered
    >
      <div style={{ textAlign: 'center', padding: '12px 0' }}>
        {platform === 'zhihu' && !sessionId && (
          <div style={{ marginBottom: 16 }}>
            <Text type="secondary" style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>选择登录方式</Text>
            <Space>
              <Button size="small" onClick={() => startQR('zhihu')}>知乎App</Button>
              <Button size="small" onClick={() => startQR('wechat')}>微信</Button>
              <Button size="small" onClick={() => startQR('qq')}>QQ</Button>
              <Button size="small" onClick={() => startQR('weibo')}>微博</Button>
            </Space>
          </div>
        )}
        {qrImage ? (
          <>
            <div style={{ display: 'inline-block', padding: 16, background: '#fff', borderRadius: 12, marginBottom: 16 }}>
              <img
                src={qrImage.startsWith('http') ? qrImage : `data:image/png;base64,${qrImage}`}
                alt="登录二维码" style={{ width: 240, height: 'auto', maxHeight: 320, display: 'block' }}
              />
            </div>
            <QRStatusIndicator status={pollData?.status} platform={platform} />
          </>
        ) : sessionId ? (
          <div style={{ padding: 60 }}>
            <Spin size="large" />
            <Paragraph type="secondary" style={{ marginTop: 16 }}>正在启动浏览器获取二维码...</Paragraph>
          </div>
        ) : null}
      </div>
    </Modal>
  )
}

// 扫码状态指示器
function QRStatusIndicator({ status, platform }: { status?: string; platform: string }) {
  const pfName = QR_PLATFORMS.find((p) => p.key === platform)?.name || ''
  if (!status || status === 'preparing') {
    return <Space><Spin size="small" /><Text type="secondary">浏览器已打开，正在获取二维码...</Text></Space>
  }
  if (status === 'waiting') {
    return (
      <Space>
        <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--wr-primary)', display: 'inline-block', animation: 'wr-pulse 1.5s infinite' }} />
        <Text type="secondary">请用{pfName}App扫码登录</Text>
      </Space>
    )
  }
  if (status === 'scanned') {
    return <Space><CheckCircleOutlined style={{ color: 'var(--wr-accent)' }} /><Text style={{ color: 'var(--wr-accent)' }}>已扫码，请在手机确认登录</Text></Space>
  }
  if (status === 'expired') return <Text type="warning">二维码已过期，请关闭后重新获取</Text>
  if (status === 'success') {
    return <Space><CheckCircleOutlined style={{ color: 'var(--wr-success)' }} /><Text style={{ color: 'var(--wr-success)' }}>登录成功，正在绑定...</Text></Space>
  }
  return <Text type="danger">扫码异常：{status}</Text>
}
