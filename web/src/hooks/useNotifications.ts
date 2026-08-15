import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../api/business'

// 通知数据契约：列表与未读数是同一份数据的两个视图。
// 此前铃铛/工作台/通知中心各用各的 queryKey（三份缓存互不失效，已读状态不同步）——
// 现在收敛为两个 key + 一个联动失效的标记已读 mutation，全站共用。

export const NOTIFY_LIST_KEY = ['notifications'] as const
export const NOTIFY_UNREAD_KEY = ['notify-unread'] as const

/** 通知列表（铃铛 / 工作台待办 / 通知中心共享同一缓存）。 */
export function useNotificationList() {
  return useQuery({
    queryKey: NOTIFY_LIST_KEY,
    queryFn: () => businessApi.listNotifications(),
    staleTime: 30_000,
  })
}

/** 未读数（30s 轮询，角标用）。 */
export function useUnreadCount() {
  return useQuery({
    queryKey: NOTIFY_UNREAD_KEY,
    queryFn: () => businessApi.notificationUnreadCount(),
    refetchInterval: 30_000,
  })
}

/** 标记已读（id 为空 = 全部已读）。成功后列表与未读数联动失效。 */
export function useMarkNotificationRead() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id?: string) => businessApi.markNotificationRead(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: NOTIFY_LIST_KEY })
      queryClient.invalidateQueries({ queryKey: NOTIFY_UNREAD_KEY })
    },
  })
}
