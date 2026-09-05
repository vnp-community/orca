import { afterEach, describe, expect, it, vi } from 'vitest'
import { isWebClientLocation } from './web-client-location'

describe('isWebClientLocation', () => {
  const originalWindow = globalThis.window

  afterEach(() => {
    if (originalWindow === undefined) {
      delete (globalThis as { window?: unknown }).window
    } else {
      globalThis.window = originalWindow
    }
  })

  it('returns false when window is undefined (non-browser environment)', () => {
    delete (globalThis as { window?: unknown }).window
    expect(isWebClientLocation()).toBe(false)
  })

  it('returns true when window.__ORCA_WEB_CLIENT__ is set, even with no location', () => {
    vi.stubGlobal('window', { __ORCA_WEB_CLIENT__: true })
    expect(isWebClientLocation()).toBe(true)
    vi.unstubAllGlobals()
  })

  it('does not throw when window exists but has no location — treats it as not web-index.html', () => {
    // Why: some test/non-browser environments define a partial `window`
    // with no `location` at all (e.g. a Node global aliased as window by an
    // unrelated polyfill) — window.location.pathname used to throw there,
    // silently breaking any caller that didn't expect isWebClientLocation
    // to throw.
    vi.stubGlobal('window', {})
    expect(() => isWebClientLocation()).not.toThrow()
    expect(isWebClientLocation()).toBe(false)
    vi.unstubAllGlobals()
  })

  it('returns true when window.location.pathname ends with /web-index.html', () => {
    vi.stubGlobal('window', { location: { pathname: '/some/path/web-index.html' } })
    expect(isWebClientLocation()).toBe(true)
    vi.unstubAllGlobals()
  })

  it('returns false for a non-web-index.html pathname with no web-client flag', () => {
    vi.stubGlobal('window', { location: { pathname: '/index.html' } })
    expect(isWebClientLocation()).toBe(false)
    vi.unstubAllGlobals()
  })
})
