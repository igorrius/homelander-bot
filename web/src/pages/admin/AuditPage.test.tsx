import { screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi, type MockRequest } from '../../test/api'
import { adminRoutes } from '../../test/fixtures'
import { renderApp } from '../../test/render'

const entries = [
  { id: 2, at: '2026-11-21T09:00:00Z', action: 'user.approved', target: 'user:5', actorUserId: 1, details: null, ip: '127.0.0.1' },
  { id: 1, at: '2026-11-21T08:00:00Z', action: 'module.x', target: 'house:1', actorUserId: null, details: null, ip: '127.0.0.1' },
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
    expect(screen.getByRole('link', { name: 'user:5' })).toHaveAttribute('href', '/admin/users/5')
    expect(screen.getByText('house:1')).toBeInTheDocument()
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
