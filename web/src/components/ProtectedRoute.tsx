import { Navigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'

// 路由守卫：无 token 时跳转登录页。
// 包裹需要认证的页面元素。
export default function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.token)
  if (!token) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}
