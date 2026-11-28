import { screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../i18n'
import uk from '../locales/uk.json'
import { callsTo, mockApi } from '../test/api'
import { admin, publicRoutes, resident, site } from '../test/fixtures'
import { renderApp } from '../test/render'

const longName = 'Об’єднання співвласників багатоквартирного будинку «Тестовий квартал на лівому березі»'

describe('SiteHeader', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('keeps a long community name on one line and the account in a menu', async () => {
    expect(longName.length).toBeGreaterThanOrEqual(80)
    const three = [1, 2, 3].map((n) => ({ ...resident.residencies[0], id: 20 + n, apartment: 10 + n }))
    mockApi({
      ...publicRoutes,
      'GET /site': { body: { ...site, communityName: { uk: longName } } },
      'GET /me': { body: { ...admin, residencies: three } },
    })
    const { user } = renderApp('/')
    const name = await screen.findByTitle(longName)
    expect(name).toBe(screen.getByTestId('site-header-name'))
    expect(name).toHaveTextContent(longName)
    expect(name).toHaveAttribute('title', longName)
    expect(name).toHaveStyle({ textOverflow: 'ellipsis', whiteSpace: 'nowrap', overflow: 'hidden' })
    // Account details are hidden until the menu opens.
    expect(screen.queryByText('House A, кв. 11')).not.toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: uk.home.admin })).not.toBeInTheDocument()
    await user.click(await screen.findByRole('button', { name: 'Adam Admin' }))
    expect(await screen.findByText('House A, кв. 11')).toBeInTheDocument()
    expect(screen.getByText('House A, кв. 13')).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: uk.home.admin })).toHaveAttribute('href', '/admin')
    expect(screen.getByRole('menuitem', { name: uk.home.logout })).toBeInTheDocument()
    // "My cabinet" comes right above "Admin panel".
    const items = screen.getAllByRole('menuitem').map((el) => el.textContent)
    expect(items.indexOf(uk.home.cabinet)).toBe(items.indexOf(uk.home.admin) - 1)
    expect(screen.getByRole('menuitem', { name: uk.home.cabinet })).toHaveAttribute('href', '/my')
  })

  it('offers log in and register to a guest', async () => {
    mockApi(publicRoutes)
    renderApp('/')
    expect(await screen.findByRole('link', { name: uk.home.login })).toHaveAttribute('href', '/login')
    expect(screen.getByRole('link', { name: uk.home.register })).toHaveAttribute('href', '/register')
  })

  it('has no admin link for a resident and logs out from the menu', async () => {
    const { calls } = mockApi({ ...publicRoutes, 'GET /me': { body: resident }, 'POST /auth/logout': { status: 204 } })
    const { user } = renderApp('/')
    await user.click(await screen.findByRole('button', { name: 'Olena Test' }))
    expect(screen.getByText('House A, кв. 12')).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: uk.home.admin })).not.toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: uk.home.cabinet })).toHaveAttribute('href', '/my')
    await user.click(screen.getByRole('menuitem', { name: uk.home.logout }))
    expect(await screen.findByRole('link', { name: uk.home.login })).toBeInTheDocument()
    expect(callsTo(calls, 'POST /auth/logout')).toHaveLength(1)
  })
})
