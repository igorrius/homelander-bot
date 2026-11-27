import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it } from 'vitest'
import App from './App'
import { Providers } from './Providers'
import en from './locales/en.json'
import uk from './locales/uk.json'
import { i18n } from './i18n'

function renderAt(path: string) {
  return render(
    <Providers>
      <MemoryRouter initialEntries={[path]}>
        <App />
      </MemoryRouter>
    </Providers>,
  )
}

describe('App routes', () => {
  beforeEach(async () => {
    localStorage.clear()
    await i18n.changeLanguage('uk')
  })

  it('shows the home page in Ukrainian by default', () => {
    renderAt('/')
    expect(screen.getByRole('heading', { name: uk.home.title })).toBeInTheDocument()
  })

  it('shows the admin area under /admin', () => {
    renderAt('/admin/users/5')
    expect(screen.getByRole('heading', { name: uk.admin.title })).toBeInTheDocument()
  })

  it('shows the register and login pages', () => {
    renderAt('/register')
    expect(screen.getByRole('heading', { name: uk.register.title })).toBeInTheDocument()
  })

  it('shows not found for unknown paths', () => {
    renderAt('/nope')
    expect(screen.getByRole('heading', { name: uk.notFound.title })).toBeInTheDocument()
  })

  it('switches language to English and remembers it', async () => {
    renderAt('/')
    await userEvent.click(screen.getByRole('radio', { name: 'EN' }))
    expect(await screen.findByRole('heading', { name: en.home.title })).toBeInTheDocument()
    expect(localStorage.getItem('locale')).toBe('en')
  })
})
