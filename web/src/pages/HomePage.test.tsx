import { screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../i18n'
import en from '../locales/en.json'
import uk from '../locales/uk.json'
import { mockApi } from '../test/api'
import { publicRoutes, site } from '../test/fixtures'
import { renderApp } from '../test/render'

describe('HomePage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('shows community name, welcome text and illustration for a guest', async () => {
    mockApi(publicRoutes)
    renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Тестова громада' })).toBeInTheDocument()
    expect(screen.getByText('Привіт')).toBeInTheDocument()
    expect(await screen.findAllByTestId('house')).toHaveLength(2)
    expect(screen.getByText('House A')).toBeInTheDocument()
    expect(screen.getByText('House B')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: uk.home.login })).toHaveAttribute('href', '/login')
    expect(screen.getByRole('link', { name: uk.home.register })).toHaveAttribute('href', '/register')
    expect(screen.queryByRole('button', { name: uk.home.logout })).not.toBeInTheDocument()
  })

  it('shows the uploaded picture instead of the illustration', async () => {
    mockApi({ ...publicRoutes, 'GET /site': { body: { ...site, pictureUrl: '/api/v1/assets/3' } } })
    renderApp('/')
    const img = await screen.findByRole('img', { name: 'Тестова громада' })
    expect(img).toHaveAttribute('src', '/api/v1/assets/3')
    expect(screen.queryByRole('img', { name: uk.home.illustration })).not.toBeInTheDocument()
  })

  it('falls back to the default-locale community name', async () => {
    await i18n.changeLanguage('en')
    mockApi({ ...publicRoutes, 'GET /site': { body: { ...site, communityName: { uk: 'Тестова громада' } } } })
    renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Тестова громада' })).toBeInTheDocument()
  })

  it('shows a generic greeting when no community name is set', async () => {
    await i18n.changeLanguage('en')
    mockApi({ ...publicRoutes, 'GET /site': { body: { ...site, communityName: {}, welcomeText: {} } } })
    renderApp('/')
    expect(await screen.findByRole('heading', { name: en.home.title })).toBeInTheDocument()
    expect(screen.getByText(en.home.welcomeDefault)).toBeInTheDocument()
  })

  it('shows the published page in a frame under our header', async () => {
    mockApi({ ...publicRoutes, 'GET /site': { body: { ...site, pageUrl: '/site/index.html' } } })
    renderApp('/')
    const frame = await waitFor(() => {
      const f = document.querySelector('iframe')
      expect(f).not.toBeNull()
      return f!
    })
    expect(frame).toHaveAttribute('title', 'Тестова громада')
    expect(frame).toHaveAttribute('src', '/site/index.html?lang=uk')
    expect(screen.getByTestId('site-header-name')).toHaveTextContent('Тестова громада')
    expect(await screen.findByRole('link', { name: uk.home.login })).toBeInTheDocument()
    expect(screen.queryByText('Привіт')).not.toBeInTheDocument()
    expect(screen.queryByTestId('house')).not.toBeInTheDocument()
  })

  it('shows the built-in page when nothing is published', async () => {
    mockApi(publicRoutes)
    renderApp('/')
    expect(await screen.findByText('Привіт')).toBeInTheDocument()
    expect(document.querySelector('iframe')).toBeNull()
  })

  it('applies the site default language when the visitor chose none', async () => {
    mockApi({ ...publicRoutes, 'GET /site': { body: { ...site, defaultLocale: 'en' } } })
    renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Test Community' })).toBeInTheDocument()
  })

  it('keeps the visitor language over the site default', async () => {
    localStorage.setItem('locale', 'uk')
    mockApi({ ...publicRoutes, 'GET /site': { body: { ...site, defaultLocale: 'en' } } })
    renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Тестова громада' })).toBeInTheDocument()
    await waitFor(() => expect(i18n.language).toBe('uk'))
  })
})
