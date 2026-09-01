import { useState } from 'react'
import { Typography, Card, Space, Tag, Modal, Input, Table, Button } from 'antd'
import { ThunderboltOutlined, EditOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { toast } from '../../utils/feedback'

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
  'geo_format_article': 'SEO 文章格式',
  'geo_format_review': '点评文案格式',
  'geo_format_xiaohongshu': '小红书笔记格式',
  'geo_format_script': '视频口播脚本格式',
  'geo_format_faq': 'FAQ 问答格式',
  'geo_format_comparison': '对比评测格式',
}

// 提示词模板（GEO 内容引擎域）：内容生成/优化系统提示词 + 各格式输出指令——热更新即时生效。
export default function AdminPromptTemplates() {
  const queryClient = useQueryClient()
  const [editingKey, setEditingKey] = useState<string | null>(null)
  const [editContent, setEditContent] = useState('')

  const { data: templates = [] } = useQuery({
    queryKey: ['admin-prompt-templates'],
    queryFn: () => businessApi.adminListPromptTemplates(),
  })

  const saveTemplateMutation = useMutation({
    mutationFn: ({ key, content }: { key: string; content: string }) =>
      businessApi.adminUpdatePromptTemplate(key, content),
    onSuccess: () => {
      toast.ok('提示词模板已保存', 'admin-prompt')
      setEditingKey(null)
      queryClient.invalidateQueries({ queryKey: ['admin-prompt-templates'] })
    },
    onError: () => toast.fail('保存失败'),
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
        <h1>提示词模板</h1>
        <p>内容生成/优化的系统提示词 + 各格式输出指令——修改即时生效，无需重启服务</p>
      </div>

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
