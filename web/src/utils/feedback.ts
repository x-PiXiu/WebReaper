import { message, notification } from './antdApp'

/** 统一轻提示时长与去重 key，避免连环 toast 刷屏 */
const D = { ok: 2.2, info: 2.5, warn: 3.2, fail: 3.8 } as const

type ToastFn = (content: string, keyOrDuration?: string | number, duration?: number) => void

function open(
  kind: 'success' | 'info' | 'warning' | 'error',
  content: string,
  defaultKey: string,
  defaultDuration: number,
  keyOrDuration?: string | number,
  duration?: number,
) {
  const key = typeof keyOrDuration === 'string' ? keyOrDuration : defaultKey
  const dur = typeof keyOrDuration === 'number'
    ? keyOrDuration
    : (duration ?? defaultDuration)
  message[kind]({ content, key, duration: dur })
}

export const toast = {
  ok: ((content, keyOrDuration, duration) =>
    open('success', content, 'wr-ok', D.ok, keyOrDuration, duration)) as ToastFn,
  info: ((content, keyOrDuration, duration) =>
    open('info', content, 'wr-info', D.info, keyOrDuration, duration)) as ToastFn,
  warn: ((content, keyOrDuration, duration) =>
    open('warning', content, 'wr-warn', D.warn, keyOrDuration, duration)) as ToastFn,
  fail: ((content, keyOrDuration, duration) =>
    open('error', content, 'wr-fail', D.fail, keyOrDuration, duration)) as ToastFn,
  /** 进度态：需用同 key 的 ok/info/fail 收尾 */
  loading: (content: string, key = 'wr-load') =>
    message.loading({ content, key, duration: 0 }),
}

/** 需要用户多看一眼的结果（如发布失败摘要）走右上角通知 */
export function notifyResult(opts: {
  type?: 'success' | 'info' | 'warning' | 'error'
  title: string
  desc?: string
  key?: string
  duration?: number
}) {
  const { type = 'info', title, desc, key = 'wr-result', duration = 4.5 } = opts
  const openFn = notification[type]
  openFn({ message: title, description: desc, key, duration, placement: 'topRight' })
}
