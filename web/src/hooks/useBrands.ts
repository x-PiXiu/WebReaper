import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import { useBrandStore } from '../store/brand'
import type { Brand } from '../types/api'

// 品牌数据契约（前端"用例层"）：
// queryKey 是前端版本的 port——同一数据全站只有一个 key，读写从这里走。

export const BRANDS_KEY = ['geo-brands'] as const

/** 品牌列表（全站唯一查询入口，与布局/搜索/各业务页共享缓存）。 */
export function useBrands() {
  return useQuery({
    queryKey: BRANDS_KEY,
    queryFn: () => businessApi.listBrands(),
  })
}

/**
 * 品牌上下文：全局当前品牌 + 自动兜底。
 *
 * 规则（纯派生，不写 store）：
 *   - store 中的品牌已不存在（被删除）→ 回落到第一个品牌
 *   - store 为空且已有品牌 → 默认第一个（新会话/老用户直接可用，不再"请先选择品牌"）
 * 用户显式切换时才写入 store。
 */
export function useBrandContext(): {
  brands: Brand[]
  isLoading: boolean
  brandId: string | undefined
  setCurrentBrand: (id: string | null) => void
} {
  const { data: brands = [], isLoading } = useBrands()
  const currentBrandId = useBrandStore((s) => s.currentBrandId)
  const setCurrentBrand = useBrandStore((s) => s.setCurrentBrand)

  const isValid = brands.some((b) => b.id === currentBrandId)
  const brandId = isValid ? (currentBrandId as string) : brands[0]?.id

  return { brands, isLoading, brandId, setCurrentBrand }
}
