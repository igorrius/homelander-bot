import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import type { AdminPost } from '../../api/types'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminPost, adminRoutes, chat, feedModule, labels, yard } from '../../test/fixtures'
import { currentLocation, renderApp } from '../../test/render'

const unused = { id: 3, name: 'unused', description: '', targets: [{ type: 'chat' as const, id: 1 }], users: [], usedByApplications: false }
const routes = {
  ...adminRoutes,
  'GET /admin/chats': { body: [chat(), yard] },
  'GET /admin/labels': { body: [...labels, unused] },
  'GET /admin/feed/settings': { body: { defaultLabelIds: [1, 3] } },
  'GET /admin/feed/posts': { body: { items: [], total: 0 } },
}
const created = (req: { body: unknown }, over: Partial<AdminPost> = {}) => {
  const b = req.body as { body: string; imageAssetId: number | null; labelIds: number[] }
  return { status: 201, body: adminPost({ id: 5, body: b.body, imageAssetId: b.imageAssetId, labelIds: b.labelIds, ...over }) }
}
const published = adminPost({
  id: 7, body: 'Lift repair on Monday', state: 'published', publishedAt: '2026-11-27T12:00:00Z', labelIds: [1, 2],
  delivery: { pending: 0, sent: 1, failed: 1 },
  deliveries: [
    { id: 70, chatTitle: 'Our yard', topicTitle: '', labelId: 1, state: 'failed', error: 'bot was kicked', sentAt: null },
    { id: 71, chatTitle: 'Our yard', topicTitle: 'Announcements', labelId: 2, state: 'sent', error: '', sentAt: '2026-11-27T12:00:05Z' },
  ],
})
const bodyField = () => screen.findByRole('textbox', { name: uk.admin.feed.editor.body })
const labelsField = () => screen.findByRole('combobox', { name: uk.admin.feed.editor.labels })
/** Ids picked in "Also post to", comma separated. */
const chosenLabels = (container: HTMLElement) => (container.querySelector('input[name="labelIds"]') as HTMLInputElement).value
const fileInput = (container: HTMLElement) => container.querySelector('input[type="file"]') as HTMLInputElement

describe('FeedPostEditor', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('prefills the default labels and offers only labels that reach an accepted chat', async () => {
    mockApi(routes)
    const { user, container } = renderApp('/admin/modules/feed/posts/new')
    const field = await labelsField()
    // The default "unused" reaches nobody, so it is not prefilled.
    expect(chosenLabels(container)).toBe('1')
    expect(screen.getByText(uk.admin.feed.editor.labelsHint)).toBeInTheDocument()
    await user.click(field)
    expect(await screen.findByRole('option', { name: 'journal' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: 'unused' })).not.toBeInTheDocument()
  })

  it('offers a label that reaches only people', async () => {
    const person = (id: number, name: string, reachable: boolean) => ({ id, name, description: '', targets: [{ type: 'user' as const, id }],
      users: [{ id, name: 'Ann Lee', reachable }], usedByApplications: false })
    mockApi({ ...routes, 'GET /admin/labels': { body: [...labels, person(8, 'testers', true), person(9, 'gone', false)] } })
    const { user } = renderApp('/admin/modules/feed/posts/new')
    await user.click(await labelsField())
    expect(await screen.findByRole('option', { name: 'testers' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: 'gone' })).not.toBeInTheDocument()
  })

  it('saves a draft and opens it', async () => {
    const { calls } = mockApi({
      ...routes,
      'POST /admin/feed/posts': (req) => created(req),
      'GET /admin/feed/posts/5': { body: adminPost({ id: 5, body: 'Hello https://example.com', labelIds: [1] }) },
    })
    const { user } = renderApp('/admin/modules/feed/posts/new')
    await user.type(await bodyField(), 'Hello https://example.com')
    // The preview reads like the cabinet: the URL is a link.
    const preview = screen.getByTestId('post-preview')
    expect(within(preview).getByRole('link', { name: 'https://example.com' })).toHaveAttribute('href', 'https://example.com')
    expect(screen.getByText('25 / 4000')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: uk.admin.feed.editor.saveDraft }))
    await waitFor(() =>
      expect(callsTo(calls, 'POST /admin/feed/posts')[0]?.body).toEqual({ body: 'Hello https://example.com', imageAssetId: null, labelIds: [1] }),
    )
    await waitFor(() => expect(currentLocation()).toBe('/admin/modules/feed/posts/5'))
    expect(callsTo(calls, 'POST /admin/feed/posts/5/publish')).toHaveLength(0)
  })

  it('does not save an empty post', async () => {
    mockApi(routes)
    renderApp('/admin/modules/feed/posts/new')
    await bodyField()
    expect(screen.getByRole('button', { name: uk.admin.feed.editor.saveDraft })).toBeDisabled()
    expect(screen.getByRole('button', { name: uk.admin.feed.editor.publish })).toBeDisabled()
  })

  it('publishes a new post with the chosen labels after confirming', async () => {
    const { calls } = mockApi({
      ...routes,
      'POST /admin/feed/posts': (req) => created(req),
      'POST /admin/feed/posts/5/publish': (req) => ({ body: adminPost({ id: 5, state: 'published', labelIds: (req.body as { labelIds: number[] }).labelIds }) }),
      'GET /admin/feed/posts/5': { body: adminPost({ id: 5, state: 'published', labelIds: [1, 2], deliveries: [] }) },
    })
    const { user } = renderApp('/admin/modules/feed/posts/new')
    await user.type(await bodyField(), 'Water is off')
    await user.click(await labelsField())
    await user.click(await screen.findByRole('option', { name: 'journal' }))
    await user.click(screen.getByRole('button', { name: uk.admin.feed.editor.publish }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(/residents, journal/)).toBeInTheDocument()
    expect(callsTo(calls, 'POST /admin/feed/posts')).toHaveLength(0)
    await user.click(within(dialog).getByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/feed/posts/5/publish')[0]?.body).toEqual({ labelIds: [1, 2] }))
    expect(callsTo(calls, 'POST /admin/feed/posts')[0]?.body).toEqual({ body: 'Water is off', imageAssetId: null, labelIds: [1, 2] })
    await waitFor(() => expect(currentLocation()).toBe('/admin/modules/feed/posts/5'))
  })

  it('saves pending edits of a draft before publishing it', async () => {
    const draft = adminPost({ id: 4, body: 'Old', labelIds: [] })
    const { calls } = mockApi({
      ...routes,
      'GET /admin/feed/posts/4': { body: draft },
      'PATCH /admin/feed/posts/4': (req) => ({ body: { ...draft, ...(req.body as object) } }),
      'POST /admin/feed/posts/4/publish': { body: { ...draft, state: 'published' } },
    })
    const { user, container } = renderApp('/admin/modules/feed/posts/4')
    const field = await bodyField()
    // A draft keeps its own label choice: no defaults are added.
    expect(chosenLabels(container)).toBe('')
    await user.clear(field)
    await user.type(field, 'New')
    await user.click(screen.getByRole('button', { name: uk.admin.feed.editor.publish }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(uk.admin.feed.editor.publishConfirmCabinetOnly)).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/feed/posts/4/publish')[0]?.body).toEqual({ labelIds: [] }))
    expect(callsTo(calls, 'PATCH /admin/feed/posts/4')[0]?.body).toEqual({ body: 'New', imageAssetId: null, labelIds: [] })
    const order = calls.map((c) => `${c.method} ${c.path}`)
    expect(order.indexOf('PATCH /admin/feed/posts/4')).toBeLessThan(order.indexOf('POST /admin/feed/posts/4/publish'))
  })

  it('shows a published post with its deliveries, fixed labels and the Telegram note', async () => {
    const { calls } = mockApi({
      ...routes,
      'GET /admin/feed/posts/7': { body: published },
      'PATCH /admin/feed/posts/7': (req) => ({ body: { ...published, ...(req.body as object) } }),
    })
    const { user } = renderApp('/admin/modules/feed/posts/7')
    const field = await bodyField()
    expect(await labelsField()).toBeDisabled()
    expect(screen.getByText(uk.admin.feed.editor.publishedNote)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: uk.admin.feed.editor.publish })).not.toBeInTheDocument()
    const failed = screen.getByTestId('delivery-70')
    expect(within(failed).getByText('Our yard')).toBeInTheDocument()
    expect(within(failed).getByText(uk.admin.feed.deliveries.state.failed)).toBeInTheDocument()
    expect(within(failed).getByText('bot was kicked')).toBeInTheDocument()
    const sent = screen.getByTestId('delivery-71')
    expect(within(sent).getByText('Our yard › Announcements')).toBeInTheDocument()
    expect(within(sent).getByText(uk.admin.feed.deliveries.state.sent)).toBeInTheDocument()
    await user.type(field, '!')
    await user.click(screen.getByRole('button', { name: uk.admin.feed.editor.saveChanges }))
    await waitFor(() => expect(callsTo(calls, 'PATCH /admin/feed/posts/7')[0]?.body).toEqual({ body: 'Lift repair on Monday!', imageAssetId: null }))
  })

  it('polls the post while a copy is still queued', { timeout: 10000 }, async () => {
    let n = 0
    const pending = { ...published, deliveries: [{ ...published.deliveries![1], state: 'pending' as const, sentAt: null }] }
    mockApi({ ...routes, 'GET /admin/feed/posts/7': () => ({ body: ++n === 1 ? pending : published }) })
    renderApp('/admin/modules/feed/posts/7')
    expect(await screen.findByText(uk.admin.feed.deliveries.state.pending)).toBeInTheDocument()
    expect(await screen.findByTestId('delivery-70', {}, { timeout: 6000 })).toBeInTheDocument()
  })

  it('says when delivery details are gone and when a post went to the cabinet only', async () => {
    mockApi({
      ...routes,
      'GET /admin/feed/posts/8': { body: adminPost({ id: 8, state: 'published', publishedAt: '2026-10-01T10:00:00Z', labelIds: [1], deliveryExpired: true, deliveries: [] }) },
      'GET /admin/feed/posts/9': { body: adminPost({ id: 9, state: 'published', publishedAt: '2026-10-01T10:00:00Z', labelIds: [], deliveries: [] }) },
    })
    const { unmount } = renderApp('/admin/modules/feed/posts/8')
    expect(await screen.findByText(uk.admin.feed.deliveries.expired)).toBeInTheDocument()
    unmount()
    renderApp('/admin/modules/feed/posts/9')
    expect(await screen.findByText(uk.admin.feed.deliveries.cabinetOnly)).toBeInTheDocument()
  })

  it('deletes a published post with the Telegram note', async () => {
    const { calls } = mockApi({ ...routes, 'GET /admin/feed/posts/7': { body: published }, 'DELETE /admin/feed/posts/7': { status: 204 } })
    const { user } = renderApp('/admin/modules/feed/posts/7')
    await user.click(await screen.findByRole('button', { name: uk.admin.feed.editor.delete }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(uk.admin.feed.editor.deleteConfirmPublished)).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'DELETE /admin/feed/posts/7')).toHaveLength(1))
    await waitFor(() => expect(currentLocation()).toBe('/admin/modules/feed'))
  })

  it('limits the text to the caption length once an image is added', async () => {
    mockApi({ ...routes, 'POST /admin/data/assets': { status: 201, body: { id: 12, url: '/api/v1/admin/assets/12' } } })
    const { user, container } = renderApp('/admin/modules/feed/posts/new')
    const field = await bodyField()
    await user.click(field)
    await user.paste('x'.repeat(1001))
    expect(screen.getByText('1001 / 4000')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: uk.admin.feed.editor.saveDraft })).toBeEnabled()
    await user.upload(fileInput(container), new File(['x'], 'a.png', { type: 'image/png' }))
    expect(await screen.findByText('1001 / 1000')).toBeInTheDocument()
    expect(screen.getByText(i18n.t('admin.feed.editor.tooLongWithImage', { max: 1000 }))).toBeInTheDocument()
    expect(screen.getByRole('button', { name: uk.admin.feed.editor.saveDraft })).toBeDisabled()
    expect(container.querySelector('img[src="/api/v1/admin/assets/12"]')).not.toBeNull()
    await user.click(screen.getByRole('button', { name: uk.admin.feed.editor.removeImage }))
    expect(await screen.findByText('1001 / 4000')).toBeInTheDocument()
  })

  it('shows the turned-off card while the module is off', async () => {
    mockApi({ ...routes, 'GET /admin/modules': { body: [feedModule({ enabled: false })] } })
    renderApp('/admin/modules/feed/posts/new')
    expect(await screen.findByText(uk.admin.moduleOff.title)).toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: uk.admin.feed.editor.body })).not.toBeInTheDocument()
  })
})
