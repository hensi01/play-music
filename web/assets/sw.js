// Play Music service worker — PWA install + app shell cache.
//
// Safety rules:
//  - NEVER intercept /api/* or /auth/* (stream redirects, JWTs, range
//    requests). The audio element must always hit the network.
//  - Only same-origin GET static assets are cached, with
//    stale-while-revalidate, so a redeploy (new ?v= URLs) always fetches
//    the current build.
//  - On activate, stale caches from previous versions are removed.

const CACHE_NAME = 'pm-shell-v2'
const SHELL = ['./', './manifest.webmanifest', './icon-192.png', './icon-512.png', './apple-touch-icon.png']

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
  if (path.startsWith('/api/') || path.startsWith('/auth/')) return

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
