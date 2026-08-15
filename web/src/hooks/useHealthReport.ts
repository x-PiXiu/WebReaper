import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import type { HealthReportView } from '../types/api'

export const HEALTH_REPORT_KEY = ['geo-health-report'] as const

/**
 * GEO 健康报告（后端聚合单一事实源：总分/五指数/环比/竞品对标/品牌级分值）。
 *
 * v3 归位：此前健康分在前端 geoHealth.ts 被三处以不同口径各自合成
 * （列表徽章/工作区头部/工作台卡片数字不同、逐品牌 N+1、口径漂移）。
 * 现统一由 GET /geo/health-report 出全量口径；接口不可用时返回 null——
 * 调用方降级到本地 geoHealth 合成（灰度兼容：旧后端无此端点仍可工作）。
 */
export function useHealthReport() {
  const { data, isLoading } = useQuery<HealthReportView | null>({
    queryKey: [...HEALTH_REPORT_KEY],
    queryFn: () => businessApi.getHealthReport().catch(() => null),
    staleTime: 60_000,
  })
  return { report: data ?? null, isLoading }
}
