import { useState } from 'react'
import { Card, Typography, Button, Input, Form, message, Table, Tag, Space, Popconfirm, Alert } from 'antd'
import { CloudUploadOutlined, ReloadOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { IndexingSubmitLog } from '../../types/api'

const { Title, Text } = Typography

// 渠道名映射
const CHANNEL_NAMES: Record<string, string> = {
  indexnow: 'IndexNow（Bing/Yandex）',
  baidu: '百度主动推送',
  all: '全渠道（补提交）',
}

export default function Indexing() {
  const queryClient = useQueryClient()
  const [form] = Form.useForm()
  const [saving, setSaving] = useState(false)
  const [resubmitting, setResubmitting] = useState(false)

  const { data: config, isLoading: cfgLoading } = useQuery({
    queryKey: ['indexing-config'],
    queryFn: () => businessApi.getIndexingConfig(),
  })

  const { data: logs = [] } = useQuery({
    queryKey: ['indexing-logs'],
    queryFn: () => businessApi.listIndexingLogs(),
    refetchInterval: 10000, // 10s 自动刷新（补提交后能看到结果）
  })

  // 保存配置
  const handleSave = async () => {
    setSaving(true)
    try {
      const v = await form.validateFields()
      await businessApi.updateIndexingConfig({
        index_now_key: v.index_now_key || '',
        baidu_site: v.baidu_site || '',
        baidu_token: v.baidu_token || '',
      })
      message.success('收录配置已保存，30 秒内生效（无需重启）')
      queryClient.invalidateQueries({ queryKey: ['indexing-config'] })
    } catch (e) {
      message.error('保存失败：' + ((e as Error)?.message || '请检查配置格式'))
    } finally {
      setSaving(false)
    }
  }

  // 手动补提交全部已发布内容
  const handleReSubmit = async () => {
    setResubmitting(true)
    try {
      const r = await businessApi.reSubmitAllIndexing()
      message.success(`补提交完成：${r.submitted} 个 URL 已推送（失败 ${r.failed}）`)
      queryClient.invalidateQueries({ queryKey: ['indexing-logs'] })
    } catch (e) {
      message.error('补提交失败：' + ((e as Error)?.message || '可能未启用任何渠道'))
    } finally {
      setResubmitting(false)
    }
  }

  const columns = [
    {
      title: '时间', dataIndex: 'submitted_at', key: 'time', width: 170,
      render: (t: string) => <Text type="secondary" style={{ fontSize: 12 }}>{t ? new Date(t).toLocaleString() : '-'}</Text>,
    },
    {
      title: '渠道', dataIndex: 'channel', key: 'channel', width: 160,
      render: (c: string) => <Tag color="purple">{CHANNEL_NAMES[c] || c}</Tag>,
    },
    {
      title: 'URL', dataIndex: 'url', key: 'url',
      render: (u: string) => <Text style={{ fontSize: 12 }} ellipsis={{ tooltip: u }}>{u}</Text>,
    },
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 90,
      render: (s: string) => s === 'success'
        ? <Tag color="success">成功</Tag>
        : <Tag color="error">失败</Tag>,
    },
    {
      title: '错误信息', dataIndex: 'error_msg', key: 'error', width: 200,
      render: (e: string) => e ? <Text type="danger" style={{ fontSize: 12 }} ellipsis={{ tooltip: e }}>{e}</Text> : '-',
    },
  ]

  return (
    <div className="wr-page-content" style={{ paddingTop: 8 }}>
      <div style={{ marginBottom: 24 }}>
        <Title level={4} style={{ margin: 0, fontSize: 26, letterSpacing: '-0.03em' }}>收录管理</Title>
        <Text type="secondary" style={{ fontSize: 14 }}>搜索引擎收录通知：运行时配置 · 提交审计 · 手动补提交</Text>
      </div>

      {/* 配置卡片 */}
      <Card title="收录渠道配置" className="wr-glass-card" style={{ marginBottom: 16 }} loading={cfgLoading}>
        <Alert
          type="info" showIcon style={{ marginBottom: 16 }}
          message="内容发布为 published 时自动推送到已配置的渠道；修改配置 30 秒内生效，无需重启"
          description="IndexNow 覆盖 Bing/Yandex/Naver（国内 AI 引擎主要走 Bing 索引）；百度主动推送覆盖百度收录。均未配置时自动跳过。"
        />
        <Form
          form={form}
          layout="vertical"
          initialValues={{
            index_now_key: config?.index_now_key || '',
            baidu_site: config?.baidu_site || '',
            baidu_token: config?.baidu_token || '',
          }}
        >
          <Form.Item
            name="index_now_key" label="IndexNow 密钥"
            extra="8-128 个字母/数字/连字符。生成方式：随机字符串（如 uuid）。密钥文件已托管于 /public/indexnow-key.txt"
            rules={[{ pattern: /^[a-zA-Z0-9-]{8,128}$/, message: '格式：8-128 个字母/数字/连字符' }]}
          >
            <Input.Password placeholder="留空 = 不启用 IndexNow" autoComplete="new-password" />
          </Form.Item>
          <Form.Item name="baidu_site" label="百度站点（已验证域名）" style={{ marginBottom: 12 }}>
            <Input placeholder="如 content.example.com（留空 = 不启用百度）" />
          </Form.Item>
          <Form.Item
            name="baidu_token" label="百度准入 Token"
            extra="在 ziyuan.baidu.com 搜索资源平台验证域名后获取"
            style={{ marginBottom: 8 }}
          >
            <Input.Password placeholder="百度主动推送准入 token" autoComplete="new-password" />
          </Form.Item>
        </Form>
        <Space>
          <Button type="primary" loading={saving} onClick={handleSave}>保存配置</Button>
          <Popconfirm
            title="补提交全部已发布内容？"
            description="会向所有已启用渠道重新推送全部 published 内容（渠道故障后重推用）"
            onConfirm={handleReSubmit}
          >
            <Button icon={<ReloadOutlined />} loading={resubmitting}>手动补提交</Button>
          </Popconfirm>
        </Space>
      </Card>

      {/* 提交日志 */}
      <Card title={<Space><CloudUploadOutlined />提交日志（最近 50 条，10 秒自动刷新）</Space>}>
        <Table
          dataSource={logs as IndexingSubmitLog[]}
          columns={columns}
          rowKey="id"
          pagination={{ pageSize: 10, size: 'small' }}
          size="small"
        />
      </Card>
    </div>
  )
}
