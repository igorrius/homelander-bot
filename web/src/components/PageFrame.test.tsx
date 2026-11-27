import { act, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
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

describe('PageFrame', () => {
  it('is sandboxed without same-origin and carries the language', () => {
    render(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    expect(PAGE_SANDBOX).toBe(SANDBOX)
    expect(frame().getAttribute('sandbox')).toBe(SANDBOX)
    expect(frame().getAttribute('sandbox')).not.toContain('allow-same-origin')
    expect(frame().getAttribute('src')).toBe('/site/index.html?lang=uk')
    expect(frame().style.height).toBe('600px')
  })

  it('takes the height only from its own frame and clamps it', () => {
    render(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
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
    const { rerender } = render(<PageFrame src="/site/index.html" lang="uk" title="Page" />)
    const spy = vi.spyOn(frame().contentWindow as Window, 'postMessage')
    rerender(<PageFrame src="/site/index.html" lang="en" title="Page" />)
    expect(spy).toHaveBeenCalledWith({ type: 'homelander:lang', lang: 'en' }, '*')
    expect(frame().getAttribute('src')).toBe('/site/index.html?lang=uk')
  })
})

describe('PageFrame navigation', () => {
  it('passes on navigation only from its own frame and only to listed pages', () => {
    const onNavigate = vi.fn()
    render(<PageFrame src="/site/index.html" lang="uk" title="Page" pages={['/', '/contacts']} onNavigate={onNavigate} />)
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
