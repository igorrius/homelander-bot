import { act, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import type { PageVersion } from '../../api/types'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes } from '../../test/fixtures'
import { currentLocation, renderApp } from '../../test/render'
import { choose } from '../../test/select'

const html = (path: string) => ({ path, size: 100, contentType: 'text/html; charset=utf-8' })

// Synthetic data only.
const draft: PageVersion = {
  id: 8, status: 'draft', note: 'Two pages', files: [html('contacts.html'), html('index.html')], totalSize: 200,
  pages: [{ name: 'index', path: '/' }, { name: 'contacts', path: '/contacts' }],
  createdAt: '2026-11-27T12:00:00Z', createdBy: { id: 2, name: 'Admin B' }, createdVia: 'Claude', publishedAt: null, publishedBy: null,
}

const routes = {
  ...adminRoutes,
  'GET /admin/pages/8': { body: draft },
  'POST /admin/pages/8/preview': { body: { url: '/site/preview/tok8/index.html' } },
}

async function previewFrame() {
  const frame = (await screen.findByTitle(uk.admin.pages.previewTitle)) as HTMLIFrameElement
  await waitFor(() => expect(frame.getAttribute('src')).toBe('/site/preview/tok8/index.html?lang=uk'))
  return frame
}

function fromFrame(frame: HTMLIFrameElement | null, path: string) {
  act(() => {
    window.dispatchEvent(new MessageEvent('message', { data: { type: 'homelander:navigate', path }, source: frame ? frame.contentWindow : window }))
  })
}

describe('PagePreviewPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('switches between the draft pages', async () => {
    mockApi(routes)
    const { user } = renderApp('/admin/pages/8')
    const frame = await previewFrame()
    expect(screen.getByRole('radio', { name: '/' })).toBeChecked()
    await user.click(screen.getByRole('radio', { name: '/contacts' }))
    await waitFor(() => expect(frame.getAttribute('src')).toBe('/site/preview/tok8/contacts.html?lang=uk'))
    await user.click(screen.getByRole('radio', { name: '/' }))
    await waitFor(() => expect(frame.getAttribute('src')).toBe('/site/preview/tok8/index.html?lang=uk'))
  })

  it('follows a link inside the draft to another of its pages', async () => {
    mockApi(routes)
    renderApp('/admin/pages/8')
    const frame = await previewFrame()
    fromFrame(frame, '/contacts')
    await waitFor(() => expect(frame.getAttribute('src')).toBe('/site/preview/tok8/contacts.html?lang=uk'))
    expect(screen.getByRole('radio', { name: '/contacts' })).toBeChecked()
  })

  it('ignores navigation from elsewhere or to pages the draft does not have', async () => {
    mockApi(routes)
    renderApp('/admin/pages/8')
    const frame = await previewFrame()
    fromFrame(null, '/contacts')
    fromFrame(frame, '/admin')
    fromFrame(frame, '/rules')
    expect(frame.getAttribute('src')).toBe('/site/preview/tok8/index.html?lang=uk')
  })

  it('uses a select when the draft has many pages', async () => {
    const names = ['a', 'b', 'c', 'd', 'e', 'f']
    const many = { ...draft, pages: [{ name: 'index', path: '/' }, ...names.map((n) => ({ name: n, path: `/${n}` }))] }
    mockApi({ ...routes, 'GET /admin/pages/8': { body: many } })
    const { user } = renderApp('/admin/pages/8')
    const frame = await previewFrame()
    expect(screen.queryByRole('radio', { name: '/' })).not.toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: uk.admin.pages.previewPage })).toHaveValue('/')
    await choose(user, uk.admin.pages.previewPage, '/f')
    await waitFor(() => expect(frame.getAttribute('src')).toBe('/site/preview/tok8/f.html?lang=uk'))
  })

  it('publishes a draft after confirmation', async () => {
    const { calls } = mockApi({ ...routes, 'POST /admin/pages/8/publish': { status: 204 } })
    const { user } = renderApp('/admin/pages/8')
    const frame = await previewFrame()
    expect(frame).toHaveAttribute('sandbox', 'allow-scripts allow-popups allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation')
    expect(screen.getByText('contacts.html')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: uk.admin.pages.publish }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/pages/8/publish')).toHaveLength(1))
  })

  it('offers no delete or publish for the published version', async () => {
    mockApi({ ...routes, 'GET /admin/pages/8': { body: { ...draft, status: 'published' } } })
    renderApp('/admin/pages/8')
    await previewFrame()
    expect(screen.getByRole('button', { name: uk.admin.pages.copy })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: uk.admin.pages.delete })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: uk.admin.pages.publish })).not.toBeInTheDocument()
  })

  it('copies a version to a new draft', async () => {
    const { calls } = mockApi({ ...routes, 'POST /admin/pages/8/copy': { status: 201, body: { ...draft, id: 9 } } })
    const { user } = renderApp('/admin/pages/8')
    await user.click(await screen.findByRole('button', { name: uk.admin.pages.copy }))
    await waitFor(() => expect(currentLocation()).toBe('/admin/pages/9'))
    expect(callsTo(calls, 'POST /admin/pages/8/copy')).toHaveLength(1)
  })

  it('goes back to the Site section', async () => {
    mockApi(routes)
    renderApp('/admin/pages/8')
    await previewFrame()
    const back = screen.getAllByRole('link', { name: uk.admin.pages.back }).find((a) => a.querySelector('svg.tabler-icon-arrow-left'))
    expect(back).toHaveAttribute('href', '/admin/site?tab=pages')
  })
})
