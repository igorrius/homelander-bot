import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi, type MockRequest } from '../../test/api'
import { adminRoutes, adminUser, houses, userCard } from '../../test/fixtures'
import { currentLocation, renderApp } from '../../test/render'
import { choose } from '../../test/select'

const routes = {
  ...adminRoutes,
  'GET /admin/houses': { body: houses },
  'GET /admin/users': { body: { items: [adminUser()], total: 1 } },
}
const lastQuery = (calls: MockRequest[]) => callsTo(calls, 'GET /admin/users').at(-1)!.query

describe('UsersPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('lists users with status and apartments', async () => {
    mockApi(routes)
    renderApp('/admin/users')
    const row = (await screen.findByRole('link', { name: 'Olena Test' })).closest('tr')!
    expect(within(row).getByText(uk.userStatus.active)).toBeInTheDocument()
    expect(within(row).getByText(/House A/)).toBeInTheDocument()
    expect(within(row).getByLabelText(uk.admin.users.linked)).toBeInTheDocument()
    expect(row.textContent).not.toContain('✓')
  })

  it('searches by text', async () => {
    const { calls } = mockApi(routes)
    const { user } = renderApp('/admin/users')
    await user.type(await screen.findByLabelText(uk.admin.users.search), 'Olena')
    await waitFor(() => expect(lastQuery(calls).get('q')).toBe('Olena'))
    expect(lastQuery(calls).get('page')).toBe('1')
    expect(callsTo(calls, 'GET /admin/users').filter((c) => c.query.get('q')?.startsWith('Ol')).length).toBe(1)
  })

  it('filters by house, status and flags', async () => {
    const { calls } = mockApi(routes)
    const { user } = renderApp('/admin/users')
    await choose(user, uk.admin.users.house, 'House A')
    await user.click(screen.getByRole('tab', { name: new RegExp(uk.userStatus.active) }))
    await choose(user, uk.admin.users.isAdmin, uk.common.yes)
    await waitFor(() => expect(lastQuery(calls).get('isAdmin')).toBe('true'))
    expect(lastQuery(calls).get('houseId')).toBe('1')
    expect(lastQuery(calls).get('status')).toBe('active')
    expect(currentLocation()).toContain('status=active')
  })

  it('selects the status tab from the address', async () => {
    const { calls } = mockApi(routes)
    renderApp('/admin/users?status=awaiting_activation')
    expect(await screen.findByRole('tab', { name: new RegExp(uk.userStatus.awaiting_activation) })).toHaveAttribute('aria-selected', 'true')
    expect(lastQuery(calls).get('status')).toBe('awaiting_activation')
  })

  it('filters by a status tab and back to all', async () => {
    const { calls } = mockApi(routes)
    const { user } = renderApp('/admin/users')
    await user.click(await screen.findByRole('tab', { name: new RegExp(uk.userStatus.disabled) }))
    await waitFor(() => expect(lastQuery(calls).get('status')).toBe('disabled'))
    await user.click(screen.getByRole('tab', { name: new RegExp(uk.admin.users.tabs.all) }))
    await waitFor(() => expect(lastQuery(calls).get('status')).toBeNull())
  })

  it('shows status counts in the tabs', async () => {
    mockApi({ ...routes, 'GET /admin/summary': { body: { pendingApplications: 4, awaitingActivation: 3, activeUsers: 7, disabledUsers: 2, rejectedUsers: 1 } } })
    renderApp('/admin/users')
    expect(await screen.findByRole('tab', { name: new RegExp(`${uk.userStatus.active}\\s*7`) })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: new RegExp(`${uk.userStatus.disabled}\\s*2`) })).toBeInTheDocument()
  })

  it('keeps the tabs when the counts fail', async () => {
    mockApi({ ...routes, 'GET /admin/summary': { status: 500, body: { error: { code: 'internal' } } } })
    renderApp('/admin/users')
    expect(await screen.findByRole('tab', { name: new RegExp(uk.userStatus.active) })).toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: uk.admin.users.status })).toBeNull()
  })

  it('pages through results', async () => {
    const { calls } = mockApi({ ...routes, 'GET /admin/users': { body: { items: [adminUser()], total: 60 } } })
    const { user } = renderApp('/admin/users')
    await user.click(await screen.findByRole('button', { name: '3' }))
    expect(screen.queryByRole('button', { name: '4' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '2' }))
    await waitFor(() => expect(lastQuery(calls).get('page')).toBe('2'))
  })

  it('adds a user and opens the card', async () => {
    const card = userCard({ id: 7, firstName: 'Petro', lastName: 'Test', phone: '+380500000003', status: 'awaiting_activation' })
    const { calls } = mockApi({ ...routes, 'POST /admin/users': { status: 201, body: card }, 'GET /admin/users/7': { body: card } })
    const { user } = renderApp('/admin/users')
    await user.click(await screen.findByRole('button', { name: uk.admin.users.add }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText(uk.register.phone), '+380500000003')
    await user.type(within(dialog).getByLabelText(uk.register.firstName), 'Petro')
    await user.type(within(dialog).getByLabelText(uk.register.lastName), 'Test')
    await user.click(within(dialog).getByRole('checkbox', { name: uk.admin.user.isAdmin }))
    await user.click(within(dialog).getByRole('button', { name: uk.common.save }))
    await waitFor(() => expect(currentLocation()).toBe('/admin/users/7'))
    expect(callsTo(calls, 'POST /admin/users').map((c) => c.body)).toEqual([
      { phone: '+380500000003', firstName: 'Petro', lastName: 'Test', isResident: false, isAdmin: true },
    ])
  })

  it('shows a taken phone', async () => {
    mockApi({ ...routes, 'POST /admin/users': { status: 422, body: { error: { code: 'validation', fields: { phone: 'taken' } } } } })
    const { user } = renderApp('/admin/users')
    await user.click(await screen.findByRole('button', { name: uk.admin.users.add }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText(uk.register.phone), '+380500000002')
    await user.type(within(dialog).getByLabelText(uk.register.firstName), 'Petro')
    await user.type(within(dialog).getByLabelText(uk.register.lastName), 'Test')
    await user.click(within(dialog).getByRole('button', { name: uk.common.save }))
    expect(await within(dialog).findByText(uk.validation.taken)).toBeInTheDocument()
  })
})
