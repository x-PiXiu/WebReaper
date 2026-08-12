import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card, Typography, Input, Switch, Button, Space, Tag, message, Alert, Descriptions, Tooltip } from 'antd'
import { CloudServerOutlined, SaveOutlined, KeyOutlined, SafetyCertificateOutlined } from '@ant-design/icons'
import { businessApi } from '../../api/business'
import type { ProviderConfig } from '../../types/api'

const { Text } = Typography

// 厂商元信息（新增厂商 = 此表加一行 + 后端 provider 实现）
const PROVIDER_META: Record<string, { name: string; desc: string; color: string }> = {
  vidu: { name: 'Vidu', desc: '视频 / 图片 / 音频 / 数字人生成（智谱清影）', color: 'geekblue' },
}

// 厂商配置管理：按厂商设置 API Key / 启用开关（保存后热生效，无需重启）。
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
        <p>按厂商区分管理生成服务——API Key / 启用开关，保存后热生效无需重启</p>
      </div>

      {providers.length === 0 && (
        <Alert
          type="info" showIcon style={{ marginBottom: 16 }}
          message="尚未配置任何厂商"
          description={`点击下方「${PROVIDER_META.vidu.name}」卡片填写 API Key 即可启用真实生成（未配置时系统运行在 mock 演示模式）`}
        />
      )}

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(460px, 1fr))', gap: 16 }}>
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
    </div>
  )
}
