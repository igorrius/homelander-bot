import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, adminUser, userCard } from '../../test/fixtures'
import { renderApp } from '../../test/render'

const pending = adminUser({ status: 'pending_approval', hasTelegram: false })
const routes = {
  ...adminRoutes,
  'GET /admin/summary': { body: { pendingApplications: 1 } },
  'GET /admin/users': { body: { items: [pending], total: 1 } },
  'POST /admin/users/5/approve': { body: userCard({ status: 'awaiting_activation' }) },
  'POST /admin/users/5/reject': { body: userCard({ status: 'rejected' }) },
}

describe('ApplicationsPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('lists pending applications', async () => {
    const { calls } = mockApi(routes)
    renderApp('/admin/applications')
    expect(await screen.findByRole('link', { name: 'Olena Test' })).toHaveAttribute('href', '/admin/users/5')
    expect(screen.getByText('+380500000002')).toBeInTheDocument()
    expect(screen.getByText(/House A/)).toBeInTheDocument()
    expect(callsTo(calls, 'GET /admin/users')[0].query.get('status')).toBe('pending_approval')
    // A list, not a table: the buttons stay visible on a phone.
    expect(screen.queryByRole('table')).toBeNull()
  })

  it('opens the registration link from the header', async () => {
    mockApi(routes)
    const { user } = renderApp('/admin/applications')
    await user.click(await screen.findByRole('button', { name: uk.admin.invite.open }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('http://localhost/register')).toBeInTheDocument()
    expect(within(dialog).getByTitle(uk.qr.title)).toBeInTheDocument()
  })

  it('approves an application', async () => {
    const { calls } = mockApi(routes)
    const { user } = renderApp('/admin/applications')
    await user.click(await screen.findByRole('button', { name: uk.admin.applications.approve }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/users/5/approve')).toHaveLength(1))
    await waitFor(() => expect(callsTo(calls, 'GET /admin/summary').length).toBeGreaterThanOrEqual(2))
  })

  it('rejects after confirmation', async () => {
    const { calls } = mockApi(routes)
    const { user } = renderApp('/admin/applications')
    await user.click(await screen.findByRole('button', { name: uk.admin.applications.reject }))
    await user.click(await screen.findByRole('button', { name: uk.common.cancel }))
    expect(callsTo(calls, 'POST /admin/users/5/reject')).toHaveLength(0)
    await user.click(screen.getByRole('button', { name: uk.admin.applications.reject }))
    await user.click(await screen.findByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/users/5/reject')).toHaveLength(1))
  })

  it('explains in the dialog when the registration link cannot be loaded', async () => {
    mockApi({ ...routes, 'GET /site': { status: 500, body: { error: { code: 'internal' } } } })
    const { user } = renderApp('/admin/applications')
    await user.click(await screen.findByRole('button', { name: uk.admin.invite.open }))
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByText(uk.errors.internal)).toBeInTheDocument()
  })

  it('shows the empty state', async () => {
    mockApi(adminRoutes)
    renderApp('/admin/applications')
    expect(await screen.findByText(uk.admin.applications.empty)).toBeInTheDocument()
  })
})
