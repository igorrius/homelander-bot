import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { cabinetPost, feedCabinetModule, residentRoutes } from '../../test/fixtures'
import { currentLocation, renderApp } from '../../test/render'

const off = { status: 404, body: { error: { code: 'module_disabled' } } }
const posts = (from: number, n: number) => Array.from({ length: n }, (_, i) => cabinetPost({ id: from + i, body: `Post ${from + i}` }))
const routes = {
  ...residentRoutes,
  'GET /modules': { body: [feedCabinetModule] },
  'GET /feed/posts': { body: { items: posts(1, 5), nextCursor: null } },
}

describe('Cabinet feed', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('adds the feed to the navigation only while the module is on', async () => {
    mockApi(routes)
    renderApp('/my/feed/3')
    const nav = await screen.findByRole('navigation', { name: uk.cabinet.nav.label })
    const link = await within(nav).findByRole('link', { name: uk.cabinet.nav.feed })
    expect(link).toHaveAttribute('href', '/my/feed')
    expect(link).toHaveAttribute('aria-current', 'page')
    // The feed comes after "My home".
    const names = within(nav).getAllByRole('link').map((a) => a.textContent)
    expect(names).toEqual([uk.cabinet.nav.home, uk.cabinet.nav.feed, uk.cabinet.nav.site])
  })

  it('has no feed in the navigation while the module is off', async () => {
    mockApi({ ...residentRoutes, 'GET /feed/posts': off })
    renderApp('/my')
    const nav = await screen.findByRole('navigation', { name: uk.cabinet.nav.label })
    await screen.findByRole('heading', { name: uk.cabinet.myHome.title })
    expect(within(nav).queryByRole('link', { name: uk.cabinet.nav.feed })).not.toBeInTheDocument()
  })

  it('shows the latest three posts on top of the home with a link to all posts', async () => {
    mockApi(routes)
    const { user } = renderApp('/my')
    const card = await screen.findByTestId('cabinet-feed')
    expect(within(card).getAllByRole('article')).toHaveLength(3)
    expect(within(card).getByText('Post 1')).toBeInTheDocument()
    expect(within(card).queryByText('Post 4')).not.toBeInTheDocument()
    // It is the first card, above "My home".
    const home = screen.getByRole('heading', { name: uk.cabinet.myHome.title })
    expect(card.compareDocumentPosition(home) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    await user.click(within(card).getByRole('link', { name: uk.cabinet.feed.allPosts }))
    await waitFor(() => expect(currentLocation()).toBe('/my/feed'))
  })

  it('leaves the feed out of the home when the module is off or there are no posts', async () => {
    mockApi({ ...routes, 'GET /feed/posts': off })
    const { unmount } = renderApp('/my')
    await screen.findByRole('heading', { name: uk.cabinet.myHome.title })
    await waitFor(() => expect(screen.queryByText(uk.cabinet.feed.latest)).not.toBeInTheDocument())
    unmount()
    const { calls } = mockApi({ ...routes, 'GET /feed/posts': { body: { items: [], nextCursor: null } } })
    renderApp('/my')
    await waitFor(() => expect(callsTo(calls, 'GET /feed/posts')).toHaveLength(1))
    await screen.findByRole('heading', { name: uk.cabinet.myHome.title })
    expect(screen.queryByTestId('cabinet-feed')).not.toBeInTheDocument()
  })

  it('lists posts with author, date, text and image, and loads more by cursor', async () => {
    const first = [cabinetPost({ id: 30, body: 'Line one\nsee https://example.com/news', imageUrl: '/api/v1/feed/posts/30/image', authorName: 'Adam' }), ...posts(10, 19)]
    const { calls } = mockApi({
      ...routes,
      'GET /feed/posts': (req) =>
        req.query.get('cursor') === 'c1'
          ? { body: { items: [cabinetPost({ id: 5, body: 'Older post' })], nextCursor: null } }
          : { body: { items: first, nextCursor: 'c1' } },
    })
    const { user } = renderApp('/my/feed')
    expect(await screen.findByRole('heading', { name: uk.cabinet.feed.title })).toBeInTheDocument()
    const top = await screen.findByTestId('feed-post-30')
    expect(within(top).getByText('Adam')).toBeInTheDocument()
    expect(within(top).getByRole('link', { name: 'https://example.com/news' })).toHaveAttribute('target', '_blank')
    const img = within(top).getByRole('img', { name: uk.cabinet.feed.image })
    expect(img).toHaveAttribute('src', '/api/v1/feed/posts/30/image')
    expect(img).toHaveAttribute('loading', 'lazy')
    expect(within(top).getByRole('link', { name: new RegExp(uk.cabinet.feed.open) })).toHaveAttribute('href', '/my/feed/30')
    expect(screen.getAllByRole('article')).toHaveLength(20)
    await user.click(screen.getByRole('button', { name: uk.cabinet.feed.showMore }))
    expect(await screen.findByText('Older post')).toBeInTheDocument()
    expect(callsTo(calls, 'GET /feed/posts')[1]?.query.get('cursor')).toBe('c1')
    expect(screen.queryByRole('button', { name: uk.cabinet.feed.showMore })).not.toBeInTheDocument()
  })

  it('explains an empty feed', async () => {
    mockApi({ ...routes, 'GET /feed/posts': { body: { items: [], nextCursor: null } } })
    renderApp('/my/feed')
    expect(await screen.findByText(uk.cabinet.feed.empty)).toBeInTheDocument()
  })

  it('opens one post with a link back to the feed', async () => {
    mockApi({ ...routes, 'GET /feed/posts/7': { body: cabinetPost({ id: 7, body: 'Lift repair on Monday' }) } })
    const { user } = renderApp('/my/feed/7')
    expect(await screen.findByText('Lift repair on Monday')).toBeInTheDocument()
    expect(screen.getAllByRole('article')).toHaveLength(1)
    await user.click(screen.getByRole('link', { name: uk.cabinet.feed.back }))
    await waitFor(() => expect(currentLocation()).toBe('/my/feed'))
  })

  it('shows not found for a missing post and for the feed while the module is off', async () => {
    mockApi({ ...routes, 'GET /feed/posts/9': { status: 404, body: { error: { code: 'not_found' } } } })
    const { unmount } = renderApp('/my/feed/9')
    expect(await screen.findByText(uk.notFound.title)).toBeInTheDocument()
    unmount()
    mockApi({ ...residentRoutes, 'GET /feed/posts': off, 'GET /feed/posts/7': off })
    const second = renderApp('/my/feed')
    expect(await screen.findByText(uk.notFound.title)).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: uk.cabinet.nav.label })).toBeInTheDocument()
    second.unmount()
    renderApp('/my/feed/7')
    expect(await screen.findByText(uk.notFound.title)).toBeInTheDocument()
  })
})
