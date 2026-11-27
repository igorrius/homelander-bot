import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, chat, labels, yard } from '../../test/fixtures'
import { renderApp } from '../../test/render'

const routes = (chats = [chat(), yard]) => ({
  ...adminRoutes,
  'GET /admin/chats': { body: chats },
  'GET /admin/labels': { body: labels },
})

async function card(title: string) {
  return (await screen.findByText(title)).closest('[data-chat]') as HTMLElement
}

describe('ChatsPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('lists new chats first and accepts one', async () => {
    const { calls } = mockApi({ ...routes(), 'POST /admin/chats/1/accept': { status: 204 } })
    const { user } = renderApp('/admin/chats')
    const cards = await screen.findAllByTestId('chat-card')
    expect(within(cards[0]).getByText('log')).toBeInTheDocument()
    expect(within(cards[0]).getByText(uk.admin.chats.state.new)).toBeInTheDocument()
    expect(screen.getByText(uk.admin.chats.newHint)).toBeInTheDocument()
    await user.click(within(cards[0]).getByRole('button', { name: uk.admin.chats.accept }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/chats/1/accept')).toHaveLength(1))
  })

  it('leaves a chat after confirmation', async () => {
    const { calls } = mockApi({ ...routes(), 'POST /admin/chats/2/leave': { status: 204 } })
    const { user } = renderApp('/admin/chats')
    await user.click(within(await card('Our yard')).getByRole('button', { name: uk.admin.chats.leave }))
    expect(callsTo(calls, 'POST /admin/chats/2/leave')).toHaveLength(0)
    await user.click(await screen.findByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/chats/2/leave')).toHaveLength(1))
  })

  it('shows topics under their forum and renames an unnamed topic', async () => {
    const { calls } = mockApi({ ...routes(), 'PATCH /admin/topics/22': { status: 204 } })
    const { user } = renderApp('/admin/chats')
    const forum = await card('Our yard')
    expect(within(forum).getByText('Announcements')).toBeInTheDocument()
    const unnamed = within(forum).getByText(i18n.t('admin.chats.unnamedTopic', { id: '42' })).closest('[data-topic]') as HTMLElement
    await user.click(within(unnamed).getByRole('button', { name: uk.admin.chats.rename }))
    const input = await screen.findByLabelText(uk.admin.chats.topicTitle)
    await user.type(input, 'Parking')
    await user.click(screen.getByRole('button', { name: uk.common.save }))
    await waitFor(() => expect(callsTo(calls, 'PATCH /admin/topics/22')[0]?.body).toEqual({ title: 'Parking' }))
  })

  it('edits labels and creates a label', async () => {
    const created = { id: 3, name: 'gate', description: '', targets: [], usedByApplications: false }
    const { calls } = mockApi({ ...routes(), 'PUT /admin/chats/1/labels': { status: 204 }, 'POST /admin/labels': { status: 201, body: created } })
    const { user } = renderApp('/admin/chats')
    await user.click(within(await card('log')).getByRole('button', { name: uk.admin.chats.editLabels }))
    await user.click(await screen.findByRole('checkbox', { name: 'journal' }))
    await user.type(screen.getByLabelText(uk.admin.labels.newLabel), 'gate')
    await user.click(screen.getByRole('button', { name: uk.admin.labels.create }))
    await waitFor(() => expect(callsTo(calls, 'POST /admin/labels')[0]?.body).toEqual({ name: 'gate', description: '' }))
    await user.click(screen.getByRole('button', { name: uk.common.save }))
    await waitFor(() => expect(callsTo(calls, 'PUT /admin/chats/1/labels')[0]?.body).toEqual({ labelIds: [2, 3] }))
  })

  it('shows the empty state', async () => {
    mockApi(routes([]))
    renderApp('/admin/chats')
    expect(await screen.findByText(uk.admin.chats.emptyTitle)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: uk.admin.chats.setupGuide })).toHaveAttribute('href', expect.stringContaining('telegram-bot-setup.md'))
  })

  it('warns when the bot cannot send', async () => {
    mockApi(routes([chat({ state: 'accepted', canSend: false, botStatus: 'restricted' })]))
    renderApp('/admin/chats')
    expect(await screen.findByText(uk.admin.chats.cannotSend)).toBeInTheDocument()
  })

  it('keeps left chats in a collapsed block with who added the bot', async () => {
    mockApi(routes([chat({ id: 9, title: 'Stranger group', state: 'left', addedBy: { externalId: '1000099', userId: null } })]))
    const { user } = renderApp('/admin/chats')
    const toggle = await screen.findByRole('button', { name: new RegExp(uk.admin.chats.leftChats) })
    expect(screen.queryByText('Stranger group')).not.toBeInTheDocument()
    await user.click(toggle)
    expect(await screen.findByText('Stranger group')).toBeInTheDocument()
    expect(screen.getByText(i18n.t('admin.chats.addedBy', { id: '1000099' }))).toBeInTheDocument()
  })

  it('reports a test send failure reason', async () => {
    mockApi({
      ...routes(),
      'POST /admin/chats/2/test': { status: 202, body: { outboxIds: [77] } },
      'GET /admin/outbox/77': { body: { state: 'failed', lastError: 'send permanent: not enough rights' } },
    })
    const { user } = renderApp('/admin/chats')
    const forum = await card('Our yard')
    await user.click(within(forum).getAllByRole('button', { name: uk.admin.chats.test })[0])
    expect(await within(forum).findByText(/not enough rights/)).toBeInTheDocument()
  })
})
