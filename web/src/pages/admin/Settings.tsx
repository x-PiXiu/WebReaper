import { useState } from 'react'
import { Typography, Switch, Card, Space, message, Alert, Tag, Modal, Input, Table, Button } from 'antd'
import { RadarChartOutlined, ThunderboltOutlined, EditOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'

const { Text } = Typography
const { TextArea } = Input

interface PromptTemplate {
  key: string
  version: number
  content: string
  updated_at: string
}

// 格式 key → 友好标签
const FORMAT_LABELS: Record<string, string> = {
  'content-generate': '内容生成（系统提示词）',
  'content-optimize': '内容优化（系统提示词）',
  'geo_format_article': '📄 SEO 文章格式',
  'geo_format_review': '📝 点评文案格式',
  'geo_format_xiaohongshu': '📕 小红书笔记格式',
  'geo_format_script': '🎬 视频口播脚本格式',
  'geo_format_faq': '❓ FAQ 问答格式',
  'geo_format_comparison': '⚖️ 对比评测格式',
}

// 平台设置（管理后台）：运行时开关 + 提示词模板管理——配置即时生效，无需重启。
export default function AdminSettings() {
  const queryClient = useQueryClient()
  const [editingKey, setEditingKey] = useState<string | null>(null)
  const [editContent, setEditContent] = useState('')

  const { data: autoMonitor, isLoading } = useQuery({
    queryKey: ['settings-auto-monitor'],
    queryFn: () => businessApi.getAutoMonitor(),
  })

  const { data: templates = [] } = useQuery({
    queryKey: ['admin-prompt-templates'],
    queryFn: () => businessApi.adminListPromptTemplates(),
  })

  const toggleMutation = useMutation({
    mutationFn: (enabled: boolean) => businessApi.setAutoMonitor(enabled),
    onSuccess: () => {
      message.success('自动盯盘已' + (autoMonitor?.auto_monitor_enabled ? '关闭' : '开启') + '（调度器即时生效）')
      queryClient.invalidateQueries({ queryKey: ['settings-auto-monitor'] })
    },
    onError: () => message.error('设置失败'),
  })

  const saveTemplateMutation = useMutation({
    mutationFn: ({ key, content }: { key: string; content: string }) =>
      businessApi.adminUpdatePromptTemplate(key, content),
    onSuccess: () => {
      message.success('提示词模板已保存（即时生效）')
      setEditingKey(null)
      queryClient.invalidateQueries({ queryKey: ['admin-prompt-templates'] })
    },
    onError: () => message.error('保存失败'),
  })

  const templateColumns = [
    {
      title: '模板', dataIndex: 'key', key: 'key',
      render: (key: string) => (
        <Space>
          <Tag color={key.startsWith('geo_format_') ? 'purple' : 'blue'} style={{ fontSize: 11 }}>
            {key.startsWith('geo_format_') ? '格式' : '核心'}
          </Tag>
          <Text strong style={{ fontSize: 13 }}>{FORMAT_LABELS[key] || key}</Text>
        </Space>
      ),
    },
    {
      title: '内容预览', dataIndex: 'content', key: 'content', ellipsis: true,
      render: (content: string) => (
        <Text type="secondary" style={{ fontSize: 12 }}>{content?.slice(0, 80)}...</Text>
      ),
    },
    {
      title: '版本', dataIndex: 'version', key: 'version', width: 70,
      render: (v: number) => <Tag>v{v}</Tag>,
    },
    {
      title: '操作', key: 'action', width: 80,
      render: (_: unknown, record: PromptTemplate) => (
        <Button size="small" type="link" icon={<EditOutlined />}
          onClick={() => { setEditingKey(record.key); setEditContent(record.content) }}>
          编辑
        </Button>
      ),
    },
  ]

  return (
    <div className="wr-page-content">
      <div className="wr-page-header">
        <h1>平台设置</h1>
        <p>平台级运行时开关 + 提示词模板管理——修改即时生效，无需重启服务</p>
      </div>

      {/* 自动盯盘开关 */}
      <div className="wr-glass-card" style={{ padding: 24, marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 24 }}>
          <div style={{ flex: 1 }}>
            <Space size={8} style={{ marginBottom: 8 }}>
              <div style={{
                width: 34, height: 34, borderRadius: 10,
                background: 'var(--wr-gradient)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                color: '#fff', fontSize: 15,
              }}>
                <RadarChartOutlined />
              </div>
              <Text strong style={{ fontSize: 15 }}>每日自动监测（自动盯盘）</Text>
              <Tag color={autoMonitor?.auto_monitor_enabled ? 'success' : 'default'}>
                {autoMonitor?.auto_monitor_enabled ? '已开启' : '已关闭'}
              </Tag>
            </Space>
            <Text type="secondary" style={{ fontSize: 13, display: 'block', marginBottom: 12, lineHeight: 1.7 }}>
              开启后调度器每天对全平台所有品牌自动执行一次监测——用户的提及率趋势图
              <strong>自动生长</strong>，无需手动点击监测。付费卖点：打开就是新鲜数据。
            </Text>
            <Alert
              type="info" showIcon style={{ maxWidth: 560 }}
              message={<Text style={{ fontSize: 12 }}>需要 DB + LLM 已配置；每次自动监测会消耗 LLM 额度（按采样次数计费）</Text>}
            />
          </div>
          <Switch
            checked={autoMonitor?.auto_monitor_enabled}
            loading={isLoading || toggleMutation.isPending}
            onChange={(v) => toggleMutation.mutate(v)}
          />
        </div>
      </div>

      {/* 提示词模板管理（格式指令/生成/优化 prompt 热更新）*/}
      <Card
        className="wr-glass-card"
        title={<Space><ThunderboltOutlined /><Text strong>提示词模板管理</Text></Space>}
        style={{ marginBottom: 16 }}
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12, fontSize: 12 }}>
          编辑内容生成/优化的系统提示词 + 各格式输出指令（字数/风格/结构）。
          修改后<strong>即时生效</strong>（下次生成内容时使用新指令），无需重启服务。
          格式模板控制用户选择"小红书/点评/脚本"等格式时的具体输出要求。
        </Text>
        <Table
          dataSource={templates}
          columns={templateColumns}
          rowKey="key"
          pagination={false}
          size="small"
        />
      </Card>

      {/* 编辑弹窗 */}
      <Modal
        title={`编辑 · ${FORMAT_LABELS[editingKey || ''] || editingKey}`}
        open={!!editingKey}
        onCancel={() => setEditingKey(null)}
        width={680}
        footer={
          <Space>
            <Button onClick={() => setEditingKey(null)}>取消</Button>
            <Button type="primary" loading={saveTemplateMutation.isPending}
              onClick={() => editingKey && saveTemplateMutation.mutate({ key: editingKey, content: editContent })}>
              保存（即时生效）
            </Button>
          </Space>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 12 }}>
          {editingKey?.startsWith('geo_format_')
            ? '格式输出指令——控制此格式的字数/风格/结构。会以【优先级最高】注入 systemPrompt 开头，覆盖默认字数要求。'
            : '系统提示词——控制 LLM 生成内容时的角色定位、优化方向和硬性要求。'}
        </Text>
        <TextArea
          value={editContent}
          onChange={(e) => setEditContent(e.target.value)}
          autoSize={{ minRows: 8, maxRows: 20 }}
          style={{ fontFamily: 'monospace', fontSize: 12 }}
        />
      </Modal>
    </div>
  )
}
