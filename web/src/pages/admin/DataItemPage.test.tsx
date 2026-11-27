import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import type { DataItem } from '../../api/types'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, contactRows, dataItems } from '../../test/fixtures'
import { currentLocation, renderApp } from '../../test/render'

const [contacts, address] = dataItems
const fileInput = (container: HTMLElement) => container.querySelector('input[type="file"]') as HTMLInputElement
const valueItem = (over: Partial<DataItem>): DataItem => ({ ...address, ...over })
const itemRoutes = (item: DataItem, rows = contactRows) => ({
  ...adminRoutes,
  'GET /admin/data': { body: dataItems },
  [`GET /admin/data/${item.key}`]: { body: { item, rows: item.kind === 'table' ? rows : [] } },
})
const rowOf = (text: string) => screen.getByDisplayValue(text).closest('tr') as HTMLElement

describe('DataItemPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('edits a text value and makes it public', async () => {
    const { calls } = mockApi({ ...itemRoutes(address), 'PATCH /admin/data/house.address': { body: address } })
    const { user } = renderApp('/admin/data/house.address')
    const field = await screen.findByLabelText(uk.admin.data.value)
    await user.clear(field)
    await user.type(field, 'Main st 2')
    await user.click(screen.getByRole('button', { name: uk.admin.data.saveValue }))
    await waitFor(() => expect(callsTo(calls, 'PATCH /admin/data/house.address')[0]?.body).toEqual({ valueType: 'text', value: 'Main st 2' }))
    await user.click(screen.getByLabelText(uk.admin.data.publicSwitch))
    await waitFor(() => expect(callsTo(calls, 'PATCH /admin/data/house.address')[1]?.body).toEqual({ public: true }))
  })

  it('edits a phone value in a text input', async () => {
    const phone = valueItem({ key: 'office.phone', valueType: 'phone', value: '+380 44 000 00 00' })
    const { calls } = mockApi({ ...itemRoutes(phone), 'PATCH /admin/data/office.phone': { body: phone } })
    const { user } = renderApp('/admin/data/office.phone')
    const field = await screen.findByLabelText(uk.admin.data.value)
    expect(field.tagName).toBe('INPUT')
    expect(field).toHaveValue('+380 44 000 00 00')
    await user.clear(field)
    await user.type(field, '+380 44 000 00 01')
    await user.click(screen.getByRole('button', { name: uk.admin.data.saveValue }))
    await waitFor(() => expect(callsTo(calls, 'PATCH /admin/data/office.phone')[0]?.body).toEqual({ valueType: 'phone', value: '+380 44 000 00 01' }))
  })

  it('saves an empty url value as null', async () => {
    const link = valueItem({ key: 'site.link', valueType: 'url', value: 'https://x.test' })
    const { calls } = mockApi({ ...itemRoutes(link), 'PATCH /admin/data/site.link': { body: link } })
    const { user } = renderApp('/admin/data/site.link')
    const field = await screen.findByLabelText(uk.admin.data.value)
    await user.clear(field)
    await user.click(screen.getByRole('button', { name: uk.admin.data.saveValue }))
    await waitFor(() => expect(callsTo(calls, 'PATCH /admin/data/site.link')[0]?.body).toEqual({ valueType: 'url', value: null }))
  })

  it('refuses invalid JSON without a request', async () => {
    const item = valueItem({ key: 'conf', valueType: 'json', value: { a: 1 } })
    const { calls } = mockApi(itemRoutes(item))
    const { user } = renderApp('/admin/data/conf')
    const field = await screen.findByLabelText(uk.admin.data.value)
    await user.clear(field)
    await user.click(field)
    await user.paste('{bad')
    await user.click(screen.getByRole('button', { name: uk.admin.data.saveValue }))
    expect(await screen.findByText(uk.admin.data.invalidJson)).toBeInTheDocument()
    expect(callsTo(calls, 'PATCH /admin/data/conf')).toHaveLength(0)
  })

  it('uploads an image value and shows it', async () => {
    let item = valueItem({ key: 'logo', valueType: 'image', value: null })
    const { calls } = mockApi({
      ...itemRoutes(item),
      'GET /admin/data/logo': () => ({ body: { item, rows: [] } }),
      'POST /admin/data/assets': { status: 201, body: { id: 5, url: '/api/v1/admin/assets/5' } },
      'PATCH /admin/data/logo': () => {
        item = { ...item, value: 5 }
        return { body: item }
      },
    })
    const { user, container } = renderApp('/admin/data/logo')
    await screen.findByText('logo')
    await user.upload(fileInput(container), new File(['x'], 'a.png', { type: 'image/png' }))
    await waitFor(() => expect(callsTo(calls, 'PATCH /admin/data/logo')[0]?.body).toEqual({ valueType: 'image', value: 5 }))
    expect(callsTo(calls, 'POST /admin/data/assets')).toHaveLength(1)
    await waitFor(() => expect(container.querySelector('img[src="/api/v1/admin/assets/5"]')).not.toBeNull())
  })

  it('adds a row and saves all rows in one request', async () => {
    const { calls } = mockApi({ ...itemRoutes(contacts), 'PUT /admin/data/contacts/rows': { body: { rows: contactRows } } })
    const { user } = renderApp('/admin/data/contacts')
    await screen.findByDisplayValue('Guard')
    await user.click(screen.getByRole('button', { name: uk.admin.data.addRow }))
    await user.type(screen.getByLabelText('Name 3'), 'Gate')
    await user.click(screen.getByRole('button', { name: uk.admin.data.saveRows }))
    await waitFor(() =>
      expect(callsTo(calls, 'PUT /admin/data/contacts/rows')[0]?.body).toEqual({
        rows: [
          { id: 7, position: 0, cells: { name: 'Office', phone: '+380 44 000 00 00' } },
          { id: 8, position: 1, cells: { name: 'Guard' } },
          { position: 2, cells: { name: 'Gate' } },
        ],
      }),
    )
  })

  it('moves a row up', async () => {
    const { calls } = mockApi({ ...itemRoutes(contacts), 'PUT /admin/data/contacts/rows': { body: { rows: contactRows } } })
    const { user } = renderApp('/admin/data/contacts')
    await screen.findByDisplayValue('Guard')
    await user.click(within(rowOf('Guard')).getByRole('button', { name: uk.admin.data.moveUp }))
    await user.click(screen.getByRole('button', { name: uk.admin.data.saveRows }))
    await waitFor(() => {
      const body = callsTo(calls, 'PUT /admin/data/contacts/rows')[0]?.body as { rows: { id: number; position: number }[] } | undefined
      const rows = body?.rows ?? []
      expect(rows.map((r) => [r.id, r.position])).toEqual([[8, 0], [7, 1]])
    })
  })

  it('deletes a saved row after confirmation', async () => {
    const { calls } = mockApi({ ...itemRoutes(contacts), 'DELETE /admin/data/contacts/rows/7': { status: 204 } })
    const { user } = renderApp('/admin/data/contacts')
    await screen.findByDisplayValue('Office')
    await user.click(within(rowOf('Office')).getByRole('button', { name: uk.common.delete }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'DELETE /admin/data/contacts/rows/7')).toHaveLength(1))
  })

  it('edits columns; existing column types are locked while there are rows', async () => {
    const { calls } = mockApi({ ...itemRoutes(contacts), 'PATCH /admin/data/contacts': { body: contacts } })
    const { user } = renderApp('/admin/data/contacts')
    await screen.findByDisplayValue('Guard')
    const types = screen.getAllByLabelText(uk.admin.data.columnType)
    expect(types[0]).toBeDisabled()
    await user.click(screen.getByRole('button', { name: uk.admin.data.removeColumn + ' phone' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(uk.admin.data.removeColumnConfirm.replace('{{name}}', 'phone'))).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: uk.common.confirm }))
    await user.click(screen.getByRole('button', { name: uk.admin.data.addColumn }))
    const names = screen.getAllByLabelText(uk.admin.data.columnName)
    await user.type(names[names.length - 1], 'email')
    await user.click(screen.getByRole('button', { name: uk.admin.data.saveColumns }))
    await waitFor(() =>
      expect(callsTo(calls, 'PATCH /admin/data/contacts')[0]?.body).toEqual({
        columns: [{ name: 'name', label: 'Name', type: 'text' }, { name: 'email', label: 'email', type: 'text' }],
      }),
    )
  })

  it('shows the public address and deletes the item', async () => {
    const { calls } = mockApi({ ...itemRoutes(contacts), 'DELETE /admin/data/contacts': { status: 204 } })
    const { user } = renderApp('/admin/data/contacts')
    expect(await screen.findByText('/api/v1/public/data/contacts')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: uk.admin.data.deleteItem }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'DELETE /admin/data/contacts')).toHaveLength(1))
    await waitFor(() => expect(currentLocation()).toBe('/admin/data'))
  })
})
