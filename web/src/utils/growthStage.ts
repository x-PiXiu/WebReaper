import { GROWTH_STAGES } from '../config/product'

export type GrowthStageKey = (typeof GROWTH_STAGES)[number]['key']

/** 根据商户数据推断当前应强调的闭环阶段 */
export function inferGrowthStage(opts: {
  brandCount: number
  hasContent: boolean
  readyCount: number
  publishedCount: number
}): GrowthStageKey {
  if (opts.brandCount === 0) return 'persona'
  if (!opts.hasContent && opts.publishedCount === 0) return 'create'
  if (opts.readyCount > 0) return 'publish'
  return 'measure'
}
