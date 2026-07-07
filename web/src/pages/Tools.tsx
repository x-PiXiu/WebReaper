import { Card, Table, Tag, Typography, Switch, message, Space } from 'antd'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import type { ToolView } from '../types/api'

const { Title, Text } = Typography

// 工具面板：动态控制哪些工具可用/禁用。
// 禁用的工具不会被 Agent 拿到（后端 ToolRegistry.All/GetByNames 自动过滤）。
export default function Tools() {
  const queryClient = useQueryClient()
  const { data: tools = [], refetch } = useQuery({
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
      <Title level={4}>工具面板</Title>
      <Text type='secondary'>
        动态控制 Agent 可用的工具。禁用的工具不会被 Agent 调用（如禁用 generate_content 则 Agent 不能生成结构化内容）。
        当前 {enabledCount}/{tools.length} 个工具启用。
      </Text>
      <Card style={{ marginTop: 16 }}>
        <Table
          dataSource={tools}
          columns={columns}
          rowKey='name'
          pagination={false}
          size='middle'
        />
      </Card>
    </div>
  )
}
