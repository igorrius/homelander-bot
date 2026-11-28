import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n, setLocale } from '../../i18n'
import en from '../../locales/en.json'
import uk from '../../locales/uk.json'
import { mockApi } from '../../test/api'
import { admin, myHome, publicRoutes, residentRoutes } from '../../test/fixtures'
import { currentLocation, renderApp } from '../../test/render'

describe('Cabinet', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('sends a guest to log in and back to the same cabinet page', async () => {
    mockApi(publicRoutes)
    renderApp('/my/feed?page=2')
    await waitFor(() => expect(currentLocation()).toBe('/login?next=' + encodeURIComponent('/my/feed?page=2')))
  })

  it("shows a resident's home and profile", async () => {
    mockApi(residentRoutes)
    renderApp('/my')
    expect(await screen.findByRole('heading', { name: uk.cabinet.myHome.title })).toBeInTheDocument()
    const home = await screen.findByTestId('cabinet-home-11')
    expect(within(home).getByText('House A')).toBeInTheDocument()
    expect(within(home).getByText('1 Test Street')).toBeInTheDocument()
    expect(within(home).getByText('12')).toBeInTheDocument()
    expect(within(home).getByText('9')).toBeInTheDocument()
    expect(within(home).getByText('Власник')).toBeInTheDocument()
    const profile = screen.getByTestId('cabinet-profile')
    expect(within(profile).getByText('Olena Test')).toBeInTheDocument()
    expect(within(profile).getByText('+••••••••0002')).toBeInTheDocument()
    expect(within(profile).queryByText('+380500000002')).not.toBeInTheDocument()
    expect(within(profile).getByText(uk.cabinet.profile.linked)).toBeInTheDocument()
    expect(within(profile).getByText(uk.language.uk)).toBeInTheDocument()
    expect(within(profile).getByText(uk.cabinet.profile.hint)).toBeInTheDocument()
    // The community name is in the cabinet header.
    expect(screen.getByTestId('cabinet-header-name')).toHaveTextContent('Тестова громада')
  })

  it('explains an empty home to a user without apartments', async () => {
    mockApi({ ...publicRoutes, 'GET /me': { body: admin }, 'GET /me/home': { body: { homes: [], telegramLinked: false } } })
    renderApp('/my')
    expect(await screen.findByText(uk.cabinet.myHome.empty)).toBeInTheDocument()
    expect(screen.getByText(uk.cabinet.profile.notLinked)).toBeInTheDocument()
  })

  it('has navigation to my home and back to the site', async () => {
    mockApi(residentRoutes)
    const { user } = renderApp('/my')
    const nav = await screen.findByRole('navigation', { name: uk.cabinet.nav.label })
    expect(within(nav).getByRole('link', { name: uk.cabinet.nav.home })).toHaveAttribute('href', '/my')
    expect(within(nav).getByRole('link', { name: uk.cabinet.nav.home })).toHaveAttribute('aria-current', 'page')
    await user.click(within(nav).getByRole('link', { name: uk.cabinet.nav.site }))
    await waitFor(() => expect(currentLocation()).toBe('/'))
  })

  it('shows texts in English', async () => {
    await setLocale('en')
    mockApi(residentRoutes)
    renderApp('/my')
    expect(await screen.findByRole('heading', { name: en.cabinet.myHome.title })).toBeInTheDocument()
    expect(await screen.findByText('Owner')).toBeInTheDocument()
  })

  it('shows a page that does not exist inside the cabinet frame', async () => {
    mockApi({ ...residentRoutes, 'GET /me/home': { body: myHome } })
    renderApp('/my/nothing-here')
    expect(await screen.findByRole('navigation', { name: uk.cabinet.nav.label })).toBeInTheDocument()
    expect(screen.getByText(uk.notFound.title)).toBeInTheDocument()
  })
})
