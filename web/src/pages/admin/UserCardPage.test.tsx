import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import type { UserCard } from '../../api/types'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi, type Route } from '../../test/api'
import { adminRoutes, houses, statuses, userCard } from '../../test/fixtures'
import { renderApp } from '../../test/render'
import { choose } from '../../test/select'

function routes(card: UserCard, extra: Record<string, Route> = {}) {
  return {
    ...adminRoutes,
    'GET /admin/houses': { body: houses },
    'GET /admin/statuses': { body: statuses },
    'GET /admin/users/5': { body: card },
    ...extra,
  }
}

describe('UserCardPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('shows the card', async () => {
    mockApi(routes(userCard()))
    renderApp('/admin/users/5')
    expect(await screen.findByRole('heading', { name: 'Olena Test' })).toBeInTheDocument()
    expect(screen.getByText(uk.userStatus.active)).toBeInTheDocument()
    expect(screen.getByText('@olena_test')).toBeInTheDocument()
    const residencies = screen.getByRole('table', { name: uk.admin.user.residencies })
    expect(within(residencies).getByText('House A')).toBeInTheDocument()
    expect(within(residencies).getByText('12')).toBeInTheDocument()
    expect(within(residencies).getByRole('columnheader', { name: uk.admin.user.residencyStatus })).toBeInTheDocument()
    expect(screen.getByText(uk.audit.action.user.approved)).toBeInTheDocument()
  })

  it('saves only changed details', async () => {
    const { calls } = mockApi(routes(userCard(), { 'PATCH /admin/users/5': { body: userCard({ firstName: 'Olha' }) } }))
    const { user } = renderApp('/admin/users/5')
    const first = await screen.findByLabelText(uk.register.firstName)
    await user.clear(first)
    await user.type(first, 'Olha')
    await user.click(screen.getByRole('button', { name: uk.admin.user.saveDetails }))
    await waitFor(() => expect(callsTo(calls, 'PATCH /admin/users/5').map((c) => c.body)).toEqual([{ firstName: 'Olha' }]))
    expect(await screen.findByRole('heading', { name: 'Olha Test' })).toBeInTheDocument()
  })

  it('toggles admin and shows last-admin refusal', async () => {
    const card = userCard({ isAdmin: true })
    const { calls } = mockApi(routes(card, { 'PATCH /admin/users/5': { status: 409, body: { error: { code: 'last_admin' } } } }))
    const { user } = renderApp('/admin/users/5')
    const toggle = await screen.findByRole('switch', { name: uk.admin.user.isAdmin })
    expect(toggle).toBeChecked()
    await user.click(toggle)
    expect(await screen.findByText(uk.errors.last_admin)).toBeInTheDocument()
    expect(callsTo(calls, 'PATCH /admin/users/5').map((c) => c.body)).toEqual([{ isAdmin: false }])
    expect(screen.getByRole('switch', { name: uk.admin.user.isAdmin })).toBeChecked()
  })

  it('unlinks Telegram after confirmation', async () => {
    const after = { ...userCard({ status: 'awaiting_activation', hasTelegram: false }), channels: [] }
    const { calls } = mockApi(routes(userCard(), { 'DELETE /admin/users/5/channels/9': { body: after } }))
    const { user } = renderApp('/admin/users/5')
    await user.click(await screen.findByRole('button', { name: uk.admin.user.unlink }))
    await user.click(await screen.findByRole('button', { name: uk.common.confirm }))
    expect(await screen.findByText(uk.userStatus.awaiting_activation)).toBeInTheDocument()
    expect(callsTo(calls, 'DELETE /admin/users/5/channels/9')).toHaveLength(1)
  })

  it('approves a pending user', async () => {
    const { calls } = mockApi(
      routes(userCard({ status: 'pending_approval' }), { 'POST /admin/users/5/approve': { body: userCard({ status: 'awaiting_activation' }) } }),
    )
    const { user } = renderApp('/admin/users/5')
    expect(screen.queryByRole('button', { name: uk.admin.user.disable })).not.toBeInTheDocument()
    await user.click(await screen.findByRole('button', { name: uk.admin.user.approve }))
    expect(await screen.findByText(uk.userStatus.awaiting_activation)).toBeInTheDocument()
    expect(callsTo(calls, 'POST /admin/users/5/approve')).toHaveLength(1)
  })

  it('enables a disabled user and hides disable', async () => {
    const { calls } = mockApi(routes(userCard({ status: 'disabled' }), { 'POST /admin/users/5/enable': { body: userCard() } }))
    const { user } = renderApp('/admin/users/5')
    await user.click(await screen.findByRole('button', { name: uk.admin.user.enable }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/users/5/enable')).toHaveLength(1))
    expect(await screen.findByRole('button', { name: uk.admin.user.disable })).toBeInTheDocument()
  })

  it('disables an active user after confirmation', async () => {
    const { calls } = mockApi(routes(userCard(), { 'POST /admin/users/5/disable': { body: userCard({ status: 'disabled' }) } }))
    const { user } = renderApp('/admin/users/5')
    await user.click(await screen.findByRole('button', { name: uk.admin.user.disable }))
    await user.click(await screen.findByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/users/5/disable')).toHaveLength(1))
  })

  it('adds a residency', async () => {
    const { calls } = mockApi(routes(userCard(), { 'POST /admin/users/5/residencies': { status: 201, body: userCard() } }))
    const { user } = renderApp('/admin/users/5')
    await user.click(await screen.findByRole('button', { name: uk.admin.user.addResidency }))
    await choose(user, uk.register.house, 'House B')
    await choose(user, uk.register.apartment, '41')
    await choose(user, uk.admin.user.residencyStatus, 'Орендар')
    await user.click(screen.getByRole('button', { name: uk.admin.user.saveResidency }))
    await waitFor(() =>
      expect(callsTo(calls, 'POST /admin/users/5/residencies').map((c) => c.body)).toEqual([{ houseId: 2, apartment: 41, statusId: 2 }]),
    )
  })

  it('edits a residency', async () => {
    const { calls } = mockApi(routes(userCard(), { 'PATCH /admin/residencies/11': { body: userCard() } }))
    const { user } = renderApp('/admin/users/5')
    await user.click(await screen.findByRole('button', { name: uk.admin.user.editResidency }))
    await choose(user, uk.register.apartment, '13')
    await user.click(screen.getByRole('button', { name: uk.admin.user.saveResidency }))
    await waitFor(() =>
      expect(callsTo(calls, 'PATCH /admin/residencies/11').map((c) => c.body)).toEqual([{ houseId: 1, apartment: 13, statusId: 1 }]),
    )
  })

  it('removes a residency after confirmation', async () => {
    const { calls } = mockApi(routes(userCard(), { 'DELETE /admin/residencies/11': { body: { ...userCard(), residencies: [] } } }))
    const { user } = renderApp('/admin/users/5')
    await user.click(await screen.findByRole('button', { name: uk.admin.user.removeResidency }))
    await user.click(await screen.findByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'DELETE /admin/residencies/11')).toHaveLength(1))
  })

  it('shows not found for a missing user', async () => {
    mockApi(routes(userCard()))
    renderApp('/admin/users/99')
    expect(await screen.findByText(uk.errors.not_found)).toBeInTheDocument()
  })

  it('clears a refused flag error after a later action succeeds', async () => {
    mockApi(
      routes(userCard({ status: 'pending_approval', isAdmin: true }), {
        'PATCH /admin/users/5': { status: 409, body: { error: { code: 'last_admin' } } },
        'POST /admin/users/5/approve': { body: userCard({ status: 'awaiting_activation', isAdmin: true }) },
      }),
    )
    const { user } = renderApp('/admin/users/5')
    await user.click(await screen.findByRole('switch', { name: uk.admin.user.isAdmin }))
    expect(await screen.findByText(uk.errors.last_admin)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: uk.admin.user.approve }))
    expect(await screen.findByText(uk.userStatus.awaiting_activation)).toBeInTheDocument()
    expect(screen.queryByText(uk.errors.last_admin)).not.toBeInTheDocument()
  })

  it("does not carry one user's errors to another user's cached card", async () => {
    const other = userCard({ id: 1, firstName: 'Adam', lastName: 'Admin', isAdmin: true })
    mockApi(
      routes(userCard({ isAdmin: true }), {
        'GET /admin/users/1': { body: other },
        'PATCH /admin/users/5': { status: 409, body: { error: { code: 'last_admin' } } },
      }),
    )
    const { user } = renderApp('/admin/users/1')
    expect(await screen.findByRole('heading', { name: 'Adam Admin' })).toBeInTheDocument()
    // Card 1 lists an audit entry targeting user:5; open it, fail a change there, then go back.
    await user.click(screen.getByRole('link', { name: 'user:5' }))
    await user.click(await screen.findByRole('switch', { name: uk.admin.user.isAdmin }))
    expect(await screen.findByText(uk.errors.last_admin)).toBeInTheDocument()
    await user.click(screen.getAllByRole('link', { name: 'user:1' })[0])
    expect(await screen.findByRole('heading', { name: 'Adam Admin' })).toBeInTheDocument()
    expect(screen.queryByText(uk.errors.last_admin)).not.toBeInTheDocument()
  })
})
