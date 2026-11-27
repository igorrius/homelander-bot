import { screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { i18n } from '../i18n'
import uk from '../locales/uk.json'
import { redirect } from '../lib/redirect'
import { callsTo, mockApi } from '../test/api'
import { admin, publicRoutes, resident } from '../test/fixtures'
import { renderApp } from '../test/render'

const cb = 'https://claude.ai/api/mcp/auth_callback'
const path =
  '/oauth/authorize?response_type=code&client_id=c1&redirect_uri=' + encodeURIComponent(cb) +
  '&code_challenge=ch&code_challenge_method=S256&state=st&resource=' + encodeURIComponent('https://example.com/mcp')

function info(over: Record<string, unknown> = {}) {
  return { body: { clientName: 'Claude', redirectHost: 'claude.ai', signedIn: true, allowed: true, ...over } }
}

describe('OAuthAuthorizePage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('asks a guest to log in and come back', async () => {
    mockApi({ ...publicRoutes, 'GET /oauth/authorize-info': info({ signedIn: false, allowed: false }) })
    renderApp(path)
    const link = await screen.findByRole('link', { name: uk.oauth.login })
    expect(link).toHaveAttribute('href', '/login?next=' + encodeURIComponent(path))
    expect(screen.queryByRole('button', { name: uk.oauth.allow })).toBeNull()
  })

  it('lets an admin with MCP allow the client', async () => {
    const assign = vi.spyOn(redirect, 'to').mockImplementation(() => {})
    const { calls } = mockApi({
      ...publicRoutes,
      'GET /me': { body: admin },
      'GET /oauth/authorize-info': info(),
      'POST /oauth/authorize': { body: { redirect: cb + '?code=abc&state=st' } },
    })
    const { user } = renderApp(path)
    expect(await screen.findByText(i18n.t('oauth.question', { client: 'Claude' }))).toBeInTheDocument()
    expect(screen.getByText(i18n.t('oauth.redirectHost', { host: 'claude.ai' }))).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: uk.oauth.allow }))
    await waitFor(() => expect(assign).toHaveBeenCalledWith(cb + '?code=abc&state=st'))
    expect(callsTo(calls, 'POST /oauth/authorize').map((c) => c.body)).toEqual([
      { clientId: 'c1', redirectUri: cb, codeChallenge: 'ch', codeChallengeMethod: 'S256', state: 'st', resource: 'https://example.com/mcp', approve: true },
    ])
    expect(callsTo(calls, 'GET /oauth/authorize-info')[0].query.get('client_id')).toBe('c1')
  })

  it('cancels with a denial sent back to the client', async () => {
    const assign = vi.spyOn(redirect, 'to').mockImplementation(() => {})
    const { calls } = mockApi({
      ...publicRoutes,
      'GET /me': { body: admin },
      'GET /oauth/authorize-info': info(),
      'POST /oauth/authorize': { body: { redirect: cb + '?error=access_denied&state=st' } },
    })
    const { user } = renderApp(path)
    await user.click(await screen.findByRole('button', { name: uk.common.cancel }))
    await waitFor(() => expect(assign).toHaveBeenCalledWith(cb + '?error=access_denied&state=st'))
    expect(callsTo(calls, 'POST /oauth/authorize')[0].body).toMatchObject({ approve: false })
  })

  it('tells an admin without MCP why they cannot allow', async () => {
    mockApi({ ...publicRoutes, 'GET /me': { body: admin }, 'GET /oauth/authorize-info': info({ allowed: false, reason: 'mcp_off' }) })
    renderApp(path)
    expect(await screen.findByText(uk.oauth.reason.mcp_off)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: uk.oauth.allow })).toBeNull()
    expect(screen.getByRole('button', { name: uk.common.cancel })).toBeInTheDocument()
  })

  it('tells a resident they are not an admin', async () => {
    mockApi({ ...publicRoutes, 'GET /me': { body: resident }, 'GET /oauth/authorize-info': info({ allowed: false, reason: 'not_admin' }) })
    renderApp(path)
    expect(await screen.findByText(uk.oauth.reason.not_admin)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: uk.oauth.allow })).toBeNull()
  })

  it('refuses a request the client did not register', async () => {
    mockApi({ ...publicRoutes, 'GET /oauth/authorize-info': { status: 400, body: { error: { code: 'invalid_request' } } } })
    renderApp(path)
    expect(await screen.findByText(uk.oauth.invalid)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: uk.oauth.allow })).toBeNull()
  })
})
