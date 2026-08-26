import { Tag } from 'antd'

/** 后端 retry_hint 三分类 → 用户可读建议（与 generation handler 派生规则同源） */
export const RETRY_HINT_META: Record<string, { color: string; label: string }> = {
  RetryAuto: { color: 'gold', label: '网络繁忙，可重试' },
  RetryManual: { color: 'orange', label: '请检查参数' },
  RetryTerminal: { color: 'red', label: '配额耗尽' },
}

export function retryHintLabel(code?: string): string | undefined {
  return code ? RETRY_HINT_META[code]?.label : undefined
}

/** 失败 toast 文案：优先 err_msg，其次 retry_hint 建议 */
export function retryFailureMessage(task: { err_msg?: string; retry_hint?: string; state?: string }, prefix = '失败') {
  const hint = retryHintLabel(task.retry_hint)
  const detail = task.err_msg || hint || task.state || '未知错误'
  return hint && task.err_msg ? `${prefix}：${task.err_msg}（${hint}）` : `${prefix}：${detail}`
}

/**
 * 生成任务失败建议 Tag（Creation / ComposeHub / 作品库共用）
 */
export default function RetryHint({ code }: { code?: string }) {
  if (!code) return null
  const meta = RETRY_HINT_META[code]
  if (!meta) return null
  return <Tag color={meta.color} style={{ fontSize: 11, margin: 0 }}>{meta.label}</Tag>
}
