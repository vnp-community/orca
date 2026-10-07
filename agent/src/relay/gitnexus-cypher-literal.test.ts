import { describe, it, expect } from 'vitest'
import {
  cypherInt,
  cypherString,
  cypherStringList,
  cypherKindList,
  GITNEXUS_EDGE_KINDS
} from './gitnexus-cypher-literal'
import { CodeIntelError } from './codeintel-errors'

describe('gitnexus-cypher-literal', () => {
  describe('cypherInt', () => {
    it('returns string representation of integer', () => {
      expect(cypherInt(5, 0, 10)).toBe('5')
    })
    it('throws if out of range', () => {
      expect(() => cypherInt(-1, 0, 10)).toThrow(CodeIntelError)
      expect(() => cypherInt(11, 0, 10)).toThrow(CodeIntelError)
    })
    it('throws if not integer', () => {
      expect(() => cypherInt(1.5, 0, 10)).toThrow(CodeIntelError)
      expect(() => cypherInt('5', 0, 10)).toThrow(CodeIntelError)
    })
  })

  describe('cypherString', () => {
    it('escapes slashes and single quotes', () => {
      expect(cypherString("it's a \\test")).toBe("'it\\'s a \\\\test'")
    })
    it('throws on control characters', () => {
      expect(() => cypherString("hello\nworld")).toThrow(CodeIntelError)
      expect(() => cypherString("hello\0world")).toThrow(CodeIntelError)
    })
    it('throws on length out of bounds', () => {
      expect(() => cypherString("")).toThrow(CodeIntelError)
      expect(() => cypherString("a".repeat(513))).toThrow(CodeIntelError)
    })
  })

  describe('cypherStringList', () => {
    it('formats an array of strings', () => {
      expect(cypherStringList(['a', "b'c"])).toBe("['a', 'b\\'c']")
    })
    it('throws on empty or too large', () => {
      expect(() => cypherStringList([])).toThrow(CodeIntelError)
      expect(() => cypherStringList(Array.from({ length: 301 }, () => 'a'))).toThrow(CodeIntelError)
    })
  })

  describe('cypherKindList', () => {
    it('formats allowed kinds', () => {
      expect(cypherKindList(['CALLS', 'IMPORTS'])).toBe("['CALLS', 'IMPORTS']")
    })
    it('throws on disallowed kinds', () => {
      expect(() => cypherKindList(['UNKNOWN'])).toThrow(CodeIntelError)
    })
  })
})
