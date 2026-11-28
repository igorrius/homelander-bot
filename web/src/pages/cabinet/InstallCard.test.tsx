import { act, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { mockApi } from '../../test/api'
import { residentRoutes } from '../../test/fixtures'
import { renderApp } from '../../test/render'

const iPhone = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1'

/** Fires the event Chromium sends when the app can be installed; returns its prompt spy. */
function offerInstall() {
  const prompt = vi.fn(async () => {})
  const ev = Object.assign(new Event('beforeinstallprompt', { cancelable: true }), {
    prompt,
    userChoice: Promise.resolve({ outcome: 'accepted' }),
  })
  act(() => {
    window.dispatchEvent(ev)
  })
  return { prompt, ev }
}

function standalone(on: boolean) {
  const real = window.matchMedia
  vi.spyOn(window, 'matchMedia').mockImplementation((q: string) => ({ ...real(q), matches: on && q.includes('standalone') }))
}

describe('InstallCard', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
    mockApi(residentRoutes)
  })
  afterEach(() => {
    window.dispatchEvent(new Event('appinstalled')) // forget a captured prompt
    vi.restoreAllMocks()
  })

  it('is hidden where the browser cannot install the app', async () => {
    renderApp('/my')
    await screen.findByTestId('cabinet-home-11')
    expect(screen.queryByText(uk.cabinet.install.title)).not.toBeInTheDocument()
  })

  it('installs through the browser prompt', async () => {
    const { prompt, ev } = offerInstall()
    expect(ev.defaultPrevented).toBe(true)
    const { user } = renderApp('/my')
    await user.click(await screen.findByRole('button', { name: uk.cabinet.install.button }))
    expect(prompt).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(screen.queryByText(uk.cabinet.install.title)).not.toBeInTheDocument())
  })

  it('shows the Add to Home Screen steps on iOS', async () => {
    vi.spyOn(navigator, 'userAgent', 'get').mockReturnValue(iPhone)
    renderApp('/my')
    expect(await screen.findByText(uk.cabinet.install.iosText)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: uk.cabinet.install.button })).not.toBeInTheDocument()
  })

  it('stays hidden once dismissed', async () => {
    offerInstall()
    const { user, unmount } = renderApp('/my')
    await user.click(await screen.findByRole('button', { name: uk.cabinet.install.later }))
    expect(screen.queryByText(uk.cabinet.install.title)).not.toBeInTheDocument()
    unmount()
    renderApp('/my')
    await screen.findByTestId('cabinet-home-11')
    expect(screen.queryByText(uk.cabinet.install.title)).not.toBeInTheDocument()
  })

  it('is hidden when already running as the installed app', async () => {
    standalone(true)
    offerInstall()
    renderApp('/my')
    await screen.findByTestId('cabinet-home-11')
    expect(screen.queryByText(uk.cabinet.install.title)).not.toBeInTheDocument()
  })
})
