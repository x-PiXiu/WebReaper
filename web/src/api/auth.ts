import { apiClient } from './client'
import type { LoginRequest, LoginResponse, RegisterRequest, RegisterResponse } from '../types/api'

// 认证 API 封装。返回值已被拦截器解包，直接是 data 部分。

export const authApi = {
  login: (data: LoginRequest) =>
    apiClient.post<unknown, LoginResponse>('/api/v1/auth/login', data),

  register: (data: RegisterRequest) =>
    apiClient.post<unknown, RegisterResponse>('/api/v1/auth/register', data),
}
