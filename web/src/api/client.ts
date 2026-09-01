import axios, { AxiosError } from 'axios'
import { message as antdMessage, modal as antdModal } from '../utils/antdApp'
import { getToken, useAuthStore } from '../store/auth'
import { clearQueryCache } from '../queryClient'
import { friendlyGenerationError } from '../utils/generationErrors'
import type { ApiEnvelope } from '../types/api'

function toastBizError(raw: string) {
  const path = typeof window !== 'undefined' ? window.location.pathname : ''
  const isGen =
    path.includes('/compose') ||
    path.includes('/creation') ||
    path.includes('/assets') ||
    path.includes('/quick') ||
    path.includes('/works') ||
    path.includes('/distribution')
  antdMessage.error({
    content: isGen ? friendlyGenerationError(raw) : (raw || '操作失败，请稍后重试'),
    key: 'wr-biz-error',
    duration: 3.5,
  })
}

// Axios 实例 + 拦截器。
//
// 匹配后端契约：
//   - 请求拦截：从 store 读 token，附 Authorization: Bearer xxx
//   - 响应拦截：解包信封 {code,msg,data}；code!==0 抛错+提示；401 清 token 跳登录
//   - 40201 配额超限：弹"去升级"引导（友好化——不把服务端原始错误直接甩给用户）

// 统一 API 前缀（生产部署在 nginx 后面分流用，如 /webreaper）
// VITE_API_PREFIX 由 vite 构建时注入（.env.production 或构建命令）；开发时为空
const API_PREFIX = import.meta.env.VITE_API_PREFIX || ''

export const apiClient = axios.create({
  baseURL: API_PREFIX || '/', // 走 Vite proxy，/api/* 被转发到后端
  timeout: 120000, // GEO 的 RAG 操作（爬全网+LLM）耗时较长，给 2 分钟
})

// 请求拦截：附 JWT token
apiClient.interceptors.request.use((config) => {
  const token = getToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 配额超限统一引导：Modal 带"去升级"跳套餐页（不弹原始服务端错误）。
// 页面 catch 里仍会收到 reject 的友好 Error（"配额已用完"），可作轻提示。
// 防抖：多引擎监测/批量操作会同时收到多个 402——5 秒窗口内只弹一次引导。
let quotaPromptShown = false
let quotaPromptTimer: ReturnType<typeof setTimeout> | null = null
function promptQuotaExceeded() {
  if (quotaPromptShown) {
    return // 防抖：已有引导弹窗
  }
  quotaPromptShown = true
  if (quotaPromptTimer) clearTimeout(quotaPromptTimer)
  quotaPromptTimer = setTimeout(() => {
    quotaPromptShown = false
  }, 5000)
  antdModal.confirm({
    title: '本月配额已用完',
    content: '当前套餐次数已用完。升级套餐可继续监测/生成，或等下月自动重置。',
    okText: '去升级',
    cancelText: '知道了',
    centered: true,
    okButtonProps: { className: 'ip-btn-primary' },
    onOk: () => {
      window.location.href = '/m/my-plan'
    },
  })
}

// 响应拦截：解包信封
apiClient.interceptors.response.use(
  (response) => {
    const env = response.data as ApiEnvelope<unknown>
    // 后端统一信封：code === 0 表示成功，返回 data
    if (env && typeof env.code === 'number') {
      if (env.code === 0) {
        return env.data // 直接返回 data，调用方无需再解包
      }
      if (env.code === 40201) {
        // 配额超限：友好引导（不显示服务端原始 msg）
        promptQuotaExceeded()
        return Promise.reject(new Error('配额已用完'))
      }
      // 其他业务错误：生成域友好化后提示并抛错
      const raw = env.msg || '请求失败'
      toastBizError(raw)
      return Promise.reject(new Error(friendlyGenerationError(raw)))
    }
    // 非信封响应（如健康检查），原样返回
    return response.data
  },
  (error: AxiosError<ApiEnvelope<unknown>>) => {
    const status = error.response?.status
    if (status === 401) {
      // token 失效：清登录态 + 清数据缓存（旧租户数据绝不残留），跳登录页
      useAuthStore.getState().clearAuth()
      clearQueryCache()
      antdMessage.error({ content: '登录已过期，请重新登录', key: 'wr-auth', duration: 3 })
      // 用 location 跳转避免在拦截器里耦合 router
      window.location.href = '/login'
      return Promise.reject(new Error('未授权'))
    }
    if (status === 402) {
      // HTTP 层 402（非信封错误路径）：同样友好引导
      promptQuotaExceeded()
      return Promise.reject(new Error('配额已用完'))
    }
    const raw = error.response?.data?.msg || error.message || '网络错误'
    toastBizError(raw)
    return Promise.reject(new Error(friendlyGenerationError(raw)))
  },
)
