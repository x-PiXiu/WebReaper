import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import type { MediaAsset } from '../types/api'

export const MEDIA_ASSETS_QUERY_KEY = ['media-assets'] as const

/** 统一解析 listAssets 响应（兼容历史缓存中的嵌套对象） */
export function normalizeMediaAssets(data: unknown): MediaAsset[] {
  if (Array.isArray(data)) return data
  if (data && typeof data === 'object' && Array.isArray((data as { assets?: unknown }).assets)) {
    return (data as { assets: MediaAsset[] }).assets
  }
  return []
}

export async function fetchMediaAssets(owner?: 'material' | 'creation' | 'all'): Promise<MediaAsset[]> {
  const res = await businessApi.listAssets(owner).catch(() => ({ assets: [] as MediaAsset[] }))
  return normalizeMediaAssets(res)
}

/** 素材库列表（queryKey 含 owner——素材/产物分开缓存）。
 * owner 缺省 material（配图/配音等生成素材场景，向后兼容）；
 * 'all' 含 AI 产物（成片视频）——发布向导选发视频用。 */
export function useMediaAssets(enabled = true, owner: 'material' | 'creation' | 'all' = 'material') {
  return useQuery({
    queryKey: ['media-assets', owner],
    queryFn: () => fetchMediaAssets(owner),
    enabled,
    select: normalizeMediaAssets,
  })
}
