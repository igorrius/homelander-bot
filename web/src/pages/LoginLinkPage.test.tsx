import { QueryClient } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { StrictMode } from 'react'
import { MemoryRouter, useLocation } from 'react-router'
import { beforeEach, describe, expect, it } from 'vitest'
import App from '../App'
import { Providers } from '../Providers'
import { i18n } from '../i18n'
import uk from '../locales/uk.json'
import { callsTo, mockApi } from '../test/api'
import { publicRoutes, resident, residentRoutes } from '../test/fixtures'
import { currentLocation, renderApp } from '../test/render'

function Probe() {
  const { pathname, search } = useLocation()
  return <div data-testid="strict-location">{pathname + search}</div>
}

/** Like renderApp, but in StrictMode, where effects run twice in development. */
function renderStrict(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  render(
    <StrictMode>
      <Providers client={client} env="test">
        <MemoryRouter initialEntries={[path]}>
          <App />
          <Probe />
        </MemoryRouter>
      </Providers>
    </StrictMode>,
  )
}

describe('LoginLinkPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('spends the link once and opens the cabinet page it names', async () => {
    let signedIn = false
    const { calls } = mockApi({
      ...residentRoutes,
      'GET /me': () => (signedIn ? { body: resident } : publicRoutes['GET /me']),
      'POST /auth/link': () => {
        signedIn = true
        return { body: { user: resident, next: '/my?from=bot' }, delay: 20 }
      },
    })
    renderStrict('/login/link?t=abc')
    await waitFor(() => expect(screen.getByTestId('strict-location')).toHaveTextContent('/my?from=bot'))
    expect(await screen.findByRole('heading', { name: uk.cabinet.myHome.title })).toBeInTheDocument()
    expect(callsTo(calls, 'POST /auth/link').map((c) => c.body)).toEqual([{ token: 'abc' }])
  })

  it('sends an expired or used link to the login page with a hint', async () => {
    mockApi({ ...publicRoutes, 'POST /auth/link': { status: 422, body: { error: { code: 'token_invalid' } } } })
    renderApp('/login/link?t=old')
    await waitFor(() => expect(currentLocation()).toBe('/login?link=expired'))
    expect(await screen.findByText(uk.login.linkExpired)).toBeInTheDocument()
    expect(screen.getByText(uk.login.linkHint)).toBeInTheDocument()
    expect(screen.getByLabelText(uk.login.phone)).toBeInTheDocument()
  })

  it('treats a link without a token as expired, without calling the API', async () => {
    const { calls } = mockApi(publicRoutes)
    renderApp('/login/link')
    await waitFor(() => expect(currentLocation()).toBe('/login?link=expired'))
    expect(callsTo(calls, 'POST /auth/link')).toHaveLength(0)
  })
})
