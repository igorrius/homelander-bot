import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
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

  it('does not throw when the browser cannot draw the QR picture', async () => {
    const user = userEvent.setup()
    render(<Providers env="test"><InviteLink url="http://localhost/register" /></Providers>)
    await user.click(screen.getByRole('button', { name: uk.admin.invite.download }))
    expect(screen.getByRole('button', { name: uk.admin.invite.download })).toBeInTheDocument()
  })
})
