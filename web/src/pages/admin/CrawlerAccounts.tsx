import { useState, useEffect, useCallback } from 'react'
import { Typography, Card, Space, message, Tag, Modal, Table, Button, Select, Form, Input } from 'antd'
import { PlusOutlined, DeleteOutlined, ReloadOutlined, QrcodeOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { CrawlerAccount } from '../../types/api'

const { Text } = Typography

const PLATFORM_OPTIONS = [
  { value: 'douyin', label: '抖音' },
  { value: 'kuaishou', label: '快手' },
  { value: 'bilibili', label: 'B站' },
  { value: 'xiaohongshu', label: '小红书' },
]

const STATUS_COLORS: Record<string, string> = {
  active: 'green',
  expired: 'orange',
  banned: 'red',
}

const HEALTH_COLORS: Record<string, string> = {
  healthy: 'green',
  unhealthy: 'red',
  unknown: 'default',
}

// 平台方账号管理页面
export default function CrawlerAccounts() {
  const queryClient = useQueryClient()
  const [isQRModalOpen, setIsQRModalOpen] = useState(false)
  const [isManualModalOpen, setIsManualModalOpen] = useState(false)
  const [qrPlatform, setQrPlatform] = useState('douyin')
  const [qrSessionId, setQrSessionId] = useState<string | null>(null)
  const [qrImage, setQrImage] = useState<string | null>(null)
  const [qrStatus, setQrStatus] = useState<string>('pending')
  const [manualForm] = Form.useForm()

  // 查询账号列表
  const { data, isLoading } = useQuery({
    queryKey: ['admin-crawler-accounts'],
    queryFn: () => businessApi.adminListCrawlerAccounts(),
  })
  const accounts = data?.accounts || []

  // 删除账号
  const deleteMutation = useMutation({
    mutationFn: (id: number) => businessApi.adminDeleteCrawlerAccount(id),
    onSuccess: () => {
      message.success('账号已删除')
      queryClient.invalidateQueries({ queryKey: ['admin-crawler-accounts'] })
    },
    onError: () => message.error('删除失败'),
  })

  // 健康检查
  const healthMutation = useMutation({
    mutationFn: ({ id, platform }: { id: number; platform: string }) =>
      businessApi.adminCheckCrawlerAccountHealth(id, platform),
    onSuccess: (data) => {
      message.success(`健康检查完成: ${data.healthy ? '健康' : '异常'}`)
      queryClient.invalidateQueries({ queryKey: ['admin-crawler-accounts'] })
    },
    onError: () => message.error('健康检查失败'),
  })

  // 手动添加账号
  const createMutation = useMutation({
    mutationFn: (data: Partial<CrawlerAccount>) => businessApi.adminCreateCrawlerAccount(data),
    onSuccess: () => {
      message.success('账号添加成功')
      setIsManualModalOpen(false)
      manualForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['admin-crawler-accounts'] })
    },
    onError: () => message.error('添加失败'),
  })

  // ---- 扫码登录流程 ----

  // 启动扫码登录
  const startQRLogin = useCallback(async () => {
    try {
      const resp = await businessApi.startQRLogin(qrPlatform, 'cookie')
      setQrSessionId(resp.session_id)
      setQrStatus('pending')
      setQrImage(null)
    } catch {
      message.error('启动扫码登录失败')
    }
  }, [qrPlatform])

  // 轮询扫码状态
  useEffect(() => {
    if (!qrSessionId || !isQRModalOpen) return

    const timer = setInterval(async () => {
      try {
        const resp = await businessApi.pollQRLogin(qrSessionId, qrPlatform, 'cookie')
        if (resp.qr_image) {
          setQrImage(resp.qr_image)
        }
        if (resp.status === 'success') {
          setQrStatus('success')
          message.success(`账号绑定成功: ${resp.account_name}`)
          setIsQRModalOpen(false)
          setQrSessionId(null)
          queryClient.invalidateQueries({ queryKey: ['admin-crawler-accounts'] })
          clearInterval(timer)
        } else if (resp.status === 'expired') {
          setQrStatus('expired')
          clearInterval(timer)
        }
      } catch {
        // 继续轮询
      }
    }, 2000)

    return () => clearInterval(timer)
  }, [qrSessionId, qrPlatform, isQRModalOpen, queryClient])

  // 取消扫码
  const cancelQRLogin = useCallback(async () => {
    if (qrSessionId) {
      await businessApi.cancelQRLogin(qrSessionId).catch(() => {})
    }
    setIsQRModalOpen(false)
    setQrSessionId(null)
    setQrImage(null)
    setQrStatus('pending')
  }, [qrSessionId])

  // 打开扫码弹窗（只打开弹窗，不自动启动扫码）
  const openQRModal = () => {
    setIsQRModalOpen(true)
    setQrStatus('pending')
    setQrImage(null)
    setQrSessionId(null)
  }

  // 表格列定义
  const columns = [
    {
      title: '平台',
      dataIndex: 'platform',
      key: 'platform',
      render: (platform: string) => {
        const labels: Record<string, string> = { douyin: '抖音', kuaishou: '快手', bilibili: 'B站', xiaohongshu: '小红书' }
        return <Tag color="blue">{labels[platform] || platform}</Tag>
      },
    },
    {
      title: '账号名称',
      dataIndex: 'account_name',
      key: 'account_name',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={STATUS_COLORS[status] || 'default'}>{status}</Tag>
      ),
    },
    {
      title: '健康状态',
      dataIndex: 'health_check_result',
      key: 'health_check_result',
      render: (result: string) => (
        <Tag color={HEALTH_COLORS[result] || 'default'}>{result}</Tag>
      ),
    },
    {
      title: '今日用量',
      key: 'usage',
      render: (_: unknown, record: CrawlerAccount) => (
        <Text>{record.daily_usage_count} / {record.daily_usage_limit}</Text>
      ),
    },
    {
      title: '代理地址',
      dataIndex: 'proxy_address',
      key: 'proxy_address',
      render: (addr: string) => addr || <Text type="secondary">无</Text>,
    },
    {
      title: '最后使用',
      dataIndex: 'last_used_at',
      key: 'last_used_at',
      render: (t: string) => t ? new Date(t).toLocaleString() : '-',
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: unknown, record: CrawlerAccount) => (
        <Space>
          <Button
            size="small"
            icon={<ReloadOutlined />}
            onClick={() => healthMutation.mutate({ id: record.id, platform: record.platform })}
          >
            健康检查
          </Button>
          <Button
            size="small"
            danger
            icon={<DeleteOutlined />}
            onClick={() => {
              Modal.confirm({
                title: '确认删除',
                content: `确定删除账号 "${record.account_name}" 吗？`,
                onOk: () => deleteMutation.mutate(record.id),
              })
            }}
          >
            删除
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title="平台方账号管理"
        extra={
          <Space>
            <Button type="primary" icon={<QrcodeOutlined />} onClick={openQRModal}>
              扫码添加
            </Button>
            <Button icon={<PlusOutlined />} onClick={() => setIsManualModalOpen(true)}>
              手动添加
            </Button>
          </Space>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
          平台方账号用于统一爬取热门视频数据。用户无需登录平台即可查看灵感广场。推荐使用扫码登录添加账号。
        </Text>
        <Table
          columns={columns}
          dataSource={accounts}
          rowKey="id"
          loading={isLoading}
          pagination={false}
        />
      </Card>

      {/* 扫码登录弹窗 */}
      <Modal
        title="扫码添加平台方账号"
        open={isQRModalOpen}
        onCancel={cancelQRLogin}
        footer={[
          <Button key="cancel" onClick={cancelQRLogin}>取消</Button>,
          qrStatus === 'expired' ? (
            <Button key="retry" type="primary" onClick={() => startQRLogin()}>重新获取二维码</Button>
          ) : null,
        ]}
        width={400}
      >
        <div style={{ textAlign: 'center', padding: 20 }}>
          {/* 平台选择 */}
          <div style={{ marginBottom: 16 }}>
            <Text>选择平台：</Text>
            <Select
              value={qrPlatform}
              onChange={(v) => {
                setQrPlatform(v)
                setQrImage(null)
                setQrSessionId(null)
                setQrStatus('pending')
              }}
              options={PLATFORM_OPTIONS}
              style={{ marginLeft: 8, width: 150 }}
            />
          </div>

          {/* 开始扫码按钮（未启动时显示） */}
          {!qrSessionId && qrStatus === 'pending' && (
            <Button
              type="primary"
              icon={<QrcodeOutlined />}
              onClick={startQRLogin}
              style={{ marginBottom: 16 }}
            >
              开始扫码
            </Button>
          )}

          {/* 二维码显示 */}
          {qrStatus === 'expired' ? (
            <div>
              <Text type="warning">二维码已过期，请点击"重新获取二维码"</Text>
            </div>
          ) : qrImage ? (
            <div>
              <img
                src={`data:image/png;base64,${qrImage}`}
                alt="扫码登录"
                style={{ maxWidth: 256, maxHeight: 256, border: '1px solid #eee' }}
              />
              <div style={{ marginTop: 8 }}>
                <Text type="secondary">请使用 {PLATFORM_OPTIONS.find(p => p.value === qrPlatform)?.label} 手机APP扫描二维码</Text>
              </div>
            </div>
          ) : qrSessionId ? (
            <div>
              <Text type="secondary">正在获取二维码...</Text>
            </div>
          ) : null}
        </div>
      </Modal>

      {/* 手动添加弹窗 */}
      <Modal
        title="手动添加账号"
        open={isManualModalOpen}
        onCancel={() => setIsManualModalOpen(false)}
        onOk={() => manualForm.submit()}
        confirmLoading={createMutation.isPending}
      >
        <Form form={manualForm} layout="vertical" onFinish={(values) => createMutation.mutate(values)}>
          <Form.Item name="platform" label="平台" rules={[{ required: true }]}>
            <Select options={PLATFORM_OPTIONS} placeholder="选择平台" />
          </Form.Item>
          <Form.Item name="account_name" label="账号名称" rules={[{ required: true }]}>
            <Input placeholder="例如：抖音工作号1" />
          </Form.Item>
          <Form.Item name="cookie" label="Cookie" rules={[{ required: true }]}>
            <Input.TextArea rows={4} placeholder="从浏览器开发者工具复制 Cookie（不推荐，推荐扫码登录）" />
          </Form.Item>
          <Form.Item name="user_agent" label="User-Agent">
            <Input placeholder="可选，留空使用默认值" />
          </Form.Item>
          <Form.Item name="proxy_address" label="代理地址">
            <Input placeholder="可选，例如 http://proxy:8080" />
          </Form.Item>
          <Form.Item name="daily_usage_limit" label="每日使用上限" initialValue={50}>
            <Input type="number" min={1} max={1000} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
