import { Button, Typography } from 'antd'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'

const { Text } = Typography

// 404 页：此前未知路由静默重定向首页——用户打错链接时毫无反馈。
export default function NotFound() {
  const navigate = useNavigate()
  const role = useAuthStore((s) => s.role)
  const home = role === 'admin' ? '/admin' : '/m/dashboard'

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--wr-bg-base)' }}>
      <div className="wr-glass-card" style={{ padding: 48, textAlign: 'center', maxWidth: 420 }}>
        <div style={{ fontSize: 56, fontWeight: 800, letterSpacing: '-0.04em' }} className="wr-gradient-text">404</div>
        <h2 style={{ margin: '8px 0 12px', fontSize: 18 }}>页面不存在</h2>
        <Text type="secondary" style={{ fontSize: 13, display: 'block', marginBottom: 24 }}>
          你访问的地址不存在或已被移动——品牌在 AI 中的表现请在「AI 可见度」查看。
        </Text>
        <Button type="primary" onClick={() => navigate(home, { replace: true })}>回工作台</Button>
      </div>
    </div>
  )
}
