import { Typography, Switch, Space, Alert, Tag, Segmented, Button, Input, Select, Spin } from 'antd'
import { RadarChartOutlined, EyeOutlined, SwapOutlined, SettingOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { message } from '../../utils/antdApp'

const { Text } = Typography

// 平台设置（管理后台）：仅平台级运行时开关——修改即时生效，无需重启。
// 提示词模板管理已迁至独立页（/admin/prompt-templates，属 GEO 内容引擎域）。
export default function AdminSettings({ embedded = false }: { embedded?: boolean }) {
  const queryClient = useQueryClient()

  const { data: autoMonitor, isLoading } = useQuery({
    queryKey: ['settings-auto-monitor'],
    queryFn: () => businessApi.getAutoMonitor(),
  })

  // 浏览器可见性（正常 hooks 写法，非 IIFE 内联）
  const { data: browserHeaded } = useQuery({
    queryKey: ['settings-browser-headed'],
    queryFn: () => businessApi.getBrowserHeaded(),
  })

  const toggleMutation = useMutation({
    mutationFn: (enabled: boolean) => businessApi.setAutoMonitor(enabled),
    onSuccess: () => {
      message.success('自动盯盘已' + (autoMonitor?.auto_monitor_enabled ? '关闭' : '开启') + '（调度器即时生效）')
      queryClient.invalidateQueries({ queryKey: ['settings-auto-monitor'] })
    },
    onError: () => message.error('设置失败'),
  })

  const toggleBrowser = useMutation({
    mutationFn: (headed: boolean) => businessApi.setBrowserHeaded(headed),
    onSuccess: () => {
      message.success('浏览器可见性已切换（下次 RPA 操作即时生效）')
      queryClient.invalidateQueries({ queryKey: ['settings-browser-headed'] })
    },
  })

  // 发布通道矩阵（三轴重构：双链路共存 + 手动切换）
  const { data: transports, refetch: refetchTransports } = useQuery({
    queryKey: ['admin-publish-transports'],
    queryFn: () => businessApi.listPublishTransports().catch(() => ({ platforms: [] })),
  })
  const setTransport = useMutation({
    mutationFn: ({ platform, kind }: { platform: string; kind: string }) => businessApi.setPublishTransport(platform, kind),
    onSuccess: (_d, v) => {
      message.success(v.kind ? `${v.platform} 已强制走 ${v.kind.toUpperCase()} 通道` : `${v.platform} 已恢复自动降级`)
      refetchTransports()
    },
    onError: () => message.error('切换失败'),
  })

  return (
    <div className={embedded ? "" : "wr-page-content"}>
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

      {/* 浏览器可见性开关 */}
      <div className="wr-glass-card" style={{ padding: 24, marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 24 }}>
          <div style={{ flex: 1 }}>
            <Space size={8} style={{ marginBottom: 8 }}>
              <div style={{ width: 34, height: 34, borderRadius: 10, background: 'var(--wr-gradient)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: 15 }}>
                <EyeOutlined />
              </div>
              <Text strong style={{ fontSize: 15 }}>浏览器窗口可见性（RPA 发布/扫码登录）</Text>
              <Tag color={browserHeaded?.headed ? 'processing' : 'default'}>
                {browserHeaded?.headed ? '显示窗口（调试）' : 'Headless（生产）'}
              </Tag>
            </Space>
            <Text type="secondary" style={{ fontSize: 13, display: 'block', lineHeight: 1.7 }}>
              开启后扫码登录/自动发布时<strong>显示浏览器窗口</strong>（可看到操作过程，调试用）。
              关闭则<strong>后台静默执行</strong>（headless，生产默认）。切换即时生效，无需重启。
            </Text>
          </div>
          <Switch checked={browserHeaded?.headed} loading={toggleBrowser.isPending} onChange={(v) => toggleBrowser.mutate(v)} />
        </div>
      </div>
      {/* 发布通道管理（双链路共存） */}
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <Space size={8} style={{ marginBottom: 8 }}>
          <div style={{ width: 34, height: 34, borderRadius: 10, background: 'var(--wr-gradient)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: 15 }}>
            <SwapOutlined />
          </div>
          <Text strong style={{ fontSize: 15 }}>发布通道管理（官方 API / 浏览器 RPA 双链路）</Text>
        </Space>
        <Text type="secondary" style={{ fontSize: 13, display: 'block', marginBottom: 16, lineHeight: 1.7 }}>
          自动策略：按账号凭证降级（OAuth→API，浏览器→RPA，兜底→半自动链接）；一条路启动失败自动切换下一条。
          此处可按平台<strong>强制指定通道</strong>（优先于自动策略；清除即恢复自动）。发布执行中失败不自动重发（防重复发布）。
        </Text>
        {(transports?.platforms || []).map((p) => (
          <div key={p.platform} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '10px 0', borderTop: '1px solid var(--wr-border)' }}>
            <Space>
              <Text strong style={{ width: 72 }}>{p.platform}</Text>
              <Tag>可用：{p.available.join(' / ')}</Tag>
              <Tag color={p.mode === 'auto' ? 'default' : 'warning'}>{p.mode === 'auto' ? '自动降级' : `手动：${p.override.toUpperCase()}`}</Tag>
            </Space>
            <Space>
              <Segmented
                value={p.override || 'auto'}
                options={[{ value: 'auto', label: '自动' }, ...p.available.map((k) => ({ value: k, label: k.toUpperCase() }))]}
                onChange={(v) => setTransport.mutate({ platform: p.platform, kind: v === 'auto' ? '' : String(v) })}
              />
              {p.override && <Button size="small" type="text" onClick={() => setTransport.mutate({ platform: p.platform, kind: '' })}>恢复自动</Button>}
            </Space>
          </div>
        ))}
      </div>

      {/* 生成域业务配置（gen_* —— 25 号计划链式形象视频/默认参数，UI 化散落的环境变量） */}
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <Space size={8} style={{ marginBottom: 8 }}>
          <div style={{ width: 34, height: 34, borderRadius: 10, background: 'var(--wr-gradient)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: 15 }}>
            <SettingOutlined />
          </div>
          <Text strong style={{ fontSize: 15 }}>生成域业务配置</Text>
        </Space>
        <Text type="secondary" style={{ fontSize: 13, display: 'block', marginBottom: 16, lineHeight: 1.7 }}>
          口播/图文/音色生成链路的运行时参数——修改即时生效，影响所有新提交的生成任务。
        </Text>
        <GenSettingsSection />
      </div>
    </div>
  )
}

// 生成域配置子组件（独立封装——data fetching + 编辑）
function GenSettingsSection() {
  const queryClient = useQueryClient()
  const { data: genSettings, isLoading } = useQuery({
    queryKey: ['admin-gen-settings'],
    queryFn: () => businessApi.adminGetGenSettings(),
    staleTime: 60_000,
  })

  const saveMutation = useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) => businessApi.adminSetGenSetting(key, value),
    onSuccess: () => {
      message.success('已保存（即时生效）')
      queryClient.invalidateQueries({ queryKey: ['admin-gen-settings'] })
    },
    onError: () => message.error('保存失败'),
  })

  if (isLoading) return <Spin size="small" />
  if (!genSettings) return <Text type="secondary">配置不可用</Text>

  const boolVal = (key: string, fallback: boolean) => {
    const v = genSettings[key]
    if (v === 'true' || v === '1') return true
    if (v === 'false' || v === '0') return false
    return fallback
  }

  return (
    <div>
      {/* 链式形象视频 */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '10px 0', borderTop: '1px solid var(--wr-border)' }}>
        <div>
          <Text strong>链式形象视频</Text>
          <Text type="secondary" style={{ display: 'block', fontSize: 12 }}>创建数字分身后自动生成 10s 形象视频（消耗 Vidu 积分）</Text>
        </div>
        <Switch
          checked={boolVal('gen_chain_avatar_video', true)}
          onChange={(v) => saveMutation.mutate({ key: 'gen_chain_avatar_video', value: String(v) })}
        />
      </div>
      {/* 默认形象 prompt */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '10px 0', borderTop: '1px solid var(--wr-border)' }}>
        <div style={{ flex: 1, marginRight: 16 }}>
          <Text strong>默认形象视频 prompt</Text>
          <Text type="secondary" style={{ display: 'block', fontSize: 12 }}>用户未填场景描述时的兜底 prompt</Text>
        </div>
        <Input
          style={{ maxWidth: 320 }}
          defaultValue={genSettings['gen_default_avatar_prompt'] || '形象展示：正面特写，微笑看向镜头，姿态自然大方，缓慢自然的肢体动作'}
          placeholder="形象展示：正面特写…"
          onBlur={(e) => {
            const key = 'gen_default_avatar_prompt'
            if (e.target.value !== (genSettings[key] || '')) {
              saveMutation.mutate({ key, value: e.target.value })
            }
          }}
        />
      </div>
      {/* 默认分辨率 */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '10px 0', borderTop: '1px solid var(--wr-border)' }}>
        <div>
          <Text strong>默认分辨率</Text>
          <Text type="secondary" style={{ display: 'block', fontSize: 12 }}>视频/图片生成的默认清晰度</Text>
        </div>
        <Select
          style={{ width: 120 }}
          defaultValue={genSettings['gen_default_resolution'] || '1080p'}
          options={[
            { value: '720p', label: '720p' },
            { value: '1080p', label: '1080p' },
            { value: '2k', label: '2K' },
          ]}
          onChange={(v) => saveMutation.mutate({ key: 'gen_default_resolution', value: v })}
        />
      </div>
      {/* 默认比例 */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '10px 0', borderTop: '1px solid var(--wr-border)' }}>
        <div>
          <Text strong>默认画面比例</Text>
          <Text type="secondary" style={{ display: 'block', fontSize: 12 }}>视频/图片生成的默认宽高比</Text>
        </div>
        <Select
          style={{ width: 120 }}
          defaultValue={genSettings['gen_default_aspect_ratio'] || '16:9'}
          options={[
            { value: '16:9', label: '16:9 横版' },
            { value: '9:16', label: '9:16 竖版' },
            { value: '1:1', label: '1:1 方形' },
          ]}
          onChange={(v) => saveMutation.mutate({ key: 'gen_default_aspect_ratio', value: v })}
        />
      </div>
      {/* 水印 */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '10px 0', borderTop: '1px solid var(--wr-border)' }}>
        <div>
          <Text strong>默认加水印</Text>
          <Text type="secondary" style={{ display: 'block', fontSize: 12 }}>傻瓜式客户端不暴露此开关——由管理后台全局控制</Text>
        </div>
        <Switch
          checked={boolVal('gen_default_watermark', false)}
          onChange={(v) => saveMutation.mutate({ key: 'gen_default_watermark', value: String(v) })}
        />
      </div>
      {/* 错峰 */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '10px 0', borderTop: '1px solid var(--wr-border)' }}>
        <div>
          <Text strong>默认错峰生成</Text>
          <Text type="secondary" style={{ display: 'block', fontSize: 12 }}>错峰更便宜但更慢</Text>
        </div>
        <Switch
          checked={boolVal('gen_default_off_peak', false)}
          onChange={(v) => saveMutation.mutate({ key: 'gen_default_off_peak', value: String(v) })}
        />
      </div>
    </div>
  )
}
