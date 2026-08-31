import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import type { SubjectAsset } from '../types/api'

export const SUBJECT_ASSETS_KEY = ['subject-assets']

/** 个人主体资产列表（26号计划——从subject_assets表读取） */
export function useSubjectAssets(opts?: {
  kind?: string
  limit?: number
  enabled?: boolean
  refetchInterval?: number | false
}) {
  const { data, ...rest } = useQuery({
    queryKey: [...SUBJECT_ASSETS_KEY, opts?.kind, opts?.limit],
    queryFn: () => businessApi.listSubjectAssets({
      kind: opts?.kind,
      limit: opts?.limit ?? 50,
    }),
    enabled: opts?.enabled !== false,
    refetchInterval: opts?.refetchInterval ?? false,
  })

  const assets = data?.assets ?? []
  const total = data?.total ?? 0

  // 人物分身（可用于reference2video）
  const persons = useMemo(() => assets.filter(a => a.kind === 'person'), [assets])
  // 环境主体（组合出镜用）
  const scenes = useMemo(() => assets.filter(a => a.kind === 'scene'), [assets])
  // 有形象视频的分身
  const withVideo = useMemo(() => assets.filter(a => a.avatar_video_url), [assets])

  return { assets, persons, scenes, withVideo, total, ...rest }
}

/** 官方主体列表（从subject_assets表读取scope=official） */
export function useOfficialSubjects(opts?: {
  kind?: string
  limit?: number
  enabled?: boolean
}) {
  const { data, ...rest } = useQuery({
    queryKey: ['official-subjects', opts?.kind, opts?.limit],
    queryFn: () => businessApi.listOfficialSubjects({
      kind: opts?.kind,
      limit: opts?.limit ?? 50,
    }),
    enabled: opts?.enabled !== false,
    staleTime: 5 * 60 * 1000, // 官方主体变化不频繁，5分钟缓存
  })

  return { subjects: data?.subjects ?? [], total: data?.total ?? 0, ...rest }
}

/** 将SubjectAsset转换为ViduSubject格式（兼容现有组件） */
export function assetToViduSubject(asset: SubjectAsset) {
  return {
    taskId: asset.source_task_id || asset.id,
    state: 'success',
    name: asset.name,
    serverId: asset.server_id,
    voiceId: asset.voice_id,
    kind: asset.kind as 'person' | 'scene',
    hasVideo: !!asset.avatar_video_url,
    imageCount: asset.portrait_url ? 1 : 0,
    portraitUrl: asset.portrait_url,
    errMsg: '',
    createdAt: asset.created_at,
    avatarTaskId: '',
  }
}
