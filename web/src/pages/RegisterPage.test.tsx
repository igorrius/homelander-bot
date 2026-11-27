import { screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../i18n'
import uk from '../locales/uk.json'
import { callsTo, mockApi } from '../test/api'
import { publicRoutes } from '../test/fixtures'
import { renderApp } from '../test/render'
import { choose } from '../test/select'

type User = ReturnType<typeof renderApp>['user']

async function fillForm(user: User) {
  await user.type(await screen.findByLabelText(uk.register.phone), '+380500000002')
  await user.type(screen.getByLabelText(uk.register.firstName), 'Olena')
  await user.type(screen.getByLabelText(uk.register.lastName), 'Test')
  await choose(user, uk.register.house, 'House A')
  await choose(user, uk.register.apartment, '12')
  await choose(user, uk.register.status, 'Власник')
  await user.click(screen.getByRole('checkbox', { name: uk.register.consent }))
}

const submit = () => screen.getByRole('button', { name: uk.register.submit })

describe('RegisterPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('submits an application and shows the bot link with a QR code', async () => {
    const { calls } = mockApi({ ...publicRoutes, 'POST /register': { status: 202, body: { status: 'received' } } })
    const { user } = renderApp('/register')
    await fillForm(user)
    await user.click(submit())
    expect(await screen.findByText(uk.register.doneTitle)).toBeInTheDocument()
    expect(callsTo(calls, 'POST /register').map((c) => c.body)).toEqual([
      { phone: '+380500000002', firstName: 'Olena', lastName: 'Test', houseId: 1, apartment: 12, statusId: 1, consent: true, locale: 'uk' },
    ])
    expect(screen.getByRole('link', { name: uk.register.openBot })).toHaveAttribute('href', 'https://t.me/test_bot')
    expect(screen.getByTitle(uk.qr.title)).toBeInTheDocument()
  })

  it("offers only the chosen house's apartments", async () => {
    mockApi(publicRoutes)
    const { user } = renderApp('/register')
    await choose(user, uk.register.house, 'House B')
    await user.click(screen.getByRole('combobox', { name: uk.register.apartment }))
    expect(await screen.findByRole('option', { name: '41' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: '80' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: '40' })).not.toBeInTheDocument()
    expect(screen.queryByRole('option', { name: '81' })).not.toBeInTheDocument()
  })

  it('clears the apartment when the house changes', async () => {
    mockApi(publicRoutes)
    const { user } = renderApp('/register')
    await fillForm(user)
    expect(submit()).toBeEnabled()
    await choose(user, uk.register.house, 'House B')
    expect(screen.getByRole('combobox', { name: uk.register.apartment })).toHaveValue('')
    expect(submit()).toBeDisabled()
  })

  it('keeps submit disabled until consent is given', async () => {
    mockApi(publicRoutes)
    const { user } = renderApp('/register')
    await fillForm(user)
    await user.click(screen.getByRole('checkbox', { name: uk.register.consent }))
    expect(submit()).toBeDisabled()
  })

  it('shows field errors from the server', async () => {
    mockApi({
      ...publicRoutes,
      'POST /register': { status: 422, body: { error: { code: 'validation', fields: { phone: 'invalid', apartment: 'out_of_range' } } } },
    })
    const { user } = renderApp('/register')
    await fillForm(user)
    await user.click(submit())
    expect(await screen.findByText(uk.validation.invalid)).toBeInTheDocument()
    expect(screen.getByText(uk.validation.out_of_range)).toBeInTheDocument()
    expect(screen.getByLabelText(uk.register.phone)).toHaveAccessibleDescription(uk.validation.invalid)
    expect(screen.getByLabelText(uk.register.firstName)).toHaveValue('Olena')
  })

  it('says try later when the queue is full', async () => {
    mockApi({ ...publicRoutes, 'POST /register': { status: 503, body: { error: { code: 'try_later' } } } })
    const { user } = renderApp('/register')
    await fillForm(user)
    await user.click(submit())
    expect(await screen.findByText(uk.errors.try_later)).toBeInTheDocument()
  })

  it('says registration is unavailable without houses', async () => {
    mockApi({ ...publicRoutes, 'GET /register/options': { body: { houses: [], statuses: [] } } })
    renderApp('/register')
    expect(await screen.findByText(uk.register.unavailable)).toBeInTheDocument()
  })
})
