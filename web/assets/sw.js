// Mi Ritmo service worker — PWA install + app shell cache.
//
// Safety rules:
//  - NEVER intercept /api/* or /auth/* (stream redirects, JWTs, range
//    requests). The audio element must always hit the network.
//  - JavaScript uses network-first; other static assets use
//    stale-while-revalidate.
//  - On activate, stale caches from previous versions are removed.

const CACHE_NAME = 'pm-shell-v19'
const SHELL = ['./', './manifest.webmanifest', './icon-192.png?v=1.18.24', './icon-512.png?v=1.18.24', './apple-touch-icon.png?v=1.18.24', './favicon-16.png?v=1.18.24', './favicon-32.png?v=1.18.24', './notification-badge.svg?v=1.18.24']

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

  // App shell: network-first for navigations (always fresh), stale-while-
  // revalidate for static assets.
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

  // Imports do not inherit the entry point's version query. Prefer the network
  // for JavaScript to avoid serving an outdated module alongside a newer app.
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

// Push messages contain only administrator-provided notification text and URL.
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
  event.waitUntil(self.registration.showNotification(title, { body, icon: '/icon-192.png?v=1.18.24', badge: '/notification-badge.svg?v=1.18.24', data: { url: target } }))
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const target = event.notification.data?.url || '/'
  event.waitUntil(clients.openWindow(target))
})
