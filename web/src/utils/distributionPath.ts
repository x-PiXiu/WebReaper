import type { WorkItem } from '../types/api'

/** 从作品库/详情跳转账号发布时的查询串（成片预填） */
export function distributionPathFromWork(w: Pick<WorkItem, 'content_id' | 'media_urls' | 'cover_url' | 'brand_id' | 'kind' | 'title'>): string {
  const q = new URLSearchParams()
  if (w.content_id) q.set('contentId', w.content_id)
  if (w.media_urls?.length) q.set('mediaUrls', w.media_urls.join(','))
  if (w.cover_url) q.set('coverUrl', w.cover_url)
  if (w.brand_id) q.set('brandId', w.brand_id)
  q.set('contentType', w.kind === 'article' ? 'article' : w.kind === 'image' ? 'image' : 'video')
  if (w.title?.trim()) q.set('title', w.title.trim())
  const s = q.toString()
  return s ? `/m/distribution?${s}` : '/m/distribution'
}

/** 口播成片完成后跳发布 */
export function distributionPathFromLipSync(opts: {
  brandId?: string
  mediaUrl?: string
  coverUrl?: string
  title?: string
  content?: string
}): string {
  const q = new URLSearchParams()
  if (opts.brandId) q.set('brandId', opts.brandId)
  if (opts.mediaUrl) q.set('mediaUrls', opts.mediaUrl)
  if (opts.coverUrl) q.set('coverUrl', opts.coverUrl)
  q.set('contentType', 'video')
  if (opts.title?.trim()) q.set('title', opts.title.trim())
  if (opts.content?.trim()) q.set('content', opts.content.trim().slice(0, 8000))
  return `/m/distribution?${q.toString()}`
}
