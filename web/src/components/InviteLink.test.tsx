import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { i18n } from '../i18n'
import uk from '../locales/uk.json'
import { Providers } from '../Providers'
import { InviteLink } from './InviteLink'

describe('InviteLink', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('shows the link, QR, copy and download', async () => {
    render(<Providers env="test"><InviteLink url="http://localhost/register" /></Providers>)
    expect(screen.getByText('http://localhost/register')).toBeInTheDocument()
    expect(screen.getByTitle(uk.qr.title)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: uk.common.copy })).toBeInTheDocument()
    expect(screen.getByText(uk.admin.invite.hint)).toBeInTheDocument()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('loads the QR picture from a data: URL (the CSP allows data: images, not blob:)', async () => {
    const sources: string[] = []
    vi.stubGlobal('Image', class {
      onload: (() => void) | null = null
      onerror: (() => void) | null = null
      set src(v: string) {
        sources.push(v)
      }
    })
    const user = userEvent.setup()
    render(<Providers env="test"><InviteLink url="http://localhost/register" /></Providers>)
    await user.click(screen.getByRole('button', { name: uk.admin.invite.download }))
    expect(sources).toHaveLength(1)
    expect(sources[0]).toMatch(/^data:image\/svg\+xml/)
  })

  it('downloads the QR with the standard 4-module white margin', async () => {
    const sources: string[] = []
    vi.stubGlobal('Image', class {
      onload: (() => void) | null = null
      set src(v: string) {
        sources.push(v)
      }
    })
    const user = userEvent.setup()
    render(<Providers env="test"><InviteLink url="http://localhost/register" /></Providers>)
    // The on-screen code has a 1-module margin; the file gets 3 more on each side.
    const shown = Number(screen.getByTitle(uk.qr.title).closest('svg')!.getAttribute('viewBox')!.split(' ')[2])
    await user.click(screen.getByRole('button', { name: uk.admin.invite.download }))
    expect(decodeURIComponent(sources[0])).toContain(`viewBox="0 0 ${shown + 6} ${shown + 6}"`)
  })

  it('does not throw when the browser cannot draw the QR picture', async () => {
    const user = userEvent.setup()
    render(<Providers env="test"><InviteLink url="http://localhost/register" /></Providers>)
    await user.click(screen.getByRole('button', { name: uk.admin.invite.download }))
    expect(screen.getByRole('button', { name: uk.admin.invite.download })).toBeInTheDocument()
  })
})
