// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from 'vitest'
import {
  clearReturnTo,
  isOAuthConsentPath,
  loginUrlFor,
  sanitizeReturnTo,
  stashReturnToFromLocation,
  takeForwardTarget
} from './oauth-return-to'

const AUTH = '/oauth/authorize?client_id=a&state=b'
const loc = (pathname: string, search = ''): { pathname: string; search: string } => ({
  pathname,
  search
})

beforeEach(() => window.sessionStorage.clear())

describe('sanitizeReturnTo', () => {
  it.each([
    ['//evil.com', null],
    ['/\\evil.com', null],
    ['javascript:alert(1)', null],
    ['https://evil.com/oauth/authorize?x=1', null],
    ['/oauth/authorize', null],
    ['/other?x=1', null],
    [`/oauth/authorize?x=${'a'.repeat(5000)}`, null],
    ['/oauth/authorize?x=1\n', null],
    ['/oauth/authorize?x=\u0000', null],
    ['/oauth/consent?request_id=short', null],
    [AUTH, AUTH],
    [
      '/oauth/consent?request_id=0b2f6c1e-1111-4222-8333-444455556666',
      '/oauth/consent?request_id=0b2f6c1e-1111-4222-8333-444455556666'
    ]
  ])('%s', (input, expected) => {
    expect(sanitizeReturnTo(input)).toBe(expected)
  })
  it('rejects empty values', () => {
    expect(sanitizeReturnTo(null)).toBeNull()
    expect(sanitizeReturnTo('')).toBeNull()
  })
})

describe('forwarding', () => {
  it('stashes a valid return_to and forwards after login', () => {
    stashReturnToFromLocation(loc('/login', `?return_to=${encodeURIComponent(AUTH)}`))
    expect(takeForwardTarget(loc('/'))).toBe(AUTH)
  })
  it('ignores invalid return_to', () => {
    stashReturnToFromLocation(loc('/login', '?return_to=%2F%2Fevil.com'))
    expect(takeForwardTarget(loc('/'))).toBeNull()
  })
  it('uses return_to straight from the query when already signed in', () => {
    expect(takeForwardTarget(loc('/login', `?return_to=${encodeURIComponent(AUTH)}`))).toBe(AUTH)
  })
  it('stashes an expired consent URL itself', () => {
    const id = '0b2f6c1e-1111-4222-8333-444455556666'
    stashReturnToFromLocation(loc('/oauth/consent', `?request_id=${id}`))
    expect(takeForwardTarget(loc('/'))).toBe(`/oauth/consent?request_id=${id}`)
  })
  it('stops after MAX_ATTEMPTS to avoid loops', () => {
    stashReturnToFromLocation(loc('/login', `?return_to=${encodeURIComponent(AUTH)}`))
    expect(takeForwardTarget(loc('/'))).toBe(AUTH)
    expect(takeForwardTarget(loc('/'))).toBe(AUTH)
    expect(takeForwardTarget(loc('/'))).toBeNull()
    expect(takeForwardTarget(loc('/'))).toBeNull()
  })
  it('clears the stash on the consent page', () => {
    stashReturnToFromLocation(loc('/login', `?return_to=${encodeURIComponent(AUTH)}`))
    expect(takeForwardTarget(loc('/oauth/consent', '?request_id=abcdefgh'))).toBeNull()
    expect(takeForwardTarget(loc('/'))).toBeNull()
  })
  it('expires stale stashes', () => {
    window.sessionStorage.setItem(
      'orca.oauth.returnTo.v1',
      JSON.stringify({ target: AUTH, at: Date.now() - 11 * 60 * 1000 })
    )
    expect(takeForwardTarget(loc('/'))).toBeNull()
  })
  it('clearReturnTo wipes state and storage failures are swallowed', () => {
    stashReturnToFromLocation(loc('/login', `?return_to=${encodeURIComponent(AUTH)}`))
    clearReturnTo()
    expect(takeForwardTarget(loc('/'))).toBeNull()
  })
})

describe('helpers', () => {
  it('detects the consent path only', () => {
    expect(isOAuthConsentPath({ pathname: '/oauth/consent' })).toBe(true)
    expect(isOAuthConsentPath({ pathname: '/oauth/consent/x' })).toBe(false)
  })
  it('builds the login url', () => {
    const id = '0b2f6c1e-1111-4222-8333-444455556666'
    expect(loginUrlFor(loc('/oauth/consent', `?request_id=${id}`))).toBe(
      `/login?return_to=${encodeURIComponent(`/oauth/consent?request_id=${id}`)}`
    )
    expect(loginUrlFor(loc('/foo'))).toBe('/login')
    expect(loginUrlFor({})).toBe('/login')
  })
})
