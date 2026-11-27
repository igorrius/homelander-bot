import { screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, site } from '../../test/fixtures'
import { renderApp } from '../../test/render'
import { choose } from '../../test/select'

const routes = { ...adminRoutes, 'GET /admin/site': { body: site }, 'GET /admin/houses': { body: [] } }

describe('SitePage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('saves texts per locale and the default language', async () => {
    const { calls } = mockApi({ ...routes, 'PUT /admin/site': { body: { ...site, defaultLocale: 'en' } } })
    const { user } = renderApp('/admin/site')
    expect(await screen.findByLabelText(`${uk.admin.site.communityName} (EN)`)).toHaveValue('Test Community')
    await choose(user, uk.admin.site.defaultLocale, 'English')
    await user.click(screen.getByRole('button', { name: uk.admin.site.save }))
    await waitFor(() =>
      expect(callsTo(calls, 'PUT /admin/site').map((c) => c.body)).toEqual([
        { communityName: { uk: 'Тестова громада', en: 'Test Community' }, welcomeText: { uk: 'Привіт' }, defaultLocale: 'en', homeItems: [] },
      ]),
    )
  })

  it('offers no picture upload any more', async () => {
    mockApi(routes)
    const { container } = renderApp('/admin/site')
    await screen.findByLabelText(`${uk.admin.site.communityName} (EN)`)
    expect(container.querySelector('input[type="file"]')).toBeNull()
    expect(screen.queryByText('http://localhost/register')).toBeNull()
  })
})
