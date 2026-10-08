import { describe, expect, it } from 'vitest'
import type { QualityFinding } from '../../../../../shared/code-intel-quality-types'
import { buildQualityMarkers } from './quality-marker-model'

const SEVERITY = { error: 8, warning: 4, info: 2, hint: 1 }

function finding(patch: Partial<QualityFinding> = {}): QualityFinding {
  return {
    fingerprint: 'fp-1',
    fpVersion: 1,
    ruleId: 'no-unused-vars',
    severity: 'error',
    category: 'lint',
    file: 'src/a.ts',
    line: 3,
    endLine: 3,
    column: 5,
    endColumn: 9,
    message: 'x is unused',
    tool: 'oxlint',
    toolVersion: '1.0.0',
    stepId: 's1',
    inScope: true,
    ...patch
  }
}

const opts = { lineCount: 10, lineMaxColumn: () => 40, severityMap: SEVERITY }

describe('buildQualityMarkers', () => {
  it('maps severities, source and code, and uses the column range when valid', () => {
    const { markers } = buildQualityMarkers(
      [
        finding(),
        finding({ fingerprint: 'w', severity: 'warning', line: 4, endLine: 4 }),
        finding({ fingerprint: 'i', severity: 'info', line: 5, endLine: 5 }),
        finding({ fingerprint: 'u', severity: 'unknown', line: 6, endLine: 6 })
      ],
      opts
    )
    expect(markers.map((m) => m.severity)).toEqual([8, 4, 2, 1])
    expect(markers[0]).toMatchObject({
      startLineNumber: 3,
      startColumn: 5,
      endLineNumber: 3,
      endColumn: 9,
      source: 'oxlint',
      code: 'no-unused-vars'
    })
  })

  it('appends fixHint to the message', () => {
    const { markers } = buildQualityMarkers([finding({ fixHint: 'remove x' })], opts)
    expect(markers[0].message).toBe('x is unused\nremove x')
  })

  it('clamps lines into [1, lineCount] and fixes endLine < line', () => {
    const { markers } = buildQualityMarkers(
      [
        finding({ line: 99, endLine: 120, column: 0, endColumn: 0 }),
        finding({ line: 5, endLine: 2, column: 0, endColumn: 0 })
      ],
      opts
    )
    expect(markers[0]).toMatchObject({ startLineNumber: 10, endLineNumber: 10 })
    expect(markers[1]).toMatchObject({ startLineNumber: 5, endLineNumber: 5 })
  })

  it('marks the whole line when the column is absent or out of range', () => {
    const { markers } = buildQualityMarkers(
      [
        finding({ column: 0, endColumn: 0 }),
        finding({ column: 500, endColumn: 600 }),
        finding({ column: 9, endColumn: 5 })
      ],
      opts
    )
    for (const m of markers) {
      expect(m).toMatchObject({ startColumn: 1, endColumn: 40 })
    }
  })

  it('widens a point column to one character', () => {
    const { markers } = buildQualityMarkers([finding({ column: 7, endColumn: 7 })], opts)
    expect(markers[0]).toMatchObject({ startColumn: 7, endColumn: 8 })
  })

  it('places file-level findings (line <= 0) on line 1', () => {
    const { markers, glyphs } = buildQualityMarkers(
      [finding({ line: 0, endLine: 0, column: 3, endColumn: 4 })],
      opts
    )
    expect(markers[0]).toMatchObject({
      startLineNumber: 1,
      endLineNumber: 1,
      startColumn: 1,
      endColumn: 40
    })
    expect(glyphs[0].line).toBe(1)
  })

  it('merges findings of one line into the highest-severity glyph with a count', () => {
    const { glyphs } = buildQualityMarkers(
      [
        finding({ fingerprint: 'a', severity: 'info', line: 2, endLine: 2 }),
        finding({ fingerprint: 'b', severity: 'error', line: 2, endLine: 2 }),
        finding({ fingerprint: 'c', severity: 'warning', line: 2, endLine: 2 }),
        finding({ fingerprint: 'd', severity: 'warning', line: 2, endLine: 2 })
      ],
      opts
    )
    expect(glyphs).toHaveLength(1)
    expect(glyphs[0]).toMatchObject({ line: 2, severity: 'error', fingerprint: 'b', count: 4 })
    expect(glyphs[0].summaries).toHaveLength(3)
  })

  it('maps unknown severity to the info glyph', () => {
    const { glyphs } = buildQualityMarkers([finding({ severity: 'unknown' })], opts)
    expect(glyphs[0].severity).toBe('info')
  })

  it('returns nothing for an empty model and survives odd data', () => {
    expect(buildQualityMarkers([finding()], { ...opts, lineCount: 0 })).toEqual({
      markers: [],
      glyphs: []
    })
    const odd = finding({
      line: Number.NaN,
      endLine: Number.POSITIVE_INFINITY,
      column: Number.NaN,
      message: undefined as unknown as string,
      fixHint: 42 as unknown as string
    })
    expect(() => buildQualityMarkers([odd], opts)).not.toThrow()
  })

  it('handles 1000 findings quickly', () => {
    const many = Array.from({ length: 1000 }, (_, i) =>
      finding({ fingerprint: `f${i}`, line: (i % 10) + 1, endLine: (i % 10) + 1 })
    )
    const start = performance.now()
    const { markers, glyphs } = buildQualityMarkers(many, opts)
    expect(markers).toHaveLength(1000)
    expect(glyphs).toHaveLength(10)
    expect(performance.now() - start).toBeLessThan(1000)
  })
})
