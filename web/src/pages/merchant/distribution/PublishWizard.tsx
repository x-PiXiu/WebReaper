import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  Alert, Button, Empty, Input, Select, Space, Spin, Steps, Switch, Tag, Typography, message,
} from 'antd'
import {
  ArrowDownOutlined, ArrowUpOutlined, CheckCircleOutlined, LinkOutlined, CloudUploadOutlined,
} from '@ant-design/icons'
import AssetPicker from '../../../components/AssetPicker'
import { PlatformBadge } from '../../../components/PlatformBadge'
import { businessApi } from '../../../api/business'
import { scoreColor } from '../../../utils/geo'
import type { Account, Brand, OptimizedContent, PublishChannelView, PublishJob } from '../../../types/api'
import {
  BILIBILI_CATEGORIES, WIZARD_STEPS, channelNeedsCategory, channelNeedsTags, channelShowsTags,
  checkCompleteness, clearDraft, emptyDraft, loadDraft, localAdaptPreview, runeLen, saveDraft,
  strictestTitleLimit, supportsAutoForm, buildPublishContent, effectiveConstraints,
  type PublishForm, type WizardDraft, type WizardStep,
} from './wizardModel'

const { Text, Paragraph } = Typography

const PLATFORM_META: Record<string, { name: string; desc: string }> = {
  douyin: { name: '抖音', desc: '短视频获客主战场' },
  kuaishou: { name: '快手', desc: '下沉市场覆盖广' },
  xiaohongshu: { name: '小红书', desc: '种草社区精准触达' },
  zhihu: { name: '知乎', desc: '长文 SEO 友好' },
  bilibili: { name: 'B站', desc: '年轻用户视频社区' },
  weixin: { name: '视频号', desc: '微信私域入口' },
}

export default function PublishWizard(props: {
  brands: Brand[]
  brandId?: string
  setBrandId: (id: string) => void
  accounts: Account[]
  channels: PublishChannelView[]
  contents: OptimizedContent[]
  initial?: Partial<WizardDraft>
  onBind: (platform: string) => void
  onPublished: (jobs: PublishJob[], mode: 'semi-auto' | 'auto') => void
}) {
  const { brands, brandId, setBrandId, accounts, channels, contents, initial, onBind, onPublished } = props
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()

  const [draft, setDraft] = useState<WizardDraft>(() => {
    const saved = loadDraft()
    const urlStep = Number(searchParams.get('step'))
    const step = (urlStep >= 1 && urlStep <= 5 ? urlStep : saved?.step || 1) as WizardStep
    return emptyDraft({ ...saved, ...initial, step })
  })
  const [publishing, setPublishing] = useState(false)
  const [savingDraft, setSavingDraft] = useState(false)
  const [pickerOpen, setPickerOpen] = useState(false)
  const [coverPickerOpen, setCoverPickerOpen] = useState(false)
  const [draftHint, setDraftHint] = useState(false)
  const [serverDraftLoaded, setServerDraftLoaded] = useState(false)
  const [loadingCloudDraft, setLoadingCloudDraft] = useState(false)

  const patch = useCallback((p: Partial<WizardDraft>) => {
    setDraft((prev) => {
      const next = { ...prev, ...p }
      saveDraft(next)
      return next
    })
  }, [])

  // URL ?step=N 同步（便于刷新回退）
  useEffect(() => {
    const cur = searchParams.get('step')
    const want = String(draft.step)
    if (cur === want) return
    const next = new URLSearchParams(searchParams)
    next.set('step', want)
    setSearchParams(next, { replace: true })
  }, [draft.step]) // eslint-disable-line react-hooks/exhaustive-deps

  // 服务端草稿（多端续填）：有 brand 时拉取，覆盖空本地草稿
  useEffect(() => {
    if (!brandId || serverDraftLoaded) return
    let cancelled = false
    setLoadingCloudDraft(true)
    businessApi.getPublishDraft(brandId).then((r) => {
      if (cancelled || !r.draft) return
      try {
        const remote = JSON.parse(r.draft) as WizardDraft
        const local = loadDraft()
        const preferRemote = !local?.title && !local?.mediaURLs?.length && !local?.content
        if (preferRemote || (!local && remote)) {
          setDraft(emptyDraft({ ...remote, brandId, ...initial }))
          saveDraft(emptyDraft({ ...remote, brandId }))
        }
      } catch { /* ignore */ }
    }).catch(() => { /* 草稿服务未开 */ }).finally(() => {
      if (!cancelled) {
        setServerDraftLoaded(true)
        setLoadingCloudDraft(false)
      }
    })
    return () => { cancelled = true }
  }, [brandId, serverDraftLoaded]) // eslint-disable-line react-hooks/exhaustive-deps

  const { data: publishConfigs = [] } = useQuery({
    queryKey: ['brand-publish-config', brandId],
    queryFn: () => businessApi.getBrandPublishConfigs(brandId!).catch(() => []),
    enabled: !!brandId,
    staleTime: 60_000,
  })

  // 人设发布配置默认标签：本地无标签时自动带入
  useEffect(() => {
    if (!brandId || draft.tags.length > 0 || publishConfigs.length === 0) return
    const merged = [...new Set(publishConfigs.flatMap((c) => c.default_tags || []).filter(Boolean))]
    if (merged.length) patch({ tags: merged.slice(0, 10) })
  }, [brandId, publishConfigs]) // eslint-disable-line react-hooks/exhaustive-deps

  // URL / 入口预填覆盖（成片、作品库）
  useEffect(() => {
    if (!initial) return
    setDraft((prev) => {
      const next = { ...prev, ...initial }
      if (initial.mediaURLs?.length || initial.contentId || initial.title) {
        if (initial.mediaURLs?.length && !initial.accountIDs?.length) next.step = 1
      }
      saveDraft(next)
      return next
    })
  }, [initial?.contentId, initial?.mediaURLs?.join(','), initial?.contentType]) // eslint-disable-line react-hooks/exhaustive-deps

  const { data: publishStats } = useQuery({
    queryKey: ['publish-stats', brandId],
    queryFn: () => businessApi.getBrandPublishStats(brandId!),
    enabled: !!brandId,
    staleTime: 60_000,
  })
  const quotaByPlatform = useMemo(() => {
    const m = new Map<string, { used_today: number; max_per_day: number; remaining: number; at_limit: boolean }>()
    for (const q of publishStats?.quotas || []) m.set(q.platform, q)
    return m
  }, [publishStats])

  const channelByPlatform = useMemo(
    () => new Map(channels.map((c) => [c.platform, c])),
    [channels],
  )
  const supportsForm = (platform: string, form: PublishForm) =>
    channelByPlatform.get(platform)?.content_types?.includes(form) ?? false
  const canAuto = (platform: string, form: PublishForm) =>
    supportsAutoForm(platform, form, channelByPlatform.get(platform))

  const formOptions = useMemo(() => {
    const all: { value: PublishForm; label: string }[] = [
      { value: 'article', label: '发文章' },
      { value: 'image', label: '发图文' },
      { value: 'video', label: '发视频' },
    ]
    const supported = new Set(channels.flatMap((c) => c.content_types || []))
    if (supported.size === 0) return all
    return all.filter((o) => supported.has(o.value))
  }, [channels])

  useEffect(() => {
    if (formOptions.length && !formOptions.some((o) => o.value === draft.contentType)) {
      patch({ contentType: formOptions[0].value })
    }
  }, [formOptions, draft.contentType, patch])

  const healthyAccounts = accounts.filter(
    (a) => a.health === 'active' && supportsForm(a.platform, draft.contentType),
  )
  const expiredAccounts = accounts.filter(
    (a) => a.health === 'expired' && supportsForm(a.platform, draft.contentType),
  )
  const selectedAccounts = accounts.filter((a) => draft.accountIDs.includes(a.id))
  const targetPlatforms = useMemo(() => {
    if (draft.autoSelect && draft.mode === 'auto') {
      return [...new Set(healthyAccounts.map((a) => a.platform))]
    }
    return [...new Set(selectedAccounts.map((a) => a.platform))]
  }, [draft.autoSelect, draft.mode, healthyAccounts, selectedAccounts])

  const platformsKey = targetPlatforms.join(',')
  const {
    data: adaptPreviews,
    isFetching: adaptLoading,
    isError: adaptError,
    refetch: refetchAdapt,
  } = useQuery({
    queryKey: ['adapt-preview', draft.title, draft.content, draft.tags.join(','), platformsKey],
    queryFn: () => businessApi.previewPublishAdapt({
      title: draft.title,
      content: draft.content,
      tags: draft.tags,
      platforms: targetPlatforms,
    }),
    enabled: draft.step === 5 && targetPlatforms.length > 0,
    staleTime: 10_000,
    retry: 1,
  })
  const adaptByPlatform = useMemo(() => {
    const m = new Map<string, { title?: string; description?: string; tags?: string[]; cta?: string; title_truncated?: boolean; error?: string }>()
    if (adaptError || !adaptPreviews?.previews?.length) {
      for (const p of localAdaptPreview(draft, targetPlatforms, channels)) {
        m.set(p.platform, p)
      }
    } else {
      for (const p of adaptPreviews.previews) m.set(p.platform, p)
    }
    return m
  }, [adaptPreviews, adaptError, draft, targetPlatforms, channels])

  const limitPlatforms = targetPlatforms.filter((p) => quotaByPlatform.get(p)?.at_limit)

  const selectedCanAuto = useMemo(() => {
    const ids = draft.accountIDs
    if (ids.length === 0) return false
    return ids.every((id) => {
      const acc = accounts.find((a) => a.id === id)
      return acc ? canAuto(acc.platform, draft.contentType) : false
    })
  }, [draft.accountIDs, draft.contentType, accounts, channelByPlatform]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (draft.mode === 'auto' && !selectedCanAuto) patch({ mode: 'semi-auto' })
  }, [draft.mode, selectedCanAuto, patch])

  // 单账号自动勾选
  useEffect(() => {
    if (healthyAccounts.length === 1 && draft.accountIDs.length === 0) {
      patch({ accountIDs: [healthyAccounts[0].id] })
    }
  }, [healthyAccounts, draft.accountIDs.length, patch])

  const selectedContent = contents.find((c) => c.id === draft.contentId)
  useEffect(() => {
    if (!selectedContent) return
    if (!draft.title && selectedContent.title) {
      patch({ title: selectedContent.title, content: selectedContent.optimized_text || draft.content })
    }
  }, [draft.contentId]) // eslint-disable-line react-hooks/exhaustive-deps

  const titleLimit = strictestTitleLimit(targetPlatforms, draft.contentType, channels)
  const titleLen = runeLen(draft.title)
  const titleOver = titleLimit > 0 && titleLen > titleLimit

  const needsBiliTags = targetPlatforms.some((p) => channelNeedsTags(p, draft.contentType, channels))
  const needsBiliCat = targetPlatforms.some((p) => channelNeedsCategory(p, draft.contentType, channels))
  const showsTopicTags = targetPlatforms.some((p) => channelShowsTags(p, draft.contentType, channels))
  const hasXhs = targetPlatforms.includes('xiaohongshu')
  const tagMax = useMemo(() => {
    let max = 0
    for (const p of targetPlatforms) {
      const m = effectiveConstraints(p, draft.contentType, channels).max_tags || 0
      if (m > 0 && (max === 0 || m < max)) max = m
    }
    return max || 10
  }, [targetPlatforms, draft.contentType, channels])
  const gaps = checkCompleteness(draft, channels, {
    accountPlatforms: targetPlatforms,
    selectedCanAuto,
  })

  const canPublish = gaps.length === 0 && (limitPlatforms.length === 0 || draft.isScheduled)

  const stepBlockers = (step: WizardStep): string | null => {
    if (step === 1 && draft.accountIDs.length === 0 && !(draft.autoSelect && draft.mode === 'auto')) {
      return '请先选择至少一个目标账号'
    }
    if (step === 2 && titleLimit > 0 && titleLen > titleLimit) {
      return `标题超限（${titleLen}/${titleLimit}），请先改短`
    }
    return null
  }

  const go = (step: WizardStep) => patch({ step })
  const next = () => {
    const block = stepBlockers(draft.step)
    if (block) {
      message.warning(block)
      return
    }
    patch({ step: Math.min(5, draft.step + 1) as WizardStep })
  }
  const prev = () => patch({ step: Math.max(1, draft.step - 1) as WizardStep })

  const handleSaveDraft = async () => {
    saveDraft(draft)
    setDraftHint(true)
    setSavingDraft(true)
    try {
      if (brandId) {
        try {
          await businessApi.savePublishDraft(brandId, JSON.stringify(draft))
          message.success('草稿已保存（本机 + 云端）')
        } catch {
          message.success('草稿已保存到本机')
        }
      } else {
        message.success('草稿已保存到本机（选人设后可同步云端）')
      }
    } finally {
      setSavingDraft(false)
      setTimeout(() => setDraftHint(false), 2000)
    }
  }

  const handleClear = async () => {
    clearDraft()
    if (brandId) {
      try { await businessApi.deletePublishDraft(brandId) } catch { /* */ }
    }
    setDraft(emptyDraft({ brandId, contentType: formOptions[0]?.value || 'article' }))
    message.info('已清空发布草稿')
  }

  const moveMedia = (index: number, dir: -1 | 1) => {
    const next = [...draft.mediaURLs]
    const j = index + dir
    if (j < 0 || j >= next.length) return
    ;[next[index], next[j]] = [next[j], next[index]]
    patch({ mediaURLs: next })
  }

  const handlePublish = async () => {
    if (gaps.length > 0) {
      message.warning(gaps[0].text)
      go(gaps[0].step)
      return
    }
    if (limitPlatforms.length > 0 && !draft.isScheduled) {
      message.warning(`${limitPlatforms.map((p) => PLATFORM_META[p]?.name || p).join('、')} 今日已达上限，请改定时明日或换平台`)
      return
    }
    setPublishing(true)
    try {
      const base = {
        brand_id: brandId || draft.brandId,
        mode: draft.mode,
        content_type: draft.contentType,
        content_id: draft.contentId,
        cover_url: draft.coverURL || undefined,
        // 以下字段 usecase/Plan-14 已设计，handler 尚未全部绑定——仍透传；正文已客户端注入兜底
        tags: draft.tags.length ? draft.tags : undefined,
        category: draft.category || undefined,
        store_address: draft.storeAddress || undefined,
        scheduled_at: draft.isScheduled && draft.scheduledAt
          ? new Date(draft.scheduledAt).toISOString()
          : undefined,
      }
      const titleFor = (platform: string) => {
        const max = effectiveConstraints(platform, draft.contentType, channels).title_max_runes || 0
        return max > 0 && draft.title ? [...draft.title].slice(0, max).join('') : draft.title
      }
      const mediaFor = (platform: string) => {
        if (draft.mediaURLs.length > 0) return draft.mediaURLs
        const c = effectiveConstraints(platform, draft.contentType, channels)
        const need = (c.min_images || 0) > 0 || (c.min_videos || 0) > 0
        return need ? draft.mediaURLs : undefined
      }

      type JobReq = Parameters<typeof businessApi.publishContent>[0]
      const requests: JobReq[] = []
      if (draft.autoSelect && draft.mode === 'auto') {
        for (const platform of targetPlatforms) {
          requests.push({
            ...base,
            account_id: '',
            platform,
            title: titleFor(platform),
            content: buildPublishContent(draft, platform),
            media_urls: mediaFor(platform),
          })
        }
      } else {
        for (const accId of draft.accountIDs) {
          const acc = accounts.find((a) => a.id === accId)
          if (!acc) continue
          requests.push({
            ...base,
            account_id: accId,
            platform: acc.platform,
            title: titleFor(acc.platform),
            content: buildPublishContent(draft, acc.platform),
            media_urls: mediaFor(acc.platform),
          })
        }
      }

      const settled = await Promise.allSettled(requests.map((req) => businessApi.publishContent(req)))
      const results: PublishJob[] = []
      let failCount = 0
      for (const s of settled) {
        if (s.status === 'fulfilled') results.push(s.value)
        else failCount += 1
      }
      if (results.length === 0) {
        message.error('全部发布请求失败，请稍后重试')
        return
      }
      clearDraft()
      if (brandId) {
        try { await businessApi.deletePublishDraft(brandId) } catch { /* */ }
      }
      onPublished(results, draft.mode)
      if (failCount > 0) {
        message.warning(`已创建 ${results.length} 个任务，另有 ${failCount} 个失败`)
      } else if (draft.mode === 'semi-auto') {
        message.success(`已生成 ${results.length} 个发布链接`)
      } else {
        message.success(`已启动 ${results.length} 个自动发布任务`)
      }
    } catch {
      /* 拦截器 */
    } finally {
      setPublishing(false)
    }
  }

  return (
    <div className="wr-glass-card ip-wizard-panel" style={{ padding: 24 }}>
      {loadingCloudDraft && (
        <div style={{ marginBottom: 12 }}>
          <Spin size="small" /> <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>正在同步云端草稿…</Text>
        </div>
      )}
      {/* 人设 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20, paddingBottom: 12, borderBottom: '1px solid var(--wr-border)' }}>
        <Text strong>人设</Text>
        <Select
          style={{ maxWidth: 320, minWidth: 200, flex: 1 }}
          placeholder="选择人设"
          value={brandId}
          onChange={(v) => { setBrandId(v); setServerDraftLoaded(false); patch({ brandId: v, contentId: undefined }) }}
          options={brands.map((b) => ({ value: b.id, label: b.name }))}
        />
        {draftHint && <Tag color="success" icon={<CloudUploadOutlined />}>草稿已存</Tag>}
      </div>

      <Steps
        size="small"
        current={draft.step - 1}
        onChange={(i) => go((i + 1) as WizardStep)}
        items={WIZARD_STEPS.map((s) => ({ title: s.short, description: s.title }))}
        style={{ marginBottom: 28 }}
      />

      {/* —— ① 平台与形态 —— */}
      {draft.step === 1 && (
        <div>
          <Text strong style={{ fontSize: 16, display: 'block', marginBottom: 4 }}>发什么 · 发到哪</Text>
          <Paragraph type="secondary" style={{ fontSize: 13, marginBottom: 16 }}>
            先定形态，再选账号——系统只展示该形态真实可用的平台
          </Paragraph>

          <div style={{ marginBottom: 20 }}>
            <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>内容形态</Text>
            <Space wrap>
              {formOptions.map((o) => {
                const active = draft.contentType === o.value
                return (
                  <div
                    key={o.value}
                    onClick={() => patch({ contentType: o.value, accountIDs: [], mediaURLs: o.value === draft.contentType ? draft.mediaURLs : [] })}
                    style={{
                      padding: '10px 18px', borderRadius: 12, cursor: 'pointer',
                      border: active ? '2px solid var(--wr-primary)' : '1px solid var(--wr-border)',
                      background: active ? 'var(--wr-primary-bg)' : 'var(--wr-bg-surface)',
                    }}
                  >
                    <Text strong>{o.label}</Text>
                  </div>
                )
              })}
            </Space>
            {channels.length > 0 && (
              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 10 }}>
                可用：{channels.filter((c) => c.content_types?.includes(draft.contentType)).map((c) => c.name || PLATFORM_META[c.platform]?.name || c.platform).join('、') || '无'}
                {' · '}全自动：{channels.filter((c) => canAuto(c.platform, draft.contentType)).map((c) => c.name || c.platform).join('、') || '暂无（半自动仍可用）'}
              </Text>
            )}
          </div>

          <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>目标账号</Text>
          {expiredAccounts.length > 0 && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
              message={`${expiredAccounts.length} 个账号登录已过期`}
              description={
                <Space wrap>
                  {expiredAccounts.slice(0, 4).map((a) => (
                    <Button key={a.id} size="small" onClick={() => onBind(a.platform)}>
                      重绑 {PLATFORM_META[a.platform]?.name || a.platform}
                    </Button>
                  ))}
                </Space>
              }
            />
          )}
          {healthyAccounts.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前形态暂无健康账号——先去绑定">
              <Space wrap>
                {Object.keys(PLATFORM_META).map((k) => (
                  <Button key={k} size="small" type="dashed" icon={<LinkOutlined />} onClick={() => onBind(k)}>
                    {PLATFORM_META[k].name}
                  </Button>
                ))}
              </Space>
            </Empty>
          ) : (
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              {healthyAccounts.map((a) => {
                const selected = draft.accountIDs.includes(a.id)
                const meta = PLATFORM_META[a.platform]
                const q = quotaByPlatform.get(a.platform)
                const atLimit = !!q?.at_limit
                return (
                  <div
                    key={a.id}
                    onClick={() => {
                      if (atLimit && !selected) {
                        message.warning(`${meta?.name || a.platform} 今日已达上限，可选中后改用定时发布`)
                      }
                      patch({
                        accountIDs: selected
                          ? draft.accountIDs.filter((x) => x !== a.id)
                          : [...draft.accountIDs, a.id],
                      })
                    }}
                    style={{
                      display: 'inline-flex', alignItems: 'center', gap: 8, padding: '10px 14px',
                      borderRadius: 14, cursor: 'pointer',
                      border: selected
                        ? (atLimit ? '2px solid var(--wr-warning)' : '2px solid var(--wr-primary)')
                        : '1px solid var(--wr-border)',
                      background: selected ? 'var(--wr-primary-bg)' : 'var(--wr-bg-surface)',
                      opacity: atLimit && !selected ? 0.85 : 1,
                      transition: 'border-color 160ms ease, background 160ms ease',
                    }}
                  >
                    <PlatformBadge platform={a.platform} size={14} />
                    <div>
                      <Text style={{ fontSize: 13, display: 'block' }}>{meta?.name || a.platform}</Text>
                      <Text type="secondary" style={{ fontSize: 11 }}>{a.display_name}</Text>
                    </div>
                    {q && q.max_per_day > 0 && (
                      <Tag color={atLimit ? 'warning' : 'default'} style={{ margin: 0, fontSize: 10 }}>
                        {q.used_today}/{q.max_per_day}
                      </Tag>
                    )}
                    {selected && <CheckCircleOutlined style={{ color: 'var(--wr-primary)' }} />}
                  </div>
                )
              })}
            </div>
          )}
        </div>
      )}

      {/* —— ② 标题正文 —— */}
      {draft.step === 2 && (
        <div>
          <Text strong style={{ fontSize: 16, display: 'block', marginBottom: 4 }}>写一份，多平台复用</Text>
          <Paragraph type="secondary" style={{ fontSize: 13, marginBottom: 16 }}>
            字数按所选平台最严红线计量；提交时服务端会再按各平台适配
          </Paragraph>

          {brandId && contents.length > 0 && (
            <div style={{ marginBottom: 16 }}>
              <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>从 GEO 文章带入（可选）</Text>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 10, maxHeight: 200, overflowY: 'auto' }}>
                {contents.slice(0, 12).map((c) => {
                  const active = draft.contentId === c.id
                  const total = c.score?.total || 0
                  return (
                    <div
                      key={c.id}
                      onClick={() => patch({
                        contentId: c.id,
                        title: c.title || draft.title,
                        content: c.optimized_text || draft.content,
                      })}
                      style={{
                        padding: 12, borderRadius: 10, cursor: 'pointer',
                        border: active ? '2px solid var(--wr-primary)' : '1px solid var(--wr-border)',
                        background: active ? 'var(--wr-primary-bg)' : 'var(--wr-bg-surface)',
                      }}
                    >
                      <Tag color={scoreColor(total)} style={{ marginBottom: 6, fontSize: 10 }}>{total.toFixed(0)}</Tag>
                      <Text ellipsis style={{ fontSize: 12, display: 'block' }}>{c.title || '(无标题)'}</Text>
                    </div>
                  )
                })}
              </div>
            </div>
          )}

          <div style={{ marginBottom: 14 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 6 }}>
              <Text strong style={{ fontSize: 13 }}>标题</Text>
              {titleLimit > 0 && (
                <Text style={{ fontSize: 12 }} type={titleOver ? 'danger' : 'secondary'}>
                  {titleLen}/{titleLimit}（最严平台）
                </Text>
              )}
            </div>
            <Input.TextArea
              rows={2}
              value={draft.title}
              onChange={(e) => patch({ title: e.target.value })}
              placeholder="一句话抓住人——小红书标题建议 ≤20 字"
              status={titleOver ? 'error' : undefined}
            />
          </div>
          <div>
            <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 6 }}>正文 / 描述</Text>
            <Input.TextArea
              rows={8}
              value={draft.content}
              onChange={(e) => patch({ content: e.target.value })}
              placeholder="支持先写 Markdown；提交时会转成平台纯文本"
              showCount
            />
          </div>
        </div>
      )}

      {/* —— ③ 素材 —— */}
      {draft.step === 3 && (
        <div>
          <Text strong style={{ fontSize: 16, display: 'block', marginBottom: 4 }}>准备素材</Text>
          <Paragraph type="secondary" style={{ fontSize: 13, marginBottom: 16 }}>
            {draft.contentType === 'video' && '视频形态需要 1 个成片（可后补，发布前会检查）'}
            {draft.contentType === 'image' && '图文需要配图（小红书首图即封面，可多选）'}
            {draft.contentType === 'article' && '长文可选手头图；没有也可直接发布'}
          </Paragraph>

          {draft.mediaURLs.length > 0 ? (
            <Space direction="vertical" style={{ width: '100%', marginBottom: 12 }} size={8}>
              {draft.mediaURLs.map((u, i) => (
                <div key={u + i} style={{ display: 'flex', gap: 8, alignItems: 'center', padding: 10, background: 'var(--wr-bg-elevated)', borderRadius: 8 }}>
                  {draft.contentType !== 'video' && u.match(/\.(jpg|jpeg|png|webp)/i) ? (
                    <img src={u} alt="" style={{ width: 48, height: 48, objectFit: 'cover', borderRadius: 6 }} />
                  ) : (
                    <Tag>{draft.contentType === 'video' ? '视频' : i === 0 ? '首图/封面' : `图 ${i + 1}`}</Tag>
                  )}
                  <Text ellipsis style={{ flex: 1, fontSize: 12 }}>{u}</Text>
                  {draft.contentType === 'image' && (
                    <Space size={0}>
                      <Button size="small" type="text" icon={<ArrowUpOutlined />} disabled={i === 0} onClick={() => moveMedia(i, -1)} />
                      <Button size="small" type="text" icon={<ArrowDownOutlined />} disabled={i === draft.mediaURLs.length - 1} onClick={() => moveMedia(i, 1)} />
                    </Space>
                  )}
                  <Button size="small" type="text" danger onClick={() => patch({ mediaURLs: draft.mediaURLs.filter((_, j) => j !== i) })}>移除</Button>
                </div>
              ))}
            </Space>
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无素材——可从素材库选，或稍后再补" style={{ marginBottom: 12 }} />
          )}

          {draft.coverURL && (
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 12 }}
              message={`封面：${draft.coverURL}`}
              action={<Button size="small" onClick={() => patch({ coverURL: '' })}>清除</Button>}
            />
          )}

          <Space wrap>
            <Button type="primary" className="ip-btn-primary" onClick={() => setPickerOpen(true)}>
              {draft.mediaURLs.length ? '更换 / 追加素材' : '从素材库选择'}
            </Button>
            {(draft.contentType === 'video' || draft.contentType === 'article') && (
              <Button onClick={() => setCoverPickerOpen(true)}>
                {draft.coverURL ? '更换封面' : '选择封面（可选）'}
              </Button>
            )}
            <Button onClick={() => navigate('/m/assets')}>去素材库生成</Button>
            <Button onClick={() => navigate('/m/compose/lipsync')}>去拍口播</Button>
          </Space>
        </div>
      )}

      {/* —— ④ 平台参数 —— */}
      {draft.step === 4 && (
        <div>
          <Text strong style={{ fontSize: 16, display: 'block', marginBottom: 4 }}>平台差异与发布方式</Text>
          <Paragraph type="secondary" style={{ fontSize: 13, marginBottom: 16 }}>
            只展示所选平台真正需要的参数——多一项都不浪费你的注意力
          </Paragraph>

          {(needsBiliCat || needsBiliTags) && (
            <div style={{ padding: 14, borderRadius: 12, background: 'var(--wr-bg-elevated)', border: '1px solid var(--wr-border)', marginBottom: 16 }}>
              <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 12 }}>B站必填</Text>
              {needsBiliCat && (
                <div style={{ marginBottom: 12 }}>
                  <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>分区</Text>
                  <Select
                    style={{ width: '100%', maxWidth: 280 }}
                    value={draft.category || '生活'}
                    onChange={(v) => patch({ category: v })}
                    options={BILIBILI_CATEGORIES.map((c) => ({ value: c, label: c }))}
                  />
                </div>
              )}
              {needsBiliTags && (
                <div>
                  <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>标签（至少 1 个）</Text>
                  <Select
                    mode="tags"
                    style={{ width: '100%' }}
                    placeholder="输入后回车，如：本地生活"
                    value={draft.tags}
                    onChange={(v) => patch({ tags: v.slice(0, tagMax) })}
                    tokenSeparators={[',', ' ']}
                  />
                </div>
              )}
            </div>
          )}

          {showsTopicTags && !needsBiliTags && (
            <div style={{ marginBottom: 16 }}>
              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>
                话题 / 标签（可选，最多 {tagMax} 个）
              </Text>
              <Select
                mode="tags"
                style={{ width: '100%' }}
                placeholder="输入话题后回车"
                value={draft.tags}
                onChange={(v) => patch({ tags: v.slice(0, tagMax) })}
                tokenSeparators={[',', ' ', '#']}
              />
            </div>
          )}

          <div style={{ marginBottom: 14 }}>
            <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>门店地址（可选，正文尾部追加定位提示）</Text>
            <Input
              placeholder="如：上海市徐汇区…"
              value={draft.storeAddress}
              onChange={(e) => patch({ storeAddress: e.target.value })}
            />
          </div>

          <div style={{ marginBottom: 14, display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
            <Switch
              checked={draft.mode === 'auto'}
              disabled={!selectedCanAuto}
              onChange={(v) => patch({ mode: v ? 'auto' : 'semi-auto' })}
            />
            <Text style={{ fontSize: 13 }}>
              {selectedCanAuto
                ? '全自动发布（浏览器代发，有风控风险）'
                : '全自动不可用——当前账号×形态仅半自动'}
            </Text>
          </div>
          {hasXhs && draft.mode === 'auto' && (
            <Alert type="warning" showIcon style={{ marginBottom: 14 }} message="小红书审核严格，建议优先半自动" />
          )}
          {draft.mode === 'auto' && selectedCanAuto && (
            <div style={{ marginBottom: 14, display: 'flex', alignItems: 'center', gap: 10 }}>
              <Switch checked={draft.autoSelect} onChange={(v) => patch({ autoSelect: v })} />
              <Text style={{ fontSize: 13 }}>自动选号（最久未发账号优先）</Text>
            </div>
          )}

          <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
            <Switch
              checked={draft.isScheduled}
              onChange={(v) => patch({ isScheduled: v })}
              checkedChildren="定时"
              unCheckedChildren="立即"
            />
            {draft.isScheduled && (
              <input
                type="datetime-local"
                value={draft.scheduledAt}
                onChange={(e) => patch({ scheduledAt: e.target.value })}
                min={new Date().toISOString().slice(0, 16)}
                style={{
                  marginLeft: 4,
                  padding: '6px 10px',
                  borderRadius: 8,
                  border: '1px solid var(--wr-border)',
                  background: 'var(--wr-bg-surface)',
                  color: 'var(--wr-text-primary)',
                  fontSize: 13,
                }}
              />
            )}
          </div>
          {draft.isScheduled && (
            <Alert
              type="warning"
              showIcon
              style={{ marginTop: 12 }}
              message="定时发布接口尚未接线（handler 未透传 scheduled_at），请改回立即发布"
            />
          )}
        </div>
      )}

      {/* —— ⑤ 预览确认 —— */}
      {draft.step === 5 && (
        <div>
          <Text strong style={{ fontSize: 16, display: 'block', marginBottom: 4 }}>适配预览 · 确认发布</Text>
          <Paragraph type="secondary" style={{ fontSize: 13, marginBottom: 16 }}>
            {adaptError
              ? '服务端适配暂不可用，下方为本地截断预览'
              : '服务端 ContentAdapter 真实适配（标题截断 / 描述 / 标签 / CTA）'}
          </Paragraph>

          {adaptError && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
              message="已用本地规则预览标题截断"
              description="发布仍会走服务端适配；可稍后重试预览"
              action={<Button size="small" onClick={() => refetchAdapt()}>重试</Button>}
            />
          )}

          <Spin spinning={adaptLoading} tip="正在适配各平台文案…">

          {limitPlatforms.length > 0 && (
            <Alert
              type="error"
              showIcon
              style={{ marginBottom: 16 }}
              message="部分平台今日已达发布上限"
              description={
                <span>
                  {limitPlatforms.map((p) => {
                    const q = quotaByPlatform.get(p)
                    return `${PLATFORM_META[p]?.name || p} ${q?.used_today}/${q?.max_per_day}`
                  }).join(' · ')}
                  ——建议改「定时明日」或换平台
                </span>
              }
            />
          )}

          {gaps.length > 0 ? (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
              message="发布前还有缺口"
              description={
                <Space direction="vertical" size={4}>
                  {gaps.map((g, i) => (
                    <a key={i} onClick={() => go(g.step)} style={{ fontSize: 13 }}>
                      · {g.text}（去第 {g.step} 步）
                    </a>
                  ))}
                </Space>
              }
            />
          ) : (
            <Alert type="success" showIcon style={{ marginBottom: 16 }} message="完备性检查通过，可以发布" />
          )}

          <div style={{ display: 'grid', gap: 10, marginBottom: 16 }}>
            {targetPlatforms.map((p) => {
              const ch = channelByPlatform.get(p)
              const adapted = adaptByPlatform.get(p)
              const q = quotaByPlatform.get(p)
              return (
                <div key={p} style={{ padding: 14, borderRadius: 10, background: 'var(--wr-bg-elevated)', border: q?.at_limit ? '1px solid var(--wr-danger)' : '1px solid var(--wr-border)' }}>
                  <Space style={{ marginBottom: 6 }} wrap>
                    <PlatformBadge platform={p} size={14} />
                    <Text strong style={{ fontSize: 13 }}>{ch?.name || PLATFORM_META[p]?.name || p}</Text>
                    <Tag style={{ margin: 0 }}>{draft.mode === 'auto' ? '全自动' : '半自动'}</Tag>
                    {adapted?.title_truncated && <Tag color="orange" style={{ margin: 0 }}>标题已截断</Tag>}
                    {q && q.max_per_day > 0 && (
                      <Tag color={q.at_limit ? 'error' : 'default'} style={{ margin: 0 }}>
                        今日 {q.used_today}/{q.max_per_day}
                      </Tag>
                    )}
                  </Space>
                  {adapted?.error ? (
                    <Text type="danger" style={{ fontSize: 12 }}>{adapted.error}</Text>
                  ) : (
                    <>
                      <Text style={{ fontSize: 14, display: 'block', marginBottom: 4 }}>{adapted?.title || draft.title || '(无标题)'}</Text>
                      <Text type="secondary" style={{ fontSize: 12, display: 'block', whiteSpace: 'pre-wrap' }}>
                        {(adapted?.description || draft.content || '').slice(0, 180)}
                        {(adapted?.description || draft.content || '').length > 180 ? '…' : ''}
                      </Text>
                      {(adapted?.tags?.length || 0) > 0 && (
                        <Space size={4} wrap style={{ marginTop: 6 }}>
                          {adapted!.tags!.map((t) => <Tag key={t} style={{ margin: 0 }}>{t}</Tag>)}
                        </Space>
                      )}
                    </>
                  )}
                  {draft.mediaURLs.length > 0 && (
                    <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 6 }}>
                      素材 ×{draft.mediaURLs.length}{draft.coverURL ? ' · 有封面' : ''}
                    </Text>
                  )}
                </div>
              )
            })}
            {targetPlatforms.length === 0 && (
              <Empty description="还没有目标平台" />
            )}
          </div>
          </Spin>

          <Button
            type="primary"
            size="large"
            block
            className="ip-btn-primary"
            loading={publishing}
            disabled={!canPublish}
            onClick={handlePublish}
          >
            {publishing ? '提交中…' : draft.isScheduled ? '定时发布' : `确认发布到 ${targetPlatforms.length || draft.accountIDs.length} 个平台`}
          </Button>
        </div>
      )}

      {/* 底部状态栏 */}
      <div style={{
        marginTop: 28, paddingTop: 16, borderTop: '1px solid var(--wr-border)',
        display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 10,
      }}>
        <Space>
          <Button icon={<CloudUploadOutlined />} loading={savingDraft} onClick={handleSaveDraft}>保存草稿</Button>
          <Button type="text" onClick={handleClear}>清空</Button>
        </Space>
        <Space>
          <Button disabled={draft.step <= 1} onClick={prev}>上一步</Button>
          {draft.step < 5 ? (
            <Button type="primary" className="ip-btn-primary" onClick={next}>下一步</Button>
          ) : (
            <Button type="primary" className="ip-btn-primary" loading={publishing} disabled={!canPublish} onClick={handlePublish}>
              发布
            </Button>
          )}
        </Space>
      </div>

      <AssetPicker
        open={pickerOpen}
        mode="multi"
        accept={draft.contentType === 'video' ? 'video' : 'image'}
        title={draft.contentType === 'video' ? '选择视频素材' : '选择配图'}
        max={draft.contentType === 'video' ? 1 : 18}
        onClose={() => setPickerOpen(false)}
        onSelect={(assets) => {
          const urls = assets.map((a) => a.url)
          patch({
            mediaURLs: draft.contentType === 'video' ? urls.slice(0, 1) : [...draft.mediaURLs, ...urls].slice(0, 18),
          })
          setPickerOpen(false)
        }}
      />
      <AssetPicker
        open={coverPickerOpen}
        mode="single"
        accept="image"
        title="选择封面图"
        max={1}
        onClose={() => setCoverPickerOpen(false)}
        onSelect={(assets) => {
          if (assets[0]?.url) patch({ coverURL: assets[0].url })
          setCoverPickerOpen(false)
        }}
      />
    </div>
  )
}
