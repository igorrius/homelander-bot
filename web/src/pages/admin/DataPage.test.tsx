import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import type { DataItem } from '../../api/types'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, dataItems } from '../../test/fixtures'
import { currentLocation, renderApp } from '../../test/render'

const dues: DataItem = { ...dataItems[1], key: 'dues.paid', visibility: 'residents' }
const routes = { ...adminRoutes, 'GET /admin/data': { body: [...dataItems, dues] } }

describe('DataPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('lists values and tables with kind, visibility and row count', async () => {
    mockApi(routes)
    renderApp('/admin/data')
    const row = (await screen.findByText('contacts')).closest('tr') as HTMLElement
    expect(within(row).getByText(uk.admin.data.kind.table)).toBeInTheDocument()
    expect(within(row).getByText(uk.admin.data.visibility.public)).toBeInTheDocument()
    expect(within(row).getByText('2')).toBeInTheDocument()
    const addr = screen.getByText('house.address').closest('tr') as HTMLElement
    expect(within(addr).getByText(uk.admin.data.visibility.admins)).toBeInTheDocument()
    const paid = screen.getByText('dues.paid').closest('tr') as HTMLElement
    expect(within(paid).getByText(uk.admin.data.visibility.residents)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: uk.admin.nav.data })).toBeInTheDocument()
  })

  it('filters the list by visibility', async () => {
    mockApi(routes)
    const { user } = renderApp('/admin/data')
    await screen.findByText('contacts')
    const filter = screen.getByRole('radiogroup', { name: uk.admin.data.filterLabel })
    await user.click(within(filter).getByText(uk.admin.data.visibility.residents))
    expect(screen.getByText('dues.paid')).toBeInTheDocument()
    expect(screen.queryByText('contacts')).not.toBeInTheDocument()
    expect(screen.queryByText('house.address')).not.toBeInTheDocument()
    await user.click(within(filter).getByText(uk.admin.data.filterAll))
    expect(screen.getByText('contacts')).toBeInTheDocument()
  })

  it('creates a value and opens it', async () => {
    const created = { ...dataItems[1], key: 'house.phone', value: null }
    const { calls } = mockApi({ ...routes, 'POST /admin/data': { status: 201, body: created }, 'GET /admin/data/house.phone': { body: { item: created, rows: [] } } })
    const { user } = renderApp('/admin/data')
    await user.click(await screen.findByRole('button', { name: uk.admin.data.add }))
    await user.type(await screen.findByLabelText(uk.admin.data.key), 'house.phone')
    await user.click(screen.getByRole('button', { name: uk.admin.data.create }))
    await waitFor(() =>
      expect(callsTo(calls, 'POST /admin/data')[0]?.body).toEqual({ key: 'house.phone', kind: 'value', valueType: 'text', description: '', visibility: 'admins' }),
    )
    await waitFor(() => expect(currentLocation()).toBe('/admin/data/house.phone'))
  })

  it('creates a value residents see', async () => {
    const { calls } = mockApi({ ...routes, 'POST /admin/data': { status: 201, body: dues } })
    const { user } = renderApp('/admin/data')
    await user.click(await screen.findByRole('button', { name: uk.admin.data.add }))
    await user.type(await screen.findByLabelText(uk.admin.data.key), 'dues.paid')
    const dialog = screen.getByRole('dialog')
    await user.click(within(within(dialog).getByRole('radiogroup', { name: uk.admin.data.access })).getByText(uk.admin.data.visibility.residents))
    await user.click(screen.getByRole('button', { name: uk.admin.data.create }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/data')[0]?.body).toMatchObject({ key: 'dues.paid', visibility: 'residents' }))
  })

  it('creates a table with a first text column', async () => {
    const { calls } = mockApi({ ...routes, 'POST /admin/data': { status: 201, body: dataItems[0] } })
    const { user } = renderApp('/admin/data')
    await user.click(await screen.findByRole('button', { name: uk.admin.data.add }))
    await user.type(await screen.findByLabelText(uk.admin.data.key), 'contacts')
    await user.click(screen.getByRole('radio', { name: uk.admin.data.kind.table }))
    await user.click(screen.getByRole('button', { name: uk.admin.data.create }))
    await waitFor(() =>
      expect(callsTo(calls, 'POST /admin/data')[0]?.body).toEqual({
        key: 'contacts', kind: 'table', columns: [{ name: 'name', label: uk.admin.data.firstColumn, type: 'text' }], description: '', visibility: 'admins',
      }),
    )
  })

  it('shows an invalid key under the field', async () => {
    mockApi({ ...routes, 'POST /admin/data': { status: 422, body: { error: { code: 'validation', fields: { key: 'invalid' } } } } })
    const { user } = renderApp('/admin/data')
    await user.click(await screen.findByRole('button', { name: uk.admin.data.add }))
    await user.type(await screen.findByLabelText(uk.admin.data.key), 'Bad Key')
    await user.click(screen.getByRole('button', { name: uk.admin.data.create }))
    expect(await screen.findByText(uk.validation.invalid)).toBeInTheDocument()
  })

  it('refuses a key under a module prefix', async () => {
    mockApi({ ...routes, 'POST /admin/data': { status: 422, body: { error: { code: 'validation', fields: { key: 'key_reserved' } } } } })
    const { user } = renderApp('/admin/data')
    await user.click(await screen.findByRole('button', { name: uk.admin.data.add }))
    await user.type(await screen.findByLabelText(uk.admin.data.key), 'feed.news')
    await user.click(screen.getByRole('button', { name: uk.admin.data.create }))
    expect(await screen.findByText(uk.validation.key_reserved)).toBeInTheDocument()
  })

  it('marks items a module manages, and those of a module that is off', async () => {
    const posts: DataItem = { ...dataItems[0], key: 'feed.posts', description: 'Feed posts', visibility: 'residents', owner: 'feed', ownerEnabled: true }
    const old: DataItem = { ...dataItems[0], key: 'feed.archive', description: '', visibility: 'residents', owner: 'feed', ownerEnabled: false }
    mockApi({ ...routes, 'GET /admin/data': { body: [...dataItems, posts, old] } })
    renderApp('/admin/data')
    const on = (await screen.findByText('feed.posts')).closest('tr') as HTMLElement
    expect(await within(on).findByText('Модуль: Стрічка новин')).toBeInTheDocument()
    expect(within(on).queryByText(uk.admin.data.ownerOff)).not.toBeInTheDocument()
    const off = screen.getByText('feed.archive').closest('tr') as HTMLElement
    expect(within(off).getByText('Модуль: Стрічка новин')).toBeInTheDocument()
    expect(within(off).getByText(uk.admin.data.ownerOff)).toBeInTheDocument()
    const own = screen.getByText('contacts').closest('tr') as HTMLElement
    expect(within(own).queryByText(/Модуль/)).not.toBeInTheDocument()
  })
})
