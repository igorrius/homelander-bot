import { Navigate, Route, Routes } from 'react-router'
import { SessionWatcher } from './components/SessionWatcher'
import { SiteLocale } from './components/SiteLocale'
import { HomePage } from './pages/HomePage'
import { LoginLinkPage } from './pages/LoginLinkPage'
import { LoginPage } from './pages/LoginPage'
import { NotFoundPage } from './pages/NotFoundPage'
import { OAuthAuthorizePage } from './pages/OAuthAuthorizePage'
import { RegisterPage } from './pages/RegisterPage'
import { SitePageRoute } from './pages/SitePageRoute'
import { CabinetHome } from './pages/cabinet/CabinetHome'
import { CabinetLayout } from './pages/cabinet/CabinetLayout'
import { AdminLayout } from './pages/admin/AdminLayout'
import { ApplicationsPage } from './pages/admin/ApplicationsPage'
import { AuditPage } from './pages/admin/AuditPage'
import { ChatsPage } from './pages/admin/ChatsPage'
import { HousePage } from './pages/admin/HousePage'
import { HousesPage } from './pages/admin/HousesPage'
import { LabelsPage } from './pages/admin/LabelsPage'
import { FeedAdminPage } from './pages/admin/FeedAdminPage'
import { ModulesPage } from './pages/admin/ModulesPage'
import { NotificationsPage } from './pages/admin/NotificationsPage'
import { OverviewPage } from './pages/admin/OverviewPage'
import { SiteSection } from './pages/admin/SiteSection'
import { DataPage } from './pages/admin/DataPage'
import { DataItemPage } from './pages/admin/DataItemPage'
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
        <Route path="/login/link" element={<LoginLinkPage />} />
        <Route path="/oauth/authorize" element={<OAuthAuthorizePage />} />
        <Route path="/my" element={<CabinetLayout />}>
          <Route index element={<CabinetHome />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
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
          <Route path="site" element={<SiteSection />} />
          <Route path="data" element={<DataPage />} />
          <Route path="data/:key" element={<DataItemPage />} />
          <Route path="pages" element={<Navigate to="/admin/site?tab=pages" replace />} />
          <Route path="pages/:id" element={<PagePreviewPage />} />
          <Route path="modules" element={<ModulesPage />} />
          <Route path="modules/feed" element={<FeedAdminPage />} />
          <Route path="notifications" element={<NotificationsPage />} />
          <Route path="audit" element={<AuditPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
        {/* A published page at its own address; after every named route. */}
        <Route path="/:page" element={<SitePageRoute />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </>
  )
}
