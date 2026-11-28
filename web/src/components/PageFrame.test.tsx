import { act, screen, waitFor } from '@testing-library/react'
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

function send(source: MessageEventSource | null, data: unknown) {
  act(() => {
    window.dispatchEvent(new MessageEvent('message', { data, source }))
  })
}

const items = { 'dues.paid': { kind: 'value', type: 'text', value: 'apartment 3' } }
const dataCalls = (spy: { mock: { calls: unknown[][] } }) =>
  spy.mock.calls.filter((args) => (args[0] as { type?: string } | null)?.type === 'homelander:data')

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

  it('is sandboxed without same-origin and carries the language', () => {
    renderWithProviders(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    expect(PAGE_SANDBOX).toBe(SANDBOX)
    expect(frame().getAttribute('sandbox')).toBe(SANDBOX)
    expect(frame().getAttribute('sandbox')).not.toContain('allow-same-origin')
    expect(frame().getAttribute('src')).toBe('/site/index.html?lang=uk')
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
    const spy = vi.spyOn(frame().contentWindow as Window, 'postMessage')
    rerender(<PageFrame src="/site/index.html" lang="en" title="Page" />)
    expect(spy).toHaveBeenCalledWith({ type: 'homelander:lang', lang: 'en' }, '*')
    expect(frame().getAttribute('src')).toBe('/site/index.html?lang=uk')
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
  it('posts nothing before the page is ready, then tells a guest page not to wait', async () => {
    const { calls } = mockApi(publicRoutes)
    renderWithProviders(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    const spy = vi.spyOn(frame().contentWindow as Window, 'postMessage')
    await settle()
    expect(dataCalls(spy)).toHaveLength(0)
    send(frame().contentWindow, { type: 'homelander:ready' })
    expect(spy).toHaveBeenCalledWith({ type: 'homelander:data', viewer: { signedIn: false }, items: null }, '*')
    expect(callsTo(calls, 'GET /me/data')).toHaveLength(0)
  })

  it('answers ready only from its own frame', async () => {
    mockApi(publicRoutes)
    renderWithProviders(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    const spy = vi.spyOn(frame().contentWindow as Window, 'postMessage')
    await settle()
    send(window, { type: 'homelander:ready' })
    send(null, { type: 'homelander:ready' })
    await settle()
    expect(dataCalls(spy)).toHaveLength(0)
  })

  it('posts residents data with the viewer once loaded, and again after a language change', async () => {
    mockApi({ ...publicRoutes, 'GET /me': { body: resident }, 'GET /me/data': { body: items } })
    const { rerender } = renderWithProviders(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    const spy = vi.spyOn(frame().contentWindow as Window, 'postMessage')
    // Ready before the data is loaded: the message follows when it is.
    send(frame().contentWindow, { type: 'homelander:ready' })
    const want = { type: 'homelander:data', viewer: { signedIn: true, resident: true, admin: false }, items }
    await waitFor(() => expect(spy).toHaveBeenCalledWith(want, '*'))
    expect(dataCalls(spy)).toHaveLength(1)
    rerender(<PageFrame src="/site/index.html" lang="en" title="Page" />)
    expect(dataCalls(spy)).toHaveLength(2)
    expect(spy).toHaveBeenLastCalledWith(want, '*')
  })

  it('tells an admin preview who is looking', async () => {
    mockApi({ ...publicRoutes, 'GET /me': { body: admin }, 'GET /me/data': { body: items } })
    renderWithProviders(<PageFrame src="/site/preview/tok/index.html" lang="uk" title="Page" />)
    const spy = vi.spyOn(frame().contentWindow as Window, 'postMessage')
    await settle()
    send(frame().contentWindow, { type: 'homelander:ready' })
    expect(spy).toHaveBeenCalledWith({ type: 'homelander:data', viewer: { signedIn: true, resident: false, admin: true }, items }, '*')
  })

  it('never loads or posts data for a frame that is not one of our pages', async () => {
    const { calls } = mockApi({ ...publicRoutes, 'GET /me': { body: resident }, 'GET /me/data': { body: items } })
    renderWithProviders(<PageFrame src="https://other.test/index.html" lang="uk" title="Page" />)
    const spy = vi.spyOn(frame().contentWindow as Window, 'postMessage')
    await settle()
    send(frame().contentWindow, { type: 'homelander:ready' })
    await settle()
    expect(dataCalls(spy)).toHaveLength(0)
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
