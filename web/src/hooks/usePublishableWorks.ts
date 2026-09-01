import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import { filterPublishableWorks } from '../utils/publishableWorks'
import { GENERATION_TASKS_KEY } from './useGenerationTasks'

/** 我的作品：过滤掉素材库 AI 产物，仅保留文章与可发布成片 */
export function usePublishableWorks(options?: { enabled?: boolean; staleTime?: number }) {
  const enabled = options?.enabled !== false

  const worksQuery = useQuery({
    queryKey: ['merchant-works'],
    // 错误不吞：失败要冒泡给页面（QueryBoundary 出错重试），吞成 [] 会把"服务挂了"伪装成"没有作品"
    queryFn: () => businessApi.listWorks(),
    staleTime: options?.staleTime,
    enabled,
  })

  const tasksQuery = useQuery({
    queryKey: GENERATION_TASKS_KEY,
    queryFn: () => businessApi.listGenerationTasks().then((r) => r.tasks).catch(() => []),
    staleTime: 30_000,
    enabled,
  })

  const works = useMemo(
    () => filterPublishableWorks(worksQuery.data ?? [], tasksQuery.data ?? []),
    [worksQuery.data, tasksQuery.data],
  )

  return {
    works,
    tasks: tasksQuery.data ?? [],
    isLoading: worksQuery.isLoading,
    isError: worksQuery.isError,
    isFetching: worksQuery.isFetching || tasksQuery.isFetching,
    refetch: worksQuery.refetch,
  }
}
