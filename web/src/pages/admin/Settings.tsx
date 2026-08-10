import { Typography, Switch, Card, Space, message, Alert, Tag } from 'antd'
import { RadarChartOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'

const { Text } = Typography

// 平台设置（管理后台）：运行时开关——配置即时生效，无需重启。
export default function AdminSettings() {
  const queryClient = useQueryClient()

  const { data: autoMonitor, isLoading } = useQuery({
    queryKey: ['settings-auto-monitor'],
    queryFn: () => businessApi.getAutoMonitor(),
  })

  const toggleMutation = useMutation({
    mutationFn: (enabled: boolean) => businessApi.setAutoMonitor(enabled),
    onSuccess: () => {
      message.success('自动盯盘已' + (autoMonitor?.auto_monitor_enabled ? '关闭' : '开启') + '（调度器即时生效）')
      queryClient.invalidateQueries({ queryKey: ['settings-auto-monitor'] })
    },
    onError: () => message.error('设置失败'),
  })

  return (
    <div className="wr-page-content">
      <div className="wr-page-header">
        <h1>平台设置</h1>
        <p>平台级运行时开关——修改即时生效，无需重启服务</p>
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

      {/* 扩展预留 */}
      <Card title={<Space><ThunderboltOutlined />更多平台级开关（预留）</Space>} className="wr-glass-card">
        <Text type="secondary" style={{ fontSize: 13 }}>
          收录管理配置在「收录管理」页；Agent/LLM 配置在「Agent 配置」页；后续平台级开关（如新用户注册、内容审核流）统一沉淀在本页。
        </Text>
      </Card>
    </div>
  )
}
