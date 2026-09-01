import { useState } from 'react'
import { Typography, Card, Space, Tag, Table, Button, Form, Select, InputNumber, Switch, Modal, Input } from 'antd'
import { ReloadOutlined, SettingOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { CrawlerConfig } from '../../types/api'
import { modal } from '../../utils/antdApp'
import { toast } from '../../utils/feedback'

const { Text } = Typography
const { TextArea } = Input

// 爬虫配置管理页面
export default function CrawlerConfigs() {
  const queryClient = useQueryClient()
  const [editingConfig, setEditingConfig] = useState<CrawlerConfig | null>(null)
  const [form] = Form.useForm()

  // 查询配置列表
  const { data, isLoading } = useQuery({
    queryKey: ['admin-crawler-configs'],
    queryFn: () => businessApi.adminListCrawlerConfigs(),
  })
  const configs = data?.configs || []

  // 更新配置
  const updateMutation = useMutation({
    mutationFn: ({ platform, data }: { platform: string; data: Partial<CrawlerConfig> }) =>
      businessApi.adminUpdateCrawlerConfig(platform, data),
    onSuccess: () => {
      toast.ok('配置已更新', 'admin-cfg')
      setEditingConfig(null)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['admin-crawler-configs'] })
    },
    onError: () => toast.fail('更新失败'),
  })

  // 测试连接
  const testMutation = useMutation({
    mutationFn: (platform: string) => businessApi.adminTestCrawlerConnection(platform),
    onSuccess: (data) => {
      toast.ok(data.alive ? '连接正常' : '连接失败', 'admin-crawl-ping')
    },
    onError: () => toast.fail('连接测试失败'),
  })

  // 手动触发采集
  const triggerMutation = useMutation({
    mutationFn: ({ platform, brandId, keywords }: { platform: string; brandId: string; keywords: string[] }) =>
      businessApi.adminTriggerCrawl(platform, { brand_id: brandId, keywords }),
    onSuccess: (data) => {
      toast.ok(`采集完成：找到 ${data.videos_found}，新增 ${data.videos_new}`, 'admin-crawl-run')
      queryClient.invalidateQueries({ queryKey: ['admin-crawler-configs'] })
    },
    onError: () => toast.fail('采集失败'),
  })

  // 表格列定义
  const columns = [
    {
      title: '平台',
      dataIndex: 'platform',
      key: 'platform',
      render: (platform: string) => {
        const labels: Record<string, string> = { douyin: '抖音', kuaishou: '快手', bilibili: 'B站' }
        return <Tag color="blue">{labels[platform] || platform}</Tag>
      },
    },
    {
      title: '启用',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'green' : 'default'}>{enabled ? '启用' : '禁用'}</Tag>
      ),
    },
    {
      title: '采集间隔',
      dataIndex: 'crawl_interval_minutes',
      key: 'crawl_interval_minutes',
      render: (min: number) => `${min} 分钟`,
    },
    {
      title: '最大结果数',
      dataIndex: 'max_results',
      key: 'max_results',
    },
    {
      title: '关键词',
      key: 'keywords',
      render: (_: unknown, record: CrawlerConfig) => {
        const keywords = [...(record.search_keywords || []), ...(record.extra_keywords || [])]
        return keywords.length > 0 ? (
          <Space wrap>
            {keywords.slice(0, 3).map((k, i) => <Tag key={i}>{k}</Tag>)}
            {keywords.length > 3 && <Tag>+{keywords.length - 3}</Tag>}
          </Space>
        ) : (
          <Text type="secondary">无</Text>
        )
      },
    },
    {
      title: '关键词池',
      dataIndex: 'keyword_pool',
      key: 'keyword_pool',
      render: (pool: string[]) => (
        <Tag color={pool && pool.length > 0 ? 'green' : 'default'}>
          {pool ? pool.length : 0} 个
        </Tag>
      ),
    },
    {
      title: '最后采集',
      dataIndex: 'last_crawled_at',
      key: 'last_crawled_at',
      render: (t: string) => t ? new Date(t).toLocaleString() : '-',
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: unknown, record: CrawlerConfig) => (
        <Space>
          <Button size="small" icon={<SettingOutlined />} onClick={() => {
            setEditingConfig(record)
            form.setFieldsValue(record)
          }}>
            配置
          </Button>
          <Button size="small" icon={<ThunderboltOutlined />}
            onClick={() => testMutation.mutate(record.platform)}>
            测试
          </Button>
          <Button size="small" type="primary" icon={<ReloadOutlined />}
            onClick={() => {
              modal.confirm({
                centered: true,
                title: '手动触发采集',
                content: `确定立即采集 ${record.platform} 平台的数据吗？`,
                onOk: () => triggerMutation.mutate({
                  platform: record.platform,
                  brandId: record.brand_id || 'all',
                  keywords: [...(record.search_keywords || []), ...(record.extra_keywords || [])],
                }),
              })
            }}>
            采集
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title="爬虫配置管理">
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
          配置各平台的采集参数。关键词池由 LLM 自动生成，管理后台可查看和补充。
        </Text>
        <Table
          columns={columns}
          dataSource={configs}
          rowKey="id"
          loading={isLoading}
          pagination={false}
        />
      </Card>

      {/* 编辑配置弹窗 */}
      <Modal
        title={`编辑 ${editingConfig?.platform} 配置`}
        open={!!editingConfig}
        onCancel={() => setEditingConfig(null)}
        onOk={() => form.submit()}
        confirmLoading={updateMutation.isPending}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={(values) => {
          if (editingConfig) {
            updateMutation.mutate({ platform: editingConfig.platform, data: values })
          }
        }}>
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="crawl_interval_minutes" label="采集间隔（分钟）">
            <InputNumber min={5} max={1440} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="max_results" label="最大结果数">
            <InputNumber min={1} max={100} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="sort_by" label="排序方式">
            <Select options={[
              { value: 'popular', label: '热门' },
              { value: 'latest', label: '最新' },
            ]} />
          </Form.Item>
          <Form.Item name="publish_time" label="时间过滤">
            <Select options={[
              { value: 'day', label: '最近一天' },
              { value: 'week', label: '最近一周' },
              { value: 'month', label: '最近一月' },
              { value: 'all', label: '不限' },
            ]} />
          </Form.Item>
          <Form.Item name="extra_keywords" label="额外关键词（每行一个）">
            <TextArea rows={3} placeholder="输入额外搜索关键词，每行一个" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
