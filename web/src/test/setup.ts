import '@testing-library/jest-dom/vitest'
import { cleanup, configure } from '@testing-library/react'
import { afterEach, vi } from 'vitest'

// Whole-app renders can take over a second on a loaded CI machine.
configure({ asyncUtilTimeout: 5000 })

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  localStorage.clear()
})

// Mantine reads these browser APIs that jsdom lacks.
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }),
})
window.HTMLElement.prototype.scrollIntoView = () => {}
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
window.ResizeObserver = ResizeObserverStub as unknown as typeof ResizeObserver
if (!('fonts' in document)) {
  Object.defineProperty(document, 'fonts', { value: { addEventListener: () => {}, removeEventListener: () => {}, ready: Promise.resolve() } })
}
