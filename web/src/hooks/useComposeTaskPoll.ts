import { useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { message } from 'antd'
import { businessApi } from '../api/business'
import { useComposeDraft } from '../store/composeDraft'
import { isTaskDone, isTaskSuccess, taskCoverUrl, taskPrimaryUrl } from '../utils/generationTask'

const POLL_MS = 3000

/**
 * 轮询创作台关联的生成任务，成功后自动回填配音 / 成片 / 配图 / 封面 URL。
 */
export function useComposeTaskPoll() {
  const voiceTaskId = useComposeDraft((s) => s.voiceTaskId)
  const avatarTaskId = useComposeDraft((s) => s.avatarTaskId)
  const coverTaskId = useComposeDraft((s) => s.coverTaskId)
  const imageTaskIds = useComposeDraft((s) => s.imageTaskIds)
  const imageUrls = useComposeDraft((s) => s.imageUrls)
  const coverUrl = useComposeDraft((s) => s.coverUrl)
  const patch = useComposeDraft((s) => s.patch)
  const queryClient = useQueryClient()
  const notified = useRef<Set<string>>(new Set())

  const pendingKey = [voiceTaskId, avatarTaskId, coverTaskId, ...(imageTaskIds || [])].filter(Boolean).join(',')

  useEffect(() => {
    if (!pendingKey) return

    let cancelled = false

    const tick = async () => {
      try {
        const { tasks } = await businessApi.listGenerationTasks()
        if (cancelled) return

        const byId = new Map(tasks.map((t) => [t.id, t]))
        const next: Record<string, unknown> = {}
        let worksDirty = false

        const handle = (
          id: string | undefined,
          onSuccess: (url: string, cover?: string | null) => void,
          label: string,
        ) => {
          if (!id) return
          const task = byId.get(id)
          if (!task || !isTaskDone(task.state)) return
          if (!isTaskSuccess(task.state)) {
            if (!notified.current.has(`fail-${id}`)) {
              notified.current.add(`fail-${id}`)
              message.error(`${label}失败：${task.err_msg || task.state}`)
            }
            return
          }
          const url = taskPrimaryUrl(task)
          if (!url || notified.current.has(`ok-${id}`)) return
          notified.current.add(`ok-${id}`)
          onSuccess(url, taskCoverUrl(task))
          message.success(`${label}已完成，已自动填入`)
          worksDirty = true
        }

        handle(voiceTaskId, (url) => {
          next.voiceUrl = url
        }, '配音')

        handle(avatarTaskId, (url, taskCover) => {
          next.avatarVideoUrl = url
          next.editedVideoUrl = url
          if (taskCover && !coverUrl) next.coverUrl = taskCover
        }, '数字人口播')

        handle(coverTaskId, (url) => {
          next.coverUrl = url
        }, '封面')

        const finishedImageIds: string[] = []
        const newImageUrls = [...(imageUrls || [])]
        for (const id of imageTaskIds || []) {
          const task = byId.get(id)
          if (!task || !isTaskDone(task.state)) continue
          finishedImageIds.push(id)
          if (!isTaskSuccess(task.state)) {
            if (!notified.current.has(`fail-${id}`)) {
              notified.current.add(`fail-${id}`)
              message.error(`配图生成失败：${task.err_msg || task.state}`)
            }
            continue
          }
          const url = taskPrimaryUrl(task)
          if (!url || notified.current.has(`ok-${id}`)) continue
          notified.current.add(`ok-${id}`)
          if (!newImageUrls.includes(url)) newImageUrls.push(url)
          message.success('配图已生成并加入列表')
          worksDirty = true
        }
        if (finishedImageIds.length) {
          next.imageTaskIds = (imageTaskIds || []).filter((id) => !finishedImageIds.includes(id))
          next.imageUrls = newImageUrls
        }

        if (Object.keys(next).length > 0) {
          patch({ ...next, lastUpdatedAt: new Date().toISOString() })
        }
        if (worksDirty) {
          queryClient.invalidateQueries({ queryKey: ['merchant-works'] })
          queryClient.invalidateQueries({ queryKey: ['generation-tasks'] })
        }
      } catch {
        /* 后端未启动时静默 */
      }
    }

    tick()
    const timer = window.setInterval(tick, POLL_MS)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [pendingKey, voiceTaskId, avatarTaskId, coverTaskId, imageTaskIds, imageUrls, coverUrl, patch, queryClient])
}
