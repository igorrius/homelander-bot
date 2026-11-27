import { screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../i18n'
import uk from '../locales/uk.json'
import { mockApi } from '../test/api'
import { publicRoutes, site } from '../test/fixtures'
import { renderApp } from '../test/render'

const published = { ...site, pageUrl: '/site/index.html', pages: ['/', '/contacts'] }

describe('SitePageRoute', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('frames a published page at its own address under our header', async () => {
    mockApi({ ...publicRoutes, 'GET /site': { body: published } })
    renderApp('/contacts')
    const frame = (await screen.findAllByTitle('Тестова громада')).find((el) => el.tagName === 'IFRAME')
    expect(frame).toHaveAttribute('src', '/site/contacts.html?lang=uk')
    expect(screen.getByTestId('site-header-name')).toHaveTextContent('Тестова громада')
  })

  it.each([
    ['a page the published version does not have', '/contacts', { ...published, pages: ['/'] }],
    ['nothing published', '/contacts', site],
    ['upper-case address', '/Contacts', published],
  ])('shows not found for %s', async (_, path, body) => {
    mockApi({ ...publicRoutes, 'GET /site': { body } })
    renderApp(path)
    expect(await screen.findByRole('heading', { name: uk.notFound.title })).toBeInTheDocument()
    expect(document.querySelector('iframe')).toBeNull()
  })
})
