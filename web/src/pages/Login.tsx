import { useRef, useState } from 'react'
import { Form, Input, Button, message } from 'antd'
import { useLocation, useNavigate } from 'react-router-dom'
import { authApi } from '../api/auth'
import { useAuthStore } from '../store/auth'

type Mode = 'login' | 'register'

export default function Login() {
  const [loading, setLoading] = useState(false)
  const [mode, setMode] = useState<Mode>('login')
  const [torchOn, setTorchOn] = useState(false)
  const brandRef = useRef<HTMLElement>(null)
  const rafRef = useRef(0)
  const navigate = useNavigate()
  const location = useLocation()
  const setAuth = useAuthStore((s) => s.setAuth)

  const moveTorch = (clientX: number, clientY: number) => {
    const el = brandRef.current
    if (!el) return
    cancelAnimationFrame(rafRef.current)
    rafRef.current = requestAnimationFrame(() => {
      const rect = el.getBoundingClientRect()
      const x = ((clientX - rect.left) / rect.width) * 100
      const y = ((clientY - rect.top) / rect.height) * 100
      el.style.setProperty('--torch-x', `${x}%`)
      el.style.setProperty('--torch-y', `${y}%`)
    })
  }

  const handleTorchEnter = (e: React.MouseEvent<HTMLElement>) => {
    setTorchOn(true)
    moveTorch(e.clientX, e.clientY)
  }

  const handleTorchMove = (e: React.MouseEvent<HTMLElement>) => {
    moveTorch(e.clientX, e.clientY)
  }

  const handleTorchLeave = () => {
    setTorchOn(false)
    cancelAnimationFrame(rafRef.current)
  }

  const handleLogin = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      const res = await authApi.login(values)
      setAuth(res.token, res.username || values.username, res.role, res.tenant_id, !!res.must_change_password)
      message.success('登录成功')
      // 回跳来源页（路由守卫记录的 from）——OAuth 回调等深链场景：登录后回到原页面而非首页。
      // 仅接受站内相对路径（防开放重定向）；无来源时走角色默认首页
      const from = (location.state as { from?: string } | null)?.from
      const fallback = res.role === 'admin' ? '/admin' : '/m/dashboard'
      navigate(from && from.startsWith('/') ? from : fallback, { replace: true })
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
      setMode('login')
    } catch {
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="wr-login">
      <aside
        ref={brandRef}
        className={`wr-login-brand${torchOn ? ' is-torch-on' : ''}`}
        aria-label="产品介绍"
        onMouseEnter={handleTorchEnter}
        onMouseMove={handleTorchMove}
        onMouseLeave={handleTorchLeave}
      >
        <div className="wr-login-texture" aria-hidden />
        <div className="wr-login-wash" aria-hidden />
        <div className="wr-login-beam" aria-hidden />
        <div className="wr-login-torch-lit" aria-hidden />
        <div className="wr-login-torch-glow" aria-hidden />
        <div className="wr-login-torch-core" aria-hidden />

        <header className="wr-login-brand-top wr-login-rise">
          <p className="wr-login-kicker">Generative Engine Optimization</p>
          <h1 className="wr-login-brand-title">获客智能体</h1>
          <span className="wr-login-title-line" aria-hidden />
        </header>

        <div className="wr-login-brand-body wr-login-rise wr-login-rise--delay">
          <h2 className="wr-login-headline">
            在生成式搜索里，
            <br />
            让品牌被真正看见。
          </h2>
          <p className="wr-login-lead">
            关键词、内容、分发与收录监测连成一条工作流——
            少一点热闹的包装，多一点可验证的可见度。
          </p>
        </div>

        <footer className="wr-login-brand-foot wr-login-rise wr-login-rise--delay2">
          <span>关键词策略</span>
          <span className="wr-login-dot" aria-hidden />
          <span>内容生成</span>
          <span className="wr-login-dot" aria-hidden />
          <span>发布与收录</span>
        </footer>
      </aside>

      <main className="wr-login-panel">
        <div className="wr-login-panel-glow" aria-hidden />
        <div className="wr-login-panel-inner wr-login-rise wr-login-rise--delay">
          <div className="wr-login-form-head">
            <h2>{mode === 'login' ? '登录' : '注册'}</h2>
            <p>
              {mode === 'login'
                ? '进入智擎AI 工作空间'
                : '创建账号后即可开始管理品牌资产'}
            </p>
          </div>

          <div className="wr-login-mode" role="tablist" aria-label="登录或注册">
            <button
              type="button"
              role="tab"
              aria-selected={mode === 'login'}
              className={mode === 'login' ? 'is-active' : undefined}
              onClick={() => setMode('login')}
            >
              登录
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={mode === 'register'}
              className={mode === 'register' ? 'is-active' : undefined}
              onClick={() => setMode('register')}
            >
              注册
            </button>
          </div>

          <Form
            key={mode}
            layout="vertical"
            onFinish={mode === 'login' ? handleLogin : handleRegister}
            autoComplete="off"
            requiredMark={false}
            className="wr-login-form"
          >
            <Form.Item
              label="用户名"
              name="username"
              rules={[{ required: true, min: 3, message: '至少 3 个字符' }]}
            >
              <Input placeholder="请输入用户名" size="large" autoFocus />
            </Form.Item>
            <Form.Item
              label="密码"
              name="password"
              rules={[{ required: true, min: 6, message: '至少 6 个字符' }]}
            >
              <Input.Password placeholder="请输入密码" size="large" />
            </Form.Item>
            <Form.Item className="wr-login-submit">
              <Button type="primary" htmlType="submit" loading={loading} block size="large">
                {mode === 'login' ? '登录' : '注册'}
              </Button>
            </Form.Item>
          </Form>

          <p className="wr-login-footnote">
            {mode === 'login' ? '还没有账号？' : '已有账号？'}
            <button type="button" onClick={() => setMode(mode === 'login' ? 'register' : 'login')}>
              {mode === 'login' ? '注册' : '登录'}
            </button>
          </p>
        </div>
      </main>
    </div>
  )
}
