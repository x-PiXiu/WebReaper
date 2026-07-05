import { useState } from 'react'
import { Card, Table, Tag, Typography, Button, Modal, Form, Input, Select, Switch, Space, message, Alert } from 'antd'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import type { ExternalSystem } from '../types/api'

const { Title, Text } = Typography

// 可映射的本系统字段（提示用户 field_mapping 可填什么）
const AVAILABLE_SOURCE_FIELDS = [
  'title', 'content', 'summary', 'source_url', 'tags',
]

export default function ExternalSystems() {
  const queryClient = useQueryClient()
  const [modalOpen, setModalOpen] = useState(false)
  const [form] = Form.useForm()

  const { data: systems = [] } = useQuery({
    queryKey: ['external-systems'],
    queryFn: () => businessApi.listExternalSystems(),
  })

  const handleCreate = async (values: ExternalSystem & { enabled?: boolean }) => {
    try {
      await businessApi.createExternalSystem({
        name: values.name,
        description: values.description || '',
        endpoint: values.endpoint,
        method: values.method || 'POST',
        headers: values.headers || '{}',
        mode: values.mode || 'raw',
        field_mapping: values.field_mapping || '',
        body_template: values.body_template || '',
        content_type: values.content_type || '',
        enabled: values.enabled !== false,
      })
      message.success(`外部系统 ${values.name} 创建成功`)
      setModalOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['external-systems'] })
    } catch {}
  }

  const handleDelete = async (name: string) => {
    Modal.confirm({
      title: `删除外部系统「${name}」？`,
      content: '删除后，已推送的记录保留，但不能再向该系统推送。',
      okText: '删除', okType: 'danger', cancelText: '取消',
      onOk: async () => {
        try {
          await businessApi.deleteExternalSystem(name)
          message.success(`已删除 ${name}`)
          queryClient.invalidateQueries({ queryKey: ['external-systems'] })
        } catch {}
      },
    })
  }

  const columns = [
    {
      title: '名称', dataIndex: 'name', key: 'name', width: 160,
      render: (n: string) => <Text strong style={{ fontFamily: 'monospace' }}>{n}</Text>,
    },
    {
      title: '描述', dataIndex: 'description', key: 'description', ellipsis: true,
      render: (d: string) => <Text type="secondary">{d || '-'}</Text>,
    },
    {
      title: '端点', dataIndex: 'endpoint', key: 'endpoint', ellipsis: true,
      render: (e: string) => <Text code style={{ fontSize: 12 }}>{e}</Text>,
    },
    {
      title: '模式', dataIndex: 'mode', key: 'mode', width: 90,
      render: (m: string) => <Tag color={m === 'raw' ? 'cyan' : 'purple'}>{m === 'raw' ? '原样转发' : '字段映射'}</Tag>,
    },
    {
      title: '类型', dataIndex: 'content_type', key: 'content_type', width: 100,
      render: (c: string) => c ? <Tag color="blue">{c}</Tag> : <Text type="secondary">-</Text>,
    },
    {
      title: '启用', dataIndex: 'enabled', key: 'enabled', width: 60,
      render: (e: boolean) => <Tag color={e ? 'success' : 'default'}>{e ? '是' : '否'}</Tag>,
    },
    {
      title: '', key: 'action', width: 60,
      render: (_: unknown, r: ExternalSystem) => (
        <Button size="small" type="text" danger onClick={() => handleDelete(r.name)}>删除</Button>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div>
          <Title level={4} style={{ margin: 0 }}>外部系统</Title>
          <Text type="secondary" style={{ fontSize: 13 }}>
            管理数据推送目标系统。配置字段映射后，可把采集的数据推送到这些系统的 API。
          </Text>
        </div>
        <Button type="primary" onClick={() => setModalOpen(true)}>+ 新增系统</Button>
      </div>

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="字段映射说明"
        description={
          <div style={{ fontSize: 13 }}>
            <div>field_mapping 格式：<Text code>{'{"本系统字段":"目标字段"}'}</Text></div>
            <div style={{ marginTop: 4 }}>可用的本系统字段：
              {AVAILABLE_SOURCE_FIELDS.map(f => <Tag key={f} style={{ marginBottom: 2 }}>{f}</Tag>)}
            </div>
            <div style={{ marginTop: 4 }}>示例：<Text code>{'{"title":"title","content":"stem","summary":"answer_good"}'}</Text></div>
          </div>
        }
      />

      <Card>
        <Table dataSource={systems} columns={columns} rowKey="name" pagination={false} size="middle" />
      </Card>

      <Modal
        title="新增外部系统"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        footer={null}
        width={680}
      >
        <Form form={form} layout="vertical" onFinish={handleCreate} requiredMark={false}
          initialValues={{ method: 'POST', mode: 'raw', enabled: true, headers: '{"X-API-Key":"your-key","Content-Type":"application/json"}' }}>
          <Form.Item label="系统名称" name="name" rules={[{ required: true, message: '请输入名称' }]}
            tooltip="唯一标识，如 agentcore-question">
            <Input placeholder="如 agentcore-question" style={{ fontFamily: 'monospace' }} />
          </Form.Item>
          <Form.Item label="描述" name="description">
            <Input placeholder="如 AgentCore 面试题入库" />
          </Form.Item>
          <Form.Item label="API 端点" name="endpoint" rules={[{ required: true, message: '请输入端点' }]}>
            <Input placeholder="https://xxx.com/api/v1/ingest/question" />
          </Form.Item>
          <Form.Item label="HTTP 方法" name="method" style={{ width: 120 }}>
            <Input placeholder="POST" />
          </Form.Item>
          <Form.Item label="请求头（JSON）" name="headers"
            tooltip='认证信息放这里，如 {"X-API-Key":"xxx"}'>
            <Input.TextArea
              placeholder='{"X-API-Key":"your-key","Content-Type":"application/json"}'
              autoSize={{ minRows: 2, maxRows: 4 }}
              style={{ fontFamily: 'monospace', fontSize: 12 }}
            />
          </Form.Item>
          <Form.Item label="推送模式" name="mode"
            tooltip="raw=直接把数据内容作为请求体转发（推荐）；mapping=按字段映射转换">
            <Select options={[
              { value: 'raw', label: '原样转发（数据内容即请求体，推荐）' },
              { value: 'mapping', label: '字段映射（手动配置字段对应关系）' },
            ]} />
          </Form.Item>
          {/* 按模式动态显示不同输入 */}
          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.mode !== cur.mode}>
            {({ getFieldValue }) => {
              const mode = getFieldValue('mode') || 'raw'
              if (mode === 'raw') {
                return (
                  <Form.Item
                    label="请求体示例（可选）"
                    name="body_template"
                    tooltip="粘贴目标系统的请求体格式作为参考。实际推送时，数据内容会按此格式原样转发。建议在 Agent 提示词里要求 LLM 按此格式生成。"
                  >
                    <Input.TextArea
                      placeholder={'{\n  "title": "说说 Go 中 channel 的底层实现",\n  "stem": "请描述 channel 的底层数据结构...",\n  "answer_good": "...",\n  "tags": ["并发"]\n}'}
                      autoSize={{ minRows: 4, maxRows: 10 }}
                      style={{ fontFamily: 'monospace', fontSize: 12 }}
                    />
                  </Form.Item>
                )
              }
              return (
                <Form.Item label="字段映射（JSON）" name="field_mapping" rules={[{ required: true, message: '请输入字段映射' }]}
                  tooltip='格式：{"数据字段":"目标字段"}。数据字段可填 title/content/summary/tags'>
                  <Input.TextArea
                    placeholder='{"title":"title","content":"stem","summary":"answer_good"}'
                    autoSize={{ minRows: 3, maxRows: 6 }}
                    style={{ fontFamily: 'monospace', fontSize: 12 }}
                  />
                </Form.Item>
              )
            }}
          </Form.Item>
          <Form.Item label="数据类型标记" name="content_type"
            tooltip="标记本系统接收的数据类型，便于 UI 分组（可选）">
            <Input placeholder="如 question / article" />
          </Form.Item>
          <Form.Item label="启用" name="enabled" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">保存</Button>
              <Button onClick={() => setModalOpen(false)}>取消</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
