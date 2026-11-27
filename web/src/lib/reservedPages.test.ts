import { describe, expect, it } from 'vitest'
import goSrc from '../../../internal/domain/page.go?raw'
import app from '../App.tsx?raw'
import { reservedPages } from './reservedPages'

describe('reservedPages', () => {
  it('covers every top-level app route', () => {
    const tops = [...app.matchAll(/<Route path="\/([a-z0-9-]+)/g)].map((m) => m[1])
    expect(tops.length).toBeGreaterThan(3)
    for (const top of tops) expect(reservedPages).toContain(top)
  })

  it('matches the server list', () => {
    const block = goSrc.slice(goSrc.indexOf('ReservedPageNames = []string{'))
    const goList = [...block.slice(0, block.indexOf('}')).matchAll(/"([a-z]+)"/g)].map((m) => m[1])
    expect([...reservedPages].sort()).toEqual(goList.sort())
  })
})
