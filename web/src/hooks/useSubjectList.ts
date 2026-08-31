import { useMemo } from 'react'
import { assetToViduSubject, useSubjectAssets } from './useSubjectAssets'
import { useGenerationTasks } from './useGenerationTasks'

/**
 * 数字分身列表（26号优化——从subject_assets表读取，替代从generation_tasks过滤）。
 *
 * 改进：
 *   - 分身列表从subject_assets读取（失败任务天然不出现）
 *   - tasks仍从generation_tasks读取（用于查找克隆音色和形象视频）
 */
export function useSubjectList(opts?: { refetchInterval?: number | false }) {
  // 分身资产（主数据源）
  const { assets, persons, scenes, withVideo, isLoading, refetch, error } = useSubjectAssets({
    refetchInterval: opts?.refetchInterval ?? false,
  })

  // 生成任务（辅助数据源：克隆音色、形象视频join）
  const { tasks } = useGenerationTasks(opts)

  // 转换为ViduSubject格式（兼容现有组件）
  const subjects = useMemo(() => assets.map(assetToViduSubject), [assets])
  const ready = useMemo(() => persons.map(assetToViduSubject), [persons])

  return {
    subjects,
    ready,
    persons,
    scenes,
    withVideo,
    tasks, // 保留：AvatarModule用tasks查找克隆音色和形象视频
    isLoading,
    refetch,
    error,
  }
}
