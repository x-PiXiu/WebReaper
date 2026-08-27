import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../api/business'

/** 服务端已启用的生成端点 / 模型（listGenerationTypes） */
export function useGenerationTypes() {
  const q = useQuery({
    queryKey: ['generation-types'],
    queryFn: () => businessApi.listGenerationTypes().then((r) => r.types),
    staleTime: 60_000,
  })

  const types = q.data ?? []

  const enabledSet = useMemo(
    () => new Set(types.map((t) => t.sub_type)),
    [types],
  )

  const isEnabled = (subType: string) => enabledSet.has(subType)

  const modelsFor = (subType: string) =>
    types.find((t) => t.sub_type === subType)?.models ?? []

  return {
    ...q,
    types,
    enabledSet,
    isEnabled,
    modelsFor,
  }
}
