import { useEffect, useState } from 'react'
import { Space, Tag, Switch, Button, Select, InputNumber, Card, Typography, message } from 'antd'
import { RadarChartOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../api/business'

const { Text } = Typography

// 自动盯盘控制（工作台极简状态行 / AI 可见度矩阵页完整配置共用）。
//
// 设计决策：盯盘是"监测执行方式"——配置归监测页（AI 可见度矩阵），
// 工作台只保留一行状态感知。此前工作台塞了完整表单（频率/采样/阈值），
// 把"看数据"的驾驶舱页变成了"配设置"的操作页。
export default function AutoMonitorControl({ compact = false }: { compact?: boolean }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [showAdvanced, setShowAdvanced] = useState(false)
  const { data, isLoading } = useQuery({
    queryKey: ['tenant-auto-monitor'],
    queryFn: () => businessApi.getTenantAutoMonitor(),
  })
  const { data: usage } = useQuery({
    queryKey: ['my-usage'],
    queryFn: () => businessApi.getMyUsage().catch(() => null),
    staleTime: 60_000,
  })
  const [frequency, setFrequency] = useState<string>('daily')
  const [sampleSize, setSampleSize] = useState<number>(5)
  const [dropThreshold, setDropThreshold] = useState<number>(20)
  const [notifyOvertake, setNotifyOvertake] = useState<boolean>(true)
  const [loadedCfg, setLoadedCfg] = useState(false)
  useEffect(() => {
    if (!data?.config || loadedCfg) return
    setLoadedCfg(true)
    setFrequency(data.config.frequency || 'daily')
    setSampleSize(data.config.sample_size || 5)
    setDropThreshold(data.config.notify_drop_threshold || 20)
    setNotifyOvertake(data.config.notify_overtake !== false)
  }, [data?.config, loadedCfg])

  const saveMutation = useMutation({
    mutationFn: ({ enabled, cfg }: { enabled: boolean; cfg?: any }) =>
      businessApi.setTenantAutoMonitor({ enabled, config: cfg }),
    onSuccess: () => {
      message.success('自动盯盘设置已保存（按配置每日自动监测，趋势自动生长）')
      queryClient.invalidateQueries({ queryKey: ['tenant-auto-monitor'] })
    },
  })

  const frequencyLabel: Record<string, string> = { daily: '每天 1 次', half_day: '每 12 小时', weekly: '每周 1 次' }
  const hasFeature = (usage?.plan?.features || []).includes('auto-monitor')
  const active = data?.platform_enabled && data?.tenant_enabled
  const cfg = data?.config || { frequency: 'daily', sample_size: 5 }

  if (compact) {
    // 极简状态行：状态 + 开关（配置入口指向 AI 可见度矩阵页）
    return (
      <div className="wr-glass-card" style={{ padding: '10px 16px', marginBottom: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
          <Space size={8}>
            <RadarChartOutlined style={{ color: 'var(--wr-primary)' }} />
            <Text strong style={{ fontSize: 13 }}>自动盯盘</Text>
            <Tag color={active ? 'success' : data?.platform_enabled ? 'warning' : 'default'} style={{ fontSize: 11, margin: 0 }}>
              {active ? '运行中' : data?.platform_enabled ? '已暂停' : '平台未开启'}
            </Tag>
            {active && (
              <Text type="secondary" style={{ fontSize: 12 }}>
                {frequencyLabel[cfg.frequency]} · 每关键词 {cfg.sample_size} 次采样
              </Text>
            )}
          </Space>
          <Space size={8}>
            {hasFeature ? (
              <Switch
                size="small"
                checked={!!data?.tenant_enabled}
                disabled={!data?.platform_enabled}
                loading={isLoading || saveMutation.isPending}
                onChange={(v) => saveMutation.mutate({ enabled: v })}
                checkedChildren="开启" unCheckedChildren="关闭"
              />
            ) : (
              <Button size="small" type="link" onClick={() => navigate('/m/my-plan')}>升级解锁 →</Button>
            )}
            <Button size="small" type="link" onClick={() => navigate('/m/analytics?tab=report')}>配置盯盘 →</Button>
          </Space>
        </div>
      </div>
    )
  }

  // 完整配置：状态 + 开关 + 高级设置（频率/采样/通知阈值）
  return (
    <Card className="wr-glass-card" styles={{ body: { padding: 16 } }} style={{ marginBottom: 16 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
        <Space size={6}>
          <RadarChartOutlined style={{ color: 'var(--wr-primary)' }} />
          <Text strong style={{ fontSize: 14 }}>自动盯盘</Text>
          <Tag color={active ? 'success' : data?.platform_enabled ? 'warning' : 'default'} style={{ fontSize: 11, margin: 0 }}>
            {active ? '运行中' : data?.platform_enabled ? '已暂停' : '平台未开启'}
          </Tag>
        </Space>
        <Space size={8}>
          <Button size="small" type="link" onClick={() => setShowAdvanced(!showAdvanced)}>
            {showAdvanced ? '收起设置 ↑' : '高级设置'}
          </Button>
          {hasFeature ? (
            <Switch
              checked={!!data?.tenant_enabled}
              disabled={!data?.platform_enabled}
              loading={isLoading || saveMutation.isPending}
              onChange={(v) => saveMutation.mutate({ enabled: v })}
              checkedChildren="开启" unCheckedChildren="关闭"
            />
          ) : (
            <Button size="small" type="link" onClick={() => navigate('/m/my-plan')}>升级解锁 →</Button>
          )}
        </Space>
      </div>
      <Text type="secondary" style={{ fontSize: 12, display: 'block', lineHeight: 1.7 }}>
        开启后系统按你设置的节奏自动监测全部关键词——趋势自动生长，无需手动点监测；
        提及率下降或竞品反超时按阈值自动通知。
        {showAdvanced && data?.platform_enabled && ` 当前：${frequencyLabel[cfg.frequency]} · 每关键词 ${cfg.sample_size} 次采样。`}
      </Text>
      {!data?.platform_enabled && (
        <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 8, color: 'var(--wr-text-muted)' }}>
          平台总开关未开启（管理员在平台设置中控制）
        </Text>
      )}

      {showAdvanced && hasFeature && (
        <div style={{ marginTop: 12, padding: '12px 14px', borderRadius: 10, background: 'var(--wr-bg-elevated)' }}>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 12 }}>
            <div>
              <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>监测频率</Text>
              <Select
                size="small" style={{ width: '100%' }} value={frequency}
                onChange={setFrequency}
                options={[
                  { value: 'daily', label: '每天 1 次（省额度）' },
                  { value: 'half_day', label: '每 12 小时（更灵敏）' },
                  { value: 'weekly', label: '每周 1 次（最省）' },
                ]}
              />
            </div>
            <div>
              <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>每关键词采样次数</Text>
              <Select
                size="small" style={{ width: '100%' }} value={sampleSize}
                onChange={setSampleSize}
                options={[
                  { value: 3, label: '3 次（快测，省 token）' },
                  { value: 5, label: '5 次（推荐，更准）' },
                  { value: 10, label: '10 次（最准，烧 token）' },
                ]}
              />
            </div>
            <div>
              <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>提及率下降通知阈值</Text>
              <InputNumber
                size="small" min={5} max={80} style={{ width: '100%' }} value={dropThreshold}
                onChange={(v) => setDropThreshold(v || 20)} addonAfter="%"
              />
            </div>
            <div>
              <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>竞品反超通知</Text>
              <Switch size="small" checked={notifyOvertake} onChange={setNotifyOvertake} checkedChildren="开" unCheckedChildren="关" />
              <Text type="secondary" style={{ fontSize: 10, display: 'block', marginTop: 2 }}>竞品提及率超过你时提醒</Text>
            </div>
          </div>
          <div style={{ marginTop: 10, display: 'flex', justifyContent: 'flex-end' }}>
            <Button size="small" type="primary" loading={saveMutation.isPending}
              onClick={() => saveMutation.mutate({
                enabled: !!data?.tenant_enabled,
                cfg: { frequency, sample_size: sampleSize, engine_name: '', notify_drop_threshold: dropThreshold, notify_overtake: notifyOvertake },
              })}>
              保存盯盘设置
            </Button>
          </div>
        </div>
      )}
    </Card>
  )
}
