// Routes Web Push notification clicks (and cold-start deep links) into the SPA.
//
// Why a registry: Settings is store-navigated, not URL-routed, and feature code
// (e.g. MCP approvals) owns how its own section opens. The worker only knows a
// URL, so features register a handler per `section` query value.

export type PushDeepLink = {
  section: string
  params: URLSearchParams
}

type PushDeepLinkHandler = (link: PushDeepLink) => void

const handlers = new Map<string, PushDeepLinkHandler>()

export function registerPushDeepLinkHandler(
  section: string,
  handler: PushDeepLinkHandler
): () => void {
  handlers.set(section, handler)
  return () => {
    if (handlers.get(section) === handler) {
      handlers.delete(section)
    }
  }
}

/** Parses "/?section=x&..." — returns null for other origins or no `section`. */
export function parsePushDeepLink(raw: unknown, origin: string): PushDeepLink | null {
  if (typeof raw !== 'string' || raw.length === 0 || raw.length > 2048) {
    return null
  }
  let url: URL
  try {
    url = new URL(raw, origin)
  } catch {
    return null
  }
  // Why: the message comes from our worker, but never trust a navigation target
  // that could point off-origin.
  if (url.origin !== origin) {
    return null
  }
  const section = url.searchParams.get('section')
  if (!section) {
    return null
  }
  return { section, params: url.searchParams }
}

export function dispatchPushDeepLink(link: PushDeepLink): boolean {
  const handler = handlers.get(link.section)
  if (!handler) {
    return false
  }
  handler(link)
  return true
}

/**
 * Installs the worker-message listener and consumes a cold-start deep link
 * (opened via clients.openWindow). Returns a cleanup function.
 */
export function installPushDeepLinkHandling(): () => void {
  const origin = window.location.origin

  const onMessage = (event: MessageEvent): void => {
    const data = event.data as { type?: unknown; url?: unknown } | null
    if (!data || data.type !== 'orca:navigate') {
      return
    }
    const link = parsePushDeepLink(data.url, origin)
    if (link) {
      dispatchPushDeepLink(link)
    }
  }
  navigator.serviceWorker?.addEventListener('message', onMessage)

  const cold = parsePushDeepLink(window.location.pathname + window.location.search, origin)
  if (cold) {
    // Why deferred: feature handlers register during app mount, after bootstrap.
    window.setTimeout(() => {
      if (dispatchPushDeepLink(cold)) {
        window.history.replaceState(null, '', window.location.pathname)
      }
    }, 0)
  }

  return () => navigator.serviceWorker?.removeEventListener('message', onMessage)
}
