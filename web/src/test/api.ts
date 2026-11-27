import { vi } from 'vitest'

export type MockResponse = { status?: number; body?: unknown; delay?: number }
export type MockRequest = { method: string; path: string; query: URLSearchParams; body: unknown }
export type Route = MockResponse | ((req: MockRequest) => MockResponse)

/**
 * Replaces fetch with a route table keyed by "METHOD /path" (without /api/v1
 * and the query string). Unlisted routes answer 404 not_found. Returns the
 * recorded calls in order.
 */
export function mockApi(routes: Record<string, Route>): { calls: MockRequest[] } {
  const calls: MockRequest[] = []
  vi.stubGlobal('fetch', async (input: RequestInfo | URL, init: RequestInit = {}) => {
    const url = new URL(String(input), 'http://localhost')
    const path = url.pathname.replace(/^\/api\/v1/, '')
    const method = (init.method ?? 'GET').toUpperCase()
    let body: unknown = init.body ?? null
    if (typeof body === 'string') {
      try {
        body = JSON.parse(body)
      } catch {
        // keep the raw string
      }
    }
    const req = { method, path, query: url.searchParams, body }
    calls.push(req)
    const route = routes[`${method} ${path}`]
    const res: MockResponse =
      route === undefined
        ? { status: 404, body: { error: { code: 'not_found' } } }
        : typeof route === 'function'
          ? route(req)
          : route
    if (res.delay) await new Promise((r) => setTimeout(r, res.delay))
    const status = res.status ?? 200
    return new Response(status === 204 || res.body === undefined ? null : JSON.stringify(res.body), {
      status,
      headers: { 'Content-Type': 'application/json' },
    })
  })
  return { calls }
}

/** Calls of one route, for assertions. */
export function callsTo(calls: MockRequest[], key: string): MockRequest[] {
  return calls.filter((c) => `${c.method} ${c.path}` === key)
}
