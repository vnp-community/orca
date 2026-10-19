import { describe, expect, it } from 'vitest'
import {
  MAX_BASE_REF_BYTES,
  parseRequestCreateResult,
  parseSetBaseRefResult,
  validateBaseRefSyntax,
  verifyBaseRefEcho,
  type BaseRefViolation
} from './request-base-ref'

// The cases below are the table of TestParseWorktreeBaseRef_Valid / _Invalid in
// backend-go/services/request-service/internal/domain/worktree_base_ref_test.go (47 cases). Add a case there and here together.
// "invalid utf-8" is a lone surrogate here: a JS string cannot hold the raw 0xff byte of the Go case.
const VALID_CASES: readonly { name: string; input: string; want: string }[] = [
  { name: 'empty is the default', input: '', want: '' },
  { name: 'simple branch', input: 'main', want: 'main' },
  { name: 'release branch', input: 'release/1.4', want: 'release/1.4' },
  { name: 'remote branch', input: 'origin/release/1.4', want: 'origin/release/1.4' },
  { name: 'tag', input: 'v1.4.0', want: 'v1.4.0' },
  { name: 'full ref name', input: 'refs/heads/main', want: 'refs/heads/main' },
  { name: 'edge whitespace is trimmed', input: '  main\t\n', want: 'main' },
  { name: 'dots inside a component', input: 'feature/a.b.c', want: 'feature/a.b.c' },
  { name: 'underscore and dash inside', input: 'fix/some_bug-12', want: 'fix/some_bug-12' },
  { name: 'at sign without brace', input: 'user@host', want: 'user@host' },
  { name: 'non-ascii letters', input: 'tính-năng/mới', want: 'tính-năng/mới' },
  { name: 'exactly 255 bytes', input: 'a'.repeat(255), want: 'a'.repeat(255) },
  { name: 'lock inside a component name', input: 'a.lockfile/b', want: 'a.lockfile/b' }
]

// `reason` is the rule the UI reports; the Go test only asserts REQUEST_BASE_REF_INVALID, so the reason mapping is pinned here.
const INVALID_CASES: readonly { name: string; input: string; reason: BaseRefViolation }[] = [
  { name: 'blank', input: '   ', reason: 'blank' },
  { name: 'blank tab and newline', input: '\t\n ', reason: 'blank' },
  { name: 'leading dash', input: '-x', reason: 'leading_dash' },
  { name: 'leading double dash', input: '--upload-pack=evil', reason: 'leading_dash' },
  { name: 'dot dot', input: 'a..b', reason: 'double_dot' },
  { name: 'dot dot at start', input: '..main', reason: 'double_dot' },
  { name: 'reflog syntax', input: 'main@{1}', reason: 'reflog_syntax' },
  { name: 'bare at sign', input: '@', reason: 'bare_at' },
  { name: 'backslash', input: 'a\\b', reason: 'forbidden_character' },
  { name: 'NUL byte', input: 'a\x00b', reason: 'nul_byte' },
  { name: 'newline', input: 'a\nb', reason: 'control_character' },
  { name: 'carriage return', input: 'a\rb', reason: 'control_character' },
  { name: 'tab inside', input: 'a\tb', reason: 'control_character' },
  { name: 'DEL', input: 'a\x7fb', reason: 'control_character' },
  { name: 'C1 control', input: 'a\u0085b', reason: 'control_character' },
  { name: 'space inside', input: 'my branch', reason: 'whitespace' },
  { name: 'non-breaking space inside', input: 'a b', reason: 'whitespace' },
  { name: 'tilde', input: 'main~1', reason: 'forbidden_character' },
  { name: 'caret', input: 'main^', reason: 'forbidden_character' },
  { name: 'colon', input: 'origin:main', reason: 'forbidden_character' },
  { name: 'question mark', input: 'release?', reason: 'forbidden_character' },
  { name: 'asterisk', input: 'release/*', reason: 'forbidden_character' },
  { name: 'open bracket', input: 'a[0]', reason: 'forbidden_character' },
  { name: 'trailing slash', input: 'release/', reason: 'trailing_slash' },
  { name: 'leading slash', input: '/release', reason: 'leading_slash' },
  { name: 'double slash', input: 'release//1.4', reason: 'double_slash' },
  { name: 'trailing dot', input: 'release.', reason: 'trailing_dot' },
  { name: 'trailing lock', input: 'main.lock', reason: 'lock_component' },
  { name: 'component ending in lock', input: 'a.lock/b', reason: 'lock_component' },
  { name: 'component starting with dot', input: 'a/.hidden', reason: 'dot_component' },
  { name: 'leading dot', input: '.hidden', reason: 'dot_component' },
  { name: 'invalid utf-8', input: 'a\ud800b', reason: 'invalid_utf8' },
  { name: '256 bytes', input: 'a'.repeat(256), reason: 'too_long' },
  { name: '256 bytes of multibyte', input: 'é'.repeat(128), reason: 'too_long' }
]

describe('validateBaseRefSyntax (cases shared with the backend)', () => {
  it('keeps the case count of the Go table', () => {
    expect(VALID_CASES.length + INVALID_CASES.length).toBe(47)
    expect(MAX_BASE_REF_BYTES).toBe(255)
  })

  it.each(VALID_CASES)('accepts: $name', ({ input, want }) => {
    expect(validateBaseRefSyntax(input)).toEqual({ ok: true, value: want })
  })

  it.each(INVALID_CASES)('rejects: $name', ({ input, reason }) => {
    const result = validateBaseRefSyntax(input)
    expect(result.ok).toBe(false)
    expect(result).toMatchObject({ ok: false, reason })
  })

  it('names the offending character for a forbidden one, and never echoes the value otherwise', () => {
    expect(validateBaseRefSyntax('a:b')).toEqual({
      ok: false,
      reason: 'forbidden_character',
      character: ':'
    })
    expect(JSON.stringify(validateBaseRefSyntax('secret-token:abc'))).not.toContain('secret-token')
  })
})

describe('validateBaseRefSyntax (edges where JS and Go disagree)', () => {
  it('trims only what Go trims: U+0085, U+00A0 and U+3000 at the edge go, U+FEFF stays', () => {
    expect(validateBaseRefSyntax('\u0085 main　')).toEqual({ ok: true, value: 'main' })
    // U+FEFF is neither space nor control for Go (JS trim() would drop it), so it is part of the ref and git decides later.
    expect(validateBaseRefSyntax('﻿main')).toEqual({ ok: true, value: '﻿main' })
  })

  it('counts UTF-8 bytes after trimming', () => {
    expect(validateBaseRefSyntax(` ${'a'.repeat(255)} `)).toEqual({
      ok: true,
      value: 'a'.repeat(255)
    })
    expect(validateBaseRefSyntax(`${'é'.repeat(127)}a`)).toEqual({
      ok: true,
      value: `${'é'.repeat(127)}a`
    })
  })

  it('treats an astral character as one valid character, not two lone surrogates', () => {
    expect(validateBaseRefSyntax('feature/😀')).toEqual({ ok: true, value: 'feature/😀' })
  })

  it('reports the first broken rule in the backend order', () => {
    // leading dash wins over a later forbidden character; length wins over everything after it.
    expect(validateBaseRefSyntax('-a:b')).toMatchObject({ reason: 'leading_dash' })
    expect(validateBaseRefSyntax(`${'a'.repeat(256)}:`)).toMatchObject({ reason: 'too_long' })
    // a forbidden character is found before a sequence rule.
    expect(validateBaseRefSyntax('a:..b')).toMatchObject({ reason: 'forbidden_character' })
  })
})

describe('verifyBaseRefEcho', () => {
  it('has nothing to verify when no branch was requested', () => {
    expect(verifyBaseRefEcho(undefined, {})).toBe('not_requested')
    expect(verifyBaseRefEcho('', { baseRef: 'main' })).toBe('not_requested')
    expect(verifyBaseRefEcho('   ', {})).toBe('not_requested')
  })

  it('matches the stored ref, ignoring the edge whitespace the server trims', () => {
    expect(verifyBaseRefEcho('release/1.4', { baseRef: 'release/1.4' })).toBe('match')
    expect(verifyBaseRefEcho('  release/1.4 ', { baseRef: 'release/1.4' })).toBe('match')
  })

  it('reports a mismatch for a different ref, an empty ref and a gateway that dropped the field', () => {
    expect(verifyBaseRefEcho('release/1.4', { baseRef: 'main' })).toBe('mismatch')
    expect(verifyBaseRefEcho('release/1.4', { baseRef: '' })).toBe('mismatch')
    expect(verifyBaseRefEcho('release/1.4', { baseRef: undefined })).toBe('mismatch')
  })
})

describe('write result parsers', () => {
  const request = { id: 'r1', baseRef: 'release/1.4', baseRefLocked: false, version: 3 }

  it('reads created and the unverified flag of request.create', () => {
    const result = parseRequestCreateResult({ request, created: true, baseRefUnverified: true })
    expect(result.request.baseRef).toBe('release/1.4')
    expect(result.created).toBe(true)
    expect(result.baseRefUnverified).toBe(true)
  })

  it('defaults to verified and leaves created undefined when the gateway does not say', () => {
    const result = parseRequestCreateResult({ request })
    expect(result.created).toBeUndefined()
    expect(result.baseRefUnverified).toBe(false)
  })

  it('accepts an unverified flag carried by the request itself', () => {
    expect(
      parseSetBaseRefResult({ request: { ...request, baseRefUnverified: true } }).baseRefUnverified
    ).toBe(true)
  })

  it('never throws on junk', () => {
    for (const junk of [null, undefined, 7, 'x', [], { request: 5 }]) {
      expect(() => parseSetBaseRefResult(junk)).not.toThrow()
      expect(() => parseRequestCreateResult(junk)).not.toThrow()
    }
    expect(parseSetBaseRefResult(null).request.baseRef).toBeUndefined()
  })
})
