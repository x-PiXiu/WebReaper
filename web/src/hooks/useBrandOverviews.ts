import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import type { Brand, BrandOverview } from '../types/api'

// 品牌可见度扇出查询：brands + Promise.all(getBrandOverview)。
// 此前 Home 与 Visibility 原样复制两份——收敛为唯一实现（一处修处处修）。

export const OVERVIEWS_KEY = ['geo-overviews'] as const

export function useBrandOverviews(brands: Brand[], enabled = true) {
  const ids = brands.map((b) => b.id).join(',')
  return useQuery<BrandOverview[]>({
    queryKey: [...OVERVIEWS_KEY, ids],
    queryFn: async () => {
      const results = await Promise.all(
        brands.map((b) => businessApi.getBrandOverview(b.id, b.name).catch(() => null)),
      )
      return results.filter(Boolean) as BrandOverview[]
    },
    enabled: enabled && brands.length > 0,
  })
}
