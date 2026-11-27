import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, dataItems } from '../../test/fixtures'
import { currentLocation, renderApp } from '../../test/render'

const routes = { ...adminRoutes, 'GET /admin/data': { body: dataItems } }

describe('DataPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('lists values and tables with kind, public badge and row count', async () => {
    mockApi(routes)
    renderApp('/admin/data')
    const row = (await screen.findByText('contacts')).closest('tr') as HTMLElement
    expect(within(row).getByText(uk.admin.data.kind.table)).toBeInTheDocument()
    expect(within(row).getByText(uk.admin.data.public)).toBeInTheDocument()
    expect(within(row).getByText('2')).toBeInTheDocument()
    const addr = screen.getByText('house.address').closest('tr') as HTMLElement
    expect(within(addr).getByText(uk.admin.data.private)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: uk.admin.nav.data })).toBeInTheDocument()
  })

  it('creates a value and opens it', async () => {
    const created = { ...dataItems[1], key: 'house.phone', value: null }
    const { calls } = mockApi({ ...routes, 'POST /admin/data': { status: 201, body: created }, 'GET /admin/data/house.phone': { body: { item: created, rows: [] } } })
    const { user } = renderApp('/admin/data')
    await user.click(await screen.findByRole('button', { name: uk.admin.data.add }))
    await user.type(await screen.findByLabelText(uk.admin.data.key), 'house.phone')
    await user.click(screen.getByRole('button', { name: uk.admin.data.create }))
    await waitFor(() =>
      expect(callsTo(calls, 'POST /admin/data')[0]?.body).toEqual({ key: 'house.phone', kind: 'value', valueType: 'text', description: '', public: false }),
    )
    await waitFor(() => expect(currentLocation()).toBe('/admin/data/house.phone'))
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
        key: 'contacts', kind: 'table', columns: [{ name: 'name', label: uk.admin.data.firstColumn, type: 'text' }], description: '', public: false,
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
})
