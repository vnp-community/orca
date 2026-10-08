import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import {
  computeOverlayFlags,
  OVERLAY_ENCODING,
  OVERLAY_FLAG_ORDER,
  overlayClassNames,
  overlayFlagsWithData,
  overlaySvgProps
} from './review-overlay-model'
import type { ChangeOverlayView } from './review-wire-types'

const ref = (key: string) => ({ key, kind: 'function', name: key, filePath: `src/${key}.ts` })

const overlay = {
  changedFiles: [{ path: 'src/a.ts', status: 'modified' as const }],
  changedSymbols: [
    { symbol: ref('a'), changeKind: 'modified', tested: 'no' },
    { symbol: ref('b'), changeKind: 'modified', tested: 'unknown' },
    { symbol: ref('c'), changeKind: 'modified', tested: 'yes' }
  ],
  uncoveredSymbols: [ref('d')],
  violations: [
    { findingKey: 'f', rule: 'r', severity: 'warn', file: 'src/a.ts', status: 'touched' }
  ]
} as unknown as ChangeOverlayView

describe('computeOverlayFlags', () => {
  it('changed + untested + violation for a changed untested symbol in a flagged file', () => {
    const f = computeOverlayFlags({ symbolKey: 'a', file: 'src/a.ts' }, overlay)
    expect([...f].sort()).toEqual(['changed', 'untested', 'violation'])
  })
  it('tested unknown never becomes untested', () => {
    expect(computeOverlayFlags({ symbolKey: 'b' }, overlay).has('untested')).toBe(false)
    expect(computeOverlayFlags({ symbolKey: 'c' }, overlay).has('untested')).toBe(false)
  })
  it('uncoveredSymbols marks untested', () => {
    expect(computeOverlayFlags({ symbolKey: 'd' }, overlay).has('untested')).toBe(true)
  })
  it('affected only when impact data names a non-changed symbol', () => {
    const impact = { affectedKeys: new Set(['x', 'a']) }
    expect(computeOverlayFlags({ symbolKey: 'x' }, overlay, impact).has('affected')).toBe(true)
    expect(computeOverlayFlags({ symbolKey: 'x' }, overlay).has('affected')).toBe(false)
    const a = computeOverlayFlags({ symbolKey: 'a' }, overlay, impact)
    expect(a.has('affected')).toBe(false)
    expect(a.has('changed')).toBe(true)
  })
  it('file-only target uses changedFiles and violations', () => {
    expect([...computeOverlayFlags({ file: 'src/a.ts' }, overlay)].sort()).toEqual([
      'changed',
      'violation'
    ])
  })
  it('null overlay gives no flags', () => {
    expect(computeOverlayFlags({ symbolKey: 'a' }, null).size).toBe(0)
  })
})

describe('overlay encoding output', () => {
  it('changed beats affected; untested only adds dashes', () => {
    const f = new Set(['changed', 'affected', 'untested'] as const)
    expect(overlayClassNames(f)).toContain('--review-changed')
    expect(overlayClassNames(f)).toContain('border-2')
    expect(overlayClassNames(f)).toContain('border-dashed')
    expect(overlaySvgProps(f)).toEqual({
      stroke: 'var(--review-changed)',
      strokeWidth: 2,
      strokeDasharray: '4 3'
    })
  })
  it('no flags => nothing; unknown tested => no dash', () => {
    expect(overlayClassNames(new Set())).toBe('')
    expect(overlaySvgProps(new Set())).toBeNull()
    expect(overlayClassNames(new Set(['changed'] as const))).not.toContain('dashed')
  })
  it('contains no hex colour', () => {
    const text = JSON.stringify(
      OVERLAY_FLAG_ORDER.map((f) => [
        OVERLAY_ENCODING[f].tokenVar,
        OVERLAY_ENCODING[f].textClass,
        OVERLAY_ENCODING[f].borderClass,
        overlaySvgProps(new Set([f])),
        overlayClassNames(new Set([f]))
      ])
    )
    expect(text).not.toMatch(/#[0-9a-f]{3,8}\b/i)
    expect(readFileSync(new URL('./review-overlay-model.ts', import.meta.url), 'utf8')).not.toMatch(
      /['"`]#[0-9a-f]{3,8}['"`]/i
    )
  })
  it('legend flags come only from present data', () => {
    expect(overlayFlagsWithData(overlay, { hasImpact: false })).toEqual([
      'changed',
      'untested',
      'violation'
    ])
    expect(overlayFlagsWithData(overlay, { hasImpact: true })).toContain('affected')
    expect(overlayFlagsWithData(null, { hasImpact: true })).toEqual([])
  })
})
