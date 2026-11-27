import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, houses } from '../../test/fixtures'
import { currentLocation, renderApp } from '../../test/render'

const archived = { ...houses[1], id: 3, name: 'House Old', archived: true }
const houseC = { id: 4, name: 'House C', address: '', latitude: null, longitude: null, floors: null, apartmentFrom: 81, apartmentTo: 120, sortOrder: 0, archived: false }
const routes = {
  ...adminRoutes,
  'GET /admin/statuses': { body: [] },
  'GET /admin/houses': (req: { query: URLSearchParams }) => ({ body: req.query.get('archived') === 'true' ? [...houses, archived] : houses }),
}

async function fillNew(user: ReturnType<typeof renderApp>['user']) {
  await user.click(await screen.findByRole('button', { name: uk.admin.houses.add }))
  const dialog = await screen.findByRole('dialog')
  await user.type(within(dialog).getByLabelText(uk.admin.house.name), 'House C')
  await user.type(within(dialog).getByLabelText(uk.admin.house.apartmentFrom), '81')
  await user.type(within(dialog).getByLabelText(uk.admin.house.apartmentTo), '120')
  await user.click(within(dialog).getByRole('button', { name: uk.common.save }))
  return dialog
}

describe('HousesPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('lists houses and shows archived on demand', async () => {
    const { calls } = mockApi(routes)
    const { user } = renderApp('/admin/houses')
    expect(await screen.findByRole('link', { name: 'House A' })).toHaveAttribute('href', '/admin/houses/1')
    expect(screen.getByText('41–80')).toBeInTheDocument()
    expect(screen.queryByText('House Old')).not.toBeInTheDocument()
    await user.click(screen.getByRole('switch', { name: uk.admin.houses.showArchived }))
    expect(await screen.findByText('House Old')).toBeInTheDocument()
    expect(screen.getByText(uk.admin.houses.archived)).toBeInTheDocument()
    expect(callsTo(calls, 'GET /admin/houses').some((c) => c.query.get('archived') === 'true')).toBe(true)
  })

  it('creates a house', async () => {
    const { calls } = mockApi({ ...routes, 'POST /admin/houses': { status: 201, body: houseC } })
    const { user } = renderApp('/admin/houses')
    await fillNew(user)
    await waitFor(() => expect(currentLocation()).toBe('/admin/houses/4'))
    expect(callsTo(calls, 'POST /admin/houses').map((c) => c.body)).toEqual([
      { name: 'House C', address: '', latitude: null, longitude: null, floors: null, apartmentFrom: 81, apartmentTo: 120, sortOrder: 0, archived: false },
    ])
  })

  it('shows range errors', async () => {
    mockApi({ ...routes, 'POST /admin/houses': { status: 422, body: { error: { code: 'validation', fields: { apartmentTo: 'out_of_range' } } } } })
    const { user } = renderApp('/admin/houses')
    const dialog = await fillNew(user)
    expect(await within(dialog).findByText(uk.validation.out_of_range)).toBeInTheDocument()
  })
})
