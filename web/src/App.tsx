import { lazy, Suspense } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { Skeleton } from 'antd'
import { useAuthStore } from './store/auth'
import MerchantLayout from './layouts/MerchantLayout'
import AdminLayout from './layouts/AdminLayout'
import ProtectedRoute from './components/ProtectedRoute'
import ErrorBoundary from './components/ErrorBoundary'

const Login = lazy(() => import('./pages/Login'))
const Dashboard = lazy(() => import('./pages/admin/Dashboard'))
const Chat = lazy(() => import('./pages/admin/Chat'))
const AgentConfigs = lazy(() => import('./pages/admin/AgentConfigs'))
const MerchantHome = lazy(() => import('./pages/merchant/Home'))
const Brands = lazy(() => import('./pages/merchant/Brands'))
const Content = lazy(() => import('./pages/merchant/Content'))
const Keywords = lazy(() => import('./pages/merchant/Keywords'))
const Distribution = lazy(() => import('./pages/merchant/Distribution'))
const CreationWorkbench = lazy(() => import('./pages/merchant/Creation'))
const MyPlan = lazy(() => import('./pages/merchant/MyPlan'))
const Visibility = lazy(() => import('./pages/merchant/Visibility'))
const IndexingReport = lazy(() => import('./pages/merchant/IndexingReport'))
const Nearby = lazy(() => import('./pages/merchant/Nearby'))
const Notifications = lazy(() => import('./pages/merchant/Notifications'))
const AdminUsers = lazy(() => import('./pages/admin/Users'))
const Indexing = lazy(() => import('./pages/admin/Indexing'))
const AdminBrands = lazy(() => import('./pages/admin/Brands'))
const AdminContents = lazy(() => import('./pages/admin/Contents'))
const AdminSettings = lazy(() => import('./pages/admin/Settings'))
const AdminBilling = lazy(() => import('./pages/admin/Billing'))
const GenerationSpecs = lazy(() => import('./pages/admin/GenerationSpecs'))
const Providers = lazy(() => import('./pages/admin/Providers'))
const AdminPromptTemplates = lazy(() => import('./pages/admin/PromptTemplates'))

function PageFallback() {
  return (
    <div className="wr-page-content" style={{ paddingTop: 8 }}>
      <Skeleton active title={{ width: 220 }} paragraph={{ rows: 2 }} style={{ marginBottom: 24 }} />
      <Skeleton active paragraph={{ rows: 6 }} />
    </div>
  )
}

function LazyPage({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  return (
    <ErrorBoundary resetKey={location.pathname}>
      <Suspense fallback={<PageFallback />}>{children}</Suspense>
    </ErrorBoundary>
  )
}

function homePath(role: string | null | undefined) {
  return role === 'admin' ? '/admin' : '/m'
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LazyPage><Login /></LazyPage>} />

        <Route element={<ProtectedRoute><MerchantLayout /></ProtectedRoute>}>
          <Route path="/m" element={<LazyPage><MerchantHome /></LazyPage>} />
          <Route path="/m/brands" element={<LazyPage><Brands /></LazyPage>} />
          <Route path="/m/keywords" element={<LazyPage><Keywords /></LazyPage>} />
          <Route path="/m/visibility" element={<LazyPage><Visibility /></LazyPage>} />
          <Route path="/m/monitor" element={<Navigate to="/m/indexing-report" replace />} />
          <Route path="/m/indexing-report" element={<LazyPage><IndexingReport /></LazyPage>} />
          <Route path="/m/nearby" element={<LazyPage><Nearby /></LazyPage>} />
          <Route path="/m/content" element={<LazyPage><Content /></LazyPage>} />
          <Route path="/m/creation" element={<LazyPage><CreationWorkbench /></LazyPage>} />
          <Route path="/m/distribution" element={<LazyPage><Distribution /></LazyPage>} />
          <Route path="/m/my-plan" element={<LazyPage><MyPlan /></LazyPage>} />
          <Route path="/m/chat" element={<LazyPage><Chat /></LazyPage>} />
          <Route path="/m/notifications" element={<LazyPage><Notifications /></LazyPage>} />
        </Route>

        <Route element={<ProtectedRoute role="admin"><AdminLayout /></ProtectedRoute>}>
          <Route path="/admin" element={<LazyPage><Dashboard /></LazyPage>} />
          <Route path="/admin/users" element={<LazyPage><AdminUsers /></LazyPage>} />
          <Route path="/admin/brands" element={<LazyPage><AdminBrands /></LazyPage>} />
          <Route path="/admin/contents" element={<LazyPage><AdminContents /></LazyPage>} />
          <Route path="/admin/settings" element={<LazyPage><AdminSettings /></LazyPage>} />
          <Route path="/admin/agent-configs" element={<LazyPage><AgentConfigs /></LazyPage>} />
          <Route path="/admin/indexing" element={<LazyPage><Indexing /></LazyPage>} />
          <Route path="/admin/generation-specs" element={<LazyPage><GenerationSpecs /></LazyPage>} />
          <Route path="/admin/providers" element={<LazyPage><Providers /></LazyPage>} />
          <Route path="/admin/prompt-templates" element={<LazyPage><AdminPromptTemplates /></LazyPage>} />
          <Route path="/admin/billing" element={<LazyPage><AdminBilling /></LazyPage>} />
          <Route path="/admin/chat" element={<LazyPage><Chat /></LazyPage>} />
        </Route>

        <Route path="/" element={<RootRedirect />} />
        <Route path="*" element={<RootRedirect />} />
      </Routes>
    </BrowserRouter>
  )
}

function RootRedirect() {
  const token = useAuthStore((s) => s.token)
  const role = useAuthStore((s) => s.role)
  if (!token) return <Navigate to="/login" replace />
  return <Navigate to={homePath(role)} replace />
}
