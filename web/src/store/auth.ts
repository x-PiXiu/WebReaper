import { create } from 'zustand'
import { persist } from 'zustand/middleware'

// 认证状态管理（Zustand + persist）。
// token 持久化到 localStorage，刷新页面不丢登录态。
// Axios 拦截器通过 getToken() 读取 token 附到请求头。

interface AuthState {
  token: string | null
  username: string | null
  setAuth: (token: string, username: string) => void
  clearAuth: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      username: null,
      setAuth: (token, username) => set({ token, username }),
      clearAuth: () => set({ token: null, username: null }),
    }),
    { name: 'webreaper-auth' }, // localStorage key
  ),
)

// 非组件场景（如 Axios 拦截器）读取 token 的工具函数。
export const getToken = (): string | null => useAuthStore.getState().token
