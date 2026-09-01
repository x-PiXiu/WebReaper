import { lazy, Suspense } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { Skeleton } from 'antd'
import { useAuthStore } from './store/auth'
import MerchantLayout from './layouts/MerchantLayout'
import AdminLayout from './layouts/AdminLayout'
import ProtectedRoute from './components/ProtectedRoute'
import ErrorBoundary from './components/ErrorBoundary'

const Login = lazy(() => import('./pages/Login'))
const NotFound = lazy(() => import('./pages/NotFound'))
const Chat = lazy(() => import('./pages/merchant/Chat'))
const Brands = lazy(() => import('./pages/merchant/Brands'))
const Distribution = lazy(() => import('./pages/merchant/Distribution'))
const Checkup = lazy(() => import('./pages/merchant/checkup/Checkup'))
const Studio = lazy(() => import('./pages/merchant/studio/Studio'))
const ComposeHub = lazy(() => import('./pages/merchant/compose/ComposeHub'))
const LipSyncWizard = lazy(() => import('./pages/merchant/compose/LipSyncWizard'))
const QuickGenerate = lazy(() => import('./pages/merchant/QuickGenerate'))
const VideoTrackRedirect = lazy(() => import('./pages/merchant/compose/VideoTrackRedirect'))
const ComposeTrackPage = lazy(() => import('./pages/merchant/compose/ComposeTrackPage'))
const BenchmarkModule = lazy(() => import('./pages/merchant/compose/modules/BenchmarkModule'))
const CopyModule = lazy(() => import('./pages/merchant/compose/modules/CopyModule'))
const TitlesModule = lazy(() => import('./pages/merchant/compose/modules/TitlesModule'))
const VoiceModule = lazy(() => import('./pages/merchant/compose/modules/VoiceModule'))
const AvatarModule = lazy(() => import('./pages/merchant/compose/modules/AvatarModule'))
const EditModule = lazy(() => import('./pages/merchant/compose/modules/EditModule'))
const CoverModule = lazy(() => import('./pages/merchant/compose/modules/CoverModule'))
const ImagesModule = lazy(() => import('./pages/merchant/compose/modules/ImagesModule'))
const InspirationPlaza = lazy(() => import('./pages/merchant/inspire/InspirationPlaza'))
const AssetLibrary = lazy(() => import('./pages/merchant/assets/AssetLibrary'))
const MyWorks = lazy(() => import('./pages/merchant/works/MyWorks'))
const WorkDetail = lazy(() => import('./pages/merchant/works/WorkDetail'))
const WorksAnalytics = lazy(() => import('./pages/merchant/analytics/WorksAnalytics'))
const MyPlan = lazy(() => import('./pages/merchant/MyPlan'))
const Notifications = lazy(() => import('./pages/merchant/Notifications'))
// 管理后台复合页（重组——减少菜单项，Tab 承载子功能；原子页面由复合页 lazy import）
const Dashboard = lazy(() => import('./pages/admin/Dashboard'))
const TenantManagement = lazy(() => import('./pages/admin/TenantManagement'))
const PlatformAssets = lazy(() => import('./pages/admin/PlatformAssets'))
const ContentEngine = lazy(() => import('./pages/admin/ContentEngine'))
const SystemConfig = lazy(() => import('./pages/admin/SystemConfig'))
const CrawlerManagement = lazy(() => import('./pages/admin/CrawlerManagement'))
const AdminBilling = lazy(() => import('./pages/admin/Billing'))
const AdminChat = lazy(() => import('./pages/admin/Chat'))
const Integrations = lazy(() => import('./pages/admin/Integrations'))

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

function RedirectPreserve({ to }: { to: string }) {
  const location = useLocation()
  const base = to.split('?')[0]
  const extra = to.includes('?') ? to.slice(to.indexOf('?') + 1) : ''
  const params = new URLSearchParams(location.search)
  if (extra) {
    new URLSearchParams(extra).forEach((v, k) => { if (!params.has(k)) params.set(k, v) })
  }
  const q = params.toString()
  return <Navigate to={q ? `${base}?${q}` : base} replace />
}

function homePath(role: string | null | undefined) {
  return role === 'admin' ? '/admin' : '/m/compose'
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LazyPage><Login /></LazyPage>} />

        <Route element={<ProtectedRoute><MerchantLayout /></ProtectedRoute>}>
          {/* 商户首页 = 工作台（原创作台）；/m/dashboard 兼容旧链接 */}
          <Route path="/m" element={<Navigate to="/m/compose" replace />} />
          <Route path="/m/dashboard" element={<Navigate to="/m/compose" replace />} />
          <Route path="/m/brands" element={<LazyPage><Brands /></LazyPage>} />
          <Route path="/m/assets" element={<LazyPage><AssetLibrary /></LazyPage>} />
          <Route path="/m/inspire" element={<LazyPage><InspirationPlaza /></LazyPage>} />
          <Route path="/m/compose" element={<LazyPage><ComposeHub /></LazyPage>} />
          <Route path="/m/compose/quick" element={<LazyPage><QuickGenerate /></LazyPage>} />
          <Route path="/m/compose/submit" element={<Navigate to="/m/compose/quick" replace />} />
          <Route path="/m/compose/lipsync" element={<LazyPage><LipSyncWizard /></LazyPage>} />
          <Route path="/m/compose/video" element={<LazyPage><VideoTrackRedirect /></LazyPage>} />
          <Route path="/m/compose/graphic" element={<LazyPage><ComposeTrackPage track="graphic" /></LazyPage>} />
          <Route path="/m/compose/benchmark" element={<LazyPage><BenchmarkModule /></LazyPage>} />
          <Route path="/m/compose/copy" element={<LazyPage><CopyModule /></LazyPage>} />
          <Route path="/m/compose/script" element={<Navigate to="/m/compose/copy" replace />} />
          <Route path="/m/compose/rewrite" element={<Navigate to="/m/compose/copy" replace />} />
          <Route path="/m/compose/titles" element={<LazyPage><TitlesModule /></LazyPage>} />
          <Route path="/m/compose/voice" element={<LazyPage><VoiceModule /></LazyPage>} />
          <Route path="/m/compose/avatar" element={<LazyPage><AvatarModule /></LazyPage>} />
          <Route path="/m/compose/edit" element={<LazyPage><EditModule /></LazyPage>} />
          <Route path="/m/compose/cover" element={<LazyPage><CoverModule /></LazyPage>} />
          <Route path="/m/compose/images" element={<LazyPage><ImagesModule /></LazyPage>} />
          <Route path="/m/compose/tools" element={<LazyPage><Studio /></LazyPage>} />
          <Route path="/m/works" element={<LazyPage><MyWorks /></LazyPage>} />
          <Route path="/m/works/:workId" element={<LazyPage><WorkDetail /></LazyPage>} />
          <Route path="/m/analytics" element={<LazyPage><WorksAnalytics /></LazyPage>} />
          {/* 兼容旧路由 */}
          {/* 旧 checkup 深链保留 ?tab=ask|report|records → 作品数据 AI Drawer */}
          <Route path="/m/checkup" element={<RedirectPreserve to="/m/analytics" />} />
          <Route path="/m/keywords" element={<RedirectPreserve to="/m/analytics" />} />
          <Route path="/m/indexing-report" element={<Navigate to="/m/analytics" replace />} />
          <Route path="/m/visibility" element={<Navigate to="/m/analytics" replace />} />
          <Route path="/m/studio" element={<RedirectPreserve to="/m/compose/benchmark" />} />
          <Route path="/m/content" element={<RedirectPreserve to="/m/compose/tools?tab=article" />} />
          <Route path="/m/creation" element={<RedirectPreserve to="/m/compose/tools?tab=media" />} />
          <Route path="/m/nearby" element={<Navigate to="/m/brands" replace />} />
          <Route path="/m/studio-legacy" element={<Navigate to="/m/compose/tools" replace />} />
          <Route path="/m/checkup-legacy" element={<LazyPage><Checkup /></LazyPage>} />
          <Route path="/m/distribution" element={<LazyPage><Distribution /></LazyPage>} />
          <Route path="/m/my-plan" element={<LazyPage><MyPlan /></LazyPage>} />
          <Route path="/m/chat" element={<LazyPage><Chat /></LazyPage>} />
          <Route path="/m/notifications" element={<LazyPage><Notifications /></LazyPage>} />
        </Route>

        <Route element={<ProtectedRoute role="admin"><AdminLayout /></ProtectedRoute>}>
          <Route path="/admin" element={<LazyPage><Dashboard /></LazyPage>} />
          {/* 复合页（新菜单结构） */}
          <Route path="/admin/tenants" element={<LazyPage><TenantManagement /></LazyPage>} />
          <Route path="/admin/assets" element={<LazyPage><PlatformAssets /></LazyPage>} />
          <Route path="/admin/engine" element={<LazyPage><ContentEngine /></LazyPage>} />
          <Route path="/admin/system" element={<LazyPage><SystemConfig /></LazyPage>} />
          <Route path="/admin/crawler" element={<LazyPage><CrawlerManagement /></LazyPage>} />
          <Route path="/admin/billing" element={<LazyPage><AdminBilling /></LazyPage>} />
          <Route path="/admin/chat" element={<LazyPage><AdminChat /></LazyPage>} />
          <Route path="/admin/integrations/:id" element={<LazyPage><Integrations /></LazyPage>} />
          {/* 旧路由重定向到复合页 */}
          <Route path="/admin/users" element={<Navigate to="/admin/tenants" replace />} />
          <Route path="/admin/brands" element={<Navigate to="/admin/tenants?tab=brands" replace />} />
          <Route path="/admin/contents" element={<Navigate to="/admin/tenants?tab=contents" replace />} />
          <Route path="/admin/voices" element={<Navigate to="/admin/assets?tab=voices" replace />} />
          <Route path="/admin/subjects" element={<Navigate to="/admin/assets?tab=subjects" replace />} />
          <Route path="/admin/settings" element={<Navigate to="/admin/system" replace />} />
          <Route path="/admin/integrations" element={<Navigate to="/admin/system?tab=integrations" replace />} />
          <Route path="/admin/agent-configs" element={<Navigate to="/admin/engine" replace />} />
          <Route path="/admin/prompt-templates" element={<Navigate to="/admin/engine?tab=prompts" replace />} />
          <Route path="/admin/generation-templates" element={<Navigate to="/admin/engine?tab=templates" replace />} />
          <Route path="/admin/indexing" element={<Navigate to="/admin/engine?tab=indexing" replace />} />
          <Route path="/admin/knowledge" element={<Navigate to="/admin/engine?tab=knowledge" replace />} />
          <Route path="/admin/crawler-accounts" element={<Navigate to="/admin/crawler?tab=accounts" replace />} />
          <Route path="/admin/crawler-configs" element={<Navigate to="/admin/crawler?tab=configs" replace />} />
          <Route path="/admin/crawler-tasks" element={<Navigate to="/admin/crawler?tab=tasks" replace />} />
          <Route path="/admin/inspirations" element={<Navigate to="/admin/crawler?tab=inspirations" replace />} />
          <Route path="/admin/generation-specs" element={<Navigate to="/admin/system?tab=integrations" replace />} />
          <Route path="/admin/providers" element={<Navigate to="/admin/system?tab=integrations" replace />} />
          <Route path="/admin/model-configs" element={<Navigate to="/admin/system?tab=integrations" replace />} />
        </Route>

        <Route path="/" element={<RootRedirect />} />
        <Route path="*" element={<LazyPage><NotFound /></LazyPage>} />
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
