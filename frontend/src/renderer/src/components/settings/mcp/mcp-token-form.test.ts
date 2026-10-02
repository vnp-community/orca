import { describe, expect, it } from 'vitest'
import {
  defaultExpiryDays,
  expiryOptions,
  hasFormErrors,
  isScopeSelectable,
  mapCreateError,
  validateTokenForm
} from './mcp-token-form'

describe('expiryOptions', () => {
  it.each([
    [1, [1]],
    [30, [7, 30]],
    [45, [7, 30, 45]],
    [90, [7, 30, 60, 90]],
    [0, []]
  ])('maxTokenDays=%s', (max, expected) => {
    expect(expiryOptions(max)).toEqual(expected)
  })
  it('defaults to min(30, max)', () => {
    expect(defaultExpiryDays(90)).toBe(30)
    expect(defaultExpiryDays(7)).toBe(7)
  })
})

describe('validateTokenForm', () => {
  const ok = { name: 'ci', scopes: ['orca:read' as const], expiresInDays: 30 }
  it('accepts a valid form', () => {
    expect(hasFormErrors(validateTokenForm(ok, 90))).toBe(false)
  })
  it('rejects bad names', () => {
    expect(validateTokenForm({ ...ok, name: '   ' }, 90).name).toBeTruthy()
    expect(validateTokenForm({ ...ok, name: 'x'.repeat(81) }, 90).name).toBeTruthy()
    expect(validateTokenForm({ ...ok, name: 'x'.repeat(80) }, 90).name).toBeUndefined()
  })
  it('rejects empty scopes and out-of-range lifetimes', () => {
    expect(validateTokenForm({ ...ok, scopes: [] }, 90).scopes).toBeTruthy()
    expect(validateTokenForm({ ...ok, expiresInDays: 0 }, 90).expiresInDays).toBeTruthy()
    expect(validateTokenForm({ ...ok, expiresInDays: 91 }, 90).expiresInDays).toBe(
      'Maximum lifetime is 90 days.'
    )
    expect(validateTokenForm({ ...ok, expiresInDays: 1.5 }, 90).expiresInDays).toBeTruthy()
  })
})

describe('isScopeSelectable', () => {
  it('limits orca:admin to admins', () => {
    expect(isScopeSelectable('orca:admin', 'developer')).toBe(false)
    expect(isScopeSelectable('orca:admin', undefined)).toBe(false)
    expect(isScopeSelectable('orca:admin', 'admin')).toBe(true)
    expect(isScopeSelectable('orca:exec', 'developer')).toBe(true)
  })
})

describe('mapCreateError', () => {
  it('maps each code to its place', () => {
    expect(mapCreateError('MCP_TOKEN_TOO_LONG', '', 30)).toMatchObject({
      field: 'expiresInDays',
      refreshServerInfo: true,
      message: 'Maximum lifetime is 30 days.'
    })
    expect(mapCreateError('MCP_SCOPE_NOT_ALLOWED', '', 30).field).toBe('scopes')
    expect(mapCreateError('MCP_SCOPE_INVALID', '', 30).field).toBe('scopes')
    expect(mapCreateError('MCP_TOKEN_LIMIT', '', 30)).toMatchObject({ banner: true })
    expect(mapCreateError('MCP_KILL_SWITCH_ACTIVE', '', 30)).toMatchObject({
      banner: true,
      disableCreate: true
    })
    expect(mapCreateError(null, 'server said no', 30)).toMatchObject({
      banner: true,
      message: 'server said no'
    })
  })
})
