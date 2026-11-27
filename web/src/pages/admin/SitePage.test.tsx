import { screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, site } from '../../test/fixtures'
import { renderApp } from '../../test/render'
import { choose } from '../../test/select'

const routes = { ...adminRoutes, 'GET /admin/site': { body: site }, 'GET /admin/houses': { body: [] } }
const fileInput = (container: HTMLElement) => container.querySelector('input[type="file"]') as HTMLInputElement

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
        { communityName: { uk: 'Тестова громада', en: 'Test Community' }, welcomeText: { uk: 'Привіт' }, defaultLocale: 'en' },
      ]),
    )
  })

  it('uploads a picture', async () => {
    const { calls } = mockApi({ ...routes, 'PUT /admin/site/picture': { body: { ...site, pictureUrl: '/api/v1/assets/7' } } })
    const { user, container } = renderApp('/admin/site')
    await screen.findByText(uk.admin.site.picture)
    const file = new File(['x'], 'a.png', { type: 'image/png' })
    await user.upload(fileInput(container), file)
    expect(await screen.findByRole('img', { name: uk.admin.site.picture })).toHaveAttribute('src', '/api/v1/assets/7')
    const body = callsTo(calls, 'PUT /admin/site/picture')[0].body as FormData
    expect(body).toBeInstanceOf(FormData)
    expect((body.get('picture') as File).name).toBe('a.png')
  })

  it('shows a refused picture type', async () => {
    mockApi({ ...routes, 'PUT /admin/site/picture': { status: 422, body: { error: { code: 'validation', fields: { picture: 'type' } } } } })
    const { user, container } = renderApp('/admin/site')
    await screen.findByText(uk.admin.site.picture)
    await user.upload(fileInput(container), new File(['x'], 'a.png', { type: 'image/png' }))
    expect(await screen.findByText(uk.validation.type)).toBeInTheDocument()
  })

  it('removes the picture after confirmation', async () => {
    const { calls } = mockApi({
      ...routes,
      'GET /admin/site': { body: { ...site, pictureUrl: '/api/v1/assets/7' } },
      'DELETE /admin/site/picture': { body: site },
    })
    const { user } = renderApp('/admin/site')
    await user.click(await screen.findByRole('button', { name: uk.admin.site.removePicture }))
    await user.click(await screen.findByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'DELETE /admin/site/picture')).toHaveLength(1))
    expect(await screen.findByRole('img', { name: uk.home.illustration })).toBeInTheDocument()
  })

  it('shows the registration link with copy button and QR', async () => {
    mockApi(routes)
    renderApp('/admin/site')
    expect(await screen.findByText('http://localhost/register')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: uk.common.copy })).toBeInTheDocument()
    expect(screen.getByTitle(uk.qr.title)).toBeInTheDocument()
  })
})
