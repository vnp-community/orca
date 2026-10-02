const LOOPBACK_HOSTS = new Set(['localhost', '127.0.0.1', '[::1]'])

/**
 * Why: the consent redirectUrl comes from the server but ends in location.assign; only https or
 * loopback http (native MCP clients, RFC 8252) is allowed, never javascript:/data:.
 */
export function validateConsentRedirect(raw: unknown): string | null {
  if (typeof raw !== 'string' || raw.length === 0 || raw.length > 8192) {
    return null
  }
  let url: URL
  try {
    url = new URL(raw)
  } catch {
    return null
  }
  if (url.username || url.password) {
    return null
  }
  if (url.protocol === 'https:') {
    return url.href
  }
  if (url.protocol === 'http:' && LOOPBACK_HOSTS.has(url.hostname)) {
    return url.href
  }
  return null
}
