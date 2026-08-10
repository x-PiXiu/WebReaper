import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import Login from './pages/Login'
// 管理后台页面（复用现有）
import Dashboard from './pages/Dashboard'
import Chat from './pages/Chat'
import AgentConfigs from './pages/AgentConfigs'
import DataItems from './pages/DataItems'
import Tasks from './pages/Tasks'
import CrawlConfigPage from './pages/CrawlConfig'
import ExternalSystems from './pages/ExternalSystems'
import Tools from './pages/Tools'
// 商户端 GEO 页面
import MerchantHome from './pages/merchant/Home'
import Brands from './pages/merchant/Brands'
import Content from './pages/merchant/Content'
import Keywords from './pages/merchant/Keywords'
import Accounts from './pages/merchant/Accounts'
import Publish from './pages/merchant/Publish'
// 管理端额外页面
import AdminUsers from './pages/admin/Users'
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
        <Route element={<ProtectedRoute role="merchant"><MerchantLayout /></ProtectedRoute>}>
          <Route path="/m" element={<MerchantHome />} />
          <Route path="/m/brands" element={<Brands />} />
          <Route path="/m/keywords" element={<Keywords />} />
          <Route path="/m/content" element={<Content />} />
          <Route path="/m/accounts" element={<Accounts />} />
          <Route path="/m/publish" element={<Publish />} />
          <Route path="/m/chat" element={<Chat />} />
        </Route>

        {/* 管理后台路由组（role=admin）*/}
        <Route element={<ProtectedRoute role="admin"><AdminLayout /></ProtectedRoute>}>
          <Route path="/admin" element={<Dashboard />} />
          <Route path="/admin/users" element={<AdminUsers />} />
          <Route path="/admin/agent-configs" element={<AgentConfigs />} />
          <Route path="/admin/data" element={<DataItems />} />
          <Route path="/admin/tasks" element={<Tasks />} />
          <Route path="/admin/tools" element={<Tools />} />
          <Route path="/admin/crawl-config" element={<CrawlConfigPage />} />
          <Route path="/admin/external-systems" element={<ExternalSystems />} />
          <Route path="/admin/chat" element={<Chat />} />
        </Route>

        {/* 根路径：按角色跳转（在 ProtectedRoute 外，用轻量判断）*/}
        <Route path="/" element={<RootRedirect />} />

        {/* 兜底：跳登录 */}
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    </BrowserRouter>
  )
}

// RootRedirect 根据登录态和角色跳转到对应首页。
import { useAuthStore } from './store/auth'
function RootRedirect() {
  const token = useAuthStore((s) => s.token)
  const role = useAuthStore((s) => s.role)
  if (!token) return <Navigate to="/login" replace />
  return <Navigate to={role === 'admin' ? '/admin' : '/m'} replace />
}
