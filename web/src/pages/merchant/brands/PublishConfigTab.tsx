import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Button, Form, Input, InputNumber, Select, Space, Switch, Table, Tag, Typography, message, Modal, Popconfirm,
} from 'antd'
import {
  PlusOutlined, DeleteOutlined, EditOutlined, LinkOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import type { BrandPublishConfig } from '../../../types/api'

const { Text } = Typography

interface Props {
  brandId: string
}

const PLATFORMS = [
  { value: 'douyin', label: '抖音' },
  { value: 'kuaishou', label: '快手' },
  { value: 'xiaohongshu', label: '小红书' },
  { value: 'weixin', label: '视频号' },
  { value: 'bilibili', label: 'B站' },
]

/**
 * 品牌发布配置 Tab
 * 管理品牌在各平台的发布配置（限速、默认标签、账号绑定）
 */
export default function PublishConfigTab({ brandId }: Props) {
  const queryClient = useQueryClient()
  const [editModalVisible, setEditModalVisible] = useState(false)
  const [bindModalVisible, setBindModalVisible] = useState(false)
  const [editingConfig, setEditingConfig] = useState<BrandPublishConfig | null>(null)
  const [selectedPlatform, setSelectedPlatform] = useState<string>('')
  const [form] = Form.useForm()
  const [bindForm] = Form.useForm()

  // 获取品牌发布配置
  const { data: configs = [], isLoading } = useQuery({
    queryKey: ['brand-publish-configs', brandId],
    queryFn: () => businessApi.getBrandPublishConfigs(brandId),
    enabled: !!brandId,
  })

  // 获取账号列表（用于绑定）
  const { data: accounts = [] } = useQuery({
    queryKey: ['accounts'],
    queryFn: () => businessApi.listAccounts(),
  })

  // 获取发布统计
  const { data: stats } = useQuery({
    queryKey: ['publish-stats', brandId],
    queryFn: () => businessApi.getPublishStats(brandId),
    enabled: !!brandId,
  })

  // 保存配置
  const saveMutation = useMutation({
    mutationFn: (config: Partial<BrandPublishConfig>) =>
      businessApi.updateBrandPublishConfig(brandId, config),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['brand-publish-configs', brandId] })
      message.success('配置已保存')
      setEditModalVisible(false)
      form.resetFields()
    },
    onError: () => message.error('保存失败'),
  })

  // 删除配置
  const deleteMutation = useMutation({
    mutationFn: (platform: string) =>
      businessApi.deleteBrandPublishConfig(brandId, platform),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['brand-publish-configs', brandId] })
      message.success('配置已删除')
    },
    onError: () => message.error('删除失败'),
  })

  // 绑定账号
  const bindMutation = useMutation({
    mutationFn: (data: { account_id: string; platform: string; is_default: boolean }) =>
      businessApi.bindAccountToBrand(brandId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['brand-publish-configs', brandId] })
      message.success('账号绑定成功')
      setBindModalVisible(false)
      bindForm.resetFields()
    },
    onError: () => message.error('绑定失败'),
  })

  // 打开编辑弹窗
  const handleEdit = (config?: BrandPublishConfig) => {
    if (config) {
      setEditingConfig(config)
      form.setFieldsValue({
        platform: config.platform,
        max_per_day: config.rate_limit?.max_per_day || 5,
        max_per_hour: config.rate_limit?.max_per_hour || 2,
        min_interval: config.rate_limit?.min_interval || 1800,
        default_tags: config.default_tags?.join(', ') || '',
        default_persona: config.default_persona || '',
        is_active: config.is_active,
      })
    } else {
      setEditingConfig(null)
      form.setFieldsValue({
        max_per_day: 5,
        max_per_hour: 2,
        min_interval: 1800,
        is_active: true,
      })
    }
    setEditModalVisible(true)
  }

  // 提交配置
  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      const config: Partial<BrandPublishConfig> = {
        platform: values.platform,
        rate_limit: {
          max_per_day: values.max_per_day,
          max_per_hour: values.max_per_hour,
          min_interval: values.min_interval,
        },
        default_tags: values.default_tags ? values.default_tags.split(',').map((t: string) => t.trim()).filter(Boolean) : [],
        default_persona: values.default_persona || undefined,
        is_active: values.is_active,
      }
      saveMutation.mutate(config)
    } catch {
      // validation failed
    }
  }

  // 打开绑定弹窗
  const handleBind = (platform: string) => {
    setSelectedPlatform(platform)
    bindForm.setFieldsValue({ platform, is_default: false })
    setBindModalVisible(true)
  }

  // 提交绑定
  const handleBindSubmit = async () => {
    try {
      const values = await bindForm.validateFields()
      bindMutation.mutate({
        account_id: values.account_id,
        platform: values.platform,
        is_default: values.is_default,
      })
    } catch {
      // validation failed
    }
  }

  // 表格列定义
  const columns = [
    {
      title: '平台',
      dataIndex: 'platform',
      key: 'platform',
      render: (platform: string) => {
        const p = PLATFORMS.find(p => p.value === platform)
        return <Tag>{p?.label || platform}</Tag>
      },
    },
    {
      title: '状态',
      dataIndex: 'is_active',
      key: 'is_active',
      render: (active: boolean) => (
        <Tag color={active ? 'green' : 'default'}>{active ? '启用' : '禁用'}</Tag>
      ),
    },
    {
      title: '每日限制',
      key: 'rate_limit',
      render: (_: unknown, record: BrandPublishConfig) => (
        <Text>{record.rate_limit?.max_per_day || '-'} 次/天</Text>
      ),
    },
    {
      title: '最小间隔',
      key: 'min_interval',
      render: (_: unknown, record: BrandPublishConfig) => {
        const interval = record.rate_limit?.min_interval
        if (!interval) return <Text>-</Text>
        return <Text>{Math.floor(interval / 60)} 分钟</Text>
      },
    },
    {
      title: '默认标签',
      key: 'default_tags',
      render: (_: unknown, record: BrandPublishConfig) => (
        <Space size={4} wrap>
          {record.default_tags?.map((tag: string, i: number) => (
            <Tag key={i}>{tag}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: '今日已发',
      key: 'usage',
      render: (_: unknown, record: BrandPublishConfig) => {
        const usage = stats?.daily_usage?.[record.platform] || 0
        const limit = record.rate_limit?.max_per_day || 0
        return <Text>{usage}/{limit}</Text>
      },
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: unknown, record: BrandPublishConfig) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
            编辑
          </Button>
          <Button size="small" icon={<LinkOutlined />} onClick={() => handleBind(record.platform)}>
            绑定账号
          </Button>
          <Popconfirm title="确定删除此配置？" onConfirm={() => deleteMutation.mutate(record.platform)}>
            <Button size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  // 过滤可用平台（未配置的）
  const availablePlatforms = PLATFORMS.filter(
    p => !configs.some(c => c.platform === p.value)
  )

  return (
    <div className="publish-config-tab">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong>发布配置</Text>
        <Space>
          {availablePlatforms.length > 0 && (
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => handleEdit()}
            >
              添加平台配置
            </Button>
          )}
        </Space>
      </div>

      <Table
        columns={columns}
        dataSource={configs}
        rowKey="platform"
        loading={isLoading}
        pagination={false}
        locale={{ emptyText: '暂无发布配置，点击"添加平台配置"开始' }}
      />

      {/* 编辑配置弹窗 */}
      <Modal
        title={editingConfig ? '编辑发布配置' : '添加发布配置'}
        open={editModalVisible}
        onOk={handleSubmit}
        onCancel={() => setEditModalVisible(false)}
        confirmLoading={saveMutation.isPending}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="platform" label="平台" rules={[{ required: true }]}>
            <Select
              options={editingConfig ? [{ value: editingConfig.platform, label: PLATFORMS.find(p => p.value === editingConfig.platform)?.label }] : availablePlatforms}
              disabled={!!editingConfig}
            />
          </Form.Item>
          <Form.Item name="is_active" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="max_per_day" label="每日最大发布数">
            <InputNumber min={1} max={100} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="max_per_hour" label="每小时最大发布数">
            <InputNumber min={1} max={50} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="min_interval" label="最小间隔（秒）">
            <InputNumber min={60} max={86400} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="default_tags" label="默认标签（逗号分隔）">
            <Input placeholder="#推荐, #种草, #好物" />
          </Form.Item>
          <Form.Item name="default_persona" label="默认人设">
            <Input placeholder="人设ID（可选）" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 绑定账号弹窗 */}
      <Modal
        title="绑定账号到品牌"
        open={bindModalVisible}
        onOk={handleBindSubmit}
        onCancel={() => setBindModalVisible(false)}
        confirmLoading={bindMutation.isPending}
      >
        <Form form={bindForm} layout="vertical">
          <Form.Item name="platform" label="平台">
            <Select disabled options={PLATFORMS} />
          </Form.Item>
          <Form.Item name="account_id" label="选择账号" rules={[{ required: true }]}>
            <Select
              showSearch
              placeholder="选择要绑定的账号"
              options={accounts
                .filter(a => a.platform === selectedPlatform)
                .map(a => ({ value: a.id, label: `${a.display_name} (${a.platform})` }))}
              filterOption={(input, option) =>
                (option?.label as string)?.toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item name="is_default" label="设为默认账号" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
