import { useEffect, useState } from 'react'
import { Checkbox, Form, Input, Button } from 'antd'
import { UserOutlined, LockOutlined } from '@ant-design/icons'
import { useLocation, useNavigate } from 'react-router-dom'
import { authApi } from '../api/auth'
import { useAuthStore } from '../store/auth'
import { toast } from '../utils/feedback'
import logoUrl from '../assets/logo-zhichen.jpg'

type Mode = 'login' | 'register'

const REMEMBER_KEY = 'wr-login-username'

export default function Login() {
  const [loading, setLoading] = useState(false)
  const [mode, setMode] = useState<Mode>('login')
  const [remember, setRemember] = useState(() => !!localStorage.getItem(REMEMBER_KEY))
  const [form] = Form.useForm()
  const navigate = useNavigate()
  const location = useLocation()
  const setAuth = useAuthStore((s) => s.setAuth)

  // 「记住用户名」：回填上次登录的账号
  useEffect(() => {
    const saved = localStorage.getItem(REMEMBER_KEY)
    if (saved) form.setFieldsValue({ username: saved })
  }, [form, mode])

  const switchMode = (next: Mode) => {
    setMode(next)
    form.resetFields(['password', 'confirm'])
  }

  const handleLogin = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      const res = await authApi.login(values)
      if (remember) localStorage.setItem(REMEMBER_KEY, values.username)
      else localStorage.removeItem(REMEMBER_KEY)
      setAuth(res.token, res.username || values.username, res.role, res.tenant_id, !!res.must_change_password)
      toast.ok('登录成功')
      // 回跳来源页（路由守卫记录的 from）——OAuth 回调等深链场景：登录后回到原页面而非首页。
      // 仅接受站内相对路径（防开放重定向）；无来源时走角色默认首页
      const from = (location.state as { from?: string } | null)?.from
      const fallback = res.role === 'admin' ? '/admin' : '/m/compose'
      navigate(from && from.startsWith('/') ? from : fallback, { replace: true })
    } catch {
      // 业务/网络错误已由 apiClient 拦截器 toast
    } finally {
      setLoading(false)
    }
  }

  const handleRegister = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      await authApi.register(values)
      toast.ok('注册成功，请登录')
      switchMode('login')
    } catch {
      // 业务/网络错误已由 apiClient 拦截器 toast
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="wr-login">
      <div className="wr-login-bg" aria-hidden>
        <svg className="wr-login-waves" viewBox="0 0 1440 900" preserveAspectRatio="xMidYMid slice">
          <path d="M-80 640 C 280 520, 560 760, 900 640 S 1420 480, 1560 560" fill="none" stroke="rgba(255,255,255,0.65)" strokeWidth="1.5" />
          <path d="M-80 700 C 300 580, 620 820, 960 700 S 1440 560, 1560 620" fill="none" stroke="rgba(255,255,255,0.45)" strokeWidth="1.2" />
          <path d="M-80 760 C 320 660, 640 880, 1000 760 S 1440 660, 1560 700" fill="none" stroke="rgba(255,255,255,0.3)" strokeWidth="1" />
          <path d="M-80 180 C 240 260, 520 80, 860 160 S 1360 260, 1560 140" fill="none" stroke="rgba(255,255,255,0.35)" strokeWidth="1.2" />
        </svg>
        <i className="wr-login-grid" aria-hidden />
      </div>

      <aside className="wr-login-brand" aria-label="产品介绍">
        <div className="wr-login-brand-main wr-login-rise">
          <div className="wr-login-logo">
            <img src={logoUrl} alt="智宸AI logo" />
          </div>
          <h1 className="wr-login-title">智宸AI获客智能体</h1>
          <p className="wr-login-lead">对标爆款 · 口播成片 · 一键分发，为人设获客而生。</p>
        </div>

        <footer className="wr-login-flow wr-login-rise wr-login-rise--delay">
          <span>建人设</span>
          <i aria-hidden />
          <span>出内容</span>
          <i aria-hidden />
          <span>发出去</span>
          <i aria-hidden />
          <span>看线索</span>
        </footer>
      </aside>

      <main className="wr-login-panel">
        <div className="wr-login-card wr-login-rise wr-login-rise--delay">
          <div className="wr-login-form-head">
            <h2>{mode === 'login' ? '欢迎回来' : '创建账号'}</h2>
            <p>
              {mode === 'login' ? '登录后继续打造账号 IP' : '注册后即可开始打造账号 IP'}
            </p>
          </div>

          <Form
            key={mode}
            form={form}
            layout="vertical"
            onFinish={mode === 'login' ? handleLogin : handleRegister}
            autoComplete="on"
            requiredMark={false}
            className="wr-login-form"
          >
            <Form.Item
              label="用户名"
              name="username"
              rules={[{ required: true, min: 3, message: '至少 3 个字符' }]}
            >
              <Input
                placeholder="请输入用户名"
                size="large"
                prefix={<UserOutlined />}
                autoFocus
                autoComplete="username"
              />
            </Form.Item>
            <Form.Item
              label="密码"
              name="password"
              rules={[{ required: true, min: 6, message: '至少 6 个字符' }]}
            >
              <Input.Password
                placeholder="请输入密码"
                size="large"
                prefix={<LockOutlined />}
                autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
              />
            </Form.Item>

            {mode === 'register' && (
              <Form.Item
                label="确认密码"
                name="confirm"
                dependencies={['password']}
                rules={[
                  { required: true, message: '请再次输入密码' },
                  ({ getFieldValue }) => ({
                    validator(_, value) {
                      if (!value || getFieldValue('password') === value) return Promise.resolve()
                      return Promise.reject(new Error('两次密码不一致'))
                    },
                  }),
                ]}
              >
                <Input.Password
                  placeholder="请再次输入密码"
                  size="large"
                  prefix={<LockOutlined />}
                  autoComplete="new-password"
                />
              </Form.Item>
            )}

            {mode === 'login' && (
              <div className="wr-login-remember">
                <Checkbox checked={remember} onChange={(e) => setRemember(e.target.checked)}>
                  记住用户名
                </Checkbox>
              </div>
            )}

            <Form.Item className="wr-login-submit">
              <Button type="primary" htmlType="submit" loading={loading} block size="large">
                {mode === 'login' ? '立即登录' : '立即注册'}
              </Button>
            </Form.Item>
          </Form>

          <p className="wr-login-footnote">
            {mode === 'login' ? '还没有账号？' : '已有账号？'}
            <button type="button" onClick={() => switchMode(mode === 'login' ? 'register' : 'login')}>
              {mode === 'login' ? '立即注册' : '直接登录'}
            </button>
          </p>
        </div>
      </main>

      <footer className="wr-login-footer">
        © {new Date().getFullYear()} 智宸AI · 获客智能体
      </footer>
    </div>
  )
}
