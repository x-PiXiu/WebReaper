import { useState } from 'react'
import { Card, Table, Tag, Typography, Button, Modal, Form, Input, InputNumber, Select, Space, Switch, Tabs } from 'antd'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { AgentConfig, LLMConfig, ToolView } from '../../types/api'
import { message, modal } from '../../utils/antdApp'

const { Text } = Typography

export default function AgentConfigs({ embedded: _embedded = false }: { embedded?: boolean }) {
  void _embedded
  const queryClient = useQueryClient()
  const [agentModalOpen, setAgentModalOpen] = useState(false)
  const [llmModalOpen, setLlmModalOpen] = useState(false)
  // 编辑模式：记录正在编辑的配置名（null = 新建模式）
  const [editingAgent, setEditingAgent] = useState<string | null>(null)
  const [editingLLM, setEditingLLM] = useState<string | null>(null)
  const [agentForm] = Form.useForm()
  const [llmForm] = Form.useForm()

  const { data: configs = [] } = useQuery({
    queryKey: ['agent-configs'],
    queryFn: () => businessApi.listAgentConfigs(),
  })
  // LLM 配置列表：用于 Agent 表单下拉 + LLM 管理表格
  const { data: llmConfigs = [] } = useQuery({
    queryKey: ['llm-configs'],
    queryFn: () => businessApi.listLLMConfigs(),
  })

  const invalidateAll = () => {
    queryClient.invalidateQueries({ queryKey: ['agent-configs'] })
    queryClient.invalidateQueries({ queryKey: ['llm-configs'] })
  }

  // ---- Agent 配置 CRUD ----
  // 新建/编辑统一入口：editingAgent 为 null 时是新建，否则是编辑该 name
  const openEditAgent = (record: { name: string; system_prompt: string; llm_config_name?: string; max_iterations?: number }) => {
    setEditingAgent(record.name)
    agentForm.setFieldsValue({
      name: record.name,
      system_prompt: record.system_prompt,
      llm_config_name: record.llm_config_name || undefined,
    })
    setAgentModalOpen(true)
  }

  const openCreateAgent = () => {
    setEditingAgent(null)
    agentForm.resetFields()
    setAgentModalOpen(true)
  }

  const handleSubmitAgent = async (values: { name: string; system_prompt: string; llm_config_name?: string; max_iterations?: number }) => {
    try {
      if (editingAgent) {
        // 编辑模式：部分更新（name 不可改）
        await businessApi.updateAgentConfig(editingAgent, {
          system_prompt: values.system_prompt,
          llm_config_name: values.llm_config_name || '',
        })
        message.success(`Agent ${editingAgent} 已更新`)
      } else {
        await businessApi.createAgentConfig({
          name: values.name,
          system_prompt: values.system_prompt,
          tools: [],
          llm_config_name: values.llm_config_name || '',
          max_iterations: 10,
        })
        message.success(`Agent ${values.name} 创建成功`)
      }
      setAgentModalOpen(false)
      agentForm.resetFields()
      setEditingAgent(null)
      invalidateAll()
    } catch {}
  }

  const handleDeleteAgent = async (name: string) => {
    modal.confirm({
      centered: true,
      title: `删除 Agent「${name}」？`,
      content: '删除后，使用该 Agent 的历史会话不受影响，但新对话将不再显示此角色。',
      okText: '删除', okType: 'danger', cancelText: '取消',
      onOk: async () => {
        try {
          await businessApi.deleteAgentConfig(name)
          message.success(`已删除 ${name}`)
          invalidateAll()
        } catch {}
      },
    })
  }

  // ---- LLM 配置 CRUD ----
  // 新建/编辑统一入口
  const openEditLLM = (record: LLMConfig) => {
    setEditingLLM(record.name)
    llmForm.setFieldsValue({
      name: record.name,
      provider: record.provider,
      usage: record.usage || '',
      api_key: record.api_key,
      base_url: record.base_url,
      model: record.model,
      cost_per_mtok: record.cost_per_mtok,
    })
    setLlmModalOpen(true)
  }

  const openCreateLLM = () => {
    setEditingLLM(null)
    llmForm.resetFields()
    setLlmModalOpen(true)
  }

  const handleSubmitLLM = async (values: LLMConfig) => {
    try {
      if (editingLLM) {
        // 编辑模式：部分更新（name 不可改）
        await businessApi.updateLLMConfig(editingLLM, {
          provider: values.provider,
          usage: values.usage || '',
          api_key: values.api_key,
          base_url: values.base_url,
          model: values.model,
          cost_per_mtok: values.cost_per_mtok,
        })
        message.success(`LLM 配置 ${editingLLM} 已更新`)
      } else {
        await businessApi.createLLMConfig(values)
        message.success(`LLM 配置 ${values.name} 创建成功`)
      }
      setLlmModalOpen(false)
      llmForm.resetFields()
      setEditingLLM(null)
      invalidateAll()
    } catch {}
  }

  const handleDeleteLLM = async (name: string) => {
    modal.confirm({
      centered: true,
      title: `删除 LLM 配置「${name}」？`,
      content: '引用此配置的 Agent 将回退到默认 LLM。',
      okText: '删除', okType: 'danger', cancelText: '取消',
      onOk: async () => {
        try {
          await businessApi.deleteLLMConfig(name)
          message.success(`已删除 ${name}`)
          invalidateAll()
        } catch {}
      },
    })
  }

  // LLM 下拉选项
  const llmOptions = llmConfigs.map((l) => ({
    value: l.name,
    label: `${l.name}（${l.provider || '未知'} · ${l.model}）`,
  }))

  // 找到 agent 引用的 LLM 配置（用于表格展示模型名）
  const llmMap = new Map(llmConfigs.map((l) => [l.name, l]))

  const agentColumns = [
    {
      title: '名称', dataIndex: 'name', key: 'name',
      render: (name: string) => <Text strong style={{ fontFamily: 'monospace' }}>{name}</Text>,
    },
    {
      title: '系统提示词', dataIndex: 'system_prompt', key: 'system_prompt', ellipsis: true,
      render: (p: string) => <Text type="secondary">{p}</Text>,
    },
    {
      title: 'LLM 配置', dataIndex: 'llm_config_name', key: 'llm_config_name', width: 220,
      render: (name: string) => {
        if (!name) return <Tag>默认</Tag>
        const cfg = llmMap.get(name)
        return <Tag color="blue">{name}{cfg ? ` · ${cfg.model}` : ''}</Tag>
      },
    },
    {
      title: '', key: 'action', width: 110,
      render: (_: unknown, record: AgentConfig) => (
        <Space size="small">
          <Button size="small" type="link" style={{ padding: 0 }} onClick={() => openEditAgent(record)}>编辑</Button>
          <Button size="small" type="text" danger onClick={() => handleDeleteAgent(record.name)}>删除</Button>
        </Space>
      ),
    },
  ]

  const llmColumns = [
    {
      title: '名称', dataIndex: 'name', key: 'name', width: 140,
      render: (name: string) => <Text strong style={{ fontFamily: 'monospace' }}>{name}</Text>,
    },
    {
      title: '厂商', dataIndex: 'provider', key: 'provider', width: 100,
      render: (p: string) => <Tag>{p || '未知'}</Tag>,
    },
    {
      title: '用途', dataIndex: 'usage', key: 'usage', width: 100,
      render: (u: string) => u === 'vision' ? <Tag color="purple">视觉</Tag> : <Tag>聊天</Tag>,
    },
    {
      title: '模型', dataIndex: 'model', key: 'model', width: 160,
      render: (m: string) => <Text code>{m}</Text>,
    },
    {
      title: 'API 端点', dataIndex: 'base_url', key: 'base_url', ellipsis: true,
      render: (u: string) => <Text type="secondary" style={{ fontSize: 12 }}>{u}</Text>,
    },
    {
      title: 'API Key', dataIndex: 'api_key', key: 'api_key', width: 160, ellipsis: true,
      render: (k: string) => <Text type="secondary" style={{ fontSize: 12, fontFamily: 'monospace' }}>{k}</Text>,
    },
    {
      title: '', key: 'action', width: 110,
      render: (_: unknown, record: LLMConfig) => (
        <Space size="small">
          <Button size="small" type="link" style={{ padding: 0 }} onClick={() => openEditLLM(record)}>编辑</Button>
          <Button size="small" type="text" danger onClick={() => handleDeleteLLM(record.name)}>删除</Button>
        </Space>
      ),
    },
  ]

  return (
    <div>
      {/* 标题区（统一 wr-page-header 规范） */}
      <div className="wr-page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h1>Agent 配置</h1>
          <p>管理 Agent、LLM 配置与工具启停；工具已全局可用，无需为 Agent 单独配置</p>
        </div>
        <Space>
          <Button onClick={openCreateLLM}>新建 LLM 配置</Button>
          <Button type="primary" onClick={openCreateAgent}>创建 Agent</Button>
        </Space>
      </div>

      <Tabs
        defaultActiveKey="agents"
        items={[
          {
            key: 'agents',
            label: 'Agent 列表',
            children: (
              <Card>
                <Table dataSource={configs} columns={agentColumns} rowKey="name" pagination={false} size="middle" />
                {configs.length === 0 && (
                  <div style={{ textAlign: 'center', padding: 48 }}>
                    <Text type="secondary" style={{ fontSize: 14 }}>
                      暂无 Agent 配置。点击「创建 Agent」添加第一个。
                    </Text>
                  </div>
                )}
              </Card>
            ),
          },
          {
            key: 'llm',
            label: 'LLM 配置',
            children: (
              <Card>
                <Table dataSource={llmConfigs} columns={llmColumns} rowKey="name" pagination={false} size="middle" />
                {llmConfigs.length === 0 && (
                  <div style={{ textAlign: 'center', padding: 48 }}>
                    <Text type="secondary" style={{ fontSize: 14 }}>
                      暂无 LLM 配置。点击「新建 LLM 配置」添加厂商/模型（首次启动会自动 seed 一个 default）。
                    </Text>
                  </div>
                )}
              </Card>
            ),
          },
          {
            key: 'tools',
            label: '工具面板',
            children: <ToolsSection />,
          },
        ]}
      />

      {/* 创建/编辑 Agent 弹窗 */}
      <Modal
        title={editingAgent ? `编辑 Agent「${editingAgent}」` : '创建 Agent 配置'}
        open={agentModalOpen}
        onCancel={() => { setAgentModalOpen(false); setEditingAgent(null); agentForm.resetFields() }}
        footer={null}
        width={640}
      >
        <Form form={agentForm} layout="vertical" onFinish={handleSubmitAgent} requiredMark={false}>
          <Form.Item label="Agent 名称" name="name" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="如 tech-blog-agent" style={{ fontFamily: 'monospace' }} disabled={!!editingAgent} />
          </Form.Item>
          <Form.Item label="系统提示词" name="system_prompt" rules={[{ required: true, message: '请输入提示词' }]}>
            <Input.TextArea
              placeholder="定义这个 Agent 的角色和目标，例如：你是一个技术文章采集助手..."
              autoSize={{ minRows: 3, maxRows: 6 }}
            />
          </Form.Item>
          <Form.Item
            label="LLM 配置"
            name="llm_config_name"
            tooltip="选择此 Agent 使用的 LLM（厂商/模型）。留空使用默认配置。"
          >
            <Select
              allowClear
              placeholder="留空使用默认 LLM 配置"
              options={llmOptions}
            />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">{editingAgent ? '保存修改' : '保存配置'}</Button>
              <Button onClick={() => { setAgentModalOpen(false); setEditingAgent(null); agentForm.resetFields() }}>取消</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 新建/编辑 LLM 配置弹窗 */}
      <Modal
        title={editingLLM ? `编辑 LLM 配置「${editingLLM}」` : '新建 LLM 配置'}
        open={llmModalOpen}
        onCancel={() => { setLlmModalOpen(false); setEditingLLM(null); llmForm.resetFields() }}
        footer={null}
        width={600}
      >
        <div style={{ marginBottom: 12 }}>
          <Text type="secondary" style={{ fontSize: 13 }}>
            {editingLLM
              ? '修改 LLM 配置。留空的字段保持原值不变（API Key 留空则不修改）。配置生效有最多 30 秒缓存延迟。'
              : '配置一个 LLM 厂商/模型。所有厂商均使用 OpenAI 兼容协议，只需填对 BaseURL 和 API Key。'}
          </Text>
        </div>
        <Form form={llmForm} layout="vertical" onFinish={handleSubmitLLM} requiredMark={false}>
          <Form.Item label="配置名称" name="name" rules={[{ required: true, message: '请输入名称' }]}
            tooltip="唯一标识，如 default、minimax-m2、deepseek-chat">
            <Input placeholder="如 deepseek-chat" style={{ fontFamily: 'monospace' }} disabled={!!editingLLM} />
          </Form.Item>
          <Form.Item label="厂商" name="provider"
            tooltip="仅作展示标签，不影响协议（统一 OpenAI 兼容）">
            <Select
              placeholder="选择厂商"
              options={[
                { value: 'minimax', label: 'MiniMax' },
                { value: 'openai', label: 'OpenAI' },
                { value: 'deepseek', label: 'DeepSeek' },
                { value: 'zhipu', label: '智谱 (Zhipu)' },
                { value: 'qwen', label: '通义千问 (Qwen)' },
                { value: 'other', label: '其他' },
              ]}
            />
          </Form.Item>
          <Form.Item label="用途" name="usage"
            tooltip="区分模型用途：聊天/内容模型（默认）用于内容生成/问答；视觉模型用于浏览器截图分析（Agent 智能化）">
            <Select
              placeholder="选择用途（默认聊天）"
              allowClear
              options={[
                { value: '', label: '聊天/内容模型（默认）' },
                { value: 'vision', label: '视觉模型（浏览器截图分析）' },
              ]}
            />
          </Form.Item>
          <Form.Item label="API Key" name="api_key" rules={editingLLM ? [] : [{ required: true, message: '请输入 API Key' }]}
            tooltip={editingLLM ? '留空则不修改原 Key' : undefined}>
            <Input.Password placeholder={editingLLM ? '留空则不修改' : 'sk-...'} style={{ fontFamily: 'monospace' }} />
          </Form.Item>
          <Form.Item label="Base URL" name="base_url"
            tooltip="OpenAI 兼容的 API 端点">
            <Input placeholder="如 https://api.minimaxi.com/v1" />
          </Form.Item>
          <Form.Item label="模型名" name="model" rules={editingLLM ? [] : [{ required: true, message: '请输入模型名' }]}>
            <Input placeholder={editingLLM ? '留空则不修改' : '如 MiniMax-M2.5、deepseek-chat、gpt-4o-mini'} style={{ fontFamily: 'monospace' }} />
          </Form.Item>
          <Form.Item label="每百万 tokens 成本（分）" name="cost_per_mtok"
            tooltip="成本分析按引擎细分的单价（默认 100 = ¥1/百万 tokens；豆包/DeepSeek 约 20，GPT 级约 300）">
            <InputNumber min={1} max={10000} style={{ width: '100%' }} placeholder="如 100" />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">{editingLLM ? '保存修改' : '保存配置'}</Button>
              <Button onClick={() => { setLlmModalOpen(false); setEditingLLM(null); llmForm.resetFields() }}>取消</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

// 工具面板（Tab）：动态控制哪些工具可用/禁用。
// 禁用的工具不会被 Agent 拿到（后端 ToolRegistry.All/GetByNames 自动过滤）。
function ToolsSection() {
  const queryClient = useQueryClient()
  const { data: tools = [] } = useQuery({
    queryKey: ['tools'],
    queryFn: () => businessApi.listTools(),
  })

  const handleToggle = async (name: string, enabled: boolean) => {
    try {
      await businessApi.toggleTool(name, enabled)
      message.success(`${enabled ? '启用' : '禁用'}工具：${name}`)
      queryClient.invalidateQueries({ queryKey: ['tools'] })
    } catch {
      // axios 拦截器已提示
    }
  }

  const columns = [
    {
      title: '工具名', dataIndex: 'name', key: 'name', width: 220,
      render: (name: string) => <Text code>{name}</Text>,
    },
    {
      title: '说明', dataIndex: 'description', key: 'description',
      ellipsis: true,
    },
    {
      title: '状态', dataIndex: 'enabled', key: 'enabled', width: 120,
      render: (enabled: boolean, record: ToolView) => (
        <Space>
          <Tag color={enabled ? 'green' : 'default'}>{enabled ? '启用' : '禁用'}</Tag>
          <Switch
            checked={enabled}
            size="small"
            onChange={(checked) => handleToggle(record.name, checked)}
          />
        </Space>
      ),
    },
  ]

  const enabledCount = tools.filter((t: ToolView) => t.enabled).length

  return (
    <div>
      <Text type="secondary">
        动态控制 Agent 可用的工具。禁用的工具不会被 Agent 调用（如禁用 generate_content 则 Agent 不能生成结构化内容）。
        当前 {enabledCount}/{tools.length} 个工具启用。
      </Text>
      <Table
        style={{ marginTop: 16 }}
        dataSource={tools}
        columns={columns}
        rowKey="name"
        pagination={false}
        size="middle"
      />
    </div>
  )
}
