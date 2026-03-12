// Thriftly Service Worker — app shell caching for offline support and fast loads
const CACHE_NAME = 'thriftly-v3'

// App shell: cache on install
const APP_SHELL = [
  '/',
  '/manifest.json',
  '/penny.png?v=2',
  '/apple-touch-icon.png',
  '/favicon-32x32.png',
]

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(APP_SHELL))
  )
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  // Clean up old caches
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k)))
    )
  )
  self.clients.claim()
})

self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url)

  // Never cache API calls — always go to network
  if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/auth/') || url.pathname.startsWith('/brand/')) {
    return
  }

  // For hashed static assets (Vite build output), cache-first (immutable)
  if (url.pathname.startsWith('/assets/') && url.pathname.match(/\.[a-f0-9]{8}\./)) {
    event.respondWith(
      caches.match(event.request).then((cached) => {
        if (cached) return cached
        return fetch(event.request).then((response) => {
          if (response.ok) {
            const clone = response.clone()
            caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone))
          }
          return response
        })
      })
    )
    return
  }

  // For navigation requests (HTML pages), network-first with offline fallback
  if (event.request.mode === 'navigate') {
    event.respondWith(
      fetch(event.request)
        .then((response) => {
          const clone = response.clone()
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone))
          return response
        })
        .catch(() => caches.match('/'))
    )
    return
  }

  // For other static files (images, fonts), stale-while-revalidate
  event.respondWith(
    caches.match(event.request).then((cached) => {
      const fetched = fetch(event.request).then((response) => {
        if (response.ok) {
          const clone = response.clone()
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone))
        }
        return response
      }).catch(() => cached)

      return cached || fetched
    })
  )
})
