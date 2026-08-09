import { describe, it, expect, afterEach } from 'vitest'
import {
  wrapDeviceToken,
  unwrapDeviceToken,
  __resetSessionXorKeyForTests
} from './web-runtime-environment-crypto'

describe('web-runtime-environment-crypto', () => {
  afterEach(() => __resetSessionXorKeyForTests())

  it('round-trips a token through wrap/unwrap', () => {
    const wrapped = wrapDeviceToken('secret-token-value')
    expect(unwrapDeviceToken(wrapped)).toBe('secret-token-value')
  })

  it('does not store the plaintext token in the wrapped output', () => {
    const wrapped = wrapDeviceToken('super-secret-plaintext')
    expect(wrapped).not.toContain('super-secret-plaintext')
  })

  it('returns null when the session key is gone (simulated reload)', () => {
    const wrapped = wrapDeviceToken('secret-token-value')
    __resetSessionXorKeyForTests()
    expect(unwrapDeviceToken(wrapped)).toBeNull()
  })

  it('reuses the same session key across multiple wrap calls', () => {
    const wrappedA = wrapDeviceToken('token-a')
    const wrappedB = wrapDeviceToken('token-b')
    expect(unwrapDeviceToken(wrappedA)).toBe('token-a')
    expect(unwrapDeviceToken(wrappedB)).toBe('token-b')
  })
})
