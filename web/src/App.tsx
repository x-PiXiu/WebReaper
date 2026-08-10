import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import Login from './pages/Login'
// 管理后台页面
import Dashboard from './pages/Dashboard'
import Chat from './pages/Chat'
import AgentConfigs from './pages/AgentConfigs'
import Tools from './pages/Tools'
// 商户端 GEO 页面
import MerchantHome from './pages/merchant/Home'
import Brands from './pages/merchant/Brands'
import Content from './pages/merchant/Content'
import Keywords from './pages/merchant/Keywords'
import Distribution from './pages/merchant/Distribution'
import VideoWorkbench from './pages/merchant/Video'
import MyPlan from './pages/merchant/MyPlan'
import Visibility from './pages/merchant/Visibility'
// 管理端额外页面
import AdminUsers from './pages/admin/Users'
import Indexing from './pages/admin/Indexing'
import AdminBrands from './pages/admin/Brands'
import AdminContents from './pages/admin/Contents'
import AdminSettings from './pages/admin/Settings'
import AdminBilling from './pages/admin/Billing'
// 布局与守卫
import MerchantLayout from './layouts/MerchantLayout'
import AdminLayout from './layouts/AdminLayout'
import ProtectedRoute from './components/ProtectedRoute'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        {/* 公开：登录 */}
        <Route path="/login" element={<Login />} />

        {/* 商户端路由组（role=merchant）*/}
        <Route element={<ProtectedRoute><MerchantLayout /></ProtectedRoute>}>
          <Route path="/m" element={<MerchantHome />} />
          <Route path="/m/brands" element={<Brands />} />
          <Route path="/m/keywords" element={<Keywords />} />
          <Route path="/m/visibility" element={<Visibility />} />
          <Route path="/m/content" element={<Content />} />
          <Route path="/m/video" element={<VideoWorkbench />} />
          <Route path="/m/distribution" element={<Distribution />} />
          <Route path="/m/my-plan" element={<MyPlan />} />
          <Route path="/m/chat" element={<Chat />} />
        </Route>

        {/* 管理后台路由组（role=admin）*/}
        <Route element={<ProtectedRoute role="admin"><AdminLayout /></ProtectedRoute>}>
          <Route path="/admin" element={<Dashboard />} />
          <Route path="/admin/users" element={<AdminUsers />} />
          <Route path="/admin/brands" element={<AdminBrands />} />
          <Route path="/admin/contents" element={<AdminContents />} />
          <Route path="/admin/settings" element={<AdminSettings />} />
          <Route path="/admin/agent-configs" element={<AgentConfigs />} />
          <Route path="/admin/tools" element={<Tools />} />
          <Route path="/admin/indexing" element={<Indexing />} />
          <Route path="/admin/billing" element={<AdminBilling />} />
          <Route path="/admin/chat" element={<Chat />} />
        </Route>

        {/* 根路径：登录后统一进入用户界面（商户端）*/}
        <Route path="/" element={<RootRedirect />} />

        {/* 未匹配路由兜底：已登录回用户界面（避免"路由缺失→莫名跳登录页"），未登录去登录页 */}
        <Route path="*" element={<RootFallback />} />
      </Routes>
    </BrowserRouter>
  )
}

// RootRedirect 登录后统一进入用户界面。
// 管理后台从用户界面的顶栏入口进入——管理员不再被直接抛进管理后台。
import { useAuthStore } from './store/auth'
function RootRedirect() {
  const token = useAuthStore((s) => s.token)
  if (!token) return <Navigate to="/login" replace />
  return <Navigate to="/m" replace />
}

// RootFallback 未匹配路由兜底。
function RootFallback() {
  const token = useAuthStore((s) => s.token)
  if (!token) return <Navigate to="/login" replace />
  return <Navigate to="/m" replace />
}
