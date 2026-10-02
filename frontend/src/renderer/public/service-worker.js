/* Orca web service worker — Web Push display + notification click routing.
 *
 * Plain JS on purpose: it is copied verbatim from Vite's publicDir to
 * /service-worker.js (registered in src/web/main-web-bootstrap.tsx) and never
 * goes through the app bundler.
 *
 * Push payload (JSON, produced by notification-service's DeliverPush):
 *   { title: string, body?: string, deepLink?: string, tag?: string }
 * deepLink must be a same-origin path such as "/?section=mcp&tab=approvals&approval=<id>".
 */

self.addEventListener('install', () => {
  // Why: a fixed push/click handler must replace the old one immediately,
  // otherwise users keep a stale worker until every tab is closed.
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  event.waitUntil(self.clients.claim())
})

/** Accept only same-origin relative paths; anything else falls back to "/". */
function safeDeepLink(raw) {
  if (typeof raw !== 'string' || raw.length === 0 || raw.length > 2048) {
    return '/'
  }
  // "//host" and "/\host" are protocol-relative / backslash tricks that browsers
  // resolve to another origin.
  if (!raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/\\')) {
    return '/'
  }
  return raw
}

self.addEventListener('push', (event) => {
  let data = {}
  try {
    data = event.data ? event.data.json() : {}
  } catch {
    data = { title: 'Orca', body: event.data ? event.data.text() : '' }
  }
  const title = typeof data.title === 'string' && data.title ? data.title : 'Orca'
  const options = {
    body: typeof data.body === 'string' ? data.body : '',
    tag: typeof data.tag === 'string' ? data.tag : undefined,
    // Why: carried to notificationclick, which has no access to the push event.
    data: { deepLink: safeDeepLink(data.deepLink) }
  }
  event.waitUntil(self.registration.showNotification(title, options))
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const deepLink = safeDeepLink(event.notification.data && event.notification.data.deepLink)

  event.waitUntil(
    (async () => {
      const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
      const existing = windows.find((c) => new URL(c.url).origin === self.location.origin)
      if (existing) {
        // Why: reuse the open app (keeps terminals/WS alive) and let the SPA
        // route in-process instead of reloading it.
        await existing.focus()
        existing.postMessage({ type: 'orca:navigate', url: deepLink })
        return
      }
      await self.clients.openWindow(deepLink)
    })()
  )
})
