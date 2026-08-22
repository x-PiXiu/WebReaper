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

export async function fetchMediaAssets(): Promise<MediaAsset[]> {
  const res = await businessApi.listAssets().catch(() => ({ assets: [] as MediaAsset[] }))
  return normalizeMediaAssets(res)
}

/** 素材库列表（全站共用 queryKey，返回 MediaAsset[]） */
export function useMediaAssets(enabled = true) {
  return useQuery({
    queryKey: MEDIA_ASSETS_QUERY_KEY,
    queryFn: fetchMediaAssets,
    enabled,
    select: normalizeMediaAssets,
  })
}
