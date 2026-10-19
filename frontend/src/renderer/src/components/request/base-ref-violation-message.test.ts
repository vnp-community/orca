import { describe, expect, it } from 'vitest'
import { validateBaseRefSyntax, type BaseRefViolation } from '../../../../shared/request-base-ref'
import { baseRefViolationMessage } from './base-ref-violation-message'

// One input per reason code, so a new code added to validateBaseRefSyntax without copy fails here.
const SAMPLES: Record<BaseRefViolation, string> = {
  blank: '   ',
  too_long: 'a'.repeat(256),
  invalid_utf8: 'bad\uD800',
  leading_dash: '-x',
  bare_at: '@',
  nul_byte: 'a\u0000b',
  control_character: 'a\u0007b',
  whitespace: 'a b',
  forbidden_character: 'a~b',
  double_dot: 'a..b',
  reflog_syntax: 'a@{1}',
  double_slash: 'a//b',
  leading_slash: '/a',
  trailing_slash: 'a/',
  trailing_dot: 'a.',
  dot_component: 'a/.b',
  lock_component: 'a.lock'
}

describe('baseRefViolationMessage', () => {
  it('has a distinct sentence for each of the 17 reasons', () => {
    const messages = new Map<string, string>()
    for (const [reason, input] of Object.entries(SAMPLES)) {
      const checked = validateBaseRefSyntax(input)
      expect(checked.ok, reason).toBe(false)
      if (checked.ok) {
        continue
      }
      expect(checked.reason).toBe(reason)
      const message = baseRefViolationMessage(checked)
      expect(message.trim(), reason).not.toBe('')
      expect(message, reason).not.toMatch(/\{\{/)
      messages.set(reason, message)
    }
    expect(messages.size).toBe(17)
    // Reasons that name a token differ by the token; the rest differ by wording.
    expect(new Set(messages.values()).size).toBe(17)
  })

  it('names the forbidden character and never repeats the whole value', () => {
    const checked = validateBaseRefSyntax('secret~branch')
    if (checked.ok) {
      throw new Error('expected a violation')
    }
    const message = baseRefViolationMessage(checked)
    expect(message).toContain('~')
    expect(message).not.toContain('secret')
  })
})
