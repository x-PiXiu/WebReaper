import { useState } from 'react'
import { Form, Input, Button, Tabs, message } from 'antd'
import { useNavigate } from 'react-router-dom'
import { authApi } from '../api/auth'
import { useAuthStore } from '../store/auth'

export default function Login() {
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)

  const handleLogin = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      const res = await authApi.login(values)
      setAuth(res.token, res.username || values.username, res.role, res.tenant_id)
      message.success('登录成功')
      const home = res.role === 'admin' ? '/admin' : '/m'
      navigate(home, { replace: true })
    } catch {
    } finally {
      setLoading(false)
    }
  }

  const handleRegister = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      await authApi.register(values)
      message.success('注册成功，请登录')
    } catch {
    } finally {
      setLoading(false)
    }
  }

  const formItems = (onFinish: (v: { username: string; password: string }) => void, btnText: string) => (
    <Form layout="vertical" onFinish={onFinish} autoComplete="off" requiredMark={false}>
      <Form.Item label="用户名" name="username" rules={[{ required: true, min: 3, message: '至少 3 个字符' }]}>
        <Input placeholder="请输入用户名" size="large" />
      </Form.Item>
      <Form.Item label="密码" name="password" rules={[{ required: true, min: 6, message: '至少 6 个字符' }]}>
        <Input.Password placeholder="请输入密码" size="large" />
      </Form.Item>
      <Form.Item style={{ marginBottom: 0, marginTop: 24 }}>
        <Button type="primary" htmlType="submit" loading={loading} block size="large">
          {btnText}
        </Button>
      </Form.Item>
    </Form>
  )

  return (
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      justifyContent: 'center',
      alignItems: 'center',
      position: 'relative',
      overflow: 'hidden',
      background: 'var(--wr-bg-base)',
    }}>
      {/* 背景光晕效果 —— 双色极光 */}
      <div style={{
        position: 'absolute', top: '10%', left: '50%', transform: 'translateX(-50%)',
        width: 700, height: 700, borderRadius: '50%',
        background: 'radial-gradient(circle, var(--wr-primary-bg) 0%, transparent 60%)',
        filter: 'blur(80px)', pointerEvents: 'none',
      }} />
      <div style={{
        position: 'absolute', bottom: '5%', right: '10%',
        width: 500, height: 500, borderRadius: '50%',
        background: 'radial-gradient(circle, var(--wr-accent-bg) 0%, transparent 60%)',
        filter: 'blur(80px)', pointerEvents: 'none',
      }} />

      {/* 登录卡片 —— 玻璃质感 */}
      <div
        className="wr-fade-in"
        style={{
          width: 420, maxWidth: '90vw', padding: '48px 36px',
          background: 'var(--wr-bg-surface)',
          backdropFilter: 'blur(24px)', WebkitBackdropFilter: 'blur(24px)',
          border: '1px solid var(--wr-border)',
          borderRadius: 20,
          boxShadow: '0 24px 64px rgba(0,0,0,0.5)',
          position: 'relative', zIndex: 1,
        }}
      >
        {/* Logo */}
        <div style={{ textAlign: 'center', marginBottom: 36 }}>
          <div style={{
            width: 56, height: 56, borderRadius: 16,
            background: 'linear-gradient(135deg, var(--wr-primary), var(--wr-accent))',
            margin: '0 auto 18px',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 28, fontWeight: 800, color: '#fff',
            boxShadow: 'var(--wr-shadow-glow)',
          }}>G</div>
          <h1 className="wr-gradient-text" style={{
            fontSize: 26, fontWeight: 800, margin: 0, letterSpacing: '-0.03em',
          }}>GEO 平台</h1>
          <p style={{ fontSize: 14, color: 'var(--wr-text-muted)', margin: '6px 0 0' }}>
            AI 搜索时代 · 生成式引擎优化
          </p>
        </div>

        <Tabs
          defaultActiveKey="login"
          centered
          items={[
            { key: 'login', label: '登录', children: formItems(handleLogin, '登录') },
            { key: 'register', label: '注册', children: formItems(handleRegister, '注册') },
          ]}
        />
      </div>
    </div>
  )
}
