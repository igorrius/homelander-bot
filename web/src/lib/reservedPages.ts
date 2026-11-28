/**
 * Page names a published site may not use: they would take over an app or server route.
 * Keep in sync with ReservedPageNames in internal/domain/page.go (a test checks both).
 */
export const reservedPages: readonly string[] = [
  'admin', 'login', 'register', 'oauth', 'site', 'api', 'mcp', 'healthz', 'my',
  'assets', 'telegram', 'favicon', 'robots', 'sitemap',
]
