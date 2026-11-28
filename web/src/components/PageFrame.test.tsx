import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { callsTo, mockApi } from '../test/api'
import { admin, publicRoutes, resident } from '../test/fixtures'
import { renderWithProviders } from '../test/render'
import { isOurPageUrl } from '../lib/pageUrl'
import { PAGE_SANDBOX, PageFrame } from './PageFrame'

const SANDBOX = 'allow-scripts allow-popups allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation'

function frame() {
  return screen.getByTitle('Page') as HTMLIFrameElement
}

function send(source: MessageEventSource | null, data: unknown, ports: unknown[] = []) {
  act(() => {
    const ev = new MessageEvent('message', { data, source })
    // jsdom has no MessagePort; the frame only calls postMessage and close on it.
    Object.defineProperty(ev, 'ports', { value: ports })
    window.dispatchEvent(ev)
  })
}

/** A stand-in for the MessagePort the page's bridge script transfers with homelander:ready. */
function fakePort() {
  return { postMessage: vi.fn(), close: vi.fn() }
}

/** The nonce of the current frame load, from its URL. */
function nonce() {
  return new URL(frame().getAttribute('src') ?? '', 'http://localhost').searchParams.get('hl')
}

/** The page's bridge saying it is ready, with the current nonce and a fresh port. */
function ready(over: { nonce?: string | null } = {}) {
  const port = fakePort()
  send(frame().contentWindow, { type: 'homelander:ready', nonce: 'nonce' in over ? over.nonce : nonce() }, [port])
  return port
}

const items = { 'dues.paid': { kind: 'value', type: 'text', value: 'apartment 3' } }
const residentData = { type: 'homelander:data', viewer: { signedIn: true, resident: true, admin: false }, items }

/** Lets pending queries and effects settle. */
async function settle() {
  await act(async () => {
    await new Promise((r) => setTimeout(r, 20))
  })
}

describe('PageFrame', () => {
  beforeEach(() => {
    mockApi(publicRoutes)
  })

  it('is sandboxed without same-origin and carries the language and a per-load nonce', () => {
    renderWithProviders(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    expect(PAGE_SANDBOX).toBe(SANDBOX)
    expect(frame().getAttribute('sandbox')).toBe(SANDBOX)
    expect(frame().getAttribute('sandbox')).not.toContain('allow-same-origin')
    expect(frame().getAttribute('src')).toMatch(/^\/site\/index\.html\?lang=uk&hl=[0-9a-f]{32}$/)
    expect(frame().style.height).toBe('600px')
  })

  it('takes the height only from its own frame and clamps it', () => {
    renderWithProviders(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    const win = frame().contentWindow
    send(win, { type: 'homelander:height', height: 900 })
    expect(frame().style.height).toBe('900px')
    send(window, { type: 'homelander:height', height: 700 })
    expect(frame().style.height).toBe('900px')
    send(win, { type: 'homelander:height', height: 999999 })
    expect(frame().style.height).toBe('20000px')
    send(win, { type: 'homelander:height', height: 3 })
    expect(frame().style.height).toBe('100px')
    send(win, { type: 'homelander:height', height: 'big' })
    send(win, { type: 'other', height: 500 })
    expect(frame().style.height).toBe('100px')
  })

  it('posts the new language into the frame without reloading it', () => {
    const { rerender } = renderWithProviders(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    const src = frame().getAttribute('src')
    const spy = vi.spyOn(frame().contentWindow as Window, 'postMessage')
    rerender(<PageFrame src="/site/index.html" lang="en" title="Page" />)
    expect(spy).toHaveBeenCalledWith({ type: 'homelander:lang', lang: 'en' }, '*')
    expect(frame().getAttribute('src')).toBe(src)
  })

  it('gets a new nonce for a new page', () => {
    const { rerender } = renderWithProviders(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    const first = nonce()
    rerender(<PageFrame src="/site/contacts.html" lang="uk" title="Page" />)
    expect(frame().getAttribute('src')).toMatch(/^\/site\/contacts\.html\?lang=uk&hl=/)
    expect(nonce()).not.toBe(first)
  })
})

describe('PageFrame navigation', () => {
  it('passes on navigation only from its own frame and only to listed pages', () => {
    mockApi(publicRoutes)
    const onNavigate = vi.fn()
    renderWithProviders(<PageFrame src="/site/index.html" lang="uk" title="Page" pages={['/', '/contacts']} onNavigate={onNavigate} />)
    const win = frame().contentWindow
    send(window, { type: 'homelander:navigate', path: '/contacts' })
    send(win, { type: 'homelander:navigate', path: '/admin' })
    send(win, { type: 'homelander:navigate', path: '/rules' })
    send(win, { type: 'homelander:navigate', path: 5 })
    expect(onNavigate).not.toHaveBeenCalled()
    send(win, { type: 'homelander:navigate', path: '/contacts' })
    expect(onNavigate).toHaveBeenCalledExactlyOnceWith('/contacts')
  })
})

describe('PageFrame data', () => {
  it('answers a guest page through its port and never through the window', async () => {
    const { calls } = mockApi(publicRoutes)
    renderWithProviders(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    const spy = vi.spyOn(frame().contentWindow as Window, 'postMessage')
    await settle()
    const port = ready()
    await waitFor(() => expect(port.postMessage).toHaveBeenCalledExactlyOnceWith({ type: 'homelander:data', viewer: { signedIn: false }, items: null }))
    expect(spy).not.toHaveBeenCalled()
    expect(callsTo(calls, 'GET /me/data')).toHaveLength(0)
  })

  it('posts nothing to a ready without the nonce of this load, without a port, or from another window', async () => {
    mockApi({ ...publicRoutes, 'GET /me': { body: resident }, 'GET /me/data': { body: items } })
    renderWithProviders(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    await settle()
    const ports = [ready({ nonce: null }), ready({ nonce: 'f'.repeat(32) }), ready({ nonce: 42 as unknown as string })]
    send(frame().contentWindow, { type: 'homelander:ready', nonce: nonce() })
    const other = fakePort()
    send(window, { type: 'homelander:ready', nonce: nonce() }, [other])
    send(null, { type: 'homelander:ready', nonce: nonce() }, [other])
    await settle()
    for (const p of [...ports, other]) expect(p.postMessage).not.toHaveBeenCalled()
  })

  it('posts residents data once loaded, and again after a language change', async () => {
    mockApi({ ...publicRoutes, 'GET /me': { body: resident }, 'GET /me/data': { body: items } })
    const { rerender } = renderWithProviders(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    // Ready before the data is loaded: the message follows when it is.
    const port = ready()
    await waitFor(() => expect(port.postMessage).toHaveBeenCalledWith(residentData))
    expect(port.postMessage).toHaveBeenCalledTimes(1)
    rerender(<PageFrame src="/site/index.html" lang="en" title="Page" />)
    expect(port.postMessage).toHaveBeenCalledTimes(2)
  })

  it('stops posting after the frame navigates, until a new ready with the nonce', async () => {
    mockApi({ ...publicRoutes, 'GET /me': { body: resident }, 'GET /me/data': { body: items } })
    const { rerender } = renderWithProviders(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    await settle()
    const first = ready()
    await waitFor(() => expect(first.postMessage).toHaveBeenCalledTimes(1))
    fireEvent.load(frame()) // the page finished loading
    rerender(<PageFrame src="/site/index.html" lang="en" title="Page" />)
    expect(first.postMessage).toHaveBeenCalledTimes(2)
    fireEvent.load(frame()) // a second load: the frame went to another document
    expect(first.close).toHaveBeenCalled()
    rerender(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    expect(first.postMessage).toHaveBeenCalledTimes(2)
    // A foreign document cannot answer: it does not know the nonce.
    const foreign = ready({ nonce: null })
    rerender(<PageFrame src="/site/index.html" lang="en" title="Page" />)
    expect(foreign.postMessage).not.toHaveBeenCalled()
    // Our page loaded again (a reload) says ready with the nonce before its load event.
    const again = ready()
    fireEvent.load(frame())
    expect(again.postMessage).toHaveBeenCalledTimes(1)
    rerender(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    expect(again.postMessage).toHaveBeenCalledTimes(2)
    expect(first.postMessage).toHaveBeenCalledTimes(2)
  })

  it('tells an admin preview who is looking', async () => {
    mockApi({ ...publicRoutes, 'GET /me': { body: admin }, 'GET /me/data': { body: items } })
    renderWithProviders(<PageFrame src="/site/preview/tok/index.html" lang="uk" title="Page" />)
    await settle()
    const port = ready()
    await waitFor(() => expect(port.postMessage).toHaveBeenCalledWith({ type: 'homelander:data', viewer: { signedIn: true, resident: false, admin: true }, items }))
  })

  it('never loads or posts data for a frame that is not one of our pages', async () => {
    const { calls } = mockApi({ ...publicRoutes, 'GET /me': { body: resident }, 'GET /me/data': { body: items } })
    renderWithProviders(<PageFrame src="https://other.test/index.html" lang="uk" title="Page" />)
    expect(nonce()).toBeNull()
    await settle()
    const port = ready({ nonce: 'x' })
    await settle()
    expect(port.postMessage).not.toHaveBeenCalled()
    expect(callsTo(calls, 'GET /me/data')).toHaveLength(0)
  })

  it('knows our page URLs', () => {
    expect(isOurPageUrl('/site/index.html')).toBe(true)
    expect(isOurPageUrl('/site/preview/abc.1.x/contacts.html')).toBe(true)
    for (const src of ['//site/index.html', '/sitex/index.html', 'https://other.test/site/index.html', 'site/index.html', '/site/../api/x']) {
      expect(isOurPageUrl(src)).toBe(false)
    }
  })
})
