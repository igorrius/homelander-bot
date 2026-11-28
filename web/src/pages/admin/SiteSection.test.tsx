import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import type { DataItem, PageVersion } from '../../api/types'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, site } from '../../test/fixtures'
import { currentLocation, renderApp } from '../../test/render'
import { choose } from '../../test/select'

const html = (path: string) => ({ path, size: 400, contentType: 'text/html; charset=utf-8' })
const author = { id: 1, name: 'Admin A' }
const home = { name: 'index', path: '/' }

// Synthetic data only.
const live: PageVersion = {
  id: 5, status: 'published', note: 'Two pages', files: [html('contacts.html'), html('index.html')], totalSize: 800,
  pages: [home, { name: 'contacts', path: '/contacts' }], createdAt: '2026-11-26T12:00:00Z', createdBy: author, createdVia: 'Claude',
  publishedAt: '2026-11-26T13:00:00Z', publishedBy: author,
}
const draft: PageVersion = {
  id: 8, status: 'draft', note: 'New look', files: [html('index.html')], totalSize: 400, pages: [home],
  createdAt: '2026-11-27T12:00:00Z', createdBy: { id: 2, name: 'Admin B' }, createdVia: 'Claude', publishedAt: null, publishedBy: null,
}
const old: PageVersion = {
  id: 2, status: 'archived', note: 'First', files: [html('index.html')], totalSize: 300, pages: [home],
  createdAt: '2026-11-20T12:00:00Z', createdBy: author, createdVia: '', publishedAt: '2026-11-20T13:00:00Z', publishedBy: author,
}

const routes = {
  ...adminRoutes,
  'GET /admin/pages': { body: [draft, live, old] },
  'GET /admin/site': { body: site },
}
const zip = () => new File(['PK'], 'site.zip', { type: 'application/zip' })

describe('SiteSection', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('has one Site item in the menu', async () => {
    mockApi(routes)
    renderApp('/admin/site')
    const nav = await screen.findByRole('link', { name: uk.admin.nav.site })
    expect(nav).toHaveAttribute('href', '/admin/site')
    expect(screen.queryByRole('link', { name: 'Сторінка сайту' })).toBeNull()
    expect(uk.admin.nav).not.toHaveProperty('pages')
  })

  it('sends the old pages address to the Pages tab', async () => {
    mockApi(routes)
    renderApp('/admin/pages')
    await waitFor(() => expect(currentLocation()).toBe('/admin/site?tab=pages'))
    expect(await screen.findByRole('tab', { name: uk.admin.site.tabs.pages })).toHaveAttribute('aria-selected', 'true')
  })

  it('shows what is live with its pages as links to the site', async () => {
    mockApi(routes)
    renderApp('/admin/site')
    const block = await screen.findByTestId('site-live')
    expect(within(block).getByText('Two pages')).toBeInTheDocument()
    expect(within(block).getByTitle('Claude')).toHaveTextContent(uk.admin.pages.viaMcp)
    expect(within(block).getByRole('link', { name: '/' })).toHaveAttribute('href', '/')
    expect(within(block).getByRole('link', { name: '/contacts' })).toHaveAttribute('href', '/contacts')
  })

  it('says when nothing is published', async () => {
    mockApi({ ...routes, 'GET /admin/pages': { body: [draft] } })
    renderApp('/admin/site')
    expect(await screen.findByText(uk.admin.site.liveEmpty)).toBeInTheDocument()
  })

  it('lists drafts with a way to preview and publish', async () => {
    mockApi(routes)
    renderApp('/admin/site?tab=pages')
    const drafts = await screen.findByTestId('site-drafts')
    expect(within(drafts).getByText('New look')).toBeInTheDocument()
    expect(within(drafts).getByRole('link', { name: uk.admin.site.previewPublish })).toHaveAttribute('href', '/admin/pages/8')
  })

  it('says when there are no drafts', async () => {
    mockApi({ ...routes, 'GET /admin/pages': { body: [live] } })
    renderApp('/admin/site')
    expect(await screen.findByText(uk.admin.site.draftsEmpty)).toBeInTheDocument()
  })

  it('restores an archived version after confirmation', async () => {
    const { calls } = mockApi({ ...routes, 'POST /admin/pages/2/publish': { status: 204 } })
    const { user } = renderApp('/admin/site')
    await user.click(await screen.findByRole('button', { name: new RegExp(uk.admin.site.archive.replace('{{count}}', '1').replace(/[()]/g, '.')) }))
    await user.click(await screen.findByRole('button', { name: uk.admin.site.restore }))
    expect(callsTo(calls, 'POST /admin/pages/2/publish')).toHaveLength(0)
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/pages/2/publish')).toHaveLength(1))
  })

  it('returns to the built-in page after confirmation', async () => {
    const { calls } = mockApi({ ...routes, 'POST /admin/pages/unpublish': { status: 204 } })
    const { user } = renderApp('/admin/site')
    await user.click(await screen.findByRole('button', { name: uk.admin.pages.unpublish }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/pages/unpublish')).toHaveLength(1))
  })

  it('uploads a zip and opens the new draft', async () => {
    const { calls } = mockApi({ ...routes, 'POST /admin/pages': { status: 201, body: draft } })
    const { user } = renderApp('/admin/site')
    await user.click(await screen.findByRole('button', { name: uk.admin.site.uploadZip }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText(uk.admin.pages.note), 'From a zip')
    await user.upload(dialog.querySelector('input[type="file"]') as HTMLInputElement, zip())
    await waitFor(() => expect(currentLocation()).toBe('/admin/pages/8'))
    const body = callsTo(calls, 'POST /admin/pages')[0].body as FormData
    expect((body.get('zip') as File).name).toBe('site.zip')
    expect(body.get('note')).toBe('From a zip')
  })

  it('names a refused page file', async () => {
    mockApi({ ...routes, 'POST /admin/pages': { status: 422, body: { error: { code: 'validation', fields: { files: 'reserved_page_name' }, details: { path: 'admin.html' } } } } })
    const { user } = renderApp('/admin/site')
    await user.click(await screen.findByRole('button', { name: uk.admin.site.uploadZip }))
    const dialog = await screen.findByRole('dialog')
    await user.upload(dialog.querySelector('input[type="file"]') as HTMLInputElement, zip())
    expect(await within(dialog).findByText(uk.admin.pages.fileError.reserved_page_name.replace('{{path}}', 'admin.html'))).toBeInTheDocument()
  })
})

describe('SiteSection built-in page tab', () => {
  const items: DataItem[] = [
    { key: 'contacts', kind: 'table', description: '', visibility: 'public', rowCount: 2, columns: [], updatedAt: '2026-11-27T10:00:00Z', updatedBy: null },
    { key: 'office.phone', kind: 'value', description: '', visibility: 'public', valueType: 'phone', value: '+380 44 000 00 00', rowCount: 0, updatedAt: '2026-11-27T10:00:00Z', updatedBy: null },
    { key: 'secret.code', kind: 'value', description: '', visibility: 'residents', valueType: 'text', value: 'x', rowCount: 0, updatedAt: '2026-11-27T10:00:00Z', updatedBy: null },
  ]
  const homeRoutes = { ...routes, 'GET /admin/data': { body: items }, 'GET /public/data': { body: {} } }

  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('saves texts and the cards in the chosen order, offering only public items', async () => {
    const { calls } = mockApi({ ...homeRoutes, 'PUT /admin/site': { body: site } })
    const { user } = renderApp('/admin/site?tab=home')
    expect(await screen.findByLabelText(`${uk.admin.site.communityName} (EN)`)).toHaveValue('Test Community')
    await user.click(await screen.findByRole('combobox', { name: uk.admin.site.addItem }))
    expect(screen.queryByRole('option', { name: 'secret.code' })).toBeNull()
    await user.click(await screen.findByRole('option', { name: 'contacts' }))
    await choose(user, uk.admin.site.addItem, 'office.phone')
    await user.type(screen.getByLabelText(`${uk.admin.site.cardTitle} (УК)`, { selector: '[data-key="office.phone"] input' }), 'Телефон')
    await user.click(screen.getAllByRole('button', { name: uk.admin.site.moveUp })[1])
    await user.click(screen.getByRole('button', { name: uk.admin.site.save }))
    await waitFor(() => expect(callsTo(calls, 'PUT /admin/site')).toHaveLength(1))
    expect(callsTo(calls, 'PUT /admin/site')[0].body).toEqual({
      communityName: { uk: 'Тестова громада', en: 'Test Community' }, welcomeText: { uk: 'Привіт' }, defaultLocale: 'uk',
      homeItems: [{ key: 'office.phone', title: { uk: 'Телефон' } }, { key: 'contacts', title: {} }],
    })
  })

  it('removes a card', async () => {
    const { calls } = mockApi({ ...homeRoutes, 'GET /admin/site': { body: { ...site, homeItems: [{ key: 'contacts', title: {} }] } }, 'PUT /admin/site': { body: site } })
    const { user } = renderApp('/admin/site?tab=home')
    await user.click(await screen.findByRole('button', { name: uk.admin.site.removeItem }))
    await user.click(screen.getByRole('button', { name: uk.admin.site.save }))
    await waitFor(() => expect(callsTo(calls, 'PUT /admin/site')[0]?.body).toMatchObject({ homeItems: [] }))
  })

  it('hints when nothing is public', async () => {
    mockApi({ ...homeRoutes, 'GET /admin/data': { body: [items[2]] } })
    renderApp('/admin/site?tab=home')
    expect(await screen.findByText(uk.admin.site.noPublic)).toBeInTheDocument()
  })
})
