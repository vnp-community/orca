import { describe, expect, it } from 'vitest'
import {
  C4_OVERRIDE_MAX_BYTES,
  lineColumnToOffset,
  offsetToLineColumn,
  validateC4OverrideDocument
} from './c4-override-document'

describe('validateC4OverrideDocument', () => {
  it('accepts empty and comment-only documents', () => {
    expect(validateC4OverrideDocument('').ok).toBe(true)
    expect(validateC4OverrideDocument('# nothing yet\n').ok).toBe(true)
  })

  it('accepts a mapping root', () => {
    const r = validateC4OverrideDocument('components:\n  - id: a\n')
    expect(r.ok).toBe(true)
    expect(r.issues).toEqual([])
  })

  it('reports syntax errors with line and column', () => {
    const r = validateC4OverrideDocument('a: [1, 2\nb: 3\n')
    expect(r.ok).toBe(false)
    expect(r.issues[0].line).toBeGreaterThan(0)
    expect(r.issues[0].column).toBeGreaterThan(0)
  })

  it('rejects duplicate keys', () => {
    const r = validateC4OverrideDocument('a: 1\na: 2\n')
    expect(r.ok).toBe(false)
    expect(r.issues[0].line).toBe(2)
  })

  it('rejects a non-mapping root', () => {
    expect(validateC4OverrideDocument('- a\n- b\n').ok).toBe(false)
    expect(validateC4OverrideDocument('just text').ok).toBe(false)
  })

  it('measures the size limit in UTF-8 bytes, not characters', () => {
    const chars = Math.floor(C4_OVERRIDE_MAX_BYTES / 3) + 1
    const text = `# ${'€'.repeat(chars)}`
    expect(text.length).toBeLessThan(C4_OVERRIDE_MAX_BYTES)
    const r = validateC4OverrideDocument(text)
    expect(r.ok).toBe(false)
    expect(r.bytes).toBeGreaterThan(C4_OVERRIDE_MAX_BYTES)
  })

  it('blocks alias bombs', () => {
    const lines = ['a: &a [x]']
    for (let i = 0; i < 120; i += 1) {
      lines.push(`k${i}: *a`)
    }
    expect(validateC4OverrideDocument(lines.join('\n')).ok).toBe(false)
  })

  it('never throws on arbitrary input', () => {
    for (const input of ['\u0000', '{{{{', '&a *a', '%YAML 9\n---\n', '\t\t- x', '"unterminated']) {
      expect(() => validateC4OverrideDocument(input)).not.toThrow()
    }
    expect(() => validateC4OverrideDocument(undefined as unknown as string)).not.toThrow()
  })
})

describe('line/column <-> offset', () => {
  const text = 'ab\ncde\n\nf'
  it('round-trips positions', () => {
    for (const offset of [0, 1, 3, 5, 7, 8, 9]) {
      const { line, column } = offsetToLineColumn(text, offset)
      expect(lineColumnToOffset(text, line, column)).toBe(offset)
    }
  })
  it('clamps out-of-range positions', () => {
    expect(lineColumnToOffset(text, 99, 1)).toBe(text.length)
    expect(lineColumnToOffset(text, 1, 99)).toBe(2)
  })
})
