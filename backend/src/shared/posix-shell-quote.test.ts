import { describe, expect, it } from 'vitest'
import { posixShellQuote, buildPosixShellCommand } from './posix-shell-quote'

describe('posixShellQuote', () => {
  it('wraps a plain value in single quotes', () => {
    expect(posixShellQuote('gh')).toBe("'gh'")
  })

  it('escapes embedded single quotes', () => {
    expect(posixShellQuote("it's")).toBe("'it'\\''s'")
  })

  it('neutralizes shell metacharacters', () => {
    const evil = '; rm -rf / #'
    const quoted = posixShellQuote(evil)
    // The whole payload must be inert once single-quoted — no unescaped `'`
    // boundary for a shell to break out of.
    expect(quoted).toBe("'; rm -rf / #'")
  })

  it('neutralizes a command-substitution attempt', () => {
    const evil = '$(rm -rf /)'
    expect(posixShellQuote(evil)).toBe("'$(rm -rf /)'")
  })

  it('handles an empty string', () => {
    expect(posixShellQuote('')).toBe("''")
  })
})

describe('buildPosixShellCommand', () => {
  it('joins quoted tokens with a single space', () => {
    expect(buildPosixShellCommand(['gh', 'auth', 'login'])).toBe("'gh' 'auth' 'login'")
  })

  it('quotes a caller-influenced token the same as a literal one', () => {
    expect(buildPosixShellCommand(['gh', 'auth', 'login', '--hostname', "evil'; rm -rf /"]))
      .toBe("'gh' 'auth' 'login' '--hostname' 'evil'\\''; rm -rf /'")
  })
})
