import { Navigate, Route, Routes } from 'react-router'
import { SessionWatcher } from './components/SessionWatcher'
import { SiteLocale } from './components/SiteLocale'
import { HomePage } from './pages/HomePage'
import { LoginPage } from './pages/LoginPage'
import { NotFoundPage } from './pages/NotFoundPage'
import { RegisterPage } from './pages/RegisterPage'
import { AdminLayout } from './pages/admin/AdminLayout'
import { ApplicationsPage } from './pages/admin/ApplicationsPage'
import { AuditPage } from './pages/admin/AuditPage'
import { HousePage } from './pages/admin/HousePage'
import { HousesPage } from './pages/admin/HousesPage'
import { SitePage } from './pages/admin/SitePage'
import { StatusesPage } from './pages/admin/StatusesPage'
import { UserCardPage } from './pages/admin/UserCardPage'
import { UsersPage } from './pages/admin/UsersPage'

export default function App() {
  return (
    <>
      <SiteLocale />
      <SessionWatcher />
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/admin" element={<AdminLayout />}>
          <Route index element={<Navigate to="applications" replace />} />
          <Route path="applications" element={<ApplicationsPage />} />
          <Route path="users" element={<UsersPage />} />
          <Route path="users/:id" element={<UserCardPage />} />
          <Route path="houses" element={<HousesPage />} />
          <Route path="houses/:id" element={<HousePage />} />
          <Route path="statuses" element={<StatusesPage />} />
          <Route path="site" element={<SitePage />} />
          <Route path="audit" element={<AuditPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </>
  )
}
