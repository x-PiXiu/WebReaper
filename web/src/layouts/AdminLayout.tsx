import { useState } from 'react'
import { Alert, Button, Form, Input, Modal } from 'antd'
import { AppShell, type NavItem } from './MainLayout'
import { useAuthStore } from '../store/auth'
import { businessApi } from '../api/business'
import { message } from '../utils/antdApp'

// 管理后台布局：菜单按功能域分组（AntD Menu 的 group 类型渲染）。
//   - 平台管理：SaaS 运营（总览/商户/品牌/内容/计费）
//   - 系统配置：平台运行时开关 + 第三方集成凭据（生成厂商/搜索/支付）
//   - GEO 内容引擎：Agent/LLM/工具、AI 对话、提示词模板、提交渠道、生成规格
// 术语：商户端"收录"指内容被引擎收录/关键词被 AI 提及；此处是 IndexNow
// 提交渠道配置——叫「提交渠道」与商户端概念解耦，避免一词三义。
const adminMenu: NavItem[] = [
  {
    key: 'platform', label: '平台管理',
    children: [
      { key: '/admin', label: '平台总览' },
      { key: '/admin/users', label: '商户管理' },
      { key: '/admin/brands', label: '品牌管理' },
      { key: '/admin/contents', label: '内容管理' },
      { key: '/admin/billing', label: '计费管理' },
      { key: '/admin/voices', label: '官方音色' },
      { key: '/admin/subjects', label: '官方主体' },
    ],
  },
  {
    key: 'system', label: '系统配置',
    children: [
      { key: '/admin/settings', label: '平台设置' },
      { key: '/admin/integrations', label: '第三方集成' },
    ],
  },
  {
    key: 'engine', label: '内容引擎',
    children: [
      { key: '/admin/chat', label: 'AI 对话' },
      { key: '/admin/agent-configs', label: 'Agent 配置' },
      { key: '/admin/prompt-templates', label: '提示词模板' },
      { key: '/admin/generation-templates', label: '生成模板' },
      { key: '/admin/indexing', label: '提交渠道' },
      { key: '/admin/knowledge', label: '知识库' },
    ],
  },
  {
    key: 'crawler', label: '爬虫管理',
    children: [
      { key: '/admin/crawler-accounts', label: '平台方账号' },
      { key: '/admin/crawler-configs', label: '爬虫配置' },
      { key: '/admin/crawler-tasks', label: '任务监控' },
      { key: '/admin/inspirations', label: '灵感运营' },
    ],
  },
]

// 管理后台布局。
// F1-5：登录态携带 must_change_password（仍在用默认口令 admin/admin123）时，
// 内容区顶部常驻告警 + 改密弹窗（PUT /auth/password，旧密码验证在用例层）。
export default function AdminLayout() {
  const mustChange = useAuthStore((s) => s.mustChangePassword)
  const setMustChange = useAuthStore((s) => s.setMustChangePassword)
  const [pwOpen, setPwOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm()

  const handleChangePassword = async (v: { old_password: string; new_password: string }) => {
    setSaving(true)
    try {
      await businessApi.changePassword(v)
      setMustChange(false)
      setPwOpen(false)
      form.resetFields()
      message.success('密码已修改')
    } catch { /* 拦截器已提示 */ } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <AppShell
        menuItems={adminMenu}
        brandName="获客智能体 · 管理"
        brandIcon="获"
        noPaddingKeys={['/admin/chat']}
        banner={mustChange ? (
          <Alert
            type="warning"
            showIcon
            banner
            style={{ marginBottom: 12 }}
            message="安全提醒：当前使用默认密码 admin123——任何知道文档的人都能登录管理后台，请立即修改"
            action={<Button size="small" danger onClick={() => setPwOpen(true)}>立即修改</Button>}
          />
        ) : undefined}
      />
      <Modal
        title="修改密码"
        open={pwOpen}
        onCancel={() => setPwOpen(false)}
        footer={null}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" onFinish={handleChangePassword}>
          <Form.Item label="当前密码" name="old_password" rules={[{ required: true, message: '请输入当前密码' }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item label="新密码" name="new_password" rules={[{ required: true, message: '请输入新密码' }, { min: 8, message: '至少 8 位' }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item label="确认新密码" name="confirm" dependencies={['new_password']} rules={[
            { required: true, message: '请再次输入新密码' },
            ({ getFieldValue }) => ({
              validator: (_, v) => (v && v !== getFieldValue('new_password')
                ? Promise.reject(new Error('两次输入不一致'))
                : Promise.resolve()),
            }),
          ]}>
            <Input.Password />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={saving} block>修改密码</Button>
        </Form>
      </Modal>
    </>
  )
}
