import { describe, it, expect } from 'vitest'
import fs from 'fs'
import path from 'path'
import { assertReadOnlyCypher } from '../gitnexus-cypher-guard'
import { CYPHER_TEMPLATES } from '../gitnexus-cypher-templates'
import {
  cypherInt,
  cypherString,
  cypherStringList,
  cypherKindList
} from '../gitnexus-cypher-literal'
import { CodeIntelError } from '../codeintel-errors'

interface InjectionVector {
  name: string
  vector: string
  type: string
}

describe('security-cypher (Task 072-04)', () => {
  const fixturesPath = path.resolve(__dirname, '__fixtures__/cypher-injection-vectors.json')
  const vectors: InjectionVector[] = JSON.parse(fs.readFileSync(fixturesPath, 'utf8'))

  it('All defined CYPHER_TEMPLATES pass assertReadOnlyCypher', () => {
    for (const [id, tmpl] of Object.entries(CYPHER_TEMPLATES)) {
      expect(tmpl.text.trimStart().startsWith('MATCH ')).toBe(true)
      expect(Buffer.byteLength(tmpl.text, 'utf8')).toBeLessThanOrEqual(16384)

      // Test that the base template string passes assertReadOnlyCypher
      // Replacing {{slot}} with dummy valid literal for validation test
      let testQuery = tmpl.text
      for (const [slot, kind] of Object.entries(tmpl.slots)) {
        if (kind === 'int') {
          testQuery = testQuery.replace(new RegExp(`\\{\\{${slot}\\}\\}`, 'g'), '10')
        } else if (kind === 'string') {
          testQuery = testQuery.replace(new RegExp(`\\{\\{${slot}\\}\\}`, 'g'), "'dummy'")
        } else if (kind === 'stringList' || kind === 'kindList') {
          testQuery = testQuery.replace(new RegExp(`\\{\\{${slot}\\}\\}`, 'g'), "['dummy']")
        }
      }

      expect(() => {
        assertReadOnlyCypher(testQuery)
      }).not.toThrow()
    }
  })

  describe('Direct Cypher Injection Vectors via assertReadOnlyCypher', () => {
    it('blocks raw non-read-only or multi-statement injection vectors', () => {
      for (const item of vectors) {
        if (item.type === 'forbidden_keyword' || item.type === 'multi_statement' || item.type === 'comment_injection') {
          expect(() => {
            assertReadOnlyCypher(item.vector)
          }).toThrow(CodeIntelError)
        }
      }
    })
  })

  describe('Literal Slot Escaping and Sanitization', () => {
    it('cypherString rejects NUL bytes and control characters', () => {
      const nulItem = vectors.find(v => v.name === 'nul_byte')!
      const newlineItem = vectors.find(v => v.name === 'newline_injection')!

      expect(() => cypherString(nulItem.vector)).toThrow(CodeIntelError)
      expect(() => cypherString(newlineItem.vector)).toThrow(CodeIntelError)
    })

    it('cypherString safely escapes quote injection vectors', () => {
      const attack1 = "' OR 1=1 //"
      const escaped1 = cypherString(attack1)
      expect(escaped1).toBe("'\\' OR 1=1 //'")

      const attack2 = "'}) MATCH (x) DETACH DELETE x //"
      const escaped2 = cypherString(attack2)
      expect(escaped2).toBe("'\\'}) MATCH (x) DETACH DELETE x //'")

      // Substituting escaped string into template leaves it inside a valid string literal
      const template = CYPHER_TEMPLATES['PR_ONE'].text
      const populated = template.replace('{{id}}', escaped2)

      // When asserted, the query passes read-only check because DETACH DELETE is inside string literal
      expect(() => {
        assertReadOnlyCypher(populated)
      }).not.toThrow()
    })

    it('cypherInt strictly validates numbers and bounds', () => {
      expect(cypherInt(10, 1, 100)).toBe('10')
      expect(() => cypherInt('10' as any, 1, 100)).toThrow(CodeIntelError)
      expect(() => cypherInt(0, 1, 100)).toThrow(CodeIntelError)
      expect(() => cypherInt(101, 1, 100)).toThrow(CodeIntelError)
      expect(() => cypherInt(NaN as any, 1, 100)).toThrow(CodeIntelError)
    })

    it('cypherKindList strictly validates allowed relationship kinds', () => {
      expect(cypherKindList(['CALLS', 'IMPORTS'])).toBe("['CALLS', 'IMPORTS']")
      expect(() => cypherKindList(['CALLS', 'FORBIDDEN_KIND'])).toThrow(CodeIntelError)
      expect(() => cypherKindList([])).toThrow(CodeIntelError)
    })

    it('cypherStringList enforces max bounds', () => {
      const valid = ['a', 'b', 'c']
      expect(cypherStringList(valid)).toBe("['a', 'b', 'c']")
      expect(() => cypherStringList([])).toThrow(CodeIntelError)
      const hugeList = Array.from({ length: 301 }, (_, i) => `item_${i}`)
      expect(() => cypherStringList(hugeList)).toThrow(CodeIntelError)
    })
  })
})
