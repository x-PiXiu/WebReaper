import { useState } from 'react'
import { Card, Typography, Button, Input, Form, Table, Tag, Space, Popconfirm, Alert, Row, Col } from 'antd'
import { CloudUploadOutlined, ReloadOutlined, KeyOutlined, SafetyCertificateOutlined, CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { IndexingSubmitLog } from '../../types/api'
import { message } from '../../utils/antdApp'

const { Text } = Typography

// 渠道名映射
const CHANNEL_NAMES: Record<string, string> = {
  indexnow: 'IndexNow（Bing/Yandex）',
  baidu: '百度主动推送',
  all: '全渠道（补提交）',
}

// 提交渠道（IndexNow/百度）：密钥自动生成与验证 · 渠道配置 · 提交审计 · 手动补提交。
// 术语：商户端「收录」指内容被引擎收录/关键词被 AI 提及；此处是提交渠道配置——
// 与商户端概念解耦，避免一词三义。
// IndexNow 协议要点（官方 FAQ）：密钥=网站所有权证明，由站长生成 GUID 并托管 {key}.txt——
// 本平台代为生成并自动托管 key 文件（/public/indexnow-key.txt），管理员只需一键生成 + 验证。
export default function Indexing() {
  const queryClient = useQueryClient()
  const [form] = Form.useForm()
  const [saving, setSaving] = useState(false)
  const [resubmitting, setResubmitting] = useState(false)
  const [generatingKey, setGeneratingKey] = useState(false)
  const [verifyResult, setVerifyResult] = useState<{ url: string; reachable: boolean; content_match: boolean; status_code: number; error: string } | null>(null)

  const { data: config, isLoading: cfgLoading } = useQuery({
    queryKey: ['indexing-config'],
    queryFn: () => businessApi.getIndexingConfig(),
  })

  const { data: logs = [] } = useQuery({
    queryKey: ['indexing-logs'],
    queryFn: () => businessApi.listIndexingLogs(),
    refetchInterval: 10000,
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

  // 一键自动生成 IndexNow 密钥（GUID；key 文件由公开站自动托管）
  const handleGenerateKey = async () => {
    setGeneratingKey(true)
    try {
      const r = await businessApi.generateIndexingKey()
      form.setFieldValue('index_now_key', r.index_now_key)
      message.success('密钥已自动生成，key 文件已托管（无需手动放置）')
      queryClient.invalidateQueries({ queryKey: ['indexing-config'] })
    } catch (e) {
      message.error('生成失败：' + ((e as Error)?.message || ''))
    } finally {
      setGeneratingKey(false)
    }
  }

  // 验证密钥文件可公开访问（搜索引擎视角）
  const handleVerifyKey = async () => {
    try {
      const r = await businessApi.verifyIndexingKey()
      setVerifyResult(r)
      if (r.content_match) {
        message.success('验证通过：key 文件可公开访问且内容一致')
      } else {
        message.warning('验证未通过：' + (r.error || `状态码 ${r.status_code}`))
      }
    } catch (e) {
      message.error('验证失败：' + ((e as Error)?.message || ''))
    }
  }

  // 手动补提交
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
      render: (s: string) => s === 'success' ? <Tag color="success">成功</Tag> : <Tag color="error">失败</Tag>,
    },
    {
      title: '错误信息', dataIndex: 'error_msg', key: 'error', width: 200,
      render: (e: string) => e ? <Text type="danger" style={{ fontSize: 12 }} ellipsis={{ tooltip: e }}>{e}</Text> : '-',
    },
  ]

  return (
    <div className="wr-page-content">
      <div className="wr-page-header">
        <h1>提交渠道</h1>
        <p>搜索引擎收录通知：密钥自动托管 · 渠道配置 · 提交审计 · 手动补提交</p>
      </div>

      {/* ① 密钥与验证 */}
      <div className="wr-glass-card" style={{ padding: 20, marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 14 }}>
          <KeyOutlined style={{ color: 'var(--wr-primary)' }} />
          <Text strong style={{ fontSize: 15 }}>IndexNow 密钥（所有权证明）</Text>
        </div>
        <Alert
          type="info" showIcon style={{ marginBottom: 16 }}
          message="密钥无需手工生成——按 IndexNow 协议，密钥是网站所有权证明：系统自动生成 GUID 并托管 {key}.txt 文件"
          description="公开站已托管 /public/indexnow-key.txt 端点。生成密钥后点「验证」即可确认搜索引擎能访问；同域名下自动生效（Bing/Yandex/Naver）。"
        />
        <Row gutter={[16, 16]} align="middle">
          <Col xs={24} md={12}>
            <Space direction="vertical" style={{ width: '100%' }} size={8}>
              <Text type="secondary" style={{ fontSize: 12 }}>当前密钥（{config?.index_now_key ? '已配置' : '未配置'}）</Text>
              <code style={{
                padding: '10px 14px', borderRadius: 8, display: 'block',
                fontSize: 12.5, wordBreak: 'break-all', fontFamily: 'JetBrains Mono, monospace',
                background: 'var(--wr-input-bg)', border: '1px solid var(--wr-border)',
              }}>
                {config?.index_now_key || '（空——点击「自动生成密钥」）'}
              </code>
            </Space>
          </Col>
          <Col xs={24} md={12}>
            <Space wrap>
              <Button type="primary" icon={<KeyOutlined />} loading={generatingKey} onClick={handleGenerateKey}>
                自动生成密钥
              </Button>
              <Button icon={<SafetyCertificateOutlined />} onClick={handleVerifyKey}>
                验证密钥文件
              </Button>
            </Space>
            {verifyResult && (
              <div style={{ marginTop: 12 }}>
                {verifyResult.content_match ? (
                  <Alert type="success" showIcon icon={<CheckCircleOutlined />}
                    message="验证通过：key 文件可公开访问且内容一致"
                    description={<Text style={{ fontSize: 12 }}>{verifyResult.url}</Text>} />
                ) : (
                  <Alert type="warning" showIcon icon={<CloseCircleOutlined />}
                    message={`验证未通过（${verifyResult.error || 'HTTP ' + verifyResult.status_code}）`}
                    description={<Text style={{ fontSize: 12 }}>{verifyResult.url}</Text>} />
                )}
              </div>
            )}
          </Col>
        </Row>
      </div>

      {/* ② 渠道配置 */}
      <Card title="渠道配置" className="wr-glass-card" style={{ marginBottom: 16 }} loading={cfgLoading}>
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
            extra="8-128 个字母/数字/连字符；留空 = 不启用。密钥文件已由公开站自动托管"
            rules={[{ pattern: /^[a-zA-Z0-9-]{8,128}$/, message: '格式：8-128 个字母/数字/连字符' }]}
          >
            <Input.Password placeholder="点击上方「自动生成密钥」" autoComplete="new-password" />
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

      {/* ③ 提交日志 */}
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
