import { getToken, useAuthStore } from '../store/auth'
import { clearQueryCache } from '../queryClient'
import type { OptimizedContent } from '../types/api'

const API_PREFIX = import.meta.env.VITE_API_PREFIX || ''

export type GenerateContentPayload = {
  keywords?: string[]
  topic?: string
  brand_info?: string
  llm_config_name?: string
  target_engine?: string
  use_diagnose?: boolean
  format?: string
  citation_toggles?: string[]
}

type ContentStreamEvent =
  | { type: 'text-delta'; textDelta?: string }
  | { type: 'result'; data?: OptimizedContent }
  | { type: 'finish' }
  | { type: 'error'; error?: string }

/**
 * 流式生成种草/口播文案（SSE）。
 * 后端逐字推送正文，避免非流式接口长时间无反馈。
 */
export async function generateContentStream(
  brandId: string,
  payload: GenerateContentPayload,
  onDelta: (chunk: string) => void,
  signal?: AbortSignal,
): Promise<OptimizedContent> {
  const token = getToken()
  const res = await fetch(`${API_PREFIX}/api/v1/merchant/brands/${brandId}/contents/generate-stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(payload),
    signal,
  })

  if (res.status === 401) {
    useAuthStore.getState().clearAuth()
    clearQueryCache()
    window.location.href = '/login'
    throw new Error('未授权')
  }
  if (!res.ok) {
    let msg = `生成失败（HTTP ${res.status}）`
    try {
      const env = await res.json()
      if (env?.msg) msg = env.msg
    } catch {
      /* 非 JSON 响应 */
    }
    throw new Error(msg)
  }

  const reader = res.body?.getReader()
  if (!reader) throw new Error('浏览器不支持流式响应')

  const dec = new TextDecoder()
  let buf = ''
  let acc = ''
  let result: OptimizedContent | null = null

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buf += dec.decode(value, { stream: true })
    const lines = buf.split('\n')
    buf = lines.pop() || ''
    for (const ln of lines) {
      if (!ln.startsWith('data: ')) continue
      let e: ContentStreamEvent
      try {
        e = JSON.parse(ln.slice(6)) as ContentStreamEvent
      } catch {
        continue
      }
      if (e.type === 'text-delta' && e.textDelta) {
        acc += e.textDelta
        onDelta(acc)
      }
      if (e.type === 'result' && e.data) {
        result = e.data
      }
      if (e.type === 'error') {
        throw new Error(e.error || '生成失败')
      }
    }
  }

  if (result) return result
  if (acc.trim()) {
    return {
      id: '',
      tenant_id: '',
      brand_id: brandId,
      keyword_id: '',
      title: '',
      original_text: '',
      optimized_text: acc,
      version: 1,
      score: { total: 0, authority: 0, specificity: 0, structure: 0, uniqueness: 0, recency: 0 },
      status: 'draft',
      created_at: new Date().toISOString(),
    }
  }
  throw new Error('生成未完成，请稍后重试')
}
