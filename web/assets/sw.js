// The service worker never intercepts /api/* or /auth/*: stream redirects,
// JWTs and range requests must always reach the network.

// The cache name carries the build version, so a new release starts a fresh
// shell cache and activate drops the previous one. No manual bump needed.
const CACHE_NAME = 'pm-shell-__ASSET_VERSION__'
const SHELL = ['./', './manifest.webmanifest', './icon-192.png?v=__ASSET_VERSION__', './icon-512.png?v=__ASSET_VERSION__', './apple-touch-icon.png?v=__ASSET_VERSION__', './favicon-16.png?v=__ASSET_VERSION__', './favicon-32.png?v=__ASSET_VERSION__', './notification-badge.svg?v=__ASSET_VERSION__']

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(SHELL)).catch(() => undefined),
  )
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k))),
    ),
  )
  self.clients.claim()
})

self.addEventListener('fetch', (event) => {
  const req = event.request
  if (req.method !== 'GET') return
  const url = new URL(req.url)
  if (url.origin !== self.location.origin) return
  const path = url.pathname
  if (path.startsWith('/api/') || path.startsWith('/auth/') || path === '/docs' || path.startsWith('/docs/')) return

  if (req.mode === 'navigate') {
    event.respondWith(
      fetch(req)
        .then((res) => {
          const copy = res.clone()
          caches.open(CACHE_NAME).then((c) => c.put(url.href, copy)).catch(() => undefined)
          return res
        })
        .catch(() => caches.match(url.href)),
    )
    return
  }

  // Module imports do not inherit the entry point's version query, so
  // JavaScript goes network-first to avoid a stale module next to a new app.
  if (path.endsWith('.js')) {
    event.respondWith(
      fetch(req).then((res) => {
        if (res.ok) {
          const copy = res.clone()
          return caches.open(CACHE_NAME).then((c) => c.put(req, copy)).catch(() => undefined).then(() => res)
        }
        return res
      }).catch(() => caches.match(req)),
    )
    return
  }

  event.respondWith(
    caches.match(req).then((cached) => {
      const network = fetch(req)
        .then((res) => {
          if (res.ok) {
            const copy = res.clone()
            caches.open(CACHE_NAME).then((c) => c.put(req, copy)).catch(() => undefined)
          }
          return res
        })
        .catch(() => undefined)
      return cached || network
    }),
  )
})

self.addEventListener('push', (event) => {
  let data = {}
  try { data = event.data ? event.data.json() : {} } catch { /* malformed payload */ }
  const title = typeof data.title === 'string' && data.title ? data.title : 'Mi Ritmo'
  const body = typeof data.body === 'string' ? data.body : ''
  let target = '/'
  try {
    const url = new URL(data.url || '/', self.location.origin)
    if ((url.protocol === 'https:' || url.origin === self.location.origin) && !url.username && !url.password) target = url.href
  } catch { /* open home */ }
  event.waitUntil(self.registration.showNotification(title, { body, icon: '/icon-192.png?v=__ASSET_VERSION__', badge: '/notification-badge.svg?v=__ASSET_VERSION__', data: { url: target } }))
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const target = event.notification.data?.url || '/'
  event.waitUntil(clients.openWindow(target))
})
