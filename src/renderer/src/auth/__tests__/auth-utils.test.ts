import { describe, expect, it } from 'vitest'
import { toLinuxUsername } from '../auth-utils'

describe('toLinuxUsername', () => {
  it('toLinuxUsername("alice@company.com") === "orca-alice"', () => {
    expect(toLinuxUsername('alice@company.com')).toBe('orca-alice')
  })

  it('Replaces dots với dashes: "alice.smith@co.com" → "orca-alice-smith"', () => {
    expect(toLinuxUsername('alice.smith@co.com')).toBe('orca-alice-smith')
  })

  it('Truncates local part tại 20 chars', () => {
    expect(toLinuxUsername('verylongemailname12345@x.co')).toBe('orca-verylongemailname123')
  })

  it('Special chars (+ filter) → only alphanumeric + dash', () => {
    expect(toLinuxUsername('alice+filter@co.com')).toBe('orca-alice-filter')
    expect(toLinuxUsername('alice!#@co.com')).toBe('orca-alice')
  })
})
