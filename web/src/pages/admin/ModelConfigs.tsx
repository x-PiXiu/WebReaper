import { Typography, Card, Space, Tag, Table, Button, Switch, Collapse } from 'antd'
import { SettingOutlined, CheckCircleFilled } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { GenerationSpec } from '../../types/api'
import QueryBoundary from '../../components/QueryBoundary'
import { message } from '../../utils/antdApp'

const { Text } = Typography

// 端点类型中文名
const SUB_TYPE_LABELS: Record<string, string> = {
  text2video: '文生视频',
  img2video: '图生视频',
  start_end2video: '首尾帧视频',
  reference2video: '参考生视频',
  multiframe: '智能多帧',
  digital_human: '数字人口播',
  lip_sync: '对口型',
  tts: '语音合成',
  voice_clone: '声音复刻',
  text2image: '图片生成',
  text2audio: '文生音频',
  sound_effect: '音效生成',
  subject: '主体创建',
}

// 模型配置管理页面（按厂商分组 → 端点分组 → 模型列表）
export default function AdminModelConfigs() {
  const queryClient = useQueryClient()

  // 查询所有模型配置
  const { data: specs = [], isLoading, isError, refetch } = useQuery({
    queryKey: ['admin-generation-specs'],
    queryFn: () => businessApi.adminListGenerationSpecs(),
  })

  // 设置默认模型
  const setDefaultMutation = useMutation({
    mutationFn: ({ subType, model, provider }: { subType: string; model: string; provider?: string }) =>
      businessApi.adminSetDefaultModel(subType, model, provider),
    onSuccess: () => {
      message.success('默认模型已更新')
      queryClient.invalidateQueries({ queryKey: ['admin-generation-specs'] })
    },
    onError: () => message.error('设置失败'),
  })

  // 切换启用状态
  const toggleEnabledMutation = useMutation({
    mutationFn: ({ subType, model, enabled }: { subType: string; model: string; enabled: boolean }) =>
      businessApi.adminSaveGenerationSpec(subType, model, { enabled }),
    onSuccess: () => {
      message.success('状态已更新')
      queryClient.invalidateQueries({ queryKey: ['admin-generation-specs'] })
    },
    onError: () => message.error('更新失败'),
  })

  // 按厂商分组
  const groupedByProvider = specs.reduce<Record<string, GenerationSpec[]>>((acc, spec) => {
    const provider = spec.provider || 'vidu'
    if (!acc[provider]) acc[provider] = []
    acc[provider].push(spec)
    return acc
  }, {})

  // 模型列表表格列
  const modelColumns = [
    {
      title: '模型',
      dataIndex: 'model',
      key: 'model',
      render: (model: string, record: GenerationSpec) => (
        <Space>
          {record.is_default && <CheckCircleFilled style={{ color: '#52c41a' }} />}
          <Text strong={record.is_default}>{model}</Text>
        </Space>
      ),
    },
    {
      title: '端点',
      dataIndex: 'sub_type',
      key: 'sub_type',
      width: 150,
      render: (subType: string) => (
        <Tag color="blue">{SUB_TYPE_LABELS[subType] || subType}</Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (enabled: boolean, record: GenerationSpec) => (
        <Switch
          checked={enabled}
          size="small"
          onChange={(checked) => toggleEnabledMutation.mutate({
            subType: record.sub_type,
            model: record.model,
            enabled: checked,
          })}
        />
      ),
    },
    {
      title: '默认',
      dataIndex: 'is_default',
      key: 'is_default',
      width: 80,
      render: (isDefault: boolean, record: GenerationSpec) => (
        <Button
          size="small"
          type={isDefault ? 'primary' : 'default'}
          onClick={() => setDefaultMutation.mutate({
            subType: record.sub_type,
            model: record.model,
            provider: record.provider,
          })}
        >
          {isDefault ? '默认' : '设为默认'}
        </Button>
      ),
    },
  ]

  // 渲染厂商面板
  const renderProviderPanel = (provider: string, providerSpecs: GenerationSpec[]) => {
    // 按端点分组
    const groupedBySubType = providerSpecs.reduce<Record<string, GenerationSpec[]>>((acc, spec) => {
      if (!acc[spec.sub_type]) acc[spec.sub_type] = []
      acc[spec.sub_type].push(spec)
      return acc
    }, {})

    return (
      <Collapse.Panel
        key={provider}
        header={
          <Space>
            <Text strong style={{ fontSize: 16 }}>{provider.toUpperCase()}</Text>
            <Tag>{providerSpecs.length} 个模型</Tag>
          </Space>
        }
      >
        {Object.entries(groupedBySubType).map(([subType, subTypeSpecs]) => (
          <Card
            key={subType}
            size="small"
            title={
              <Space>
                <Tag color="blue">{SUB_TYPE_LABELS[subType] || subType}</Tag>
                <Text type="secondary">{subTypeSpecs.length} 个模型</Text>
              </Space>
            }
            style={{ marginBottom: 12 }}
          >
            <Table
              dataSource={subTypeSpecs}
              columns={modelColumns}
              rowKey={(record) => `${record.sub_type}-${record.model}`}
              pagination={false}
              size="small"
            />
          </Card>
        ))}
      </Collapse.Panel>
    )
  }

  return (
    <div className="wr-page-content">
      <QueryBoundary loading={isLoading} error={isError} onRetry={() => refetch()}>
      <>
      <div className="wr-page-header">
        <h1>模型配置管理</h1>
        <p>按厂商分组管理模型配置，设置每个端点的默认模型——修改即时生效</p>
      </div>

      <Card
        className="wr-glass-card"
        title={
          <Space>
            <SettingOutlined />
            <Text strong>模型配置</Text>
          </Space>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 16, fontSize: 12 }}>
          按厂商分组查看和管理模型配置。可以启用/禁用模型，设置默认模型。
          修改后<strong>即时生效</strong>，无需重启服务。
        </Text>

        <Collapse defaultActiveKey={Object.keys(groupedByProvider)}>
          {Object.entries(groupedByProvider).map(([provider, providerSpecs]) =>
            renderProviderPanel(provider, providerSpecs)
          )}
        </Collapse>
      </Card>
      </>
      </QueryBoundary>
    </div>
  )
}
