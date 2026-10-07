import { describe, it, expect } from 'vitest'
import { assertReadOnlyCypher } from './gitnexus-cypher-guard'
import { CodeIntelError } from './codeintel-errors'

describe('gitnexus-cypher-guard', () => {
  it('accepts valid read-only query', () => {
    expect(assertReadOnlyCypher('MATCH (n) RETURN n')).toBe('MATCH (n) RETURN n')
  })

  it('rejects query not starting with MATCH', () => {
    expect(() => assertReadOnlyCypher('RETURN 1')).toThrow(CodeIntelError)
  })

  it('rejects multiple statements', () => {
    expect(() => assertReadOnlyCypher('MATCH (n); MATCH (m)')).toThrow(CodeIntelError)
  })

  it('rejects forbidden keywords', () => {
    expect(() => assertReadOnlyCypher('MATCH (n) DELETE n')).toThrow(/Forbidden Cypher keyword: DELETE/i)
    expect(() => assertReadOnlyCypher('MATCH (n) call proc()')).toThrow(/Forbidden Cypher keyword: call/i)
  })

  it('accepts forbidden keywords inside string literals', () => {
    expect(assertReadOnlyCypher("MATCH (n) WHERE n.name = 'DELETE' RETURN n")).toBe("MATCH (n) WHERE n.name = 'DELETE' RETURN n")
    expect(assertReadOnlyCypher("MATCH (n) WHERE n.name = 'call' RETURN n")).toBe("MATCH (n) WHERE n.name = 'call' RETURN n")
  })

  it('handles escaped quotes in strings correctly', () => {
    expect(assertReadOnlyCypher("MATCH (n) WHERE n.name = '\\'DELETE' RETURN n")).toBe("MATCH (n) WHERE n.name = '\\'DELETE' RETURN n")
    expect(() => assertReadOnlyCypher("MATCH (n) WHERE n.name = '\\\\' DELETE n")).toThrow(/Forbidden Cypher keyword: DELETE/i)
  })

  it('rejects unclosed strings', () => {
    expect(() => assertReadOnlyCypher("MATCH (n) WHERE n.name = 'abc")).toThrow(/Unclosed string literal/i)
  })

  it('accepts valid identifiers that contain forbidden words as substrings', () => {
    expect(assertReadOnlyCypher('MATCH (n) RETURN n.deleteFile')).toBe('MATCH (n) RETURN n.deleteFile')
  })
})
