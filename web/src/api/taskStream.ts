// 通用任务执行的 SSE 流式 API。
//
// 不用 axios（不擅长流式），用原生 fetch + ReadableStream（与 Chat.tsx 同模式）。
// 后端端点 POST /api/v1/agents/execute/stream 推送 SSE 事件。

import { getToken } from '../store/auth'

// 任务执行事件（与后端 port.TaskEvent 对齐）
export interface TaskStreamEvent {
  type: 'text-delta' | 'tool-call' | 'tool-result' | 'finish' | 'error'
  text?: string // text-delta 的增量文本
  tool_name?: string // tool-call / tool-result 的工具名
  tool_args?: string // tool-call 的参数（JSON）
  tool_result?: string // tool-result 的返回值（截断）
  error?: string // error 的错误信息
}

// executeTaskStream 发起任务执行并流式消费 SSE 事件。
//
// 用法：
//   const stop = executeTaskStream('采集 X', (evt) => { ... })
//   // 需要中止时调 stop()
//
// 返回一个 abort 函数（取消请求）。
export function executeTaskStream(
  task: string,
  tools: string[],
  systemPrompt: string,
  onEvent: (evt: TaskStreamEvent) => void,
  onError?: (err: Error) => void,
): () => void {
  const controller = new AbortController()

  ;(async () => {
    try {
      const token = getToken()
      const res = await fetch('/api/v1/agents/execute/stream', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ task, tools, system_prompt: systemPrompt }),
        signal: controller.signal,
      })
      if (res.status === 401) {
        window.location.href = '/login'
        return
      }
      if (!res.ok) throw new Error(`HTTP ${res.status}`)

      const reader = res.body?.getReader()
      const dec = new TextDecoder()
      let buf = ''
      while (reader) {
        const { done, value } = await reader.read()
        if (done) break
        buf += dec.decode(value, { stream: true })
        const lines = buf.split('\n')
        buf = lines.pop() || '' // 保留最后不完整的行
        for (const ln of lines) {
          if (!ln.startsWith('data: ')) continue
          try {
            const evt = JSON.parse(ln.slice(6)) as TaskStreamEvent
            onEvent(evt)
          } catch {
            // 忽略解析失败的行（如心跳/注释）
          }
        }
      }
    } catch (err) {
      // AbortError 是主动取消，不报错
      if ((err as Error).name === 'AbortError') return
      onError?.(err as Error)
    }
  })()

  return () => controller.abort()
}
