/** Full-page navigation to another site (an OAuth client's redirect URI); an object so tests can spy on it. */
export const redirect = {
  to(url: string) {
    window.location.assign(url)
  },
}

/** A same-site path to return to after login, or null for anything else (another origin, //host, garbage). */
export function safeNext(next: string | null): string | null {
  if (!next || !next.startsWith('/') || next.startsWith('//') || next.startsWith('/\\')) return null
  return next
}
