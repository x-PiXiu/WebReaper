import { useEffect } from 'react'
import { Card, Typography, Form, InputNumber, Switch, Button, Space, message, Alert } from 'antd'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import type { CrawlConfig } from '../types/api'

const { Title, Text } = Typography

export default function CrawlConfigPage() {
  const queryClient = useQueryClient()
  const [form] = Form.useForm<CrawlConfig>()

  const { data, isLoading } = useQuery({
    queryKey: ['crawl-config'],
    queryFn: () => businessApi.getCrawlConfig(),
  })

  // 数据加载后回填表单
  useEffect(() => {
    if (data) form.setFieldsValue(data)
  }, [data, form])

  const onSave = async (values: CrawlConfig) => {
    try {
      await businessApi.updateCrawlConfig(values)
      message.success('采集配置已保存')
      queryClient.invalidateQueries({ queryKey: ['crawl-config'] })
    } catch {}
  }

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>采集配置</Title>
        <Text type="secondary" style={{ fontSize: 13 }}>
          调整爬虫的请求速率与合规策略。修改后立即生效（下次采集任务即按新配置执行）。
        </Text>
      </div>

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="速率与合规说明"
        description={
          <ul style={{ margin: 0, paddingLeft: 20, fontSize: 13 }}>
            <li><b>请求间隔</b>：同域名两次请求的最小间隔。值越大越礼貌（不压垮目标站），但采集越慢。</li>
            <li><b>请求超时</b>：单个页面的最长等待时间。超时则该页面采集失败。</li>
            <li><b>遵守 robots.txt</b>：开启后，目标站 robots.txt 禁止的路径不会被爬取。<b>关闭有法律风险，仅在采集自有站点时关闭</b>。</li>
          </ul>
        }
      />

      <Card title="速率与合规" loading={isLoading}>
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 520 }}>
          <Form.Item
            label="请求间隔（毫秒）"
            name="request_interval_ms"
            rules={[{ required: true, message: '请输入请求间隔' }]}
            tooltip="同域名两次请求的最小间隔。建议 ≥ 1000ms（1秒），礼貌爬取"
          >
            <InputNumber min={0} max={60000} step={500} style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item
            label="请求超时（毫秒）"
            name="request_timeout_ms"
            rules={[{ required: true, message: '请输入请求超时' }]}
            tooltip="单个页面最长等待时间，默认 30000ms（30秒）"
          >
            <InputNumber min={1000} max={120000} step={1000} style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item
            label="最大重试次数"
            name="max_retries"
            tooltip="单个页面采集失败后的重试次数。0 = 不重试"
          >
            <InputNumber min={0} max={5} style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item
            label="遵守 robots.txt"
            name="respect_robots"
            valuePropName="checked"
            tooltip="开启后遵守目标站的 robots.txt。关闭有法律风险"
          >
            <Switch />
          </Form.Item>

          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">保存配置</Button>
              <Button onClick={() => data && form.setFieldsValue(data)}>重置</Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}
