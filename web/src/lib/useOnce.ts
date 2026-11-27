import { useRef } from 'react'

/**
 * Wraps an async action so a second call while the first is running is
 * ignored (a double click sends one request, even before React re-renders
 * the disabled button).
 */
export function useOnce<A extends unknown[]>(fn: (...args: A) => Promise<unknown>): (...args: A) => void {
  const running = useRef(false)
  return (...args: A) => {
    if (running.current) return
    running.current = true
    fn(...args)
      .catch(() => {})
      .finally(() => {
        running.current = false
      })
  }
}
