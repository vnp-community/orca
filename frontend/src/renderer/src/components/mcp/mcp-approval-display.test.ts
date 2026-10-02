import { describe, expect, it } from 'vitest'
import { APPROVE_LOCK_MS, formatCountdown, visualizeControlChars } from './mcp-approval-display'

describe('visualizeControlChars', () => {
  it('shows bidi/zero-width/control chars, keeps newline/tab and markup as-is', () => {
    expect(visualizeControlChars('a‮b')).toBe('a\\u{202E}b')
    expect(visualizeControlChars('x​y\u0007')).toBe('x\\u{200B}y\\u{0007}')
    expect(visualizeControlChars('l1\n\tl2 <img src=x onerror=1> [REDACTED]')).toBe(
      'l1\n\tl2 <img src=x onerror=1> [REDACTED]'
    )
  })
})

describe('approval display constants', () => {
  it('locks riskier approvals longer', () => {
    expect(APPROVE_LOCK_MS.read).toBe(0)
    expect(APPROVE_LOCK_MS.exec).toBe(1500)
    expect(APPROVE_LOCK_MS.destructive).toBe(2000)
  })
  it('formats countdowns', () => {
    expect(formatCountdown(272_000)).toBe('4:32')
    expect(formatCountdown(-5)).toBe('0:00')
  })
})
