import { screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from './i18n'
import en from './locales/en.json'
import uk from './locales/uk.json'
import { mockApi } from './test/api'
import { adminRoutes, publicRoutes } from './test/fixtures'
import { renderApp } from './test/render'

describe('App routes', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
    mockApi(publicRoutes)
  })

  it('shows the admin area under /admin', async () => {
    mockApi(adminRoutes)
    renderApp('/admin/users')
    expect(await screen.findByRole('heading', { name: uk.admin.title })).toBeInTheDocument()
  })

  it('shows the register page', async () => {
    renderApp('/register')
    expect(await screen.findByRole('heading', { name: uk.register.title })).toBeInTheDocument()
  })

  it('shows not found for unknown paths', async () => {
    renderApp('/nope')
    expect(await screen.findByRole('heading', { name: uk.notFound.title })).toBeInTheDocument()
  })

  it('switches language to English and remembers it', async () => {
    const { user } = renderApp('/register')
    await user.click(screen.getByRole('radio', { name: 'EN' }))
    expect(await screen.findByRole('heading', { name: en.register.title })).toBeInTheDocument()
    expect(localStorage.getItem('locale')).toBe('en')
  })
})
