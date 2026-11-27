import { Route, Routes } from 'react-router'
import { SessionWatcher } from './components/SessionWatcher'
import { SiteLocale } from './components/SiteLocale'
import { HomePage } from './pages/HomePage'
import { LoginPage } from './pages/LoginPage'
import { NotFoundPage } from './pages/NotFoundPage'
import { OAuthAuthorizePage } from './pages/OAuthAuthorizePage'
import { RegisterPage } from './pages/RegisterPage'
import { AdminLayout } from './pages/admin/AdminLayout'
import { ApplicationsPage } from './pages/admin/ApplicationsPage'
import { AuditPage } from './pages/admin/AuditPage'
import { ChatsPage } from './pages/admin/ChatsPage'
import { HousePage } from './pages/admin/HousePage'
import { HousesPage } from './pages/admin/HousesPage'
import { LabelsPage } from './pages/admin/LabelsPage'
import { NotificationsPage } from './pages/admin/NotificationsPage'
import { OverviewPage } from './pages/admin/OverviewPage'
import { SitePage } from './pages/admin/SitePage'
import { DataPage } from './pages/admin/DataPage'
import { DataItemPage } from './pages/admin/DataItemPage'
import { PagesPage } from './pages/admin/PagesPage'
import { PagePreviewPage } from './pages/admin/PagePreviewPage'
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
        <Route path="/oauth/authorize" element={<OAuthAuthorizePage />} />
        <Route path="/admin" element={<AdminLayout />}>
          <Route index element={<OverviewPage />} />
          <Route path="applications" element={<ApplicationsPage />} />
          <Route path="users" element={<UsersPage />} />
          <Route path="users/:id" element={<UserCardPage />} />
          <Route path="houses" element={<HousesPage />} />
          <Route path="houses/:id" element={<HousePage />} />
          <Route path="chats" element={<ChatsPage />} />
          <Route path="chats/labels" element={<LabelsPage />} />
          <Route path="statuses" element={<StatusesPage />} />
          <Route path="site" element={<SitePage />} />
          <Route path="data" element={<DataPage />} />
          <Route path="data/:key" element={<DataItemPage />} />
          <Route path="pages" element={<PagesPage />} />
          <Route path="pages/:id" element={<PagePreviewPage />} />
          <Route path="notifications" element={<NotificationsPage />} />
          <Route path="audit" element={<AuditPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </>
  )
}
