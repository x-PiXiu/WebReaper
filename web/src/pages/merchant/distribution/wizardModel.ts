import type { PublishChannelConstraints, PublishChannelView } from '../../../types/api'

export type PublishForm = 'article' | 'image' | 'video'
export type WizardStep = 1 | 2 | 3 | 4 | 5

export interface WizardDraft {
  step: WizardStep
  contentType: PublishForm
  title: string
  content: string
  mediaURLs: string[]
  coverURL: string
  accountIDs: string[]
  contentId?: string
  brandId?: string
  tags: string[]
  category: string
  storeAddress: string
  mode: 'semi-auto' | 'auto'
  autoSelect: boolean
  scheduledAt: string
  isScheduled: boolean
}

export interface CompletenessGap {
  step: WizardStep
  text: string
}

export const WIZARD_STEPS: { key: WizardStep; title: string; short: string }[] = [
  { key: 1, title: '平台与形态', short: '发哪里' },
  { key: 2, title: '标题与正文', short: '写什么' },
  { key: 3, title: '素材', short: '带什么' },
  { key: 4, title: '平台参数', short: '怎么发' },
  { key: 5, title: '预览确认', short: '确认发' },
]

export const BILIBILI_CATEGORIES = [
  '生活', '知识', '科技', '美食', '游戏', '娱乐', '动画', '音乐', '舞蹈', '运动', '汽车', '时尚',
]

/**
 * Plan-14 诚实矩阵兜底（与 Docs/Plans/14 第三章同源）。
 * 服务端 ChannelConstraints 目前只有 title/min_images/min_videos；
 * require_tags / require_category / max_tags / 全自动形态尚未下发时，用本表补齐——「不撒谎」。
 */
type MatrixCell = Partial<PublishChannelConstraints> & { auto?: boolean }

export const CAPABILITY_FALLBACK: Record<string, Partial<Record<PublishForm, MatrixCell>>> = {
  douyin: {
    video: { title_max_runes: 55, min_videos: 1, max_tags: 3 },
    image: { title_max_runes: 55, min_images: 1 },
  },
  kuaishou: {
    video: { title_max_runes: 80, min_videos: 1, max_tags: 2 },
  },
  xiaohongshu: {
    image: { title_max_runes: 20, min_images: 1, auto: true },
    video: { title_max_runes: 20, min_videos: 1 },
    article: { title_max_runes: 20 },
  },
  weixin: {
    video: { title_max_runes: 40, min_videos: 1 },
  },
  bilibili: {
    video: {
      title_max_runes: 80,
      min_videos: 1,
      require_tags: true,
      require_category: true,
      max_tags: 10,
    },
  },
  zhihu: {
    article: { title_max_runes: 100, auto: true },
  },
}

const DRAFT_KEY = 'wr_publish_wizard_v1'

export function emptyDraft(partial?: Partial<WizardDraft>): WizardDraft {
  return {
    step: 1,
    // 默认 video：产品主流程是视频获客（口播向导/成片分发）——此前默认 article，
    // "发视频"入口未带形态参数时落进"发文章"高亮（用户实测困惑）。
    contentType: 'video',
    title: '',
    content: '',
    mediaURLs: [],
    coverURL: '',
    accountIDs: [],
    tags: [],
    category: '生活',
    storeAddress: '',
    mode: 'semi-auto',
    autoSelect: false,
    scheduledAt: '',
    isScheduled: false,
    ...partial,
  }
}

export function loadDraft(): WizardDraft | null {
  try {
    const raw = localStorage.getItem(DRAFT_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as WizardDraft
    if (!parsed || typeof parsed !== 'object') return null
    return emptyDraft(parsed)
  } catch {
    return null
  }
}

export function saveDraft(d: WizardDraft) {
  try {
    localStorage.setItem(DRAFT_KEY, JSON.stringify(d))
  } catch { /* quota */ }
}

export function clearDraft() {
  try {
    localStorage.removeItem(DRAFT_KEY)
  } catch { /* */ }
}

export function runeLen(s: string) {
  return [...(s || '')].length
}

/**
 * 合并服务端 Constraints + Plan-14 矩阵。
 * 服务端有值优先；缺字段用矩阵补齐（能力声明驱动，矩阵仅作诚实兜底）。
 */
export function effectiveConstraints(
  platform: string,
  contentType: PublishForm,
  channels: PublishChannelView[],
): PublishChannelConstraints {
  const server = channels.find((c) => c.platform === platform)?.constraints?.[contentType] || {}
  const fb = CAPABILITY_FALLBACK[platform]?.[contentType] || {}
  return {
    title_max_runes: server.title_max_runes || fb.title_max_runes || 0,
    min_images: server.min_images || fb.min_images || 0,
    min_videos: server.min_videos || fb.min_videos || 0,
    require_tags: server.require_tags ?? fb.require_tags ?? false,
    require_category: server.require_category ?? fb.require_category ?? false,
    max_tags: server.max_tags || fb.max_tags || 0,
  }
}

/** 所选平台中当前形态的最严标题上限（0=不限） */
export function strictestTitleLimit(
  platforms: string[],
  contentType: PublishForm,
  channels: PublishChannelView[],
): number {
  let limit = 0
  for (const p of platforms) {
    const max = effectiveConstraints(p, contentType, channels).title_max_runes || 0
    if (max > 0 && (limit === 0 || max < limit)) limit = max
  }
  return limit
}

export function channelNeedsTags(platform: string, contentType: PublishForm, channels: PublishChannelView[]) {
  return !!effectiveConstraints(platform, contentType, channels).require_tags
}

export function channelNeedsCategory(platform: string, contentType: PublishForm, channels: PublishChannelView[]) {
  return !!effectiveConstraints(platform, contentType, channels).require_category
}

export function channelShowsTags(platform: string, contentType: PublishForm, channels: PublishChannelView[]) {
  const cs = effectiveConstraints(platform, contentType, channels)
  return !!(cs.require_tags || (cs.max_tags && cs.max_tags > 0))
}

/**
 * 全自动是否可用（Plan-14「不撒谎」）。
 * 优先消费服务端 auto_content_types；未下发时仅信任矩阵中 auto=true 的格子。
 */
export function supportsAutoForm(
  platform: string,
  contentType: PublishForm,
  ch: PublishChannelView | undefined,
): boolean {
  if (ch?.auto_content_types?.length) {
    return !!ch.auto && ch.auto_content_types.includes(contentType)
  }
  return !!CAPABILITY_FALLBACK[platform]?.[contentType]?.auto
}

export function adaptPreviewTitle(title: string, max: number) {
  if (max <= 0) return title
  const chars = [...title]
  if (chars.length <= max) return title
  return chars.slice(0, max).join('') + '…'
}

/** 本地适配预览（服务端 adapt-preview 已上线——本地降级仅做网络故障兜底） */
export function localAdaptPreview(
  draft: WizardDraft,
  platforms: string[],
  channels: PublishChannelView[],
): Array<{ platform: string; title: string; description: string; tags: string[]; title_truncated: boolean }> {
  return platforms.map((platform) => {
    const max = effectiveConstraints(platform, draft.contentType, channels).title_max_runes || 0
    const title = adaptPreviewTitle(draft.title, max)
    return {
      platform,
      title,
      description: draft.content,
      tags: draft.tags,
      title_truncated: max > 0 && runeLen(draft.title) > max,
    }
  })
}

/**
 * 组装发布正文：门店地址等仍可在正文尾部追加（tags/category 已由 publish handler 独立字段消费）。
 */
export function buildPublishContent(draft: WizardDraft, platform: string): string {
  let body = draft.content || ''
  if (draft.storeAddress.trim()) {
    body = `${body.replace(/\s+$/, '')}\n\n📍 门店地址：${draft.storeAddress.trim()}`
  }
  if (platform === 'bilibili' && draft.category.trim()) {
    body = `${body.replace(/\s+$/, '')}\n\n【分区】${draft.category.trim()}`
  }
  if (draft.tags.length > 0) {
    const tagLine = draft.tags.map((t) => (t.startsWith('#') ? t : `#${t}`)).join(' ')
    body = `${body.replace(/\s+$/, '')}\n\n${tagLine}`
  }
  return body
}

/** 发布前完备性检查 */
export function checkCompleteness(
  draft: WizardDraft,
  channels: PublishChannelView[],
  opts: {
    accountPlatforms: string[]
    selectedCanAuto: boolean
  },
): CompletenessGap[] {
  const gaps: CompletenessGap[] = []
  const { accountPlatforms, selectedCanAuto } = opts

  if (accountPlatforms.length === 0 && !draft.autoSelect) {
    gaps.push({ step: 1, text: '请选择至少一个目标账号' })
  }

  const titleLimit = strictestTitleLimit(accountPlatforms, draft.contentType, channels)
  const tLen = runeLen(draft.title)
  if (titleLimit > 0) {
    if (tLen === 0) gaps.push({ step: 2, text: '请填写标题' })
    else if (tLen > titleLimit) gaps.push({ step: 2, text: `标题超限（${tLen}/${titleLimit}）` })
  } else if (!draft.title.trim() && !draft.content.trim() && draft.mediaURLs.length === 0) {
    gaps.push({ step: 2, text: '请填写标题或正文，或准备素材' })
  }

  if (draft.contentType === 'article' && !draft.content.trim() && !draft.title.trim()) {
    gaps.push({ step: 2, text: '文章请至少填写标题或正文' })
  }

  let minImages = 0
  let minVideos = 0
  let needTags = false
  let needCategory = false
  let maxTags = 0
  for (const p of accountPlatforms) {
    const cs = effectiveConstraints(p, draft.contentType, channels)
    minImages = Math.max(minImages, cs.min_images || 0)
    minVideos = Math.max(minVideos, cs.min_videos || 0)
    if (cs.require_tags) needTags = true
    if (cs.require_category) needCategory = true
    if ((cs.max_tags || 0) > 0) {
      maxTags = maxTags ? Math.min(maxTags, cs.max_tags!) : cs.max_tags!
    }
  }

  if (draft.contentType === 'image' || minImages > 0) {
    const need = Math.max(1, minImages)
    if (draft.mediaURLs.length < need) {
      gaps.push({ step: 3, text: `图文至少需要 ${need} 张图（当前 ${draft.mediaURLs.length}）` })
    }
  }
  if (draft.contentType === 'video' || minVideos > 0) {
    const need = Math.max(1, minVideos)
    if (draft.mediaURLs.length < need) {
      gaps.push({ step: 3, text: `视频至少需要 ${need} 个文件（当前 ${draft.mediaURLs.length}）` })
    }
  }

  if (needCategory && !draft.category.trim()) {
    gaps.push({ step: 4, text: 'B站等平台需要选择分区' })
  }
  if (needTags && draft.tags.length === 0) {
    gaps.push({ step: 4, text: 'B站等平台至少填写 1 个标签' })
  }
  if (maxTags > 0 && draft.tags.length > maxTags) {
    gaps.push({ step: 4, text: `标签过多（${draft.tags.length}/${maxTags}）` })
  }

  if (draft.mode === 'auto' && !selectedCanAuto) {
    gaps.push({ step: 4, text: '所选平台当前形态不支持全自动，请改用半自动' })
  }
  // 定时发布：后端已全链路支持（scheduled_at 绑定 + 落库排期 + 定时任务执行），
  // 此前的"接口尚未接线"自锁已移除。

  return gaps
}
