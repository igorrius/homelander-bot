import { describe, expect, it, vi } from 'vitest'
import source from '../../public/sw.js?raw'

type Listener = (event: unknown) => void

/** Runs sw.js against a fake worker scope and returns its listeners and fakes. */
function load(fetchImpl: (req: unknown) => Promise<unknown>) {
  const listeners: Record<string, Listener> = {}
  const cache = { add: vi.fn(async () => {}) }
  const offline = { offline: true }
  const caches = {
    open: vi.fn(async () => cache),
    keys: vi.fn(async () => ['homelander-old', 'other']),
    delete: vi.fn(async () => true),
    match: vi.fn(async (url: string) => (url === '/offline.html' ? offline : undefined)),
  }
  const self = {
    location: { origin: 'https://example.com' },
    addEventListener: (type: string, fn: Listener) => {
      listeners[type] = fn
    },
    skipWaiting: vi.fn(async () => {}),
    clients: { claim: vi.fn(async () => {}) },
  }
  const fetch = vi.fn(fetchImpl)
  new Function('self', 'caches', 'fetch', source)(self, caches, fetch)
  return { listeners, cache, caches, self, fetch, offline }
}

function fetchEvent(url: string, mode = 'navigate', method = 'GET') {
  let response: Promise<unknown> | undefined
  return {
    request: { url, mode, method },
    respondWith: vi.fn((r: Promise<unknown>) => {
      response = r
    }),
    response: () => response,
  }
}

async function waitUntil(type: string, sw: ReturnType<typeof load>) {
  let done: Promise<unknown> = Promise.resolve()
  sw.listeners[type]({ waitUntil: (p: Promise<unknown>) => (done = p) })
  await done
}

describe('service worker', () => {
  it('keeps only the offline page on install and drops older caches on activate', async () => {
    const sw = load(async () => ({}))
    await waitUntil('install', sw)
    expect(sw.cache.add).toHaveBeenCalledWith('/offline.html')
    expect(sw.self.skipWaiting).toHaveBeenCalled()
    await waitUntil('activate', sw)
    expect(sw.caches.delete.mock.calls.map((c) => (c as unknown[])[0])).toEqual(['homelander-old', 'other'])
    expect(sw.self.clients.claim).toHaveBeenCalled()
  })

  it('serves the offline page when a cabinet navigation fails', async () => {
    const sw = load(async () => {
      throw new TypeError('offline')
    })
    const ev = fetchEvent('https://example.com/my')
    sw.listeners.fetch(ev)
    expect(ev.respondWith).toHaveBeenCalled()
    await expect(ev.response()).resolves.toBe(sw.offline)
  })

  it('passes a working navigation through without caching it', async () => {
    const page = { page: true }
    const sw = load(async () => page)
    const ev = fetchEvent('https://example.com/contacts')
    sw.listeners.fetch(ev)
    await expect(ev.response()).resolves.toBe(page)
    expect(sw.cache.add).not.toHaveBeenCalled()
  })

  it('leaves the API, page files, MCP, OAuth and every non-navigation request alone', () => {
    const sw = load(async () => ({}))
    for (const ev of [
      fetchEvent('https://example.com/api/v1/me'),
      fetchEvent('https://example.com/api'),
      fetchEvent('https://example.com/site/index.html'),
      fetchEvent('https://example.com/mcp'),
      fetchEvent('https://example.com/oauth/authorize?client_id=x'),
      fetchEvent('https://example.com/telegram/webhook/x'),
      fetchEvent('https://example.com/healthz'),
      fetchEvent('https://example.com/my', 'cors'),
      fetchEvent('https://example.com/assets/app.js', 'no-cors'),
      fetchEvent('https://example.com/my', 'navigate', 'POST'),
      fetchEvent('https://elsewhere.example/my'),
    ]) {
      sw.listeners.fetch(ev)
      expect(ev.respondWith, ev.request.url + ' ' + ev.request.mode).not.toHaveBeenCalled()
    }
    expect(sw.fetch).not.toHaveBeenCalled()
  })
})
