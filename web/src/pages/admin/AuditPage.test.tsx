import { screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi, type MockRequest } from '../../test/api'
import { adminRoutes } from '../../test/fixtures'
import { renderApp } from '../../test/render'

const base = { targetName: '', targetNames: null, actorUserId: null, actorName: '', details: null, ip: '127.0.0.1' }
const entries = [
  { ...base, id: 2, at: '2026-11-21T09:00:00Z', action: 'user.approved', target: 'user:5', targetName: 'Olena Test', actorUserId: 1, actorName: 'Adam Admin' },
  { ...base, id: 1, at: '2026-11-21T08:00:00Z', action: 'module.x', target: 'house:1' },
]
const lastQuery = (calls: MockRequest[]) => callsTo(calls, 'GET /admin/audit').at(-1)!.query

describe('AuditPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('lists entries with translated actions', async () => {
    mockApi({ ...adminRoutes, 'GET /admin/audit': { body: { items: entries, total: 2 } } })
    renderApp('/admin/audit')
    expect(await screen.findByText(uk.audit.action.user.approved)).toBeInTheDocument()
    expect(screen.getByText('module.x')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Olena Test' })).toHaveAttribute('href', '/admin/users/5')
    expect(screen.getByRole('link', { name: 'Adam Admin' })).toHaveAttribute('href', '/admin/users/1')
    // Names are not unique: the user's number stays visible beside it.
    const row = screen.getByRole('link', { name: 'Olena Test' }).closest('tr')!
    expect(row.textContent).toContain('#5')
    expect(row.textContent).toContain('#1')
    // No name came with it: the raw target stays.
    expect(screen.getByText('house:1')).toBeInTheDocument()
  })

  it('names houses and statuses with links', async () => {
    const named = [
      { ...base, id: 3, at: '2026-11-21T09:00:00Z', action: 'house.created', target: 'house:2', targetName: 'House B' },
      { ...base, id: 4, at: '2026-11-21T09:00:00Z', action: 'status.updated', target: 'status:2', targetNames: { uk: 'Орендар', en: 'Tenant' } },
    ]
    mockApi({ ...adminRoutes, 'GET /admin/audit': { body: { items: named, total: 2 } } })
    renderApp('/admin/audit')
    expect(await screen.findByRole('link', { name: 'House B' })).toHaveAttribute('href', '/admin/houses/2')
    expect(screen.getByRole('link', { name: 'Орендар' })).toHaveAttribute('href', '/admin/statuses')
  })

  it('pages', async () => {
    const { calls } = mockApi({ ...adminRoutes, 'GET /admin/audit': { body: { items: entries, total: 60 } } })
    const { user } = renderApp('/admin/audit')
    await user.click(await screen.findByRole('button', { name: '2' }))
    await waitFor(() => expect(lastQuery(calls).get('page')).toBe('2'))
  })

  it('filters by user from the URL', async () => {
    const { calls } = mockApi({ ...adminRoutes, 'GET /admin/audit': { body: { items: entries, total: 2 } } })
    const { user } = renderApp('/admin/audit?userId=5')
    await waitFor(() => expect(lastQuery(calls).get('userId')).toBe('5'))
    await user.click(await screen.findByRole('button', { name: uk.admin.audit.clearFilter }))
    await waitFor(() => expect(lastQuery(calls).has('userId')).toBe(false))
  })
})
