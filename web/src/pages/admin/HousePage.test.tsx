import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, adminUser, houses, resident, statuses } from '../../test/fixtures'
import { renderApp } from '../../test/render'

const stats = {
  apartmentsTotal: 40,
  apartmentsWithResidents: 2,
  people: 3,
  byResidencyStatus: { '1': 2, '2': 1 },
  byUserStatus: { active: 2, pending_approval: 1 },
  apartments: { '12': 2, '30': 1 },
}
const routes = {
  ...adminRoutes,
  'GET /admin/statuses': { body: statuses },
  'GET /admin/houses/1': { body: { house: houses[0], stats } },
  'GET /admin/houses/1/apartments/12': { body: [{ user: adminUser(), residency: resident.residencies[0] }] },
}
const cell = (n: number, count: number) => i18n.t('admin.house.apartmentCell', { n, count })

describe('HousePage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('has breadcrumbs and no text arrows', async () => {
    mockApi(routes)
    renderApp('/admin/houses/1')
    await screen.findByRole('heading', { name: 'House A' })
    const main = within(screen.getByRole('main'))
    expect(main.getByRole('link', { name: uk.admin.nav.houses })).toHaveAttribute('href', '/admin/houses')
    expect(document.body.textContent).not.toContain('←')
  })

  it('shows statistics', async () => {
    mockApi(routes)
    renderApp('/admin/houses/1')
    expect(await screen.findByRole('heading', { name: 'House A' })).toBeInTheDocument()
    expect(screen.getByText('2 / 40')).toBeInTheDocument()
    const people = screen.getByText(uk.admin.house.people).parentElement!
    expect(within(people).getByText('3')).toBeInTheDocument()
    expect(await screen.findByText('Власник: 2')).toBeInTheDocument()
    expect(screen.getByText('Орендар: 1')).toBeInTheDocument()
    expect(screen.getByText(`${uk.userStatus.pending_approval}: 1`)).toBeInTheDocument()
  })

  it('renders one cell per apartment', async () => {
    mockApi(routes)
    renderApp('/admin/houses/1')
    const grid = await screen.findByRole('group', { name: uk.admin.house.grid })
    expect(within(grid).getAllByRole('button')).toHaveLength(40)
    expect(within(grid).getByRole('button', { name: cell(12, 2) })).toBeEnabled()
    expect(within(grid).getByRole('button', { name: cell(13, 0) })).toBeDisabled()
  })

  it('opens the residents of an apartment', async () => {
    const { calls } = mockApi(routes)
    const { user } = renderApp('/admin/houses/1')
    await user.click(await screen.findByRole('button', { name: cell(12, 2) }))
    const drawer = await screen.findByRole('dialog')
    expect(await within(drawer).findByRole('link', { name: 'Olena Test' })).toHaveAttribute('href', '/admin/users/5')
    expect(callsTo(calls, 'GET /admin/houses/1/apartments/12')).toHaveLength(1)
  })

  it('saves settings', async () => {
    const { calls } = mockApi({ ...routes, 'PATCH /admin/houses/1': { body: { ...houses[0], floors: 9 } } })
    const { user } = renderApp('/admin/houses/1')
    await user.type(await screen.findByLabelText(uk.admin.house.floors), '9')
    await user.click(screen.getByRole('button', { name: uk.common.save }))
    await waitFor(() =>
      expect(callsTo(calls, 'PATCH /admin/houses/1').map((c) => c.body)).toEqual([
        { name: 'House A', address: '', latitude: null, longitude: null, floors: 9, apartmentFrom: 1, apartmentTo: 40, sortOrder: 0, archived: false },
      ]),
    )
  })

  it('shows has_residents when the range would drop residents', async () => {
    mockApi({ ...routes, 'PATCH /admin/houses/1': { status: 422, body: { error: { code: 'validation', fields: { apartmentTo: 'has_residents' } } } } })
    const { user } = renderApp('/admin/houses/1')
    const to = await screen.findByLabelText(uk.admin.house.apartmentTo)
    await user.clear(to)
    await user.type(to, '20')
    await user.click(screen.getByRole('button', { name: uk.common.save }))
    expect(await screen.findByText(uk.validation.has_residents)).toBeInTheDocument()
  })
})
