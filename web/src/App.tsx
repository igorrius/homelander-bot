import { Navigate, Route, Routes } from 'react-router'
import { SessionWatcher } from './components/SessionWatcher'
import { SiteLocale } from './components/SiteLocale'
import { HomePage } from './pages/HomePage'
import { LoginPage } from './pages/LoginPage'
import { NotFoundPage } from './pages/NotFoundPage'
import { RegisterPage } from './pages/RegisterPage'
import { AdminLayout } from './pages/admin/AdminLayout'
import { ApplicationsPage } from './pages/admin/ApplicationsPage'

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
          <Route path="*" element={<NotFoundPage />} />
        </Route>
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </>
  )
}
