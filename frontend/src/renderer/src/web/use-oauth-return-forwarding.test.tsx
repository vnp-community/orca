// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, renderHook } from '@testing-library/react'
import type { AuthUser } from '../auth/auth-types'
import { useOAuthReturnForwarding } from './use-oauth-return-forwarding'

const AUTH = '/oauth/authorize?client_id=a&state=b'
const user = { id: 'u' } as AuthUser
const replace = vi.fn()

function setLocation(pathname: string, search = ''): void {
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { pathname, search, replace }
  })
}

beforeEach(() => {
  replace.mockReset()
  window.sessionStorage.clear()
})
afterEach(cleanup)

describe('useOAuthReturnForwarding', () => {
  it('stashes return_to while signed out and forwards after sign-in without mounting App', () => {
    setLocation('/login', `?return_to=${encodeURIComponent(AUTH)}`)
    const signedOut = renderHook(() => useOAuthReturnForwarding(null))
    expect(signedOut.result.current).toBeNull()
    expect(replace).not.toHaveBeenCalled()
    signedOut.unmount()

    setLocation('/')
    const signedIn = renderHook(() => useOAuthReturnForwarding(user))
    expect(signedIn.result.current).toBe(AUTH)
    expect(replace).toHaveBeenCalledWith(AUTH)
  })

  it('forwards a valid return_to straight from the query when already signed in', () => {
    setLocation('/login', `?return_to=${encodeURIComponent(AUTH)}`)
    const { result } = renderHook(() => useOAuthReturnForwarding(user))
    expect(result.current).toBe(AUTH)
    expect(replace).toHaveBeenCalledWith(AUTH)
  })

  it('ignores hostile return_to values', () => {
    setLocation('/login', '?return_to=%2F%2Fevil.com')
    const { result } = renderHook(() => useOAuthReturnForwarding(user))
    expect(result.current).toBeNull()
    expect(replace).not.toHaveBeenCalled()
  })

  it('does not forward from the consent page itself', () => {
    setLocation('/oauth/consent', '?request_id=0b2f6c1e-1111-4222-8333-444455556666')
    const { result } = renderHook(() => useOAuthReturnForwarding(user))
    expect(result.current).toBeNull()
    expect(replace).not.toHaveBeenCalled()
  })
})
