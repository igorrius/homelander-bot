/** Error returned by the API as {"error":{"code":...}}. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  /** Field name → reason code; only for code "validation". */
  readonly fields: Record<string, string>

  constructor(status: number, code: string, fields: Record<string, string> = {}) {
    super(`API error ${status}: ${code}`)
    this.status = status
    this.code = code
    this.fields = fields
  }
}

/**
 * Calls the JSON API under /api/v1. Sends the CSRF backstop header and the
 * session cookie; throws ApiError on a non-2xx answer.
 */
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('X-Requested-With', 'homelander')
  headers.set('Accept', 'application/json')
  if (init.body !== undefined && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const res = await fetch(`/api/v1${path}`, { ...init, headers, credentials: 'same-origin' })
  const body: unknown = res.status === 204 ? null : await res.json().catch(() => null)
  if (!res.ok) {
    const error = (body as { error?: { code?: string; fields?: Record<string, string> } } | null)?.error
    throw new ApiError(res.status, error?.code ?? 'unknown', error?.fields ?? {})
  }
  return body as T
}
