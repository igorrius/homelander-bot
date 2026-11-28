import { describe, expect, it } from 'vitest'
import type { Label } from '../api/types'
import { i18n } from '../i18n'
import { labelPlaces, reachesSomeone } from './chats'

const onlyUser = (reachable: boolean): Label => ({
  id: 4, name: 'testers', description: '', targets: [{ type: 'user', id: 7 }],
  users: [{ id: 7, name: 'Ann Lee', reachable }], usedByApplications: false,
})

describe('reachesSomeone', () => {
  it('counts a label that reaches only a person with the bot', () => {
    expect(reachesSomeone(onlyUser(true), [])).toBe(true)
  })

  it('does not count a person who cannot receive messages', () => {
    expect(reachesSomeone(onlyUser(false), [])).toBe(false)
  })
})

describe('labelPlaces', () => {
  it('names the people a label hangs on', () => {
    expect(labelPlaces(i18n.t, onlyUser(true), [])).toEqual(['Ann Lee'])
  })
})
