import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import type { PageVersion } from '../../api/types'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes } from '../../test/fixtures'
import { currentLocation, renderApp } from '../../test/render'

const file = (path: string, size: number, contentType = 'text/html; charset=utf-8') => ({ path, size, contentType })
const author = { id: 1, name: 'Admin A' }

// Synthetic data only.
const versions: PageVersion[] = [
  { id: 3, status: 'draft', note: 'New look', files: [file('index.html', 1200), file('app.js', 300, 'text/javascript; charset=utf-8')],
    totalSize: 1500, createdAt: '2026-11-27T12:00:00Z', createdBy: { id: 2, name: 'Admin B' }, publishedAt: null, publishedBy: null },
  { id: 2, status: 'published', note: '', files: [file('index.html', 800)], totalSize: 800, createdAt: '2026-11-26T12:00:00Z',
    createdBy: author, publishedAt: '2026-11-26T13:00:00Z', publishedBy: author },
  { id: 1, status: 'archived', note: 'First', files: [file('index.html', 500)], totalSize: 500, createdAt: '2026-11-25T12:00:00Z',
    createdBy: author, publishedAt: '2026-11-25T13:00:00Z', publishedBy: author },
]

const routes = {
  ...adminRoutes,
  'GET /admin/pages': { body: versions },
  'GET /admin/pages/3': { body: versions[0] },
  'GET /admin/pages/2': { body: versions[1] },
  'POST /admin/pages/3/preview': { body: { url: '/site/preview/tok3/index.html' } },
  'POST /admin/pages/2/preview': { body: { url: '/site/preview/tok2/index.html' } },
}
const fileInput = (container: HTMLElement) => container.querySelector('input[type="file"]') as HTMLInputElement
const zip = () => new File(['PK'], 'page.zip', { type: 'application/zip' })

describe('PagesPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('lists versions with status and author', async () => {
    mockApi(routes)
    renderApp('/admin/pages')
    const draft = (await screen.findByText('New look')).closest('tr') as HTMLElement
    expect(within(draft).getByText(uk.admin.pages.status.draft)).toBeInTheDocument()
    expect(within(draft).getByText('Admin B')).toBeInTheDocument()
    const first = screen.getByText('First').closest('tr') as HTMLElement
    expect(within(first).getByText(uk.admin.pages.status.archived)).toBeInTheDocument()
    expect(screen.getAllByText(uk.admin.pages.status.published).length).toBeGreaterThan(0)
    expect(screen.getByRole('link', { name: uk.admin.nav.pages })).toBeInTheDocument()
  })

  it('uploads a zip and opens the new draft', async () => {
    const { calls } = mockApi({ ...routes, 'POST /admin/pages': { status: 201, body: versions[0] } })
    const { user, container } = renderApp('/admin/pages')
    await screen.findByText('New look')
    await user.upload(fileInput(container), zip())
    await waitFor(() => expect(currentLocation()).toBe('/admin/pages/3'))
    const body = callsTo(calls, 'POST /admin/pages')[0].body as FormData
    expect(body).toBeInstanceOf(FormData)
    expect((body.get('zip') as File).name).toBe('page.zip')
  })

  it('shows why an upload was refused', async () => {
    mockApi({ ...routes, 'POST /admin/pages': { status: 422, body: { error: { code: 'validation', fields: { files: 'index_required' } } } } })
    const { user, container } = renderApp('/admin/pages')
    await screen.findByText('New look')
    await user.upload(fileInput(container), zip())
    expect(await screen.findByText(uk.validation.index_required)).toBeInTheDocument()
  })

  it('goes back to the built-in page', async () => {
    const { calls } = mockApi({ ...routes, 'POST /admin/pages/unpublish': { status: 204 } })
    const { user } = renderApp('/admin/pages')
    await user.click(await screen.findByRole('button', { name: uk.admin.pages.unpublish }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/pages/unpublish')).toHaveLength(1))
  })

  it('previews and publishes a draft', async () => {
    const { calls } = mockApi({ ...routes, 'POST /admin/pages/3/publish': { status: 204 } })
    const { user } = renderApp('/admin/pages/3')
    const frame = await screen.findByTitle(uk.admin.pages.previewTitle)
    await waitFor(() => expect(frame.getAttribute('src')).toBe('/site/preview/tok3/index.html?lang=uk'))
    expect(frame).toHaveAttribute('sandbox', 'allow-scripts allow-popups allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation')
    expect(screen.getByText('app.js')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: uk.admin.pages.delete })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: uk.admin.pages.publish }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/pages/3/publish')).toHaveLength(1))
  })

  it('offers no delete or publish for the published version', async () => {
    mockApi(routes)
    renderApp('/admin/pages/2')
    await screen.findByTitle(uk.admin.pages.previewTitle)
    expect(screen.getByRole('button', { name: uk.admin.pages.copy })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: uk.admin.pages.delete })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: uk.admin.pages.publish })).not.toBeInTheDocument()
  })

  it('copies a version to a new draft', async () => {
    const { calls } = mockApi({ ...routes, 'POST /admin/pages/2/copy': { status: 201, body: versions[0] } })
    const { user } = renderApp('/admin/pages/2')
    await user.click(await screen.findByRole('button', { name: uk.admin.pages.copy }))
    await waitFor(() => expect(currentLocation()).toBe('/admin/pages/3'))
    expect(callsTo(calls, 'POST /admin/pages/2/copy')).toHaveLength(1)
  })
})
