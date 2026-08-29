import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card, Typography, Input, Switch, Button, Space, Tag, Alert, Descriptions, Tooltip, Form, Select } from 'antd'
import { CloudServerOutlined, SaveOutlined, KeyOutlined, SafetyCertificateOutlined, SearchOutlined, WalletOutlined } from '@ant-design/icons'
import { businessApi } from '../../api/business'
import type { ProviderConfig } from '../../types/api'
import QueryBoundary from '../../components/QueryBoundary'
import { message } from '../../utils/antdApp'

const { Text } = Typography

// 厂商元信息（新增厂商 = 此表加一行 + 后端 provider 实现）
const PROVIDER_META: Record<string, { name: string; desc: string; color: string }> = {
  vidu: { name: 'Vidu', desc: '视频 / 图片 / 音频 / 数字人生成（智谱清影）', color: 'geekblue' },
}

// 厂商配置管理：第三方集成凭据（生成厂商 / 搜索 API / 支付网关）——保存后热生效（Tavily 改 Key 需重启）。
export default function Providers() {
  const queryClient = useQueryClient()
  // 编辑态：provider → { api_key(空=不修改), base_url, enabled }
  const [drafts, setDrafts] = useState<Record<string, { api_key: string; base_url: string; enabled: boolean }>>({})

  const { data: providers = [] } = useQuery({
    queryKey: ['admin-provider-configs'],
    queryFn: () => businessApi.listProviderConfigs().then(r => r.providers),
  })

  const saveMutation = useMutation({
    mutationFn: (p: { provider: string; data: { api_key?: string; base_url?: string; enabled?: boolean } }) =>
      businessApi.saveProviderConfig(p.provider, p.data),
    onSuccess: () => {
      message.success('已保存（对已接入厂商即时生效）')
      queryClient.invalidateQueries({ queryKey: ['admin-provider-configs'] })
    },
    onError: (e) => message.error(`保存失败：${(e as Error).message}`),
  })

  const draftOf = (p: string): { api_key: string; base_url: string; enabled: boolean } =>
    drafts[p] || { api_key: '', base_url: '', enabled: true }

  const setDraft = (p: string, patch: Partial<{ api_key: string; base_url: string; enabled: boolean }>) => {
    setDrafts(prev => ({ ...prev, [p]: { ...draftOf(p), ...patch } }))
  }

  const save = (cfg: ProviderConfig) => {
    const d = draftOf(cfg.provider)
    // api_key 为空 = 不修改（保留原 Key）
    saveMutation.mutate({
      provider: cfg.provider,
      data: { api_key: d.api_key || undefined, base_url: d.base_url || undefined, enabled: d.enabled },
    })
    setDrafts(prev => ({ ...prev, [cfg.provider]: { ...d, api_key: '' } }))
  }

  // 当前没有厂商配置时显示空态引导
  const known = Object.keys(PROVIDER_META)

  return (
    <div className="wr-page-content" style={{ paddingTop: 8 }}>
      <div className="wr-page-header">
        <h1>厂商配置</h1>
        <p>第三方集成凭据统一管理——生成厂商 / 搜索 API / 支付网关，保存即生效</p>
      </div>

      {providers.length === 0 && (
        <Alert
          type="info" showIcon style={{ marginBottom: 16 }}
          message="尚未配置任何厂商"
          description={`点击下方「${PROVIDER_META.vidu.name}」卡片填写 API Key 即可启用真实生成（未配置时系统运行在 mock 演示模式）`}
        />
      )}

      {/* 生成厂商卡片 */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(460px, 1fr))', gap: 16, marginBottom: 16 }}>
        {known.map(p => {
          const meta = PROVIDER_META[p]
          const cfg = providers.find(x => x.provider === p)
          const d = draftOf(p)
          return (
            <Card
              key={p}
              className="wr-glass-card"
              title={
                <Space>
                  <CloudServerOutlined style={{ color: 'var(--wr-primary)' }} />
                  <Text strong>{meta.name}</Text>
                  <Tag color={meta.color} style={{ fontSize: 11 }}>{p}</Tag>
                  {cfg ? (
                    cfg.enabled
                      ? <Tag color="success" style={{ fontSize: 11 }}>已启用</Tag>
                      : <Tag color="default" style={{ fontSize: 11 }}>已停用</Tag>
                  ) : (
                    <Tag style={{ fontSize: 11 }}>未配置</Tag>
                  )}
                </Space>
              }
            >
              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 16 }}>{meta.desc}</Text>

              <div style={{ marginBottom: 16 }}>
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <div>
                    <Text style={{ fontSize: 13 }}><KeyOutlined /> API Key</Text>
                    <div style={{ marginTop: 4 }}>
                      <Input.Password
                        placeholder={cfg?.has_key ? `已配置（${cfg.api_key}）——留空 = 不修改` : '输入厂商 API Key'}
                        value={d.api_key}
                        onChange={e => setDraft(p, { api_key: e.target.value })}
                        style={{ maxWidth: 380 }}
                      />
                    </div>
                  </div>
                  <div>
                    <Text style={{ fontSize: 13 }}>Base URL（可选）</Text>
                    <div style={{ marginTop: 4 }}>
                      <Input
                        placeholder={cfg?.base_url || 'https://api.vidu.cn'}
                        value={d.base_url}
                        onChange={e => setDraft(p, { base_url: e.target.value })}
                        style={{ maxWidth: 380 }}
                      />
                    </div>
                  </div>
                  <div>
                    <Space>
                      <Text style={{ fontSize: 13 }}>启用</Text>
                      <Tooltip title="停用后该厂商的生成提交将被拒绝（mock 模式除外）">
                        <Switch checked={d.enabled} onChange={v => setDraft(p, { enabled: v })} />
                      </Tooltip>
                    </Space>
                  </div>
                </Space>
              </div>

              <Descriptions size="small" column={2} style={{ marginBottom: 16 }}>
                <Descriptions.Item label="状态">{cfg ? (cfg.enabled ? '启用中' : '停用') : '未配置（mock 模式）'}</Descriptions.Item>
                <Descriptions.Item label="更新时间">{cfg?.updated_at ? cfg.updated_at.replace('T', ' ').slice(0, 19) : '—'}</Descriptions.Item>
              </Descriptions>

              {/* Vidu 剩余积分（排查 CreditInsufficient——充值后点刷新） */}
              {p === 'vidu' && <ViduCredits enabled={!!cfg?.enabled} />}

              <Button
                type="primary" icon={<SaveOutlined />} loading={saveMutation.isPending}
                onClick={() => cfg ? save(cfg) : save({ provider: p, api_key: '', has_key: false, base_url: '', enabled: true, updated_at: '' })}
              >
                保存配置
              </Button>
              {cfg && <Text type="secondary" style={{ fontSize: 12, marginLeft: 12 }}><SafetyCertificateOutlined /> Key 已加密存储，列表仅显示掩码</Text>}
            </Card>
          )
        })}
      </div>

      {/* Tavily 搜索 API */}
      <TavilySection />

      {/* ZPAY 支付网关 */}
      <PaymentSection />
    </div>
  )
}

// Vidu 剩余积分（GET /admin/provider-configs/vidu/credits——积分不足导致任务被拒时先看这里）。
function ViduCredits({ enabled }: { enabled: boolean }) {
  const { data, isLoading, refetch, isFetching, error } = useQuery({
    queryKey: ['vidu-credits'],
    queryFn: () => businessApi.getProviderCredits('vidu'),
    enabled,
    staleTime: 60_000,
  })
  if (!enabled) return null
  return (
    <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center', gap: 10 }}>
      <Text style={{ fontSize: 13 }}>剩余积分：</Text>
      {isLoading ? <Text type="secondary">查询中…</Text> : error ? (
        <Text type="danger" style={{ fontSize: 12 }}>查询失败（{(error as Error).message || '服务不可用'}）</Text>
      ) : (
        <Tag color={(data?.credits ?? 0) > 0 ? 'green' : 'red'} style={{ fontSize: 13 }}>{data?.credits ?? 0}</Tag>
      )}
      <Button size="small" loading={isFetching} onClick={() => refetch()}>刷新</Button>
      {(data?.credits ?? 0) === 0 && !isLoading && !error && (
        <Text type="warning" style={{ fontSize: 12 }}>积分为 0——生成任务会被拒（CreditInsufficient），请到 platform.vidu.cn 充值</Text>
      )}
    </div>
  )
}

// Tavily 搜索 API 配置（专为 AI 设计的高质量搜索源；Key 在 .env 配置，改 Key 需重启）。
function TavilySection() {
  const queryClient = useQueryClient()
  const { data: status, isLoading, isError, refetch } = useQuery({
    queryKey: ['tavily-status'],
    queryFn: () => businessApi.getTavilyStatus(),
  })

  const handleToggle = async (enabled: boolean) => {
    try {
      await businessApi.updateTavilyKey({ enabled })
      message.success(enabled ? 'Tavily 已启用' : 'Tavily 已禁用')
      queryClient.invalidateQueries({ queryKey: ['tavily-status'] })
    } catch {}
  }

  const registered = status?.registered
  const enabled = status?.enabled

  return (
    <Card
      className="wr-glass-card"
      style={{ marginBottom: 16 }}
      title={<Space><SearchOutlined /><Text strong>Tavily 搜索 API</Text><Tag color={enabled ? 'success' : 'default'} style={{ fontSize: 11 }}>{enabled ? '已启用' : '未启用'}</Tag></Space>}
    >
      <QueryBoundary loading={isLoading} error={isError} onRetry={() => refetch()}>
        <div>
          <Text type="secondary" style={{ fontSize: 13, display: 'block', marginBottom: 12 }}>
            Tavily 是专为 AI 设计的高质量搜索 API。配置后，GEO 监测的 Agent 会使用它搜索全网，
            返回比普通搜索引擎更干净、更适合 AI 分析的内容。
          </Text>
          <Space direction="vertical" size={8}>
            <div>
              <Text type="secondary" style={{ fontSize: 13 }}>配置方式：</Text>
              <Text code style={{ fontSize: 12 }}>在 .env 文件设置 TAVILY_API_KEY=tvly-xxxxx</Text>
              <Text type="warning" style={{ fontSize: 12, marginLeft: 8 }}>（修改 Key 需重启服务）</Text>
            </div>
            <div>
              <Text type="secondary" style={{ fontSize: 13 }}>获取 Key：</Text>
              <a href="https://tavily.com" target="_blank" rel="noopener" style={{ fontSize: 13 }}>tavily.com</a>
              <Text type="secondary" style={{ fontSize: 13 }}>（免费 1000 次/月）</Text>
            </div>
            {registered ? (
              <div style={{ marginTop: 8 }}>
                <Switch checked={enabled} onChange={handleToggle} />
                <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>
                  {enabled ? 'Agent 监测时使用 Tavily 搜索' : 'Agent 监测时使用 Bing 搜索（降级）'}
                </Text>
              </div>
            ) : (
              <Text type="warning" style={{ fontSize: 13 }}>
                Tavily 工具未注册，请在 .env 配置 TAVILY_API_KEY 后重启服务
              </Text>
            )}
          </Space>
        </div>
      </QueryBoundary>
    </Card>
  )
}

// ZPAY 支付网关配置（收款凭据；保存后需重启服务生效——网关在启动时读取配置初始化）。
function PaymentSection() {
  const queryClient = useQueryClient()
  const [form] = Form.useForm()
  const { data: cfgRes } = useQuery({
    queryKey: ['payment-config'],
    queryFn: () => businessApi.adminGetPaymentConfig(),
  })
  const config = cfgRes?.config || {}
  const isZpay = config.gateway === 'zpay'

  const saveMut = useMutation({
    mutationFn: (vals: any) => businessApi.adminSetPaymentConfig(vals),
    onSuccess: () => { message.success('支付配置已保存（重启服务后生效）'); queryClient.invalidateQueries({ queryKey: ['payment-config'] }) },
    onError: () => message.error('保存失败'),
  })

  return (
    <Card
      className="wr-glass-card"
      style={{ marginBottom: 16 }}
      title={<Space><WalletOutlined /><Text strong>支付网关（ZPAY）</Text><Tag color={isZpay ? 'green' : 'default'} style={{ fontSize: 11 }}>{isZpay ? 'ZPAY（真实收款）' : 'Mock（开发演示）'}</Tag></Space>}
    >
      {!isZpay && <Text type="secondary" style={{ display: 'block', marginBottom: 12, fontSize: 12 }}>
        未配置 ZPAY 或配置不完整——商户购买走 mock 自动确认。配置完整后重启服务切换为真实收款。
      </Text>}
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          gateway: config.gateway || 'mock',
          pid: config.pid || '',
          key: config.key || '',
          notify_url: config.notify_url || '',
          return_url: config.return_url || '',
        }}
        onFinish={(vals) => saveMut.mutate(vals)}
        style={{ maxWidth: 560 }}
      >
        <Form.Item name="gateway" label="支付通道" rules={[{ required: true }]}>
          <Select options={[{ value: 'mock', label: 'Mock（开发演示）' }, { value: 'zpay', label: 'ZPAY 易支付' }]} />
        </Form.Item>
        <Form.Item name="pid" label="商户 ID（PID）">
          <Input placeholder="ZPAY 后台获取的商户 ID" />
        </Form.Item>
        <Form.Item name="key" label="商户密钥（KEY）" tooltip="保存后脱敏显示（只显示前 4 位）。留空则不修改原密钥。">
          <Input.Password placeholder="ZPAY 后台获取的商户密钥" />
        </Form.Item>
        <Form.Item name="notify_url" label="异步回调地址" tooltip="支付成功后 ZPAY 服务器回调此地址，必须公网可达。">
          <Input placeholder="https://your-domain.com/api/v1/billing/webhook/zpay" />
        </Form.Item>
        <Form.Item name="return_url" label="支付完成跳转地址" tooltip="用户支付完成后浏览器跳转的地址。">
          <Input placeholder="https://your-domain.com/m/my-plan" />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit" loading={saveMut.isPending}>保存配置</Button>
        </Form.Item>
      </Form>
      <Text type="secondary" style={{ fontSize: 12 }}>
        注意：配置保存后需重启服务才生效（支付网关在启动时读取配置初始化）。
        回调地址必须公网可达，否则 ZPAY 服务器无法通知支付结果。
      </Text>
    </Card>
  )
}
