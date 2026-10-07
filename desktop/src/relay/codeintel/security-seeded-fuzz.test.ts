import { describe, it, expect } from 'vitest'
import { SeededRandom } from './seeded-random'
import {
  assertSafeClientString,
  assertRelativeRepoPath,
  validateCodeIntelParams
} from '../codeintel-params-validation'
import { cypherString, cypherInt } from '../gitnexus-cypher-literal'
import { assertReadOnlyCypher } from '../gitnexus-cypher-guard'
import { CodeIntelError } from '../codeintel-errors'

describe('security-seeded-fuzz (Task 072-08)', () => {
  const seeds = [42, 1337, 2026]
  const iterations = process.env.ORCA_FUZZ_ITERATIONS ? parseInt(process.env.ORCA_FUZZ_ITERATIONS, 10) : 100

  for (const seed of seeds) {
    describe(`Seeded Fuzz with seed=${seed} (${iterations} iterations)`, () => {
      it('assertSafeClientString: either throws CodeIntelError(CODEINTEL_INVALID_PARAMS) or satisfies safe grammar', () => {
        const rng = new SeededRandom(seed)

        for (let i = 0; i < iterations; i++) {
          const input = rng.nextString(64)
          try {
            assertSafeClientString(input, 'fuzzField')

            // If it didn't throw, assert that the invariants hold:
            expect(input.length).toBeGreaterThanOrEqual(1)
            expect(input.length).toBeLessThanOrEqual(512)
            expect(/[\x00-\x1F\x7F]/.test(input)).toBe(false)
            expect(/^\s/.test(input)).toBe(false)
            const n = input.normalize('NFKC')
            expect(n.startsWith('-')).toBe(false)
            expect(n.startsWith('\uFF0D')).toBe(false)
            expect(n.startsWith('\u2212')).toBe(false)
          } catch (err: any) {
            expect(err).toBeInstanceOf(CodeIntelError)
            expect(err.code).toBe('CODEINTEL_INVALID_PARAMS')
          }
        }
      })

      it('assertRelativeRepoPath: either throws CodeIntelError or satisfies relative path invariants', () => {
        const rng = new SeededRandom(seed + 1)

        for (let i = 0; i < iterations; i++) {
          const input = rng.nextString(64)
          try {
            assertRelativeRepoPath(input, 'fuzzPath')

            // If it didn't throw:
            expect(input.startsWith('/')).toBe(false)
            expect(input.startsWith('\\')).toBe(false)
            expect(input.includes('\\')).toBe(false)
            expect(input.includes('..')).toBe(false)
            expect(/[\x00-\x1F\x7F]/.test(input)).toBe(false)
          } catch (err: any) {
            expect(err).toBeInstanceOf(CodeIntelError)
            expect(err.code).toBe('CODEINTEL_INVALID_PARAMS')
          }
        }
      })

      it('cypherString: properly escapes strings or rejects with CodeIntelError', () => {
        const rng = new SeededRandom(seed + 2)

        for (let i = 0; i < iterations; i++) {
          const input = rng.nextString(64)
          try {
            const escaped = cypherString(input)

            // Must start and end with single quote
            expect(escaped.startsWith("'")).toBe(true)
            expect(escaped.endsWith("'")).toBe(true)

            // When parsed in Cypher read-only check as part of a query:
            const testQuery = `MATCH (n) WHERE n.name = ${escaped} RETURN n`
            expect(() => assertReadOnlyCypher(testQuery)).not.toThrow()
          } catch (err: any) {
            expect(err).toBeInstanceOf(CodeIntelError)
            expect(err.code).toBe('CODEINTEL_INVALID_PARAMS')
          }
        }
      })
    })
  }
})
