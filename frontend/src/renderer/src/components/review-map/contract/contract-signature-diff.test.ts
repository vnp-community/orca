import { describe, expect, it } from 'vitest'
import { diffSignatureTokens } from './contract-signature-diff'

const texts = (tokens: { text: string; state: string }[], state: string): string[] =>
  tokens.filter((t) => t.state === state).map((t) => t.text)

describe('diffSignatureTokens', () => {
  it('marks removed and added tokens on a changed signature', () => {
    const d = diffSignatureTokens('int32 timeout_ms = 3;', 'int64 timeout_ms = 3;')
    expect(texts(d.before, 'removed')).toEqual(['int32'])
    expect(texts(d.after, 'added')).toEqual(['int64'])
    expect(texts(d.before, 'same')).toEqual(['timeout_ms', '=', '3', ';'])
  })

  it('treats a missing side as fully added or removed', () => {
    const removed = diffSignatureTokens('a b', undefined)
    expect(removed.after).toEqual([])
    expect(removed.before.every((t) => t.state === 'removed')).toBe(true)
    const added = diffSignatureTokens('', 'x')
    expect(added.after).toEqual([{ text: 'x', state: 'added' }])
  })

  it('ignores whitespace-only differences', () => {
    const d = diffSignatureTokens('foo( a )', 'foo(   a)')
    expect(d.before.every((t) => t.state === 'same')).toBe(true)
    expect(d.after.every((t) => t.state === 'same')).toBe(true)
  })

  it('returns empty arrays for two empty inputs', () => {
    expect(diffSignatureTokens(undefined, '')).toEqual({ before: [], after: [] })
  })
})
