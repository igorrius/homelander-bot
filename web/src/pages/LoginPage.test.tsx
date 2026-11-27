import { screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../i18n'
import uk from '../locales/uk.json'
import { callsTo, mockApi } from '../test/api'
import { admin, publicRoutes, resident } from '../test/fixtures'
import { currentLocation, renderApp } from '../test/render'

const codeOk = { status: 202, body: { status: 'sent_if_allowed' } }

async function enterPhone(user: ReturnType<typeof renderApp>['user']) {
  await user.type(await screen.findByLabelText(uk.login.phone), '+380500000001')
  await user.click(screen.getByRole('button', { name: uk.login.sendCode }))
}

async function enterCode(user: ReturnType<typeof renderApp>['user'], code: string) {
  const inputs = await screen.findAllByLabelText(uk.login.code)
  await user.click(inputs[0])
  await user.keyboard(code)
}

describe('LoginPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('asks for a code, then logs an admin in to /admin', async () => {
    const { calls } = mockApi({ ...publicRoutes, 'POST /auth/code': codeOk, 'POST /auth/verify': { body: { user: admin } } })
    const { user } = renderApp('/login')
    await enterPhone(user)
    expect(await screen.findByText(uk.login.codeSent)).toBeInTheDocument()
    expect(callsTo(calls, 'POST /auth/code').map((c) => c.body)).toEqual([{ phone: '+380500000001' }])
    await enterCode(user, '123456')
    await user.click(screen.getByRole('button', { name: uk.login.submit }))
    expect(await screen.findByRole('heading', { name: uk.admin.title })).toBeInTheDocument()
    expect(callsTo(calls, 'POST /auth/verify').map((c) => c.body)).toEqual([{ phone: '+380500000001', code: '123456' }])
  })

  it('sends a resident to /', async () => {
    mockApi({ ...publicRoutes, 'POST /auth/code': codeOk, 'POST /auth/verify': { body: { user: resident } } })
    const { user } = renderApp('/login')
    await enterPhone(user)
    await enterCode(user, '123456')
    await user.click(screen.getByRole('button', { name: uk.login.submit }))
    expect(await screen.findByRole('heading', { name: 'Тестова громада' })).toBeInTheDocument()
  })

  it('shows invalid code and clears the input', async () => {
    mockApi({
      ...publicRoutes,
      'POST /auth/code': codeOk,
      'POST /auth/verify': { status: 401, body: { error: { code: 'invalid_code' } } },
    })
    const { user } = renderApp('/login')
    await enterPhone(user)
    await enterCode(user, '123456')
    await user.click(screen.getByRole('button', { name: uk.login.submit }))
    expect(await screen.findByText(uk.errors.invalid_code)).toBeInTheDocument()
    for (const input of screen.getAllByLabelText(uk.login.code)) expect(input).toHaveValue('')
  })

  it('shows rate limiting', async () => {
    mockApi({ ...publicRoutes, 'POST /auth/code': { status: 429, body: { error: { code: 'rate_limited' } } } })
    const { user } = renderApp('/login')
    await enterPhone(user)
    expect(await screen.findByText(uk.errors.rate_limited)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: uk.login.sendCode })).toBeInTheDocument()
  })

  it('sends one code request on double click', async () => {
    const { calls } = mockApi({ ...publicRoutes, 'POST /auth/code': { ...codeOk, delay: 50 } })
    const { user } = renderApp('/login')
    await user.type(await screen.findByLabelText(uk.login.phone), '+380500000001')
    await user.dblClick(screen.getByRole('button', { name: uk.login.sendCode }))
    expect(await screen.findByText(uk.login.codeSent)).toBeInTheDocument()
    expect(callsTo(calls, 'POST /auth/code')).toHaveLength(1)
  })

  it('goes back to change the number', async () => {
    mockApi({ ...publicRoutes, 'POST /auth/code': codeOk })
    const { user } = renderApp('/login')
    await enterPhone(user)
    await user.click(await screen.findByRole('button', { name: uk.login.changePhone }))
    expect(screen.getByLabelText(uk.login.phone)).toHaveValue('+380500000001')
  })

  it('redirects a signed-in admin away from login', async () => {
    mockApi({ ...publicRoutes, 'GET /me': { body: admin } })
    renderApp('/login')
    await waitFor(() => expect(screen.getByRole('heading', { name: uk.admin.title })).toBeInTheDocument())
  })

  it('goes back to a same-site next address after login', async () => {
    mockApi({ ...publicRoutes, 'POST /auth/code': codeOk, 'POST /auth/verify': { body: { user: admin } }, 'GET /oauth/authorize-info': { body: { clientName: 'Claude', redirectHost: 'claude.ai', signedIn: true, allowed: true } } })
    const { user } = renderApp('/login?next=' + encodeURIComponent('/oauth/authorize?client_id=c1'))
    await enterPhone(user)
    await enterCode(user, '123456')
    await user.click(screen.getByRole('button', { name: uk.login.submit }))
    await waitFor(() => expect(currentLocation()).toBe('/oauth/authorize?client_id=c1'))
  })

  it('ignores a next address on another site', async () => {
    mockApi({ ...publicRoutes, 'POST /auth/code': codeOk, 'POST /auth/verify': { body: { user: admin } } })
    const { user } = renderApp('/login?next=' + encodeURIComponent('//evil.example/x'))
    await enterPhone(user)
    await enterCode(user, '123456')
    await user.click(screen.getByRole('button', { name: uk.login.submit }))
    await waitFor(() => expect(currentLocation()).toBe('/admin'))
  })
})
