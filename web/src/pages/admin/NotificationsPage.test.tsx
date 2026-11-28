import { screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, chat, labels, yard } from '../../test/fixtures'
import { renderApp } from '../../test/render'
import { choose } from '../../test/select'

const routes = (applicationsLabelId: number | null) => ({
  ...adminRoutes,
  'GET /admin/chats': { body: [chat(), yard] },
  'GET /admin/labels': { body: [...labels, { id: 3, name: 'unused', description: '', targets: [{ type: 'chat', id: 1 }], users: [], usedByApplications: false }] },
  'GET /admin/settings/notifications': { body: { applicationsLabelId } },
})

describe('NotificationsPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('saves and clears the label', async () => {
    const { calls } = mockApi({
      ...routes(null),
      'PUT /admin/settings/notifications': (req) => ({ body: req.body }),
    })
    const { user } = renderApp('/admin/notifications')
    await choose(user, uk.admin.notifications.applications, 'journal')
    await user.click(screen.getByRole('button', { name: uk.common.save }))
    await waitFor(() => expect(callsTo(calls, 'PUT /admin/settings/notifications')[0]?.body).toEqual({ applicationsLabelId: 2 }))
    await user.click(await screen.findByRole('button', { name: uk.admin.notifications.off }))
    await user.click(screen.getByRole('button', { name: uk.common.save }))
    await waitFor(() => expect(callsTo(calls, 'PUT /admin/settings/notifications')[1]?.body).toEqual({ applicationsLabelId: null }))
  })

  it('warns when the label reaches no chat', async () => {
    mockApi(routes(3))
    renderApp('/admin/notifications')
    expect(await screen.findByText(uk.admin.notifications.noChat)).toBeInTheDocument()
  })

  it('is in the settings menu', async () => {
    mockApi(routes(null))
    renderApp('/admin/notifications')
    expect(await screen.findByRole('link', { name: new RegExp(uk.admin.nav.notifications) })).toHaveAttribute('href', '/admin/notifications')
  })
})
