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
      setAuth(res.token, values.username)
      message.success('登录成功')
      navigate('/dashboard', { replace: true })
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
      background: '#0a0a0f',
    }}>
      {/* 背景光晕效果 */}
      <div style={{
        position: 'absolute',
        top: '20%',
        left: '50%',
        transform: 'translateX(-50%)',
        width: 600,
        height: 600,
        borderRadius: '50%',
        background: 'radial-gradient(circle, rgba(99,102,241,0.15) 0%, transparent 60%)',
        filter: 'blur(60px)',
        pointerEvents: 'none',
      }} />
      <div style={{
        position: 'absolute',
        bottom: '10%',
        right: '15%',
        width: 400,
        height: 400,
        borderRadius: '50%',
        background: 'radial-gradient(circle, rgba(34,211,238,0.1) 0%, transparent 60%)',
        filter: 'blur(60px)',
        pointerEvents: 'none',
      }} />

      {/* 登录卡片 */}
      <div
        className="wr-fade-in"
        style={{
          width: 400,
          maxWidth: '90vw',
          padding: '40px 32px',
          background: 'rgba(18,18,24,0.6)',
          backdropFilter: 'blur(20px)',
          WebkitBackdropFilter: 'blur(20px)',
          border: '1px solid rgba(255,255,255,0.06)',
          borderRadius: 16,
          boxShadow: '0 24px 48px rgba(0,0,0,0.4)',
          position: 'relative',
          zIndex: 1,
        }}
      >
        {/* Logo */}
        <div style={{ textAlign: 'center', marginBottom: 32 }}>
          <div style={{
            width: 48, height: 48, borderRadius: 12,
            background: 'linear-gradient(135deg, #6366f1, #22d3ee)',
            margin: '0 auto 16px',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 24, fontWeight: 800, color: '#fff',
            boxShadow: '0 0 24px rgba(99,102,241,0.4)',
          }}>W</div>
          <h1 style={{
            fontSize: 22, fontWeight: 700, margin: 0,
            color: '#e4e4e7', letterSpacing: '-0.02em',
          }}>WebReaper</h1>
          <p style={{ fontSize: 13, color: '#71717a', margin: '4px 0 0' }}>
            数据采集与智能加工平台
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
