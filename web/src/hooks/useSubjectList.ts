import { useMemo } from 'react'
import { listSubjectsFromTasks, readySubjects } from '../utils/subjectTask'
import { useGenerationTasks } from './useGenerationTasks'

/** 数字分身列表（来自 subject 任务） */
export function useSubjectList(opts?: { refetchInterval?: number | false }) {
  const { tasks, ...rest } = useGenerationTasks(opts)
  const subjects = useMemo(() => listSubjectsFromTasks(tasks), [tasks])
  const ready = useMemo(() => readySubjects(subjects), [subjects])
  return { subjects, ready, tasks, ...rest }
}
