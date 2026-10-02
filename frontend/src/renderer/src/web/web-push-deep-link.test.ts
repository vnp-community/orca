import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  dispatchPushDeepLink,
  parsePushDeepLink,
  registerPushDeepLinkHandler
} from './web-push-deep-link'

const ORIGIN = 'https://orca.example.com'

describe('parsePushDeepLink', () => {
  it('parses a same-origin section link', () => {
    const link = parsePushDeepLink('/?section=mcp&tab=approvals&approval=a1', ORIGIN)
    expect(link?.section).toBe('mcp')
    expect(link?.params.get('approval')).toBe('a1')
  })

  it.each([
    ['other origin', 'https://evil.example/?section=mcp'],
    ['protocol-relative', '//evil.example/?section=mcp'],
    ['no section', '/?tab=approvals'],
    ['not a string', 42],
    ['empty', '']
  ])('rejects %s', (_name, raw) => {
    expect(parsePushDeepLink(raw, ORIGIN)).toBeNull()
  })
})

describe('dispatchPushDeepLink', () => {
  const cleanups: (() => void)[] = []
  afterEach(() => {
    cleanups.splice(0).forEach((c) => c())
  })

  it('calls the registered handler for the section', () => {
    const handler = vi.fn()
    cleanups.push(registerPushDeepLinkHandler('mcp', handler))
    const link = parsePushDeepLink('/?section=mcp&approval=a1', ORIGIN)!
    expect(dispatchPushDeepLink(link)).toBe(true)
    expect(handler).toHaveBeenCalledWith(link)
  })

  it('returns false when nobody handles the section', () => {
    const link = parsePushDeepLink('/?section=unknown', ORIGIN)!
    expect(dispatchPushDeepLink(link)).toBe(false)
  })

  it('unregister removes only its own handler', () => {
    const first = vi.fn()
    const second = vi.fn()
    const offFirst = registerPushDeepLinkHandler('mcp', first)
    cleanups.push(registerPushDeepLinkHandler('mcp', second))
    offFirst()
    dispatchPushDeepLink(parsePushDeepLink('/?section=mcp', ORIGIN)!)
    expect(first).not.toHaveBeenCalled()
    expect(second).toHaveBeenCalled()
  })
})
