import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Chat from './pages/Chat'
import AgentConfigs from './pages/AgentConfigs'
import DataItems from './pages/DataItems'
import Tasks from './pages/Tasks'
import CrawlConfigPage from './pages/CrawlConfig'
import ExternalSystems from './pages/ExternalSystems'
import MainLayout from './layouts/MainLayout'
import ProtectedRoute from './components/ProtectedRoute'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route element={<ProtectedRoute><MainLayout /></ProtectedRoute>}>
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/chat" element={<Chat />} />
          <Route path="/agent-configs" element={<AgentConfigs />} />
          <Route path="/data" element={<DataItems />} />
          <Route path="/tasks" element={<Tasks />} />
          <Route path="/crawl-config" element={<CrawlConfigPage />} />
          <Route path="/external-systems" element={<ExternalSystems />} />
          <Route path="/" element={<Navigate to="/chat" replace />} />
        </Route>
        <Route path="*" element={<Navigate to="/chat" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
