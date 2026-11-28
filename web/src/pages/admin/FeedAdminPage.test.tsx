import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminPost, adminRoutes, chat, feedModule, labels, yard } from '../../test/fixtures'
import { currentLocation, renderApp } from '../../test/render'

const off = { status: 404, body: { error: { code: 'module_disabled' } } }
// A label on a chat nobody accepted reaches no one: it is not offered.
const unused = { id: 3, name: 'unused', description: '', targets: [{ type: 'chat' as const, id: 1 }], usedByApplications: false }
const routes = {
  ...adminRoutes,
  'GET /admin/chats': { body: [chat(), yard] },
  'GET /admin/labels': { body: [...labels, unused] },
  'GET /admin/feed/settings': { body: { defaultLabelIds: [1] } },
  'GET /admin/feed/posts': { body: { items: [], total: 0 } },
}

describe('FeedAdminPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('shows only the turned-off card and the switch while the module is off', async () => {
    let feed = feedModule({ enabled: false })
    const { calls } = mockApi({
      ...routes,
      'GET /admin/modules': () => ({ body: [feed] }),
      'PUT /admin/modules/feed': () => {
        feed = feedModule({ enabled: true })
        return { body: feed }
      },
      'GET /admin/feed/posts': off,
      'GET /admin/feed/settings': off,
    })
    const { user } = renderApp('/admin/modules/feed')
    expect(await screen.findByText(uk.admin.moduleOff.title)).toBeInTheDocument()
    expect(screen.getByText(uk.admin.moduleOff.text)).toBeInTheDocument()
    expect(screen.queryByRole('tab')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: uk.admin.feed.newPost })).not.toBeInTheDocument()
    expect(callsTo(calls, 'GET /admin/feed/posts')).toHaveLength(0)
    await user.click(screen.getByRole('switch'))
    await waitFor(() => expect(callsTo(calls, 'PUT /admin/modules/feed')[0]?.body).toEqual({ enabled: true }))
    expect(await screen.findByRole('tab', { name: uk.admin.feed.tabs.posts })).toBeInTheDocument()
  })

  it('lists posts with their state and delivery counts', async () => {
    const published = adminPost({
      id: 2, body: 'Lift repair on Monday', state: 'published', publishedAt: '2026-11-27T12:00:00Z', labelIds: [1],
      delivery: { pending: 1, sent: 2, failed: 3 },
    })
    const cabinetOnly = adminPost({ id: 3, body: 'Cabinet news', state: 'published', publishedAt: '2026-11-27T13:00:00Z' })
    mockApi({ ...routes, 'GET /admin/feed/posts': { body: { items: [cabinetOnly, published, adminPost()], total: 3 } } })
    renderApp('/admin/modules/feed')
    const pub = await screen.findByTestId('post-2')
    expect(within(pub).getByText('Lift repair on Monday')).toBeInTheDocument()
    expect(within(pub).getByText(uk.admin.feed.state.published)).toBeInTheDocument()
    expect(within(pub).getByText('Надіслано: 2')).toBeInTheDocument()
    expect(within(pub).getByText('У черзі: 1')).toBeInTheDocument()
    expect(within(pub).getByText('Помилок: 3')).toBeInTheDocument()
    expect(within(pub).getByRole('link')).toHaveAttribute('href', '/admin/modules/feed/posts/2')
    const draft = screen.getByTestId('post-1')
    expect(within(draft).getByText(uk.admin.feed.state.draft)).toBeInTheDocument()
    expect(within(draft).queryByText(/Надіслано/)).not.toBeInTheDocument()
    expect(within(screen.getByTestId('post-3')).getByText(uk.admin.feed.delivery.cabinetOnly)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: uk.admin.nav.feed })).toHaveAttribute('data-active')
  })

  it('explains an empty feed and opens a new post', async () => {
    mockApi(routes)
    const { user } = renderApp('/admin/modules/feed')
    expect(await screen.findByText(uk.admin.feed.empty)).toBeInTheDocument()
    await user.click(screen.getByRole('link', { name: uk.admin.feed.newPost }))
    await waitFor(() => expect(currentLocation()).toBe('/admin/modules/feed/posts/new'))
  })

  it('pages through many posts', async () => {
    const { calls } = mockApi({
      ...routes,
      'GET /admin/feed/posts': (req) => ({ body: { items: [adminPost({ id: Number(req.query.get('page')) * 100 })], total: 45 } }),
    })
    const { user } = renderApp('/admin/modules/feed')
    await screen.findByTestId('post-100')
    expect(callsTo(calls, 'GET /admin/feed/posts')[0]?.query.get('pageSize')).toBe('20')
    await user.click(screen.getByRole('button', { name: '2' }))
    expect(await screen.findByTestId('post-200')).toBeInTheDocument()
    expect(currentLocation()).toContain('page=2')
  })

  it('saves the default labels, offering only labels that reach an accepted chat', async () => {
    const { calls } = mockApi({ ...routes, 'PUT /admin/feed/settings': (req) => ({ body: req.body }) })
    const { user } = renderApp('/admin/modules/feed?tab=settings')
    const field = await screen.findByRole('combobox', { name: uk.admin.feed.settings.defaultLabels })
    await user.click(field)
    expect(await screen.findByRole('option', { name: 'journal' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: 'unused' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('option', { name: 'journal' }))
    await user.click(screen.getByRole('button', { name: uk.common.save }))
    await waitFor(() => expect(callsTo(calls, 'PUT /admin/feed/settings')[0]?.body).toEqual({ defaultLabelIds: [1, 2] }))
    expect(await screen.findByText(uk.common.saved)).toBeInTheDocument()
  })
})
