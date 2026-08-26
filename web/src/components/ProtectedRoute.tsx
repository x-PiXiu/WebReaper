import { Navigate, useLocation } from 'react-router-dom'
import { useAuthStore, type UserRole } from '../store/auth'

// 路由守卫：
//   - 无 token 时跳转登录页，并记录来源路径（登录成功后回跳——OAuth 授权回调等深链场景
//     依赖此机制：回调窗口 token 过期 → 登录 → 回到 /m/distribution?douyin_oauth=... 弹绑定结果）
//   - 指定 role 时，校验当前用户角色是否匹配，不匹配跳转到其首页（角色越权保护）
//   - 指定 role 时，未指定 role 时不做角色校验（兼容旧用法）
//
// 用法：
//   <ProtectedRoute role="merchant"><MerchantLayout /></ProtectedRoute>
//   <ProtectedRoute role="admin"><AdminLayout /></ProtectedRoute>
export default function ProtectedRoute({ children, role }: { children: React.ReactNode; role?: UserRole }) {
  const token = useAuthStore((s) => s.token)
  const currentRole = useAuthStore((s) => s.role)
  const location = useLocation()

  if (!token) {
    return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />
  }

  // 角色守卫：商户访问管理端（或反之）时跳回自己的首页
  if (role && currentRole && role !== currentRole) {
    const home = currentRole === 'admin' ? '/admin' : '/m/compose'
    return <Navigate to={home} replace />
  }

  return <>{children}</>
}
