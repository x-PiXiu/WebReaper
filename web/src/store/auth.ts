import { create } from 'zustand'
import { persist } from 'zustand/middleware'

// 认证状态管理（Zustand + persist）。
// token/role 持久化到 localStorage，刷新页面不丢登录态。
// Axios 拦截器通过 getToken() 读取 token 附到请求头。
// 前端通过 role 分流到商户端 / 管理后台。

export type UserRole = 'admin' | 'merchant'

interface AuthState {
  token: string | null
  username: string | null
  role: UserRole | null
  tenantId: string | null
  mustChangePassword: boolean // 仍在使用默认口令（F1-5：admin/admin123）——管理端常驻提醒
  setAuth: (token: string, username: string, role: UserRole, tenantId: string, mustChangePassword?: boolean) => void
  setMustChangePassword: (v: boolean) => void
  clearAuth: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      username: null,
      role: null,
      tenantId: null,
      mustChangePassword: false,
      setAuth: (token, username, role, tenantId, mustChangePassword = false) => set({ token, username, role, tenantId, mustChangePassword }),
      setMustChangePassword: (v) => set({ mustChangePassword: v }),
      clearAuth: () => set({ token: null, username: null, role: null, tenantId: null, mustChangePassword: false }),
    }),
    { name: 'webreaper-auth' }, // localStorage key
  ),
)

// 非组件场景（如 Axios 拦截器）读取 token 的工具函数。
export const getToken = (): string | null => useAuthStore.getState().token

// 非组件场景读取角色。
export const getRole = (): UserRole | null => useAuthStore.getState().role
