import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '../../i18n'
import uk from '../../locales/uk.json'
import { callsTo, mockApi } from '../../test/api'
import { adminRoutes, chat, labels, yard } from '../../test/fixtures'
import { renderApp } from '../../test/render'

const routes = {
  ...adminRoutes,
  'GET /admin/chats': { body: [chat(), yard] },
  'GET /admin/labels': { body: labels },
}

describe('LabelsPage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('lists labels with where they hang and what uses them', async () => {
    mockApi(routes)
    renderApp('/admin/chats/labels')
    const row = (await screen.findByText('journal')).closest('tr') as HTMLElement
    expect(within(row).getByText('Our yard › Announcements')).toBeInTheDocument()
    expect(within(row).getByText(uk.admin.labels.usedByApplications)).toBeInTheDocument()
  })

  it('lists the people a label hangs on', async () => {
    mockApi({ ...routes, 'GET /admin/labels': { body: [{ id: 4, name: 'testers', description: '', targets: [{ type: 'user', id: 7 }],
      users: [{ id: 7, name: 'Ann Lee', reachable: true }], usedByApplications: false }] } })
    renderApp('/admin/chats/labels')
    const row = (await screen.findByText('testers')).closest('tr') as HTMLElement
    expect(within(row).getByText('Ann Lee')).toBeInTheDocument()
  })

  it('creates, renames and refuses a taken name', async () => {
    const { calls } = mockApi({
      ...routes,
      'POST /admin/labels': { status: 422, body: { error: { code: 'validation', fields: { name: 'taken' } } } },
      'PATCH /admin/labels/1': { body: { ...labels[0], name: 'neighbours' } },
    })
    const { user } = renderApp('/admin/chats/labels')
    await user.click(await screen.findByRole('button', { name: uk.admin.labels.add }))
    await user.type(await screen.findByLabelText(uk.admin.labels.name), 'Journal')
    await user.click(screen.getByRole('button', { name: uk.common.save }))
    expect(await screen.findByText(uk.validation.taken)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: uk.common.close }))

    const row = (await screen.findByText('residents')).closest('tr') as HTMLElement
    await user.click(within(row).getByRole('button', { name: uk.common.edit }))
    const name = await screen.findByLabelText(uk.admin.labels.name)
    await user.clear(name)
    await user.type(name, 'neighbours')
    await user.click(screen.getByRole('button', { name: uk.common.save }))
    await waitFor(() => expect(callsTo(calls, 'PATCH /admin/labels/1')[0]?.body).toEqual({ name: 'neighbours', description: '' }))
  })

  it('delete dialog names the chats and the setting that lose the label', async () => {
    const { calls } = mockApi({ ...routes, 'DELETE /admin/labels/2': { status: 204 } })
    const { user } = renderApp('/admin/chats/labels')
    const row = (await screen.findByText('journal')).closest('tr') as HTMLElement
    await user.click(within(row).getByRole('button', { name: uk.common.delete }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(/Our yard › Announcements/)).toBeInTheDocument()
    expect(within(dialog).getByText(new RegExp(uk.admin.labels.usedByApplications))).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: uk.common.confirm }))
    await waitFor(() => expect(callsTo(calls, 'DELETE /admin/labels/2')).toHaveLength(1))
  })
})
