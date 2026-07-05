import axios, { AxiosError } from 'axios'
import { message as antdMessage } from 'antd'
import { getToken, useAuthStore } from '../store/auth'
import type { ApiEnvelope } from '../types/api'

// Axios 实例 + 拦截器。
//
// 匹配后端契约：
//   - 请求拦截：从 store 读 token，附 Authorization: Bearer xxx
//   - 响应拦截：解包信封 {code,msg,data}；code!==0 抛错+提示；401 清 token 跳登录

export const apiClient = axios.create({
  baseURL: '/', // 走 Vite proxy，/api/* 被转发到后端
  timeout: 30000,
})

// 请求拦截：附 JWT token
apiClient.interceptors.request.use((config) => {
  const token = getToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截：解包信封
apiClient.interceptors.response.use(
  (response) => {
    const env = response.data as ApiEnvelope<unknown>
    // 后端统一信封：code === 0 表示成功，返回 data
    if (env && typeof env.code === 'number') {
      if (env.code === 0) {
        return env.data // 直接返回 data，调用方无需再解包
      }
      // 业务错误：提示并抛错
      antdMessage.error(env.msg || '请求失败')
      return Promise.reject(new Error(env.msg || `业务错误 ${env.code}`))
    }
    // 非信封响应（如健康检查），原样返回
    return response.data
  },
  (error: AxiosError<ApiEnvelope<unknown>>) => {
    const status = error.response?.status
    if (status === 401) {
      // token 失效：清登录态，跳登录页
      useAuthStore.getState().clearAuth()
      antdMessage.error('登录已过期，请重新登录')
      // 用 location 跳转避免在拦截器里耦合 router
      window.location.href = '/login'
      return Promise.reject(new Error('未授权'))
    }
    const msg = error.response?.data?.msg || error.message || '网络错误'
    antdMessage.error(msg)
    return Promise.reject(error)
  },
)
