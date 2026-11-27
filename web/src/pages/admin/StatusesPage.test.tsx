import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, statuses } from '../../test/fixtures'
import { renderApp } from '../../test/render'

const routes = { ...adminRoutes, 'GET /admin/statuses': { body: statuses } }
const nameUk = `${uk.admin.statuses.name} (УК)`
const nameEn = `${uk.admin.statuses.name} (EN)`

describe('StatusesPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('lists statuses in the current language', async () => {
    mockApi(routes)
    renderApp('/admin/statuses')
    expect(await screen.findByText('Власник')).toBeInTheDocument()
    expect(screen.getByText('Орендар')).toBeInTheDocument()
    expect(screen.getByText('tenant')).toBeInTheDocument()
  })

  it('adds a status with names per locale', async () => {
    const created = { id: 3, code: 'relative', names: { uk: 'Родич', en: 'Relative' }, sortOrder: 3, archived: false }
    const { calls } = mockApi({ ...routes, 'POST /admin/statuses': { status: 201, body: created } })
    const { user } = renderApp('/admin/statuses')
    await user.click(await screen.findByRole('button', { name: uk.admin.statuses.add }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText(uk.admin.statuses.code), 'relative')
    await user.type(within(dialog).getByLabelText(nameUk), 'Родич')
    await user.type(within(dialog).getByLabelText(nameEn), 'Relative')
    const order = within(dialog).getByLabelText(uk.admin.statuses.sortOrder)
    await user.clear(order)
    await user.type(order, '3')
    await user.click(within(dialog).getByRole('button', { name: uk.common.save }))
    await waitFor(() =>
      expect(callsTo(calls, 'POST /admin/statuses').map((c) => c.body)).toEqual([
        { code: 'relative', names: { uk: 'Родич', en: 'Relative' }, sortOrder: 3, archived: false },
      ]),
    )
  })

  it('edits and archives', async () => {
    const { calls } = mockApi({ ...routes, 'PATCH /admin/statuses/2': { body: { ...statuses[1], archived: true } } })
    const { user } = renderApp('/admin/statuses')
    const row = (await screen.findByText('Орендар')).closest('tr')!
    await user.click(within(row).getByRole('button', { name: uk.common.edit }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByLabelText(uk.admin.statuses.code)).toBeDisabled()
    await user.click(within(dialog).getByRole('switch', { name: uk.admin.statuses.archived }))
    await user.click(within(dialog).getByRole('button', { name: uk.common.save }))
    await waitFor(() =>
      expect(callsTo(calls, 'PATCH /admin/statuses/2').map((c) => c.body)).toEqual([
        { code: 'tenant', names: { uk: 'Орендар', en: 'Tenant' }, sortOrder: 2, archived: true },
      ]),
    )
  })

  it('shows a taken code', async () => {
    mockApi({ ...routes, 'POST /admin/statuses': { status: 422, body: { error: { code: 'validation', fields: { code: 'taken' } } } } })
    const { user } = renderApp('/admin/statuses')
    await user.click(await screen.findByRole('button', { name: uk.admin.statuses.add }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText(uk.admin.statuses.code), 'owner')
    await user.type(within(dialog).getByLabelText(nameUk), 'Власник')
    await user.click(within(dialog).getByRole('button', { name: uk.common.save }))
    expect(await within(dialog).findByText(uk.validation.taken)).toBeInTheDocument()
  })
})
