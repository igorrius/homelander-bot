import { screen, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { mockApi } from '../../test/api'
import { adminRoutes, publicRoutes, resident } from '../../test/fixtures'
import { renderApp } from '../../test/render'

describe('AdminLayout', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('sends a guest to /login', async () => {
    mockApi(publicRoutes)
    renderApp('/admin')
    expect(await screen.findByRole('heading', { name: uk.login.title })).toBeInTheDocument()
  })

  it('sends a resident to /', async () => {
    mockApi({ ...publicRoutes, 'GET /me': { body: resident } })
    renderApp('/admin/users')
    expect(await screen.findByRole('heading', { name: 'Тестова громада' })).toBeInTheDocument()
  })

  it('opens the applications queue by default', async () => {
    mockApi(adminRoutes)
    renderApp('/admin')
    expect(await screen.findByRole('heading', { name: uk.admin.applications.title })).toBeInTheDocument()
  })

  it('shows the pending counter in the menu', async () => {
    mockApi({ ...adminRoutes, 'GET /admin/summary': { body: { pendingApplications: 3 } } })
    renderApp('/admin')
    const link = await screen.findByRole('link', { name: new RegExp(uk.admin.nav.applications) })
    expect(await within(link).findByText('3')).toBeInTheDocument()
  })

  it('redirects to login when a request answers 401', async () => {
    mockApi({ ...adminRoutes, 'GET /admin/users': { status: 401, body: { error: { code: 'unauthorized' } } } })
    renderApp('/admin/applications')
    expect(await screen.findByRole('heading', { name: uk.login.title })).toBeInTheDocument()
  })
})
