import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, adminUser, site, userCard } from '../../test/fixtures'
import { renderApp } from '../../test/render'

/** Links inside the page, not the menu (the menu also has "Houses"). */
const main = () => within(screen.getByRole('main'))
const pending = adminUser({ status: 'pending_approval', hasTelegram: false })

describe('OverviewPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('shows the counters with links to the lists', async () => {
    mockApi({ ...adminRoutes, 'GET /admin/summary': { body: { pendingApplications: 2, awaitingActivation: 3, activeUsers: 7 } } })
    renderApp('/admin')
    const pendingCard = await screen.findByRole('link', { name: new RegExp(uk.admin.overview.stats.pending) })
    expect(pendingCard).toHaveAttribute('href', '/admin/applications')
    expect(await within(pendingCard).findByText('2')).toBeInTheDocument()
    const awaiting = screen.getByRole('link', { name: new RegExp(uk.admin.overview.stats.awaiting) })
    expect(awaiting).toHaveAttribute('href', '/admin/users?status=awaiting_activation')
    expect(within(awaiting).getByText('3')).toBeInTheDocument()
    const active = screen.getByRole('link', { name: new RegExp(uk.admin.overview.stats.active) })
    expect(active).toHaveAttribute('href', '/admin/users?status=active')
    expect(within(active).getByText('7')).toBeInTheDocument()
    const houses = main().getByRole('link', { name: new RegExp(uk.admin.overview.stats.houses) })
    expect(houses).toHaveAttribute('href', '/admin/houses')
    expect(await within(houses).findByText('2')).toBeInTheDocument()
  })

  it('shows the registration link with its QR code', async () => {
    mockApi(adminRoutes)
    renderApp('/admin')
    expect(await screen.findByText('http://localhost/register')).toBeInTheDocument()
    expect(screen.getByTitle(uk.qr.title)).toBeInTheDocument()
  })

  it('lists what to set up while registration cannot open', async () => {
    mockApi({ ...adminRoutes, 'GET /admin/houses': { body: [] } })
    renderApp('/admin')
    expect(await screen.findByRole('link', { name: new RegExp(uk.admin.overview.setup.house) })).toHaveAttribute('href', '/admin/houses')
    expect(screen.getByText('http://localhost/register')).toBeInTheDocument()
  })

  it('hides the setup list once houses and statuses exist', async () => {
    mockApi(adminRoutes)
    renderApp('/admin')
    await screen.findByText('http://localhost/register')
    await main().findByRole('link', { name: new RegExp(`${uk.admin.overview.stats.houses}\\s*2`) })
    expect(screen.queryByText(uk.admin.overview.setup.title)).toBeNull()
  })

  it('suggests a community name when none is set', async () => {
    mockApi({ ...adminRoutes, 'GET /site': { body: { ...site, communityName: {} } } })
    renderApp('/admin')
    expect(await screen.findByRole('link', { name: new RegExp(uk.admin.overview.setup.community) })).toHaveAttribute('href', '/admin/site')
  })

  it('approves an application from the overview', async () => {
    const { calls } = mockApi({
      ...adminRoutes,
      'GET /admin/users': { body: { items: [pending], total: 1 } },
      'POST /admin/users/5/approve': { body: userCard({ status: 'awaiting_activation' }) },
    })
    const { user } = renderApp('/admin')
    await user.click(await screen.findByRole('button', { name: uk.admin.applications.approve }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/users/5/approve')).toHaveLength(1))
    expect(callsTo(calls, 'GET /admin/users')[0].query.get('status')).toBe('pending_approval')
    expect(screen.getByRole('link', { name: uk.admin.overview.allApplications })).toHaveAttribute('href', '/admin/applications')
  })

  it('still shows the link and placeholders when the counters fail', async () => {
    mockApi({ ...adminRoutes, 'GET /admin/summary': { status: 500, body: { error: { code: 'internal' } } } })
    renderApp('/admin')
    expect(await screen.findByText('http://localhost/register')).toBeInTheDocument()
    const pendingCard = screen.getByRole('link', { name: new RegExp(uk.admin.overview.stats.pending) })
    expect(await within(pendingCard).findByText('—')).toBeInTheDocument()
  })
})
