import { useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useComposeDraft } from '../store/composeDraft'

/** 创作草稿与作品库联动：记录 contentId、刷新作品列表 */
export function useComposeWorkSync() {
  const draft = useComposeDraft()
  const queryClient = useQueryClient()

  const rememberContentId = useCallback((contentId: string, title?: string) => {
    if (!contentId) return
    const patch: Record<string, string> = { contentId, lastUpdatedAt: new Date().toISOString() }
    if (title && !draft.selectedTitle) patch.selectedTitle = title
    draft.patch(patch)
    queryClient.invalidateQueries({ queryKey: ['merchant-works'] })
  }, [draft, queryClient])

  const touchDraft = useCallback(() => {
    draft.patch({ lastUpdatedAt: new Date().toISOString() })
  }, [draft])

  const invalidateWorks = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ['merchant-works'] })
  }, [queryClient])

  return { rememberContentId, touchDraft, invalidateWorks }
}
