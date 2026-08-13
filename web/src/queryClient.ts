import { QueryClient } from '@tanstack/react-query'

// 独立 queryClient，避免 api/client ↔ main 循环依赖。
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      gcTime: 5 * 60_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

/** 登出 / 401 时清空缓存，防止账号间数据串扰。 */
export function clearQueryCache() {
  queryClient.clear()
}
