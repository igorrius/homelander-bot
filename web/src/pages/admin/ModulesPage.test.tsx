import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, feedModule } from '../../test/fixtures'
import { renderApp } from '../../test/render'

describe('ModulesPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('lists the feed with its name, description, switch and link', async () => {
    mockApi({ ...adminRoutes, 'GET /admin/modules': { body: [feedModule({ updatedAt: '2026-11-27T10:00:00Z', updatedBy: { id: 1, name: 'Adam Admin' } })] } })
    renderApp('/admin/modules')
    const card = await screen.findByTestId('module-feed')
    expect(within(card).getByRole('heading', { name: uk.modules.feed.name })).toBeInTheDocument()
    expect(within(card).getByText(uk.modules.feed.description)).toBeInTheDocument()
    expect(within(card).getByRole('switch')).toBeChecked()
    expect(within(card).getByRole('link', { name: uk.admin.modules.open })).toHaveAttribute('href', '/admin/modules/feed')
    expect(within(card).getByText(/Adam Admin/)).toBeInTheDocument()
  })

  it('turns the feed off', async () => {
    let feed = feedModule()
    const { calls } = mockApi({
      ...adminRoutes,
      'GET /admin/modules': () => ({ body: [feed] }),
      'PUT /admin/modules/feed': (req) => {
        feed = feedModule({ enabled: (req.body as { enabled: boolean }).enabled })
        return { body: feed }
      },
    })
    const { user } = renderApp('/admin/modules')
    const card = await screen.findByTestId('module-feed')
    await user.click(within(card).getByRole('switch'))
    await waitFor(() => expect(callsTo(calls, 'PUT /admin/modules/feed')[0]?.body).toEqual({ enabled: false }))
    await waitFor(() => expect(within(card).getByRole('switch')).not.toBeChecked())
  })

  it('is in the settings menu; the feed is in the community menu while it is on', async () => {
    mockApi(adminRoutes)
    renderApp('/admin/modules')
    const modules = await screen.findByRole('link', { name: uk.admin.nav.modules })
    expect(modules).toHaveAttribute('href', '/admin/modules')
    expect(modules).toHaveAttribute('data-active')
    const feed = await screen.findByRole('link', { name: uk.admin.nav.feed })
    expect(feed).toHaveAttribute('href', '/admin/modules/feed')
    expect(feed).not.toHaveAttribute('data-active')
  })

  it('has no feed menu item while the feed is off', async () => {
    mockApi({ ...adminRoutes, 'GET /admin/modules': { body: [feedModule({ enabled: false })] } })
    renderApp('/admin/modules')
    await screen.findByTestId('module-feed')
    expect(screen.queryByRole('link', { name: uk.admin.nav.feed })).not.toBeInTheDocument()
  })
})
