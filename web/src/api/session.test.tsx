import { screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../i18n'
import uk from '../locales/uk.json'
import { mockApi } from '../test/api'
import { myHome, publicRoutes, resident, residentRoutes } from '../test/fixtures'
import { currentLocation, renderApp } from '../test/render'
import type { MyHome } from './types'

// What user A's cabinet had loaded in this tab before their session ended.
const homeOfA: MyHome = { telegramLinked: false, homes: [{ ...myHome.homes[0], id: 99, houseName: 'House of A' }] }

describe('switching accounts in one tab', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it("never shows user B what user A's cabinet loaded", async () => {
    let signedIn = false
    mockApi({
      ...residentRoutes,
      'GET /me': () => (signedIn ? { body: resident } : publicRoutes['GET /me']),
      'GET /me/home': { body: myHome, delay: 100 },
      'POST /auth/code': { status: 202, body: { status: 'sent_if_allowed' } },
      'POST /auth/verify': () => {
        signedIn = true
        return { body: { user: resident } }
      },
    })
    const { user, client } = renderApp('/login')
    client.setQueryData(['cabinet', 'home'], homeOfA)
    client.setQueryData(['admin', 'summary'], { pendingApplications: 3 })
    await user.type(await screen.findByLabelText(uk.login.phone), '+380500000002')
    await user.click(screen.getByRole('button', { name: uk.login.sendCode }))
    const inputs = await screen.findAllByLabelText(uk.login.code)
    await user.click(inputs[0])
    await user.keyboard('123456')
    await user.click(screen.getByRole('button', { name: uk.login.submit }))
    await waitFor(() => expect(currentLocation()).toBe('/my'))
    expect(screen.queryByText('House of A')).not.toBeInTheDocument()
    expect(client.getQueryData(['admin', 'summary'])).toBeUndefined()
    expect(await screen.findByText('House A')).toBeInTheDocument()
    expect(screen.queryByText('House of A')).not.toBeInTheDocument()
  })

  it('drops the cached cabinet when the session expires', async () => {
    let expired = false
    mockApi({
      ...residentRoutes,
      'GET /me': () => (expired ? publicRoutes['GET /me'] : { body: resident }),
      'GET /me/home': () => (expired ? { status: 401, body: { error: { code: 'unauthorized' } } } : { body: myHome }),
    })
    const { client } = renderApp('/my')
    expect(await screen.findByText('House A')).toBeInTheDocument()
    expired = true
    await client.refetchQueries({ queryKey: ['cabinet', 'home'] })
    await waitFor(() => expect(currentLocation()).toBe('/login?next=%2Fmy'))
    expect(client.getQueryData(['cabinet', 'home'])).toBeUndefined()
  })
})
