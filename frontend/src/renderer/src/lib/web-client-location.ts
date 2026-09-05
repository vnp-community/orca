export function isWebClientLocation(): boolean {
  if (typeof window === 'undefined') {
    return false
  }
  if ((window as unknown as { __ORCA_WEB_CLIENT__?: boolean }).__ORCA_WEB_CLIENT__) {
    return true
  }
  // Why: some test/non-browser environments define a partial `window`
  // (e.g. a Node global aliased as window by an unrelated polyfill) with no
  // `location` at all — window.location.pathname would throw there. A
  // missing location safely means "not a web-index.html location."
  return Boolean(window.location?.pathname?.endsWith('/web-index.html'))
}
