/*
 * Homelander service worker. It makes the app installable and shows an
 * offline page when a navigation fails; it caches nothing else. API answers,
 * page files, MCP and OAuth always go to the network untouched.
 *
 * Web Push (later) adds its 'push' and 'notificationclick' listeners here.
 * Bump CACHE when offline.html changes.
 */
const CACHE = 'homelander-offline-v1'
const OFFLINE_URL = '/offline.html'
const BYPASS = ['/api', '/site', '/mcp', '/oauth', '/telegram', '/healthz']

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches
      .open(CACHE)
      .then((cache) => cache.add(OFFLINE_URL))
      .then(() => self.skipWaiting()),
  )
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  )
})

/** Only same-site page loads outside the bypassed paths are ours. */
function handles(request) {
  if (request.mode !== 'navigate' || request.method !== 'GET') return false
  const url = new URL(request.url)
  if (url.origin !== self.location.origin) return false
  return !BYPASS.some((p) => url.pathname === p || url.pathname.startsWith(p + '/'))
}

self.addEventListener('fetch', (event) => {
  if (!handles(event.request)) return
  event.respondWith(fetch(event.request).catch(() => caches.match(OFFLINE_URL).then((r) => r || Response.error())))
})
