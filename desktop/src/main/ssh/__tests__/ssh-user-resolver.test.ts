/**
 * SSH User Resolver Tests
 */

import { describe, it, expect } from 'vitest'
import { toLinuxUsername, isValidLinuxUsername, resolveUserSshTarget } from '../ssh-user-resolver'
import type { SshTarget } from '../../../shared/ssh-types'

const baseTarget: SshTarget = {
  id:       'target-1',
  label:    'Dev Server',
  host:     '172.20.2.31',
  port:     22,
  username: 'ubuntu'
}

describe('toLinuxUsername', () => {
  it('converts email local part to safe linux username with orca- prefix', () => {
    expect(toLinuxUsername('alice@company.com')).toBe('orca-alice')
  })

  it('replaces dots and special chars with hyphens', () => {
    expect(toLinuxUsername('alice.smith@co.com')).toBe('orca-alice-smith')
  })

  it('collapses consecutive hyphens', () => {
    const result = toLinuxUsername('alice--double@test.com')
    expect(result).not.toContain('--')
  })

  it('handles names with numbers', () => {
    expect(toLinuxUsername('user123@test.com')).toBe('orca-user123')
  })

  it('truncates to max 25 chars total (5 prefix + 20 local)', () => {
    const result = toLinuxUsername('averylongemaillocalpartthatexceedslimit@test.com')
    expect(result.length).toBeLessThanOrEqual(25)
  })

  it('produces valid linux username', () => {
    const result = toLinuxUsername('test-user@example.com')
    expect(isValidLinuxUsername(result)).toBe(true)
  })

  it('handles email with leading/trailing hyphens in local part', () => {
    const result = toLinuxUsername('.user.@example.com')
    expect(result).not.toMatch(/orca--/)  // no double hyphen
    expect(isValidLinuxUsername(result)).toBe(true)
  })
})

describe('toLinuxUsername — with userId (collision avoidance)', () => {
  it('generates different usernames for similar emails with different userIds', () => {
    const a = toLinuxUsername('alice.smith@a.com', 'uid-aaa')
    const b = toLinuxUsername('alice-smith@b.com', 'uid-bbb')
    expect(a).not.toBe(b)
  })

  it('returns same result for same email + userId (deterministic)', () => {
    const r1 = toLinuxUsername('same@test.com', 'uid-123')
    const r2 = toLinuxUsername('same@test.com', 'uid-123')
    expect(r1).toBe(r2)
  })

  it('includes 4-char hash suffix', () => {
    const result = toLinuxUsername('bob@test.com', 'uid-xyz')
    // Format: orca-{local}-{4char}
    const withoutPrefix = result.replace(/^orca-/, '')
    const parts         = withoutPrefix.split('-')
    const suffix        = parts[parts.length - 1]!
    expect(suffix).toHaveLength(4)
  })

  it('total length does not exceed 25 chars with suffix', () => {
    const result = toLinuxUsername('averylongemail@example.com', 'uid-long-user-id')
    expect(result.length).toBeLessThanOrEqual(25)
  })
})

describe('isValidLinuxUsername', () => {
  it('accepts valid usernames', () => {
    expect(isValidLinuxUsername('orca-alice')).toBe(true)
    expect(isValidLinuxUsername('orca-user123')).toBe(true)
    expect(isValidLinuxUsername('a')).toBe(true)
  })

  it('rejects usernames starting with numbers', () => {
    expect(isValidLinuxUsername('1alice')).toBe(false)
  })

  it('rejects usernames over 32 chars', () => {
    expect(isValidLinuxUsername('a'.repeat(33))).toBe(false)
  })

  it('rejects usernames with spaces', () => {
    expect(isValidLinuxUsername('orca alice')).toBe(false)
  })

  it('rejects usernames with uppercase', () => {
    expect(isValidLinuxUsername('Orca-alice')).toBe(false)
  })

  it('accepts exactly 32-char username', () => {
    expect(isValidLinuxUsername('a' + 'b'.repeat(31))).toBe(true)
  })
})

describe('resolveUserSshTarget', () => {
  it('overrides username with per-user linux username (includes suffix)', () => {
    const resolved = resolveUserSshTarget(baseTarget, 'uid-1', 'alice@test.com')
    // When userId is provided, username includes 4-char hash suffix
    expect(resolved.username).toMatch(/^orca-alice-[a-f0-9]{4}$/)
    expect(resolved.username).not.toBe('ubuntu')
  })

  it('preserves all other SshTarget fields', () => {
    const resolved = resolveUserSshTarget(baseTarget, 'uid-1', 'alice@test.com')
    expect(resolved.id).toBe(baseTarget.id)
    expect(resolved.label).toBe(baseTarget.label)
    expect(resolved.host).toBe(baseTarget.host)
    expect(resolved.port).toBe(baseTarget.port)
  })

  it('does not mutate the original target', () => {
    resolveUserSshTarget(baseTarget, 'uid-1', 'alice@test.com')
    expect(baseTarget.username).toBe('ubuntu')  // unchanged
  })

  it('uses toLinuxUsername with userId for suffix', () => {
    const resolved = resolveUserSshTarget(baseTarget, 'uid-abc', 'bob@test.com')
    // Must differ from simple (no-userId) version since userId adds suffix
    const simple = toLinuxUsername('bob@test.com')
    expect(resolved.username).not.toBe(simple)
  })
})
