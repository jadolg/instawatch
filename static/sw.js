const CACHE_NAME = 'instawatch-v1';
const OFFLINE_URL = '/';

// Assets to pre-cache for a shell offline experience
const PRECACHE_ASSETS = [
  '/',
  '/static/css/style.css',
  '/static/js/app.js',
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(PRECACHE_ASSETS))
  );
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k)))
    )
  );
  self.clients.claim();
});

self.addEventListener('fetch', (event) => {
  // Only handle GET requests; skip cross-origin, video/media requests
  if (event.request.method !== 'GET') return;
  const url = new URL(event.request.url);
  if (url.origin !== self.location.origin) return;
  // Skip video files — too large to cache
  if (url.pathname.startsWith('/video/')) return;

  event.respondWith(
    fetch(event.request)
      .then((response) => {
        // Cache only successful responses for pre-cached assets
        if (response.ok && PRECACHE_ASSETS.includes(url.pathname)) {
          const clone = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone));
        }
        return response;
      })
      .catch(() => caches.match(event.request).then((cached) => cached || caches.match(OFFLINE_URL)))
  );
});
