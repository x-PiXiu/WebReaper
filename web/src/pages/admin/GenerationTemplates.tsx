import { useState } from 'react'
import { Typography, Card, Space, Tag, Modal, Input, Table, Button, Form, Select, InputNumber, Switch } from 'antd'
import { ThunderboltOutlined, EditOutlined, PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { GenerationTemplate } from '../../types/api'
import { message, modal } from '../../utils/antdApp'

const { Text } = Typography
const { TextArea } = Input

const MATERIAL_LABELS: Record<string, string> = {
  image: '图片',
  video: '视频',
  audio: '音频',
  text: '文案',
  subject: '主体',
}

const SUB_TYPE_OPTIONS = [
  { value: 'text2video', label: '文生视频' },
  { value: 'img2video', label: '图生视频' },
  { value: 'start_end2video', label: '首尾帧视频' },
  { value: 'reference2video', label: '参考生视频' },
  { value: 'digital_human', label: '数字人口播' },
  { value: 'lip_sync', label: '对口型' },
  { value: 'tts', label: '语音合成' },
  { value: 'voice_clone', label: '声音复刻' },
  { value: 'text2image', label: '图片生成' },
  { value: 'text2audio', label: '文生音频' },
]

// 生成模板管理页面
export default function AdminGenerationTemplates() {
  const queryClient = useQueryClient()
  const [editingTemplate, setEditingTemplate] = useState<GenerationTemplate | null>(null)
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)
  const [form] = Form.useForm()

  // 查询模板列表
  const { data: templates = [], isLoading } = useQuery({
    queryKey: ['admin-generation-templates'],
    queryFn: () => businessApi.adminListGenerationTemplates(),
  })

  // 创建模板
  const createMutation = useMutation({
    mutationFn: (data: Partial<GenerationTemplate>) => businessApi.adminCreateGenerationTemplate(data),
    onSuccess: () => {
      message.success('模板创建成功')
      setIsCreateModalOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['admin-generation-templates'] })
    },
    onError: () => message.error('创建失败'),
  })

  // 更新模板
  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<GenerationTemplate> }) =>
      businessApi.adminUpdateGenerationTemplate(id, data),
    onSuccess: () => {
      message.success('模板更新成功')
      setEditingTemplate(null)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['admin-generation-templates'] })
    },
    onError: () => message.error('更新失败'),
  })

  // 删除模板
  const deleteMutation = useMutation({
    mutationFn: (id: string) => businessApi.adminDeleteGenerationTemplate(id),
    onSuccess: () => {
      message.success('模板删除成功')
      queryClient.invalidateQueries({ queryKey: ['admin-generation-templates'] })
    },
    onError: () => message.error('删除失败'),
  })

  // 表格列定义
  const columns = [
    {
      title: '图标',
      dataIndex: 'icon',
      key: 'icon',
      width: 60,
      render: (icon: string) => <span style={{ fontSize: 24 }}>{icon}</span>,
    },
    {
      title: '模板名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: GenerationTemplate) => (
        <Space direction="vertical" size={0}>
          <Text strong>{name}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>{record.description}</Text>
        </Space>
      ),
    },
    {
      title: '端点类型',
      dataIndex: 'sub_type',
      key: 'sub_type',
      width: 120,
      render: (subType: string) => {
        const option = SUB_TYPE_OPTIONS.find(o => o.value === subType)
        return <Tag color="blue">{option?.label || subType}</Tag>
      },
    },
    {
      title: '必需素材',
      dataIndex: 'required_materials',
      key: 'required_materials',
      width: 120,
      render: (materials: string[]) => (
        <Space size={4} wrap>
          {materials?.map(m => (
            <Tag key={m} color={m === 'image' ? 'green' : m === 'video' ? 'orange' : m === 'audio' ? 'purple' : 'default'}>
              {MATERIAL_LABELS[m] || m}
            </Tag>
          ))}
          {!materials?.length && <Text type="secondary" style={{ fontSize: 12 }}>仅文案</Text>}
        </Space>
      ),
    },
    {
      title: '默认参数',
      dataIndex: 'default_params',
      key: 'default_params',
      width: 200,
      render: (params: Record<string, unknown>) => (
        <Text type="secondary" style={{ fontSize: 12 }}>
          {JSON.stringify(params)}
        </Text>
      ),
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'success' : 'default'}>
          {enabled ? '启用' : '禁用'}
        </Tag>
      ),
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_: unknown, record: GenerationTemplate) => (
        <Space>
          <Button
            size="small"
            type="link"
            icon={<EditOutlined />}
            onClick={() => {
              setEditingTemplate(record)
              form.setFieldsValue(record)
            }}
          >
            编辑
          </Button>
          <Button
            size="small"
            type="link"
            danger
            icon={<DeleteOutlined />}
            onClick={() => {
              modal.confirm({
                centered: true,
                title: '确认删除',
                content: `确定要删除模板"${record.name}"吗？`,
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

  // 表单提交
  const handleSubmit = () => {
    form.validateFields().then(values => {
      if (editingTemplate) {
        updateMutation.mutate({ id: editingTemplate.id, data: values })
      } else {
        createMutation.mutate(values)
      }
    })
  }

  return (
    <div className="wr-page-content">
      <div className="wr-page-header">
        <h1>生成模板管理</h1>
        <p>管理生成模板，配置默认参数和必需素材——用户端选择模板后自动填充</p>
      </div>

      <Card
        className="wr-glass-card"
        title={
          <Space>
            <ThunderboltOutlined />
            <Text strong>生成模板列表</Text>
          </Space>
        }
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditingTemplate(null)
              form.resetFields()
              setIsCreateModalOpen(true)
            }}
          >
            新建模板
          </Button>
        }
      >
        <Table
          dataSource={templates}
          columns={columns}
          rowKey="id"
          loading={isLoading}
          pagination={false}
          size="small"
        />
      </Card>

      {/* 创建/编辑弹窗 */}
      <Modal
        title={editingTemplate ? '编辑模板' : '新建模板'}
        open={isCreateModalOpen || !!editingTemplate}
        onCancel={() => {
          setIsCreateModalOpen(false)
          setEditingTemplate(null)
          form.resetFields()
        }}
        width={680}
        footer={
          <Space>
            <Button onClick={() => {
              setIsCreateModalOpen(false)
              setEditingTemplate(null)
              form.resetFields()
            }}>
              取消
            </Button>
            <Button
              type="primary"
              loading={createMutation.isPending || updateMutation.isPending}
              onClick={handleSubmit}
            >
              {editingTemplate ? '保存' : '创建'}
            </Button>
          </Space>
        }
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="id"
            label="模板ID"
            rules={[{ required: true, message: '请输入模板ID' }]}
          >
            <Input placeholder="如：brand_promo" disabled={!!editingTemplate} />
          </Form.Item>

          <Form.Item
            name="name"
            label="模板名称"
            rules={[{ required: true, message: '请输入模板名称' }]}
          >
            <Input placeholder="如：品牌宣传视频" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <TextArea placeholder="模板描述" autoSize={{ minRows: 2, maxRows: 4 }} />
          </Form.Item>

          <Form.Item name="icon" label="图标">
            <Input placeholder="如：🎬" />
          </Form.Item>

          <Form.Item
            name="sub_type"
            label="端点类型"
            rules={[{ required: true, message: '请选择端点类型' }]}
          >
            <Select options={SUB_TYPE_OPTIONS} placeholder="选择端点类型" />
          </Form.Item>

          <Form.Item name="required_materials" label="必需素材">
            <Select mode="multiple" placeholder="选择必需素材类型">
              <Select.Option value="image">图片</Select.Option>
              <Select.Option value="video">视频</Select.Option>
              <Select.Option value="audio">音频</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item name="optional_materials" label="可选素材">
            <Select mode="multiple" placeholder="选择可选素材类型">
              <Select.Option value="image">图片</Select.Option>
              <Select.Option value="video">视频</Select.Option>
              <Select.Option value="audio">音频</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item name="sort_order" label="排序">
            <InputNumber min={0} max={999} placeholder="排序值" />
          </Form.Item>

          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
