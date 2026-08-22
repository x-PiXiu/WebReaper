/** 社交平台元数据（标签、品牌色、图标来源） */
export type PlatformKey =
  | 'douyin'
  | 'kuaishou'
  | 'xiaohongshu'
  | 'bilibili'
  | 'weishi'
  | 'shipinhao'
  | 'channels'
  | 'wechat'
  | 'zhihu'
  | 'weibo'
  | 'qq'
  | 'web'

export type PlatformMeta = {
  label: string
  color: string
  /** 徽章浅底 */
  bg: string
}

/** 图标来源：simple-icons（https://simpleicons.org）或官网静态资源 */
export type PlatformIconSource =
  | { type: 'simple-icons'; slug: string }
  | { type: 'official-asset'; src: string; note?: string }

const META: Record<string, PlatformMeta> = {
  douyin: { label: '抖音', color: '#000000', bg: 'rgba(0,0,0,0.08)' },
  kuaishou: { label: '快手', color: '#FF4906', bg: 'rgba(255,73,6,0.12)' },
  xiaohongshu: { label: '小红书', color: '#FF2442', bg: 'rgba(255,36,66,0.12)' },
  bilibili: { label: 'B站', color: '#00A1D6', bg: 'rgba(0,161,214,0.12)' },
  weishi: { label: '微视', color: '#1E80FF', bg: 'rgba(30,128,255,0.12)' },
  shipinhao: { label: '视频号', color: '#FA9D3B', bg: 'rgba(250,157,59,0.14)' },
  channels: { label: '视频号', color: '#FA9D3B', bg: 'rgba(250,157,59,0.14)' },
  wechat_channels: { label: '视频号', color: '#FA9D3B', bg: 'rgba(250,157,59,0.14)' },
  wechat: { label: '微信', color: '#07C160', bg: 'rgba(7,193,96,0.12)' },
  zhihu: { label: '知乎', color: '#0084FF', bg: 'rgba(0,132,255,0.12)' },
  weibo: { label: '微博', color: '#E6162D', bg: 'rgba(230,22,45,0.12)' },
  qq: { label: 'QQ', color: '#12B7F5', bg: 'rgba(18,183,245,0.12)' },
  web: { label: '网页', color: '#64748b', bg: 'rgba(100,116,139,0.12)' },
}

/** simple-icons slug 映射（CC0 品牌矢量，来源 https://github.com/simple-icons/simple-icons） */
const SIMPLE_ICON_SLUG: Record<string, string> = {
  kuaishou: 'siKuaishou',
  xiaohongshu: 'siXiaohongshu',
  bilibili: 'siBilibili',
  wechat: 'siWechat',
  weibo: 'siSinaweibo',
  qq: 'siQq',
  zhihu: 'siZhihu',
}

/** 官网静态资源（抖音无 simple-icons 条目，使用 douyin.com favicon） */
const OFFICIAL_ASSET: Record<string, PlatformIconSource> = {
  douyin: {
    type: 'official-asset',
    src: '/platform-icons/douyin.ico',
    note: '来源：https://www.douyin.com/favicon.ico',
  },
}

const ALIASES: Record<string, string> = {
  tiktok: 'douyin',
  xhs: 'xiaohongshu',
  redbook: 'xiaohongshu',
  b站: 'bilibili',
  weixin: 'wechat',
  wx: 'wechat',
  weixin_channels: 'shipinhao',
  wechat_channels: 'shipinhao',
  video_account: 'shipinhao',
  视频号: 'shipinhao',
  sinaweibo: 'weibo',
}

export function normalizePlatform(platform?: string): string {
  if (!platform) return 'web'
  const key = platform.trim().toLowerCase()
  return ALIASES[key] || key
}

export function getPlatformMeta(platform?: string): PlatformMeta {
  const key = normalizePlatform(platform)
  return META[key] || { label: platform || '社交', color: '#64748b', bg: 'rgba(100,116,139,0.12)' }
}

export function getPlatformLabel(platform?: string): string {
  return getPlatformMeta(platform).label
}

export function getPlatformIconSource(platform?: string): PlatformIconSource | null {
  const key = normalizePlatform(platform)
  if (OFFICIAL_ASSET[key]) return OFFICIAL_ASSET[key]
  const slug = SIMPLE_ICON_SLUG[key]
  if (slug) return { type: 'simple-icons', slug }
  return null
}

export const PLATFORM_OPTIONS = Object.entries(META)
  .filter(([k]) => !['channels', 'wechat_channels'].includes(k))
  .map(([value, m]) => ({ value, label: m.label }))
