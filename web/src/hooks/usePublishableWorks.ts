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
    queryFn: () => businessApi.listWorks().catch(() => []),
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
    isLoading: worksQuery.isLoading,
    isFetching: worksQuery.isFetching || tasksQuery.isFetching,
    refetch: worksQuery.refetch,
  }
}
